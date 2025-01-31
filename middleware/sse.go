package middleware

import (
    "github.com/gin-gonic/gin"
    "github.com/NomadCrew/nomad-crew-backend/logger"
)

type SSEConfig struct {
    AllowedOrigins []string
    MaxConnections int
}

func SSEMiddleware(cfg SSEConfig) gin.HandlerFunc {
    return func(c *gin.Context) {
        log := logger.GetLogger()

        // Set SSE headers
        c.Header("Content-Type", "text/event-stream")
        c.Header("Cache-Control", "no-cache")
        c.Header("Connection", "keep-alive")
        c.Header("Transfer-Encoding", "chunked")

        // CORS headers
        origin := c.GetHeader("Origin")
        if len(cfg.AllowedOrigins) == 0 {
            c.Header("Access-Control-Allow-Origin", "*")
        } else {
            for _, allowed := range cfg.AllowedOrigins {
                if origin == allowed {
                    c.Header("Access-Control-Allow-Origin", origin)
                    break
                }
            }
        }
        c.Header("Access-Control-Allow-Credentials", "true")
        c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, apikey")

        log.Debug("SSE headers set for request")
        c.Next()
    }
}