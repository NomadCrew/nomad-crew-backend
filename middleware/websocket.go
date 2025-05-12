package middleware

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/internal/ws"
	"github.com/gin-gonic/gin"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/NomadCrew/nomad-crew-backend/logger"
)

// Context keys
const (
	WebSocketConnectionKey = "wsConnection"
	UserProfileKey         = "userProfile"
)

var (
	ErrBackpressure = fmt.Errorf("client cannot keep up with message rate")
)

var activeConnectionCount int64

type WSConfig struct {
	AllowedOrigins   []string
	CheckOrigin      func(r *http.Request) bool
	WriteBufferSize  int           // Default 1024
	ReadBufferSize   int           // Default 1024
	MaxMessageSize   int64         // Default 512KB
	WriteWait        time.Duration // Time allowed to write a message
	PongWait         time.Duration // Time allowed to read the next pong message
	PingPeriod       time.Duration // Send pings to peer with this period
	ReauthInterval   time.Duration // JWT revalidation interval
	BufferHighWater  int           // Backpressure threshold
	BufferLowWater   int           // Backpressure release threshold
	ReconnectBackoff time.Duration // For client reconnect attempts
}

type WSMetrics struct {
	connectionCount   prometheus.Gauge
	messageSize       prometheus.Histogram
	messageLatency    prometheus.Histogram
	errorCount        prometheus.Counter
	bufferUtilization prometheus.Gauge
	pingPongLatency   prometheus.Histogram
	ConnectionsActive prometheus.Gauge
	MessagesReceived  prometheus.Counter
	MessagesSent      prometheus.Counter
	ErrorsTotal       *prometheus.CounterVec
}

// Improve the SafeConn struct to better handle connection state
type SafeConn struct {
	*websocket.Conn
	closed       int32
	mu           sync.Mutex
	metrics      *WSMetrics
	writeBuffer  chan []byte
	readBuffer   chan []byte
	done         chan struct{}
	UserID       string
	TripID       string
	bufferStatus int32
	config       WSConfig
}

// ReadChannel returns the read channel for receiving WebSocket messages
func (sc *SafeConn) ReadChannel() <-chan []byte {
	if sc == nil {
		log := logger.GetLogger()
		log.Error("ReadChannel called on nil SafeConn")
		return nil
	}
	return sc.readBuffer
}

// DoneChannel returns a channel that is closed when the connection is closed
func (sc *SafeConn) DoneChannel() <-chan struct{} {
	if sc == nil {
		log := logger.GetLogger()
		log.Error("DoneChannel called on nil SafeConn")
		return nil
	}
	return sc.done
}

func DefaultWSConfig() WSConfig {
	return WSConfig{
		WriteBufferSize: 512, // 512 bytes is plenty for our message types
		ReadBufferSize:  512,
		MaxMessageSize:  4096, // 4KB is more than enough for our largest possible message

		// Timeouts tuned for mobile clients:
		WriteWait:  5 * time.Second,  // Mobile networks can be slow
		PongWait:   45 * time.Second, // Bit more generous for mobile clients
		PingPeriod: 30 * time.Second, // More frequent pings for connection health
	}
}

func GetActiveConnectionCount() int {
	return int(atomic.LoadInt64(&activeConnectionCount))
}

func NewSafeConn(conn *websocket.Conn, metrics *WSMetrics, config WSConfig) *SafeConn {
	log := logger.GetLogger()

	// Validate input parameters
	if conn == nil {
		log.Errorw("NewSafeConn called with nil connection")
		return nil
	}

	// Apply default configuration values if not specified
	if config.WriteWait == 0 {
		config.WriteWait = 10 * time.Second
	}
	if config.PongWait == 0 {
		config.PongWait = 60 * time.Second
	}
	if config.PingPeriod == 0 {
		config.PingPeriod = 30 * time.Second
	}
	if config.MaxMessageSize == 0 {
		config.MaxMessageSize = 1024 * 1024 // 1MB default
	}
	if config.ReadBufferSize == 0 {
		config.ReadBufferSize = 1024
	}
	if config.WriteBufferSize == 0 {
		config.WriteBufferSize = 1024
	}

	atomic.AddInt64(&activeConnectionCount, 1)

	sc := &SafeConn{
		Conn:        conn,
		metrics:     metrics,
		writeBuffer: make(chan []byte, config.WriteBufferSize),
		readBuffer:  make(chan []byte, config.ReadBufferSize),
		done:        make(chan struct{}),
		config:      config,
		closed:      0,            // Explicitly initialize to 0 (not closed)
		mu:          sync.Mutex{}, // Initialize mutex
	}

	if metrics != nil {
		metrics.connectionCount.Inc()
	} else {
		log.Warnw("NewSafeConn called with nil metrics")
	}

	// Start write pump
	go sc.writePump()
	// Start read pump
	go sc.readPump()

	return sc
}

