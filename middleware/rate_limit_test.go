package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthRateLimiter(t *testing.T) {
	// Set gin to test mode
	gin.SetMode(gin.TestMode)

	// Setup Redis test client (using miniredis would be better for unit tests)
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Skip test if Redis is not available
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available for testing")
	}

	// Clean up any existing test keys
	testKeyPattern := "ratelimit:auth:*"
	keys, _ := redisClient.Keys(ctx, testKeyPattern).Result()
	if len(keys) > 0 {
		redisClient.Del(ctx, keys...)
	}

	defer func() {
		// Clean up after test
		keys, _ := redisClient.Keys(ctx, testKeyPattern).Result()
		if len(keys) > 0 {
			redisClient.Del(ctx, keys...)
		}
		redisClient.Close()
	}()

	t.Run("allows requests under limit", func(t *testing.T) {
		router := gin.New()
		router.Use(AuthRateLimiter(redisClient, 5, 60*time.Second))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// Make 5 requests (under the limit)
		for i := 0; i < 5; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "192.168.1.1:1234"
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Header().Get("X-RateLimit-Limit"), "5")
			assert.NotEmpty(t, w.Header().Get("X-RateLimit-Remaining"))
		}

		// Clean up
		redisClient.Del(ctx, "ratelimit:auth:192.168.1.1")
	})

	t.Run("blocks requests over limit", func(t *testing.T) {
		router := gin.New()
		router.Use(AuthRateLimiter(redisClient, 3, 60*time.Second))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// Make 3 requests (at the limit)
		for i := 0; i < 3; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "192.168.1.2:1234"
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}

		// Make 4th request (over the limit)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.2:1234"
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.Contains(t, w.Header().Get("X-RateLimit-Remaining"), "0")
		assert.NotEmpty(t, w.Header().Get("Retry-After"))

		// Clean up
		redisClient.Del(ctx, "ratelimit:auth:192.168.1.2")
	})

	t.Run("uses X-Forwarded-For header", func(t *testing.T) {
		router := gin.New()
		router.Use(AuthRateLimiter(redisClient, 2, 60*time.Second))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// Make 2 requests with X-Forwarded-For header
		for i := 0; i < 2; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}

		// Make 3rd request (over the limit)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusTooManyRequests, w.Code)

		// Clean up
		redisClient.Del(ctx, "ratelimit:auth:10.0.0.1")
	})

	t.Run("handles Redis connection failure gracefully", func(t *testing.T) {
		// Create a Redis client pointing to an invalid address
		badRedisClient := redis.NewClient(&redis.Options{
			Addr: "localhost:9999", // Non-existent Redis server
		})
		defer badRedisClient.Close()

		router := gin.New()
		router.Use(AuthRateLimiter(badRedisClient, 5, 60*time.Second))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// Request should still succeed even though Redis is unavailable
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.3:1234"
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestGetClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name              string
		remoteAddr        string
		xForwardedFor     string
		xRealIP           string
		expectedIP        string
	}{
		{
			name:       "uses X-Forwarded-For first IP",
			remoteAddr: "192.168.1.1:1234",
			xForwardedFor: "10.0.0.1, 10.0.0.2, 10.0.0.3",
			expectedIP: "10.0.0.1",
		},
		{
			name:       "uses X-Real-IP when X-Forwarded-For is empty",
			remoteAddr: "192.168.1.1:1234",
			xRealIP:    "10.0.0.1",
			expectedIP: "10.0.0.1",
		},
		{
			name:       "falls back to RemoteAddr",
			remoteAddr: "192.168.1.1:1234",
			expectedIP: "192.168.1.1",
		},
		{
			name:              "prefers X-Forwarded-For over X-Real-IP",
			remoteAddr:        "192.168.1.1:1234",
			xForwardedFor:     "10.0.0.1",
			xRealIP:           "10.0.0.2",
			expectedIP:        "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			req, _ := http.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr

			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}

			c.Request = req

			ip := getClientIP(c)
			assert.Equal(t, tt.expectedIP, ip)
		})
	}
}

func TestAuthRateLimiterHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available for testing")
	}

	defer func() {
		keys, _ := redisClient.Keys(ctx, "ratelimit:auth:*").Result()
		if len(keys) > 0 {
			redisClient.Del(ctx, keys...)
		}
		redisClient.Close()
	}()

	router := gin.New()
	router.Use(AuthRateLimiter(redisClient, 10, 60*time.Second))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	t.Run("includes correct rate limit headers", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.100:1234"
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		// Check headers
		assert.Equal(t, "10", w.Header().Get("X-RateLimit-Limit"))
		assert.Equal(t, "9", w.Header().Get("X-RateLimit-Remaining"))
		assert.NotEmpty(t, w.Header().Get("X-RateLimit-Reset"))

		// Make another request
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.100:1234"
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "10", w.Header().Get("X-RateLimit-Limit"))
		assert.Equal(t, "8", w.Header().Get("X-RateLimit-Remaining"))

		// Clean up
		redisClient.Del(ctx, "ratelimit:auth:192.168.1.100")
	})
}

func ExampleAuthRateLimiter() {
	// Setup Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Create Gin router
	router := gin.New()

	// Apply rate limiter: 10 requests per minute
	router.Use(AuthRateLimiter(redisClient, 10, 60*time.Second))

	// Define routes
	router.POST("/auth/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "login successful"})
	})

	// Start server
	fmt.Println("Server starting with rate limiting enabled")
	// Output: Server starting with rate limiting enabled
}
