package middleware

import (
	"net/http"
	"sync/atomic"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type WSConfig struct {
	AllowedOrigins []string
	CheckOrigin    func(r *http.Request) bool
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type SafeConn struct {
	*websocket.Conn
	closed int32 // atomic flag
}

func (sc *SafeConn) Close() error {
	atomic.StoreInt32(&sc.closed, 1)
	return sc.Conn.Close()
}

func ConnIsClosed(sc *SafeConn) bool {
	return atomic.LoadInt32(&sc.closed) == 1
}

func WSMiddleware(cfg WSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Copy WebSocket-specific auth parameters to headers for middleware chain
		if token := c.Query("token"); token != "" {
			c.Request.Header.Set("Authorization", "Bearer "+token)
		}
		if apiKey := c.Query("apikey"); apiKey != "" {
			c.Request.Header.Set("apikey", apiKey)
		}

		// Set origin checker from config
		upgrader.CheckOrigin = cfg.CheckOrigin

		// Perform upgrade
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			c.AbortWithStatusJSON(400, gin.H{"error": "WebSocket upgrade failed"})
			return
		}

		// Store connection in context
		c.Set("wsConnection", &SafeConn{Conn: conn})
		c.Next()
	}
}