func (sc *SafeConn) writePump() {
	log := logger.GetLogger()

	// Add validation for sc itself
	if sc == nil {
		log.Errorw("writePump called on nil SafeConn")
		return
	}

	log.Debugw("Starting write pump", "userID", sc.UserID, "tripID", sc.TripID)

	// Safety check for nil channels or connection
	if sc.done == nil || sc.writeBuffer == nil || sc.Conn == nil {
		log.Errorw("Write pump initialized with nil values",
			"userID", sc.UserID,
			"tripID", sc.TripID,
			"done_nil", sc.done == nil,
			"writeBuffer_nil", sc.writeBuffer == nil,
			"conn_nil", sc.Conn == nil)
		// Ensure connection is closed to clean up resources
		if err := sc.Close(); err != nil {
			log.Errorw("Error closing connection", "error", err, "userID", sc.UserID, "tripID", sc.TripID)
		}
		return
	}

	// Check if config is properly initialized
	if sc.config.WriteWait == 0 || sc.config.PingPeriod == 0 {
		log.Warnw("Write pump initialized with invalid config",
			"userID", sc.UserID,
			"tripID", sc.TripID,
			"writeWait", sc.config.WriteWait,
			"pingPeriod", sc.config.PingPeriod)
		if err := sc.Close(); err != nil {
			log.Errorw("Error closing connection with invalid config", "error", err, "userID", sc.UserID, "tripID", sc.TripID)
		}
		return
	}

	pingInterval := sc.config.PingPeriod
	if pingInterval == 0 {
		pingInterval = DefaultWSConfig().PingPeriod
		log.Warnw("Using default ping period", "userID", sc.UserID, "tripID", sc.TripID)
	}

	ticker := time.NewTicker(pingInterval)
	defer func() {
		// Recover from any panics
		if r := recover(); r != nil {
			stack := make([]byte, 4096)
			stack = stack[:runtime.Stack(stack, false)]
			log.Errorw("Panic in writePump recovered",
				"recover", r,
				"userID", sc.UserID,
				"tripID", sc.TripID,
				"stack", string(stack))
		}

		log.Debugw("Stopping write pump", "userID", sc.UserID, "tripID", sc.TripID)
		ticker.Stop()

		// Create a local copy of the connection state to avoid race conditions
		isClosed := atomic.LoadInt32(&sc.closed) == 1

		// Only attempt to close if not already closed
		if !isClosed {
			if err := sc.Close(); err != nil {
				log.Warnw("Error closing connection in write pump defer", "error", err, "userID", sc.UserID, "tripID", sc.TripID)
			}
		} else {
			log.Debugw("Connection already closed, skipping Close() call in writePump", "userID", sc.UserID, "tripID", sc.TripID)
		}
	}()

	// Store local copies of channels to avoid race conditions
	doneChannel := sc.done
	writeBufferChannel := sc.writeBuffer

	for {
		select {
		case message, ok := <-writeBufferChannel:
			// Check if channel is closed
			if !ok {
				log.Debugw("Write buffer channel closed", "userID", sc.UserID, "tripID", sc.TripID)

				// Check if connection is still valid before sending close message
				if atomic.LoadInt32(&sc.closed) == 0 && sc.Conn != nil {
					sc.mu.Lock()
					if sc.Conn != nil {
						if err := sc.Conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
							log.Warnw("Error writing close message", "error", err, "userID", sc.UserID, "tripID", sc.TripID)
						}
					}
					sc.mu.Unlock()
				}
				return
			}

			start := time.Now()
			sc.mu.Lock()

			// Double check if connection is closed
			if atomic.LoadInt32(&sc.closed) == 1 {
				sc.mu.Unlock()
				log.Debugw("Skipping write to closed connection", "userID", sc.UserID, "tripID", sc.TripID)
				return
			}

			// Check if Conn is nil
			if sc.Conn == nil {
				sc.mu.Unlock()
				log.Warnw("Nil connection in write pump", "userID", sc.UserID, "tripID", sc.TripID)
				if err := sc.Close(); err != nil { // Ensure cleanup
					log.Warnw("Error closing nil connection in write pump", "error", err, "userID", sc.UserID, "tripID", sc.TripID)
				}
				return
			}

			// Set a hard deadline for writes to prevent socket leakage
			writeWait := sc.config.WriteWait
			if writeWait == 0 {
				writeWait = DefaultWSConfig().WriteWait
				log.Warnw("Using default write wait", "userID", sc.UserID, "tripID", sc.TripID)
			}

			deadline := time.Now().Add(writeWait)
			if err := sc.Conn.SetWriteDeadline(deadline); err != nil {
				sc.mu.Unlock()
				log.Warnw("SetWriteDeadline failed",
					"error", err,
					"userID", sc.UserID,
					"tripID", sc.TripID)
				return
			}

			err := sc.Conn.WriteMessage(websocket.TextMessage, message)
			sc.mu.Unlock()

			if err != nil {
				log.Warnw("Error writing message",
					"error", err,
					"userID", sc.UserID,
					"tripID", sc.TripID,
					"msgSize", len(message))

				// Check if this is an expected closure
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Debugw("Client closed connection normally",
						"userID", sc.UserID,
						"tripID", sc.TripID)
				} else if websocket.IsUnexpectedCloseError(err) {
					log.Warnw("Unexpected close error",
						"error", err,
						"userID", sc.UserID,
						"tripID", sc.TripID)
				}

				// Check if metrics is nil before accessing it
				if sc.metrics != nil {
					sc.metrics.errorCount.Inc()
				}

				// Ensure connection is closed to clean up resources
				if err := sc.Close(); err != nil {
					log.Warnw("Error closing connection in write pump", "error", err, "userID", sc.UserID, "tripID", sc.TripID)
				}
				return
			}

			// Check if metrics is nil before accessing it
			if sc.metrics != nil {
				latency := time.Since(start).Seconds()
				if sc.metrics.messageLatency != nil {
					sc.metrics.messageLatency.Observe(latency)
				}
				if sc.metrics.messageSize != nil {
					sc.metrics.messageSize.Observe(float64(len(message)))
				}

				// Only log periodic samples of message metrics to avoid verbose logging
				if secureRandomFloat() < 0.01 { // Log ~1% of messages
					log.Debugw("Message sent",
						"userID", sc.UserID,
						"tripID", sc.TripID,
						"latency_ms", latency*1000,
						"size", len(message))
				}
			}

		case <-ticker.C:
			// Skip ping if connection is already closed
			if atomic.LoadInt32(&sc.closed) == 1 {
				log.Debugw("Skipping ping for closed connection", "userID", sc.UserID, "tripID", sc.TripID)
				return
			}

			start := time.Now()
			sc.mu.Lock()

			// Check if Conn is nil
			if sc.Conn == nil {
				sc.mu.Unlock()
				log.Warnw("Nil connection during ping", "userID", sc.UserID, "tripID", sc.TripID)
				if err := sc.Close(); err != nil { // Ensure cleanup
					log.Warnw("Error closing nil connection during ping", "error", err, "userID", sc.UserID, "tripID", sc.TripID)
				}
				return
			}

			writeWait := sc.config.WriteWait
			if writeWait == 0 {
				writeWait = DefaultWSConfig().WriteWait
				log.Warnw("Using default write wait for ping", "userID", sc.UserID, "tripID", sc.TripID)
			}

			pingDeadline := time.Now().Add(writeWait)
			if err := sc.Conn.SetWriteDeadline(pingDeadline); err != nil {
				sc.mu.Unlock()
				log.Warnw("SetWriteDeadline failed for ping",
					"error", err,
					"userID", sc.UserID,
					"tripID", sc.TripID)
				return
			}

			if err := sc.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Warnw("Failed to write ping message",
					"error", err,
					"userID", sc.UserID,
					"tripID", sc.TripID)

				// Check if this is an expected closure
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Debugw("Client closed connection during ping",
						"userID", sc.UserID,
						"tripID", sc.TripID)
				}

				// Check if metrics is nil before accessing it
				if sc.metrics != nil {
					sc.metrics.errorCount.Inc()
				}
				sc.mu.Unlock()

				// Ensure connection is closed to clean up resources
				if err := sc.Close(); err != nil {
					log.Warnw("Error closing connection in write pump", "error", err, "userID", sc.UserID, "tripID", sc.TripID)
				}
				return
			}
			sc.mu.Unlock()

			// Check if metrics is nil before accessing it
			if sc.metrics != nil && sc.metrics.pingPongLatency != nil {
				sc.metrics.pingPongLatency.Observe(time.Since(start).Seconds())
			}

		case <-doneChannel:
			log.Debugw("Done signal received in write pump", "userID", sc.UserID, "tripID", sc.TripID)
			return
		}
	}
}

