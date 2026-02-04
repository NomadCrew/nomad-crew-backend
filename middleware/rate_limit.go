package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// InMemoryRateLimiter provides a fallback when Redis is unavailable
type InMemoryRateLimiter struct {
	mu        sync.RWMutex
	counts    map[string]*rateLimitEntry
	limit     int
	window    time.Duration
	lastClean time.Time
}

type rateLimitEntry struct {
	count     int
	expiresAt time.Time
}

// NewInMemoryRateLimiter creates a new in-memory rate limiter for fallback
func NewInMemoryRateLimiter(limit int, window time.Duration) *InMemoryRateLimiter {
	return &InMemoryRateLimiter{
		counts:    make(map[string]*rateLimitEntry),
		limit:     limit,
		window:    window,
		lastClean: time.Now(),
	}
}

// Allow checks if the request should be allowed and returns (allowed, remaining)
func (l *InMemoryRateLimiter) Allow(key string) (allowed bool, remaining int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Periodic cleanup of expired entries
	if time.Since(l.lastClean) > l.window {
		l.cleanup()
		l.lastClean = time.Now()
	}

	now := time.Now()
	entry, exists := l.counts[key]

	if !exists || now.After(entry.expiresAt) {
		// New entry or expired
		l.counts[key] = &rateLimitEntry{
			count:     1,
			expiresAt: now.Add(l.window),
		}
		return true, l.limit - 1
	}

	entry.count++
	if entry.count > l.limit {
		return false, 0
	}
	return true, l.limit - entry.count
}

func (l *InMemoryRateLimiter) cleanup() {
	now := time.Now()
	for key, entry := range l.counts {
		if now.After(entry.expiresAt) {
			delete(l.counts, key)
		}
	}
}

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

// AuthRateLimiterWithFallback creates a rate limiter middleware that uses Redis
// with an in-memory fallback when Redis is unavailable.
// SECURITY: This ensures rate limiting is ALWAYS enforced (fail-closed behavior).
func AuthRateLimiterWithFallback(
	redisClient *redis.Client,
	fallback *InMemoryRateLimiter,
	requestsPerMinute int,
	window time.Duration,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := getClientIP(c)
		key := fmt.Sprintf("ratelimit:auth:%s", ip)

		// Try Redis first
		pipe := redisClient.TxPipeline()
		incr := pipe.Incr(c.Request.Context(), key)
		pipe.Expire(c.Request.Context(), key, window)

		_, err := pipe.Exec(c.Request.Context())
		if err != nil {
			// Redis failed - use in-memory fallback (STILL ENFORCES LIMITS)
			// Log at warn level since this is degraded but functional
			allowed, remaining := fallback.Allow(key)
			if !allowed {
				c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", requestsPerMinute))
				c.Header("X-RateLimit-Remaining", "0")
				c.Header("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
				c.Header("X-RateLimit-Mode", "fallback")
				_ = c.Error(apperrors.RateLimitExceeded(
					"Too many requests. Please try again later.",
					int(window.Seconds()),
				))
				c.Abort()
				return
			}

			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", requestsPerMinute))
			c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
			c.Header("X-RateLimit-Mode", "fallback")
			c.Next()
			return
		}

		// Redis succeeded - normal flow
		count := incr.Val()
		if count > int64(requestsPerMinute) {
			ttl, err := redisClient.TTL(c.Request.Context(), key).Result()
			if err != nil || ttl < 0 {
				ttl = window
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
// SECURITY: Uses Gin's built-in ClientIP() which:
// 1. Only parses X-Forwarded-For/X-Real-IP if RemoteAddr is from a trusted proxy
// 2. Falls back to RemoteAddr if the request is not from a trusted proxy
// 3. Trusted proxies are configured via r.SetTrustedProxies() in router setup
func getClientIP(c *gin.Context) string {
	return c.ClientIP()
}
