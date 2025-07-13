package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func WSRateLimiter(redisClient *redis.Client, maxConnPerUser int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			return
		}

		key := fmt.Sprintf("ws_conn:%s", userID)

		// Use pipeline for atomic operations
		pipe := redisClient.TxPipeline()
		incr := pipe.Incr(c.Request.Context(), key)
		pipe.Expire(c.Request.Context(), key, window)

		_, err := pipe.Exec(c.Request.Context())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Rate limit check failed",
			})
			return
		}

		if incr.Val() > int64(maxConnPerUser) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "Too many WebSocket connections",
				"retry_after": window.Seconds(),
			})
			return
		}

		c.Set("ws_rate_key", key)

		defer func() {
			// Only decrement if WebSocket upgrade wasn't successful
			if c.Writer.Status() != http.StatusSwitchingProtocols {
				redisClient.Decr(c.Request.Context(), key)
			}
		}()

		c.Next()
	}
}