// isConnectionValid checks if the connection is still valid and can be used
func (sc *SafeConn) isConnectionValid() bool {
	log := logger.GetLogger()

	// Basic checks
	if sc == nil {
		log.Errorw("isConnectionValid called on nil SafeConn")
		return false
	}

	if atomic.LoadInt32(&sc.closed) == 1 {
		log.Debugw("Connection already marked as closed", "userID", sc.UserID, "tripID", sc.TripID)
		return false
	}

	if sc.Conn == nil {
		log.Errorw("Connection is nil", "userID", sc.UserID, "tripID", sc.TripID)
		atomic.StoreInt32(&sc.closed, 1) // Mark as closed to avoid further use
		return false
	}

	// Try sending a ping frame to check connection status
	// This is more reliable than just checking state variables
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Use shorter timeout than normal for connection check
	pingTimeout := sc.config.WriteWait
	if pingTimeout == 0 {
		pingTimeout = 5 * time.Second
	}

	err := sc.Conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(pingTimeout))
	if err != nil {
		log.Warnw("Connection validity check failed (ping error)",
			"error", err,
			"userID", sc.UserID,
			"tripID", sc.TripID,
			"isClosed", atomic.LoadInt32(&sc.closed) == 1)

		// Try to identify the specific error type for better logging
		if websocket.IsUnexpectedCloseError(err) {
			log.Warnw("Unexpected close error during validity check",
				"error", err,
				"userID", sc.UserID,
				"tripID", sc.TripID)
		} else if isNetworkError(err) {
			log.Warnw("Network error during validity check",
				"error", err,
				"userID", sc.UserID,
				"tripID", sc.TripID)
		} else if isTimeoutError(err) {
			log.Warnw("Timeout error during validity check",
				"error", err,
				"userID", sc.UserID,
				"tripID", sc.TripID)
		}

		// If not already marked closed, clean up
		if atomic.LoadInt32(&sc.closed) == 0 {
			// Don't use Close() here as it could recursively call this method
			atomic.StoreInt32(&sc.closed, 1)

			// Close the underlying connection without using our Close method
			closeErr := sc.Conn.Close()
			if closeErr != nil {
				log.Warnw("Error closing invalid connection",
					"error", closeErr,
					"userID", sc.UserID,
					"tripID", sc.TripID)
			}

			// Signal close to other goroutines
			close(sc.done)
		}

		return false
	}

	return true
}

