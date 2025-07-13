package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/services"
	"github.com/gin-gonic/gin"
)

// RateLimitConfig holds configuration for rate limiting
type RateLimitConfig struct {
	// RequestsPerMinute is the number of requests allowed per minute
	RequestsPerMinute int
	// RequestsPerHour is the number of requests allowed per hour
	RequestsPerHour int
	// BurstSize is the maximum burst size allowed
	BurstSize int
}

// DefaultRateLimitConfig returns default rate limit configuration
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerMinute: 60,  // 1 request per second average
		RequestsPerHour:   1000, // Conservative hourly limit
		BurstSize:         10,   // Allow short bursts
	}
}

// APIRateLimiter creates a middleware for rate limiting API requests
// It uses a sliding window algorithm with Redis to track request counts
func APIRateLimiter(rateLimiter services.RateLimiterInterface, config RateLimitConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get identifier for rate limiting
		// Priority: authenticated user ID > IP address
		identifier := getRateLimitIdentifier(c)
		
		// Check minute rate limit
		minuteKey := fmt.Sprintf("api:minute:%s", identifier)
		allowed, retryAfter, err := rateLimiter.CheckLimit(
			c.Request.Context(),
			minuteKey,
			config.RequestsPerMinute,
			time.Minute,
		)
		
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Rate limit check failed",
			})
			return
		}
		
		if !allowed {
			setRateLimitHeaders(c, config.RequestsPerMinute, 0, retryAfter)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded",
				"retry_after": int(retryAfter.Seconds()),
				"message":     "Too many requests. Please try again later.",
			})
			return
		}
		
		// Check hourly rate limit
		hourKey := fmt.Sprintf("api:hour:%s", identifier)
		allowed, retryAfter, err = rateLimiter.CheckLimit(
			c.Request.Context(),
			hourKey,
			config.RequestsPerHour,
			time.Hour,
		)
		
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Rate limit check failed",
			})
			return
		}
		
		if !allowed {
			setRateLimitHeaders(c, config.RequestsPerHour, 0, retryAfter)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "Hourly rate limit exceeded",
				"retry_after": int(retryAfter.Seconds()),
				"message":     "Too many requests in the last hour. Please try again later.",
			})
			return
		}
		
		// Set rate limit headers for successful requests
		setRateLimitHeaders(c, config.RequestsPerMinute, config.RequestsPerMinute-1, 0)
		
		c.Next()
	}
}

// EndpointRateLimiter creates a middleware for rate limiting specific endpoints
// This allows different rate limits for different endpoints
func EndpointRateLimiter(rateLimiter services.RateLimiterInterface, requests int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		identifier := getRateLimitIdentifier(c)
		endpoint := c.Request.Method + ":" + c.FullPath()
		
		key := fmt.Sprintf("endpoint:%s:%s", endpoint, identifier)
		allowed, retryAfter, err := rateLimiter.CheckLimit(
			c.Request.Context(),
			key,
			requests,
			window,
		)
		
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Rate limit check failed",
			})
			return
		}
		
		if !allowed {
			setRateLimitHeaders(c, requests, 0, retryAfter)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "Endpoint rate limit exceeded",
				"retry_after": int(retryAfter.Seconds()),
				"message":     fmt.Sprintf("Too many requests to this endpoint. Please try again in %d seconds.", int(retryAfter.Seconds())),
			})
			return
		}
		
		c.Next()
	}
}

// getRateLimitIdentifier returns the identifier to use for rate limiting
// Authenticated users are rate limited by user ID, anonymous users by IP
func getRateLimitIdentifier(c *gin.Context) string {
	// Check for authenticated user
	if userID := c.GetString(string(UserIDKey)); userID != "" {
		return "user:" + userID
	}
	
	// Fallback to IP address for anonymous users
	clientIP := c.ClientIP()
	return "ip:" + clientIP
}

// setRateLimitHeaders sets the standard rate limit headers
func setRateLimitHeaders(c *gin.Context, limit int, remaining int, retryAfter time.Duration) {
	c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
	c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
	
	if retryAfter > 0 {
		c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(retryAfter).Unix(), 10))
		c.Header("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
	}
}