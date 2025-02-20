package middleware

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type WSConfig struct {
	AllowedOrigins  []string
	CheckOrigin     func(r *http.Request) bool
	WriteBufferSize int           // Default 1024
	ReadBufferSize  int           // Default 1024
	MaxMessageSize  int64         // Default 512KB
	WriteWait       time.Duration // Time allowed to write a message
	PongWait        time.Duration // Time allowed to read the next pong message
	PingPeriod      time.Duration // Send pings to peer with this period
}

type WSMetrics struct {
	connectionCount   prometheus.Gauge
	messageSize       prometheus.Histogram
	messageLatency    prometheus.Histogram
	errorCount        prometheus.Counter
	bufferUtilization prometheus.Gauge
	pingPongLatency   prometheus.Histogram
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type SafeConn struct {
	*websocket.Conn
	closed  int32 // atomic flag
	mu      sync.Mutex
	metrics *WSMetrics

	// Add buffer management
	writeBuffer chan []byte
	readBuffer  chan []byte
	done        chan struct{}
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

func initWSMetrics() *WSMetrics {
	return &WSMetrics{
		connectionCount: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "nomadcrew_ws_connections_total",
			Help: "Number of active WebSocket connections",
		}),
		messageSize: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "nomadcrew_ws_message_size_bytes",
			Help:    "Size of WebSocket messages in bytes",
			Buckets: []float64{64, 128, 256, 512, 1024, 2048, 4096},
		}),
		messageLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "nomadcrew_ws_message_latency_seconds",
			Help:    "Latency of WebSocket message processing",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25},
		}),
		errorCount: promauto.NewCounter(prometheus.CounterOpts{
			Name: "nomadcrew_ws_errors_total",
			Help: "Total number of WebSocket errors",
		}),
		bufferUtilization: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "nomadcrew_ws_buffer_utilization",
			Help: "Current WebSocket buffer utilization percentage",
		}),
		pingPongLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "nomadcrew_ws_ping_pong_latency_seconds",
			Help:    "Latency of WebSocket ping/pong operations",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25},
		}),
	}
}

func NewSafeConn(conn *websocket.Conn, metrics *WSMetrics) *SafeConn {
	sc := &SafeConn{
		Conn:        conn,
		metrics:     metrics,
		writeBuffer: make(chan []byte, DefaultWSConfig().WriteBufferSize),
		readBuffer:  make(chan []byte, DefaultWSConfig().ReadBufferSize),
		done:        make(chan struct{}),
	}

	metrics.connectionCount.Inc()

	// Start write pump
	go sc.writePump()
	// Start read pump
	go sc.readPump()

	return sc
}

func (sc *SafeConn) writePump() {
	ticker := time.NewTicker(DefaultWSConfig().PingPeriod)
	defer func() {
		ticker.Stop()
		sc.Close()
	}()

	for {
		select {
		case message := <-sc.writeBuffer:
			start := time.Now()
			sc.mu.Lock()
			sc.Conn.SetWriteDeadline(time.Now().Add(DefaultWSConfig().WriteWait))
			err := sc.Conn.WriteMessage(websocket.TextMessage, message)
			sc.mu.Unlock()

			if err != nil {
				sc.metrics.errorCount.Inc()
				return
			}

			sc.metrics.messageLatency.Observe(time.Since(start).Seconds())
			sc.metrics.messageSize.Observe(float64(len(message)))

		case <-ticker.C:
			start := time.Now()
			sc.mu.Lock()
			sc.Conn.SetWriteDeadline(time.Now().Add(DefaultWSConfig().WriteWait))
			if err := sc.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				sc.metrics.errorCount.Inc()
				sc.mu.Unlock()
				return
			}
			sc.mu.Unlock()
			sc.metrics.pingPongLatency.Observe(time.Since(start).Seconds())

		case <-sc.done:
			return
		}
	}
}

// Single Close implementation
func (sc *SafeConn) Close() error {
	if !atomic.CompareAndSwapInt32(&sc.closed, 0, 1) {
		return nil
	}

	close(sc.done)
	sc.metrics.connectionCount.Dec()

	// Drain buffers
	for range sc.writeBuffer {
	}
	for range sc.readBuffer {
	}

	return sc.Conn.Close()
}

func (sc *SafeConn) readPump() {
	defer func() {
		sc.Close()
	}()

	sc.Conn.SetReadLimit(DefaultWSConfig().MaxMessageSize)
	sc.Conn.SetReadDeadline(time.Now().Add(DefaultWSConfig().PongWait))
	sc.Conn.SetPongHandler(func(string) error {
		sc.Conn.SetReadDeadline(time.Now().Add(DefaultWSConfig().PongWait))
		return nil
	})

	for {
		_, message, err := sc.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				sc.metrics.errorCount.Inc()
			}
			return
		}

		select {
		case sc.readBuffer <- message:
			sc.metrics.messageSize.Observe(float64(len(message)))
		default:
			// Buffer full - implement backpressure
			sc.metrics.errorCount.Inc()
			return
		}
	}
}

func (sc *SafeConn) WriteMessage(messageType int, data []byte) error {
	if atomic.LoadInt32(&sc.closed) == 1 {
		return websocket.ErrCloseSent
	}

	// Implement backpressure with non-blocking channel send
	select {
	case sc.writeBuffer <- data:
		sc.metrics.bufferUtilization.Set(float64(len(sc.writeBuffer)) / float64(cap(sc.writeBuffer)) * 100)
		return nil
	default:
		sc.metrics.errorCount.Inc()
		return errors.New(errors.ServerError, "Write buffer full", "write buffer is at capacity")
	}
}

func (sc *SafeConn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.Conn.WriteControl(messageType, data, deadline)
}

func ConnIsClosed(sc *SafeConn) bool {
	return atomic.LoadInt32(&sc.closed) == 1
}

func WSMiddleware(cfg WSConfig) gin.HandlerFunc {
	metrics := initWSMetrics()
	upgrader := websocket.Upgrader{
		ReadBufferSize:  cfg.ReadBufferSize,
		WriteBufferSize: cfg.WriteBufferSize,
		CheckOrigin:     cfg.CheckOrigin,
	}

	return func(c *gin.Context) {
		// Copy WebSocket-specific auth parameters to headers
		if token := c.Query("token"); token != "" {
			c.Request.Header.Set("Authorization", "Bearer "+token)
		}
		if apiKey := c.Query("apikey"); apiKey != "" {
			c.Request.Header.Set("apikey", apiKey)
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			metrics.errorCount.Inc()
			c.AbortWithStatusJSON(400, gin.H{"error": "WebSocket upgrade failed"})
			return
		}

		// Store enhanced SafeConn in context
		c.Set("wsConnection", NewSafeConn(conn, metrics))
		c.Next()
	}
}