// Close safely closes the WebSocket connection and cleans up resources
func (sc *SafeConn) Close() error {
	log := logger.GetLogger()

	if sc == nil {
		log.Errorw("Close called on nil SafeConn")
		return fmt.Errorf("nil SafeConn")
	}

	// Use CAS operation to ensure we only close once
	if !atomic.CompareAndSwapInt32(&sc.closed, 0, 1) {
		// Already closed, just log and return
		log.Debugw("Connection already closed", "userID", sc.UserID, "tripID", sc.TripID)
		return nil
	}

	log.Infow("Closing WebSocket connection", "userID", sc.UserID, "tripID", sc.TripID)

	// Update metrics
	if sc.metrics != nil {
		sc.metrics.connectionCount.Dec()
		atomic.AddInt64(&activeConnectionCount, -1)
	}

	// Lock to prevent concurrent operations on connection
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Close the done channel to signal goroutines to exit
	// Use a defer and recover in case the channel is already closed
	defer func() {
		if r := recover(); r != nil {
			log.Warnw("Recovered while closing done channel",
				"recover", r,
				"userID", sc.UserID,
				"tripID", sc.TripID)
		}
	}()

	// Close the underlying connection with a graceful close frame
	var closeErr error
	if sc.Conn != nil {
		// Send close message with normal closure code
		closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "connection closed")
		// Use a short timeout for the close message
		deadline := time.Now().Add(1 * time.Second)

		// Only try to write a close frame if the connection appears valid
		writeErr := sc.Conn.WriteControl(websocket.CloseMessage, closeMsg, deadline)
		if writeErr != nil {
			log.Warnw("Failed to send close frame",
				"error", writeErr,
				"userID", sc.UserID,
				"tripID", sc.TripID)
		}

		// Close the underlying connection
		closeErr = sc.Conn.Close()
	}

	// Close the done channel to signal all goroutines
	close(sc.done)

	return closeErr
}

