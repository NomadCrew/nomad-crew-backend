package middleware

import (
	"net/http"

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

func WSMiddleware(cfg WSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set origin checker from config
		upgrader.CheckOrigin = cfg.CheckOrigin

		// Perform upgrade
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			c.AbortWithStatusJSON(400, gin.H{"error": "WebSocket upgrade failed"})
			return
		}

		// Store connection in context
		c.Set("wsConnection", conn)
		c.Next()
	}
}
