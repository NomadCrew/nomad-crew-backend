package middleware

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/NomadCrew/nomad-crew-backend/logger"
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
	atomic.AddInt64(&activeConnectionCount, 1)

	sc := &SafeConn{
		Conn:        conn,
		metrics:     metrics,
		writeBuffer: make(chan []byte, config.WriteBufferSize),
		readBuffer:  make(chan []byte, config.ReadBufferSize),
		done:        make(chan struct{}),
		config:      config,
	}

	metrics.connectionCount.Inc()

	// Start write pump
	go sc.writePump()
	// Start read pump
	go sc.readPump()

	return sc
}

func (sc *SafeConn) writePump() {
	log := logger.GetLogger()
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

	pingInterval := DefaultWSConfig().PingPeriod
	ticker := time.NewTicker(pingInterval)
	defer func() {
		// Recover from any panics
		if r := recover(); r != nil {
			log.Errorw("Panic in writePump recovered",
				"recover", r,
				"userID", sc.UserID,
				"tripID", sc.TripID)
		}

		log.Debugw("Stopping write pump", "userID", sc.UserID, "tripID", sc.TripID)
		ticker.Stop()
		if err := sc.Close(); err != nil {
			log.Warnw("Error closing connection in write pump defer", "error", err, "userID", sc.UserID, "tripID", sc.TripID)
		}
	}()

	for {
		select {
		case message, ok := <-sc.writeBuffer:
			// Check if channel is closed
			if !ok {
				log.Debugw("Write buffer channel closed", "userID", sc.UserID, "tripID", sc.TripID)
				if sc.Conn != nil {
					if err := sc.Conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
						log.Warnw("Error writing close message", "error", err, "userID", sc.UserID, "tripID", sc.TripID)
					}
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
			deadline := time.Now().Add(DefaultWSConfig().WriteWait)
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

			pingDeadline := time.Now().Add(DefaultWSConfig().WriteWait)
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

		case <-sc.done:
			log.Debugw("Done signal received in write pump", "userID", sc.UserID, "tripID", sc.TripID)
			return
		}
	}
}

// Single Close implementation
func (sc *SafeConn) Close() error {
	log := logger.GetLogger()

	// Only close once
	if !atomic.CompareAndSwapInt32(&sc.closed, 0, 1) {
		log.Debugw("Close called on already closed connection",
			"userID", sc.UserID,
			"tripID", sc.TripID)
		return nil
	}

	log.Debugw("Closing WebSocket connection",
		"userID", sc.UserID,
		"tripID", sc.TripID)

	atomic.AddInt64(&activeConnectionCount, -1)

	// Signal done to stop goroutines
	if sc.done != nil {
		close(sc.done)
	} else {
		log.Warnw("Close called on connection with nil done channel",
			"userID", sc.UserID,
			"tripID", sc.TripID)
	}

	// Update metrics
	if sc.metrics != nil {
		sc.metrics.ConnectionsActive.Dec()
	}

	// Close the actual connection
	var err error
	if sc.Conn != nil {
		err = sc.Conn.Close()
		if err != nil {
			log.Warnw("Error closing underlying WebSocket connection",
				"error", err,
				"userID", sc.UserID,
				"tripID", sc.TripID)
		}
	} else {
		log.Warnw("Close called on connection with nil Conn",
			"userID", sc.UserID,
			"tripID", sc.TripID)
	}

	// Drain buffers to prevent goroutine leaks
	// Use non-blocking operations to avoid deadlocks
	if sc.writeBuffer != nil {
		select {
		case <-sc.writeBuffer:
		default:
		}
	} else {
		log.Warnw("Close called on connection with nil writeBuffer",
			"userID", sc.UserID,
			"tripID", sc.TripID)
	}

	if sc.readBuffer != nil {
		select {
		case <-sc.readBuffer:
		default:
		}
	} else {
		log.Warnw("Close called on connection with nil readBuffer",
			"userID", sc.UserID,
			"tripID", sc.TripID)
	}

	log.Debugw("WebSocket connection closed successfully",
		"userID", sc.UserID,
		"tripID", sc.TripID)

	return err
}

func (sc *SafeConn) readPump() {
	log := logger.GetLogger()
	log.Debugw("Starting read pump", "userID", sc.UserID, "tripID", sc.TripID)

	defer func() {
		// Recover from any panics
		if r := recover(); r != nil {
			log.Errorw("Panic in readPump recovered",
				"recover", r,
				"userID", sc.UserID,
				"tripID", sc.TripID)
		}

		log.Debugw("Stopping read pump", "userID", sc.UserID, "tripID", sc.TripID)
		if err := sc.Close(); err != nil {
			log.Warnw("Error closing connection in read pump", "error", err, "userID", sc.UserID, "tripID", sc.TripID)
		}
	}()

	// Check if Conn is nil
	if sc.Conn == nil {
		log.Warnw("Read pump initialized with nil connection",
			"userID", sc.UserID,
			"tripID", sc.TripID)
		return
	}

	// Configure connection
	sc.Conn.SetReadLimit(sc.config.MaxMessageSize)
	if err := sc.Conn.SetReadDeadline(time.Now().Add(sc.config.PongWait)); err != nil {
		log.Warnw("Failed to set initial read deadline",
			"error", err,
			"userID", sc.UserID,
			"tripID", sc.TripID)
		return
	}

	// Track when we last received data from the client
	var lastMessageTime time.Time = time.Now()
	connectionStartTime := time.Now()

	for {
		// Check if connection is already closed
		if atomic.LoadInt32(&sc.closed) == 1 {
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

		messageType, message, err := sc.Conn.ReadMessage()

		if err != nil {
			// Connection closed or error
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived) {
				// Normal closure
				log.Infow("Client closed WebSocket connection normally",
					"userID", sc.UserID,
					"tripID", sc.TripID,
					"connectionDuration", time.Since(connectionStartTime).String(),
					"closeCode", getCloseErrorCode(err))
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

			if sc.metrics != nil {
				sc.metrics.errorCount.Inc()
			}
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

		// Check if readBuffer is nil
		if sc.readBuffer == nil {
			log.Warnw("Read buffer is nil",
				"userID", sc.UserID,
				"tripID", sc.TripID)
			return
		}

		select {
		case sc.readBuffer <- message:
			if sc.metrics != nil {
				sc.metrics.messageSize.Observe(float64(len(message)))
				sc.metrics.MessagesReceived.Inc()
			}
		default:
			// Buffer full - implement backpressure
			log.Warnw("Read buffer full, implementing backpressure",
				"userID", sc.UserID,
				"tripID", sc.TripID,
				"bufferSize", cap(sc.readBuffer))

			if sc.metrics != nil {
				sc.metrics.errorCount.Inc()
				sc.metrics.ErrorsTotal.WithLabelValues("read_buffer_full").Inc()
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
	// First defensive check - is the connection itself nil?
	if sc == nil {
		return fmt.Errorf("attempted to write to nil connection")
	}

	// Log the write attempt with connection info
	log := logger.GetLogger()
	log.Debugw("WriteMessage called",
		"messageType", messageType,
		"dataSize", len(data),
		"userID", sc.UserID,
		"tripID", sc.TripID,
		"connClosed", atomic.LoadInt32(&sc.closed) == 1)

	// Check if connection is closed
	if atomic.LoadInt32(&sc.closed) == 1 {
		log.Debugw("WriteMessage called on closed connection",
			"userID", sc.UserID,
			"tripID", sc.TripID)
		return websocket.ErrCloseSent
	}

	// Check if writeBuffer is nil
	if sc.writeBuffer == nil {
		log.Warnw("WriteMessage called with nil writeBuffer",
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

	// Calculate fill percentage for buffer - with safety checks
	var fillPercentage float64
	bufLen := len(sc.writeBuffer)
	bufCap := cap(sc.writeBuffer)

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
	case sc.writeBuffer <- data:
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
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.Conn.WriteControl(messageType, data, deadline)
}

func ConnIsClosed(sc *SafeConn) bool {
	// Safety check for nil connection
	if sc == nil {
		log := logger.GetLogger()
		log.Warn("ConnIsClosed called with nil connection")
		return true
	}

	return atomic.LoadInt32(&sc.closed) == 1
}

func WSMiddleware(config WSConfig, metrics *WSMetrics) gin.HandlerFunc {
	return func(c *gin.Context) {
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
		log := logger.GetLogger()
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

			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "User authentication required for WebSocket connections",
			})
			return
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

		// Check if there are any pre-existing connections for this user+trip
		// Track active connections for each user to help diagnose client connection issues
		// This could be enhanced with a proper connection registry

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

		// Create a safe connection wrapper
		safeConn := &SafeConn{
			Conn:        conn,
			metrics:     metrics,
			writeBuffer: make(chan []byte, config.WriteBufferSize),
			readBuffer:  make(chan []byte, config.ReadBufferSize),
			done:        make(chan struct{}),
			UserID:      userID,
			TripID:      tripID,
			config:      config,
		}

		// Set the pong handler to keep the connection alive
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

		// Update metrics
		if metrics != nil {
			metrics.ConnectionsActive.Inc()
		}

		// Set connection in context
		c.Set("wsConnection", safeConn)

		// Start connection management goroutines
		go safeConn.writePump()
		go safeConn.readPump()
		go safeConn.monitorBackpressure()

		// Log successful connection establishment
		log.Infow("WebSocket connection established",
			"userID", userID,
			"tripID", tripID,
			"remoteAddr", c.Request.RemoteAddr,
			"clientIP", clientIP,
			"userAgent", userAgent,
			"config", map[string]interface{}{
				"writeWait":  config.WriteWait,
				"pongWait":   config.PongWait,
				"pingPeriod": config.PingPeriod,
				"bufferSize": config.WriteBufferSize,
			},
		)

		// Ensure connection is closed after handler completes
		defer func() {
			if err := safeConn.Close(); err != nil {
				log.Warnw("Error closing WebSocket connection", "error", err, "userID", userID)
			}
			if metrics != nil {
				metrics.ConnectionsActive.Dec()
			}
			log.Infow("WebSocket connection closed",
				"userID", userID,
				"tripID", tripID,
				"remoteAddr", c.Request.RemoteAddr,
				"clientIP", clientIP,
				"userAgent", userAgent,
			)
		}()

		// Continue to the handler
		c.Next()
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