func (sc *SafeConn) readPump() {
	log := logger.GetLogger()

	// Add validation for sc itself
	if sc == nil {
		log.Errorw("readPump called on nil SafeConn")
		return
	}

	log.Debugw("Starting read pump", "userID", sc.UserID, "tripID", sc.TripID)

	defer func() {
		// Recover from any panics
		if r := recover(); r != nil {
			stack := make([]byte, 4096)
			stack = stack[:runtime.Stack(stack, false)]
			log.Errorw("Panic in readPump recovered",
				"recover", r,
				"userID", sc.UserID,
				"tripID", sc.TripID,
				"stack", string(stack))
		}

		log.Debugw("Stopping read pump", "userID", sc.UserID, "tripID", sc.TripID)

		// Create a local copy of the connection state to avoid race conditions
		isClosed := atomic.LoadInt32(&sc.closed) == 1

		// Only attempt to close if not already closed
		if !isClosed && sc.Conn != nil {
			if err := sc.Close(); err != nil {
				log.Warnw("Error closing connection from readPump",
					"error", err,
					"userID", sc.UserID,
					"tripID", sc.TripID)
			}
		}
	}()

	// Initialize time tracking
	connectionStartTime := time.Now()
	lastMessageTime := connectionStartTime

	// Main read loop
	for {
		// Check if connection is closed before continuing
		if atomic.LoadInt32(&sc.closed) == 1 {
			log.Debugw("Exiting readPump - connection is closed",
				"userID", sc.UserID,
				"tripID", sc.TripID)
			return
		}

		// Double-check Conn is still valid before using it
		if sc.Conn == nil {
			log.Warnw("Connection became nil during readPump loop",
				"userID", sc.UserID,
				"tripID", sc.TripID)
			return
		}

		// Set read deadline to detect silent clients
		if err := sc.Conn.SetReadDeadline(time.Now().Add(sc.config.PongWait)); err != nil {
			log.Warnw("Failed to set read deadline",
				"error", err,
				"userID", sc.UserID,
				"tripID", sc.TripID)
			return
		}

		// Use a mutex to protect the read operation
		sc.mu.Lock()
		// Double check connection is still valid after acquiring lock
		if sc.Conn == nil || atomic.LoadInt32(&sc.closed) == 1 {
			sc.mu.Unlock()
			log.Debugw("Connection became invalid after lock acquisition in readPump",
				"userID", sc.UserID,
				"tripID", sc.TripID)
			return
		}
		messageType, message, err := sc.Conn.ReadMessage()
		sc.mu.Unlock()

		if err != nil {
			// Connection closed or error
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived) {
				// Normal closure - mark connection as closed immediately to prevent further operations
				atomic.StoreInt32(&sc.closed, 1)

				log.Infow("Client closed WebSocket connection normally",
					"userID", sc.UserID,
					"tripID", sc.TripID,
					"connectionDuration", time.Since(connectionStartTime).String(),
					"closeCode", getCloseErrorCode(err))

				// For normal closures, we should exit immediately without further processing
				// This prevents accessing fields that might be nullified during cleanup
				return
			} else if websocket.IsUnexpectedCloseError(err) {
				// Unexpected closure
				log.Warnw("Unexpected WebSocket close error",
					"error", err,
					"errorText", err.Error(),
					"userID", sc.UserID,
					"tripID", sc.TripID,
					"connectionDuration", time.Since(connectionStartTime).String(),
					"timeSinceLastMsg", time.Since(lastMessageTime).String(),
					"closeCode", getCloseErrorCode(err))
			} else if isNetworkError(err) {
				// Network error
				log.Warnw("Network error on WebSocket connection",
					"error", err.Error(),
					"userID", sc.UserID,
					"tripID", sc.TripID,
					"connectionDuration", time.Since(connectionStartTime).String(),
					"timeSinceLastMsg", time.Since(lastMessageTime).String())
			} else if isTimeoutError(err) {
				// Timeout
				log.Warnw("Client timeout (no pong response)",
					"error", err.Error(),
					"userID", sc.UserID,
					"tripID", sc.TripID,
					"connectionDuration", time.Since(connectionStartTime).String(),
					"timeSinceLastMsg", time.Since(lastMessageTime).String())
			} else {
				// Other errors
				log.Errorw("Error reading from WebSocket",
					"error", err,
					"errorType", fmt.Sprintf("%T", err),
					"userID", sc.UserID,
					"tripID", sc.TripID,
					"connectionDuration", time.Since(connectionStartTime).String())
			}

			// Mark connection as closed before metrics to prevent race conditions
			atomic.StoreInt32(&sc.closed, 1)

			// Safely increment error count if metrics is available
			if sc.metrics != nil {
				sc.metrics.errorCount.Inc()
			}

			// Exit the read pump on any error
			return
		}

		// Update last message time
		lastMessageTime = time.Now()

		// Log client message
		log.Debugw("Received message from client",
			"userID", sc.UserID,
			"tripID", sc.TripID,
			"messageType", messageTypeToString(messageType),
			"messageSize", len(message))

		// Check if connection was closed during message processing
		if atomic.LoadInt32(&sc.closed) == 1 {
			log.Debugw("Connection closed during message processing",
				"userID", sc.UserID,
				"tripID", sc.TripID)
			return
		}

		// Check if readBuffer is nil again (could have been closed during operation)
		if sc.readBuffer == nil {
			log.Warnw("Read buffer became nil during operation",
				"userID", sc.UserID,
				"tripID", sc.TripID)
			return
		}

		// Use a non-blocking select with a default case to avoid deadlocks
		select {
		case sc.readBuffer <- message:
			// Safely update metrics if available
			if sc.metrics != nil {
				if sc.metrics.messageSize != nil {
					sc.metrics.messageSize.Observe(float64(len(message)))
				}
				if sc.metrics.MessagesReceived != nil {
					sc.metrics.MessagesReceived.Inc()
				}
			}
		default:
			// Buffer full - implement backpressure
			log.Warnw("Read buffer full, implementing backpressure",
				"userID", sc.UserID,
				"tripID", sc.TripID,
				"bufferSize", cap(sc.readBuffer))

			// Safely update metrics if available
			if sc.metrics != nil {
				if sc.metrics.errorCount != nil {
					sc.metrics.errorCount.Inc()
				}
				if sc.metrics.ErrorsTotal != nil {
					sc.metrics.ErrorsTotal.WithLabelValues("read_buffer_full").Inc()
				}
			}
			return
		}
	}
}

// Helper functions for readPump

func messageTypeToString(messageType int) string {
	switch messageType {
	case websocket.TextMessage:
		return "text"
	case websocket.BinaryMessage:
		return "binary"
	case websocket.CloseMessage:
		return "close"
	case websocket.PingMessage:
		return "ping"
	case websocket.PongMessage:
		return "pong"
	default:
		return fmt.Sprintf("unknown(%d)", messageType)
	}
}

func getCloseErrorCode(err error) int {
	if closeErr, ok := err.(*websocket.CloseError); ok {
		return closeErr.Code
	}
	return -1
}

func isNetworkError(err error) bool {
	errorStr := err.Error()
	return strings.Contains(errorStr, "broken pipe") ||
		strings.Contains(errorStr, "connection reset by peer") ||
		strings.Contains(errorStr, "use of closed network connection") ||
		strings.Contains(errorStr, "connection refused")
}

func isTimeoutError(err error) bool {
	errorStr := err.Error()
	return strings.Contains(errorStr, "timeout") ||
		strings.Contains(errorStr, "deadline exceeded") ||
		strings.Contains(errorStr, "i/o timeout")
}

