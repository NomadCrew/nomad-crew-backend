package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// RequestIDKey is the key used to store the request ID in the gin context
	RequestIDKey = "request_id"
)

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if request already has an ID from a load balancer or proxy
		requestID := c.GetHeader("X-Request-ID")

		// If no request ID exists, generate one
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Add the request ID to the context and response headers
		c.Set(RequestIDKey, requestID)
		c.Header("X-Request-ID", requestID)

		c.Next()
	}
}
