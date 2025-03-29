package ws

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	maxRetryInterval = 30 * time.Second
	initialRetry     = 100 * time.Millisecond
)

// Client represents a WebSocket client connection with enhanced error handling
type Client struct {
	conn        *websocket.Conn
	send        chan []byte
	done        chan struct{}
	userID      string
	mu          sync.Mutex
	closed      int32
	ctx         context.Context
	cancel      context.CancelFunc
	maxRetries  int
	retryBucket *TokenBucket
}

// NewClient creates a new WebSocket client with improved error handling
func NewClient(conn *websocket.Conn, userID string, bufferSize int) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	client := &Client{
		conn:        conn,
		send:        make(chan []byte, bufferSize),
		done:        make(chan struct{}),
		userID:      userID,
		ctx:         ctx,
		cancel:      cancel,
		closed:      0,
		maxRetries:  5,
		retryBucket: NewTokenBucket(3, 10*time.Second), // 3 reconnects per 10 seconds max
	}

	// Start read/write pumps
	go client.readPump()
	go client.writePump()

	return client
}

// TokenBucket implements a simple rate limiter
type TokenBucket struct {
	tokens    int
	capacity  int
	refillAt  time.Time
	refillDur time.Duration
	mu        sync.Mutex
}

// NewTokenBucket creates a new token bucket rate limiter
func NewTokenBucket(capacity int, refillDuration time.Duration) *TokenBucket {
	return &TokenBucket{
		tokens:    capacity,
		capacity:  capacity,
		refillAt:  time.Now().Add(refillDuration),
		refillDur: refillDuration,
		mu:        sync.Mutex{},
	}
}

// Take attempts to take a token from the bucket
func (tb *TokenBucket) Take() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	// Refill if needed
	now := time.Now()
	if now.After(tb.refillAt) {
		tb.tokens = tb.capacity
		tb.refillAt = now.Add(tb.refillDur)
	}

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}
	return false
}

// readPump pumps messages from the WebSocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.Close()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			_, _, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err) {
					zap.L().Info("WebSocket connection closed",
						zap.String("userID", c.userID),
						zap.Error(err))
				}
				return
			}

			// Process received message...
			// Implementation depends on your application logic
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	retryCount := 0
	for {
		select {
		case <-c.ctx.Done():
			return

		case <-ticker.C:
			// Send ping for keepalive
			if err := c.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
				zap.L().Warn("Failed to write ping message",
					zap.String("userID", c.userID),
					zap.Error(err))

				if c.shouldReconnect(err) {
					if c.attemptReconnect(retryCount) {
						retryCount++
						continue
					}
				}
				return
			}

		case message, ok := <-c.send:
			if !ok {
				// Channel closed, terminate connection properly
				if err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
					zap.L().Debug("Error sending close message",
						zap.String("userID", c.userID),
						zap.Error(err))
				}
				return
			}

			// Write message with retry logic
			err := c.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				zap.L().Warn("Failed to write message",
					zap.String("userID", c.userID),
					zap.Int("messageSize", len(message)),
					zap.Error(err))

				if c.shouldReconnect(err) {
					if c.attemptReconnect(retryCount) {
						// Try sending the message again
						if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
							retryCount++
						} else {
							retryCount = 0 // Reset on successful write
						}
						continue
					}
				}
				return
			}
			retryCount = 0 // Reset on successful write
		}
	}
}

// shouldReconnect determines if a connection error is retryable
func (c *Client) shouldReconnect(err error) bool {
	// Don't reconnect if context is already canceled
	select {
	case <-c.ctx.Done():
		return false
	default:
		// Continue evaluating error
	}

	// Check if closed normally
	if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		return false
	}

	// Check common network errors
	if websocket.IsUnexpectedCloseError(err) {
		return true
	}

	// Check if connection is already closed
	if atomic.LoadInt32(&c.closed) == 1 {
		return false
	}

	return true
}

// attemptReconnect tries to re-establish the connection with exponential backoff
func (c *Client) attemptReconnect(retryCount int) bool {
	// Check rate limit before attempting reconnect
	if !c.retryBucket.Take() {
		zap.L().Warn("Reconnect rate limit exceeded",
			zap.String("userID", c.userID))
		return false
	}

	// Calculate backoff with jitter
	retryInterval := initialRetry * time.Duration(math.Pow(2, float64(retryCount)))
	if retryInterval > maxRetryInterval {
		retryInterval = maxRetryInterval
	}

	// Add jitter (±20%)
	jitter := float64(retryInterval) * (0.8 + 0.4*rand.Float64())
	retryInterval = time.Duration(jitter)

	zap.L().Info("Attempting WebSocket reconnect",
		zap.String("userID", c.userID),
		zap.Duration("backoff", retryInterval),
		zap.Int("attempt", retryCount+1))

	time.Sleep(retryInterval)

	// Maximum retries exceeded
	if retryCount >= c.maxRetries {
		zap.L().Error("Max reconnect attempts reached",
			zap.String("userID", c.userID),
			zap.Int("maxRetries", c.maxRetries))
		return false
	}

	// Implement actual reconnection logic
	// This would typically involve creating a new connection and
	// updating c.conn with the new connection
	// ...

	return true
}

// SendMessage sends a message through the WebSocket connection
func (c *Client) SendMessage(message []byte) error {
	if atomic.LoadInt32(&c.closed) == 1 {
		return fmt.Errorf("connection closed")
	}

	select {
	case c.send <- message:
		return nil
	case <-c.done:
		return fmt.Errorf("connection closed")
	default:
		// Channel buffer is full - could implement dropping policy
		// or increase buffer size
		return fmt.Errorf("send buffer full")
	}
}

// Close closes the WebSocket connection
func (c *Client) Close() error {
	// Only close once
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil
	}

	c.cancel() // Cancel context

	c.mu.Lock()
	defer c.mu.Unlock()

	// Close send channel
	close(c.done)
	close(c.send)

	// Close connection
	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}