func (sc *SafeConn) WriteMessage(messageType int, data []byte) error {
	// Use our validation function to check if the connection is valid
	if !sc.isConnectionValid() {
		log := logger.GetLogger()
		log.Warnw("WriteMessage called on invalid connection",
			"userID", sc.UserID,
			"tripID", sc.TripID,
			"messageType", messageType,
			"dataSize", len(data))
		return fmt.Errorf("attempted to write to invalid connection")
	}

	// Log the write attempt with connection info
	log := logger.GetLogger()
	log.Debugw("WriteMessage called",
		"messageType", messageType,
		"dataSize", len(data),
		"userID", sc.UserID,
		"tripID", sc.TripID,
		"connClosed", atomic.LoadInt32(&sc.closed) == 1)

	// Double-check if connection is closed
	if atomic.LoadInt32(&sc.closed) == 1 {
		log.Debugw("WriteMessage called on closed connection",
			"userID", sc.UserID,
			"tripID", sc.TripID)
		return websocket.ErrCloseSent
	}

	// Check backpressure status
	if atomic.LoadInt32(&sc.bufferStatus) == 1 {
		if sc.metrics != nil {
			sc.metrics.ErrorsTotal.WithLabelValues("backpressure").Inc()
		}
		return ErrBackpressure
	}

	// Create a local copy of the write buffer to avoid race conditions
	writeBufferChannel := sc.writeBuffer

	// Check if writeBuffer is nil
	if writeBufferChannel == nil {
		log.Warnw("WriteMessage called with nil writeBuffer",
			"userID", sc.UserID,
			"tripID", sc.TripID)
		return fmt.Errorf("write buffer is nil")
	}

	// Calculate fill percentage for buffer - with safety checks
	var fillPercentage float64
	bufLen := len(writeBufferChannel)
	bufCap := cap(writeBufferChannel)

	// Avoid division by zero
	if bufCap > 0 {
		fillPercentage = float64(bufLen) / float64(bufCap)
	} else {
		log.Warnw("WriteBuffer has zero capacity",
			"userID", sc.UserID,
			"tripID", sc.TripID)
		fillPercentage = 1.0 // Assume full buffer
	}

	// Update metrics if available
	if sc.metrics != nil && sc.metrics.bufferUtilization != nil {
		sc.metrics.bufferUtilization.Set(fillPercentage * 100)
	}

	// Update backpressure status if buffer is getting full
	if fillPercentage > 0.8 {
		atomic.StoreInt32(&sc.bufferStatus, 1)
	} else if fillPercentage < 0.2 {
		atomic.StoreInt32(&sc.bufferStatus, 0)
	}

	// Check one more time if connection is closed before sending
	if atomic.LoadInt32(&sc.closed) == 1 {
		log.Debugw("Connection closed just before sending message",
			"userID", sc.UserID,
			"tripID", sc.TripID)
		return websocket.ErrCloseSent
	}

	// Non-blocking send to avoid hanging
	select {
	case writeBufferChannel <- data:
		// Update metrics if available
		if sc.metrics != nil && sc.metrics.MessagesSent != nil {
			sc.metrics.MessagesSent.Inc()
		}
		return nil
	default:
		if sc.metrics != nil {
			sc.metrics.ErrorsTotal.WithLabelValues("buffer_full").Inc()
		}
		log.Warnw("WriteBuffer full",
			"userID", sc.UserID,
			"tripID", sc.TripID,
			"bufferLen", bufLen,
			"bufferCap", bufCap)
		return ErrBackpressure
	}
}

func (sc *SafeConn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	// Use our validation function to check if the connection is valid
	if !sc.isConnectionValid() {
		log := logger.GetLogger()
		log.Warnw("WriteControl called on invalid connection",
			"userID", sc.UserID,
			"tripID", sc.TripID,
			"messageType", messageType)
		return fmt.Errorf("attempted to write control message to invalid connection")
	}

	// Double-check if connection is closed
	if atomic.LoadInt32(&sc.closed) == 1 {
		log := logger.GetLogger()
		log.Debugw("WriteControl called on closed connection",
			"userID", sc.UserID,
			"tripID", sc.TripID,
			"messageType", messageType)
		return websocket.ErrCloseSent
	}

	// Use a mutex to ensure thread safety
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Check again if connection is nil or closed after acquiring the lock
	if sc.Conn == nil {
		log := logger.GetLogger()
		log.Warnw("WriteControl found nil connection after lock",
			"userID", sc.UserID,
			"tripID", sc.TripID,
			"messageType", messageType)
		return fmt.Errorf("connection is nil")
	}

	if atomic.LoadInt32(&sc.closed) == 1 {
		log := logger.GetLogger()
		log.Debugw("WriteControl found closed connection after lock",
			"userID", sc.UserID,
			"tripID", sc.TripID,
			"messageType", messageType)
		return websocket.ErrCloseSent
	}

	// Perform the actual write operation
	return sc.Conn.WriteControl(messageType, data, deadline)
}

func ConnIsClosed(sc *SafeConn) bool {
	// Safety check for nil connection
	if sc == nil {
		log := logger.GetLogger()
		log.Warn("ConnIsClosed called with nil connection")
		return true
	}

	// If the connection is not valid, consider it closed
	if !sc.isConnectionValid() {
		return true
	}

	return atomic.LoadInt32(&sc.closed) == 1
}

