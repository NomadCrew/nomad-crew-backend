package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func WSRateLimiter(redisClient *redis.Client, maxConnPerUser int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString(string(UserIDKey))
		if userID == "" {
			_ = c.Error(apperrors.Unauthorized("missing_auth", "Authentication required"))
			c.Abort()
			return
		}

		key := fmt.Sprintf("ws_conn:%s", userID)

		// Use pipeline for atomic operations
		pipe := redisClient.TxPipeline()
		incr := pipe.Incr(c.Request.Context(), key)
		pipe.Expire(c.Request.Context(), key, window)

		_, err := pipe.Exec(c.Request.Context())
		if err != nil {
			_ = c.Error(apperrors.InternalServerError("Rate limit check failed"))
			c.Abort()
			return
		}

		if incr.Val() > int64(maxConnPerUser) {
			_ = c.Error(apperrors.RateLimitExceeded("Too many WebSocket connections", int(window.Seconds())))
			c.Abort()
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

// AuthRateLimiter creates a rate limiter middleware for authentication endpoints.
// It uses Redis for distributed rate limiting based on IP address to prevent brute force attacks.
// The limiter uses a sliding window approach with Redis INCR and EXPIRE commands.
func AuthRateLimiter(redisClient *redis.Client, requestsPerMinute int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client IP address
		ip := getClientIP(c)

		// Create rate limit key based on IP
		key := fmt.Sprintf("ratelimit:auth:%s", ip)

		// Use pipeline for atomic operations
		pipe := redisClient.TxPipeline()
		incr := pipe.Incr(c.Request.Context(), key)
		pipe.Expire(c.Request.Context(), key, window)

		_, err := pipe.Exec(c.Request.Context())
		if err != nil {
			// Log error but don't block the request on Redis failures
			// This ensures the API remains available even if Redis is down
			c.Next()
			return
		}

		count := incr.Val()

		// Check if limit exceeded
		if count > int64(requestsPerMinute) {
			// Get TTL for retry-after header
			ttl, err := redisClient.TTL(c.Request.Context(), key).Result()
			if err != nil {
				ttl = window // fallback to window duration
			}

			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", requestsPerMinute))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(ttl).Unix()))
			c.Header("Retry-After", fmt.Sprintf("%d", int(ttl.Seconds())))

			_ = c.Error(apperrors.RateLimitExceeded("Too many requests. Please try again later.", int(ttl.Seconds())))
			c.Abort()
			return
		}

		// Add rate limit headers for successful requests
		remaining := requestsPerMinute - int(count)
		if remaining < 0 {
			remaining = 0
		}

		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", requestsPerMinute))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(window).Unix()))

		c.Next()
	}
}

// getClientIP extracts the real client IP from the request.
// It checks X-Forwarded-For and X-Real-IP headers first (for proxies/load balancers),
// then falls back to RemoteAddr.
func getClientIP(c *gin.Context) string {
	// Check X-Forwarded-For header (can contain multiple IPs)
	if forwarded := c.GetHeader("X-Forwarded-For"); forwarded != "" {
		// Take the first IP in the chain
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if realIP := c.GetHeader("X-Real-IP"); realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	return c.ClientIP()
}