func WSMiddleware(config WSConfig, metrics *WSMetrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		log := logger.GetLogger()

		// Validate config and set defaults if needed
		if config.WriteWait == 0 {
			config.WriteWait = 10 * time.Second
		}
		if config.PongWait == 0 {
			config.PongWait = 60 * time.Second
		}
		if config.PingPeriod == 0 {
			config.PingPeriod = 30 * time.Second
		}
		if config.MaxMessageSize == 0 {
			config.MaxMessageSize = 1024 * 1024 // 1MB default
		}
		if config.BufferHighWater == 0 {
			config.BufferHighWater = 256
		}
		if config.BufferLowWater == 0 {
			config.BufferLowWater = 64
		}
		if config.ReadBufferSize == 0 {
			config.ReadBufferSize = 1024
		}
		if config.WriteBufferSize == 0 {
			config.WriteBufferSize = 1024
		}

		upgrader := websocket.Upgrader{
			ReadBufferSize:  config.ReadBufferSize,
			WriteBufferSize: config.WriteBufferSize,
			CheckOrigin: func(r *http.Request) bool {
				if config.CheckOrigin != nil {
					return config.CheckOrigin(r)
				}

				// Default implementation: check origins against allowed list
				if len(config.AllowedOrigins) == 0 || contains(config.AllowedOrigins, "*") {
					return true
				}

				origin := r.Header.Get("Origin")
				if origin == "" {
					return true // No origin header, accept the request
				}

				for _, allowed := range config.AllowedOrigins {
					if allowed == origin {
						return true
					}
				}

				return false
			},
			EnableCompression: true,
		}

		// Get user and trip IDs from the context
		userID := c.GetString("user_id")
		tripID := c.Param("id")

		// Check for client information to help debug
		clientIP := c.ClientIP()
		userAgent := c.Request.UserAgent()

		// Log connection attempt with client info
		log.Infow("WebSocket connection attempt",
			"userID", userID,
			"tripID", tripID,
			"remoteAddr", c.Request.RemoteAddr,
			"clientIP", clientIP,
			"userAgent", userAgent,
			"path", c.Request.URL.Path,
			"queryParams", c.Request.URL.RawQuery,
		)

		// Validate that we have a user ID
		if userID == "" {
			log.Warnw("WebSocket connection attempt without user ID",
				"remoteAddr", c.Request.RemoteAddr,
				"tripID", tripID,
				"path", c.Request.URL.Path,
				"clientIP", clientIP,
				"userAgent", userAgent)

			// Check if this is a WebSocket upgrade request
			if strings.ToLower(c.GetHeader("Connection")) == "upgrade" &&
				strings.ToLower(c.GetHeader("Upgrade")) == "websocket" {
				// For WebSocket upgrade requests, we'll continue and let the auth middleware handle it
				log.Debugw("Continuing WebSocket connection without user ID for auth middleware to handle",
					"path", c.Request.URL.Path)
			} else {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "User authentication required for WebSocket connections",
				})
				return
			}
		}

		// Upgrade the connection
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			if metrics != nil {
				metrics.ErrorsTotal.WithLabelValues("upgrade").Inc()
			}
			log.Errorw("WebSocket upgrade failed",
				"error", err,
				"userID", userID,
				"remoteAddr", c.Request.RemoteAddr,
				"clientIP", clientIP,
				"userAgent", userAgent,
			)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error":  "WebSocket upgrade failed",
				"detail": "Could not establish WebSocket connection",
			})
			return
		}

		// Check if connection is nil (should never happen, but being defensive)
		if conn == nil {
			log.Errorw("Upgrader returned nil connection",
				"userID", userID,
				"tripID", tripID,
				"remoteAddr", c.Request.RemoteAddr)

			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to establish WebSocket connection",
			})
			return
		}

		// Create a safe connection wrapper with all fields properly initialized
		safeConn := &SafeConn{
			Conn:        conn,
			metrics:     metrics,
			writeBuffer: make(chan []byte, config.WriteBufferSize),
			readBuffer:  make(chan []byte, config.ReadBufferSize),
			done:        make(chan struct{}),
			UserID:      userID,
			TripID:      tripID,
			config:      config,
			closed:      0,            // Explicitly initialize to 0 (not closed)
			mu:          sync.Mutex{}, // Initialize mutex
		}

		// Set the pong handler to keep the connection alive
		if conn != nil {
			conn.SetPongHandler(func(string) error {
				// Update deadline when pong received
				if err := conn.SetReadDeadline(time.Now().Add(config.PongWait)); err != nil {
					log.Warnw("Failed to set read deadline in pong handler", "error", err, "userID", userID, "tripID", tripID)
				}
				log.Debugw("Pong received from client",
					"userID", userID,
					"tripID", tripID,
					"remoteAddr", conn.RemoteAddr().String())
				return nil
			})
		}

		// Update metrics
		if metrics != nil {
			metrics.ConnectionsActive.Inc()
		}

		// Set connection in context
		c.Set("wsConnection", safeConn)

		// Start connection management goroutines in a controlled way
		// Use a WaitGroup to ensure goroutines are properly started before continuing
		var wg sync.WaitGroup
		wg.Add(2)

		// Start write pump with proper error handling
		go func() {
			defer func() {
				if r := recover(); r != nil {
					stack := make([]byte, 4096)
					stack = stack[:runtime.Stack(stack, false)]
					log.Errorw("Panic in writePump goroutine",
						"recover", r,
						"userID", userID,
						"tripID", tripID,
						"stack", string(stack))
				}
			}()

			wg.Done() // Signal that goroutine has started
			safeConn.writePump()
		}()

		// Start read pump with proper error handling
		go func() {
			defer func() {
				if r := recover(); r != nil {
					stack := make([]byte, 4096)
					stack = stack[:runtime.Stack(stack, false)]
					log.Errorw("Panic in readPump goroutine",
						"recover", r,
						"userID", userID,
						"tripID", tripID,
						"stack", string(stack))
				}
			}()

			wg.Done() // Signal that goroutine has started
			safeConn.readPump()
		}()

		// Wait for goroutines to start
		wg.Wait()

		// Log successful connection establishment
		log.Infow("WebSocket connection established successfully",
			"userID", userID,
			"tripID", tripID,
			"remoteAddr", c.Request.RemoteAddr,
			"clientIP", clientIP,
			"userAgent", userAgent)

		// Continue with the request
		c.Next()

		// Ensure connection is closed when the handler completes
		// This is important for cases where the handler exits without properly closing the connection
		if safeConn != nil && atomic.LoadInt32(&safeConn.closed) == 0 {
			log.Debugw("Closing WebSocket connection after handler completion",
				"userID", userID,
				"tripID", tripID)

			if err := safeConn.Close(); err != nil {
				log.Warnw("Error closing WebSocket connection after handler completion",
					"error", err,
					"userID", userID,
					"tripID", tripID)
			}
		}
	}
}

// Helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Enhanced backpressure monitoring
// nolint:unused
func (sc *SafeConn) monitorBackpressure() {
	log := logger.GetLogger()

	// Safety check - if connection already has issues, exit early
	if sc == nil {
		log.Warnw("Backpressure monitor initialized with nil connection")
		return
	}

	log.Debugw("Starting backpressure monitor", "userID", sc.UserID, "tripID", sc.TripID)

	if sc.done == nil || sc.writeBuffer == nil {
		log.Warnw("Backpressure monitor initialized with nil values",
			"done_chan_nil", sc.done == nil,
			"write_buffer_nil", sc.writeBuffer == nil)
		return
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sc.done:
			log.Debugw("Backpressure monitor shutting down", "userID", sc.UserID, "tripID", sc.TripID)
			return
		case <-ticker.C:
			// Double-check if writeBuffer is nil before accessing it
			if sc.writeBuffer == nil {
				log.Warnw("WriteBuffer became nil during backpressure monitoring",
					"userID", sc.UserID,
					"tripID", sc.TripID)
				return
			}

			bufferLen := len(sc.writeBuffer)
			bufferCap := cap(sc.writeBuffer)

			// Update metrics only if metrics isn't nil
			if sc.metrics != nil && sc.metrics.bufferUtilization != nil {
				sc.metrics.bufferUtilization.Set(float64(bufferLen) / float64(bufferCap) * 100)
			}

			// Update backpressure status based on config
			if bufferLen >= sc.config.BufferHighWater {
				atomic.StoreInt32(&sc.bufferStatus, 1)
				log.Debugw("Backpressure high", "bufferLen", bufferLen, "threshold", sc.config.BufferHighWater)
			} else if bufferLen <= sc.config.BufferLowWater {
				atomic.StoreInt32(&sc.bufferStatus, 0)
			}
		}
	}
}

func IsWebSocket(c *gin.Context) bool {
	return strings.Contains(strings.ToLower(c.GetHeader("Connection")), "upgrade") &&
		strings.EqualFold(c.GetHeader("Upgrade"), "websocket")
}

// secureRandomFloat returns a cryptographically secure random float64 between 0 and 1
func secureRandomFloat() float64 {
	var buf [8]byte
	_, err := rand.Read(buf[:])
	if err != nil {
		// If crypto/rand fails, return 1.0 to ensure logging happens rather than silently failing
		return 1.0
	}
	return float64(binary.LittleEndian.Uint64(buf[:])) / float64(1<<64)
}

// SendMessage sends a message through the websocket connection with proper error handling
func (sc *SafeConn) SendMessage(message []byte) error {
	log := logger.GetLogger()

	if !sc.isConnectionValid() {
		return fmt.Errorf("cannot send message on invalid connection")
	}

	select {
	case sc.writeBuffer <- message:
		return nil
	default:
		log.Warnw("Write buffer full, message dropped",
			"userID", sc.UserID,
			"tripID", sc.TripID,
			"messageSize", len(message))
		return fmt.Errorf("write buffer full")
	}
}

// GetWSClient returns a ws.Client for a given SafeConn
func GetWSClient(sc *SafeConn) *ws.Client {
	if sc == nil || sc.Conn == nil {
		return nil
	}

	// Create a new ws.Client with the connection from SafeConn
	// Using a 32 buffer size for outgoing messages
	return ws.NewClient(sc.Conn, sc.UserID, 32)
}
