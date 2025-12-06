package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name                string
		environment         config.Environment
		expectedHSTS        bool
		expectedHSTSValue   string
		expectedFrameOption string
		expectedContentType string
		expectedXSS         string
		expectedReferrer    string
	}{
		{
			name:                "Development environment - no HSTS",
			environment:         config.EnvDevelopment,
			expectedHSTS:        false,
			expectedFrameOption: "DENY",
			expectedContentType: "nosniff",
			expectedXSS:         "1; mode=block",
			expectedReferrer:    "strict-origin-when-cross-origin",
		},
		{
			name:                "Production environment - with HSTS",
			environment:         config.EnvProduction,
			expectedHSTS:        true,
			expectedHSTSValue:   "max-age=31536000; includeSubDomains",
			expectedFrameOption: "DENY",
			expectedContentType: "nosniff",
			expectedXSS:         "1; mode=block",
			expectedReferrer:    "strict-origin-when-cross-origin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test configuration
			cfg := &config.Config{
				Server: config.ServerConfig{
					Environment: tt.environment,
					Port:        "8080",
				},
			}

			// Create test router with the security headers middleware
			router := gin.New()
			router.Use(SecurityHeadersMiddleware(cfg))
			router.GET("/test", func(c *gin.Context) {
				c.String(http.StatusOK, "test")
			})

			// Create test request
			req, err := http.NewRequest(http.MethodGet, "/test", nil)
			assert.NoError(t, err)

			// Record the response
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Assert status code
			assert.Equal(t, http.StatusOK, w.Code)

			// Assert X-Frame-Options header
			assert.Equal(t, tt.expectedFrameOption, w.Header().Get("X-Frame-Options"),
				"X-Frame-Options header should be set correctly")

			// Assert X-Content-Type-Options header
			assert.Equal(t, tt.expectedContentType, w.Header().Get("X-Content-Type-Options"),
				"X-Content-Type-Options header should be set correctly")

			// Assert X-XSS-Protection header
			assert.Equal(t, tt.expectedXSS, w.Header().Get("X-XSS-Protection"),
				"X-XSS-Protection header should be set correctly")

			// Assert Referrer-Policy header
			assert.Equal(t, tt.expectedReferrer, w.Header().Get("Referrer-Policy"),
				"Referrer-Policy header should be set correctly")

			// Assert Strict-Transport-Security header based on environment
			if tt.expectedHSTS {
				assert.Equal(t, tt.expectedHSTSValue, w.Header().Get("Strict-Transport-Security"),
					"Strict-Transport-Security header should be set in production")
			} else {
				assert.Empty(t, w.Header().Get("Strict-Transport-Security"),
					"Strict-Transport-Security header should not be set in development")
			}
		})
	}
}

func TestSecurityHeadersMiddleware_MultipleRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Environment: config.EnvProduction,
			Port:        "8080",
		},
	}

	router := gin.New()
	router.Use(SecurityHeadersMiddleware(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "test")
	})

	// Make multiple requests to ensure headers are consistently set
	for i := 0; i < 5; i++ {
		req, err := http.NewRequest(http.MethodGet, "/test", nil)
		assert.NoError(t, err)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
		assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
		assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
		assert.Equal(t, "max-age=31536000; includeSubDomains", w.Header().Get("Strict-Transport-Security"))
	}
}

func TestSecurityHeadersMiddleware_DifferentHTTPMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Environment: config.EnvProduction,
			Port:        "8080",
		},
	}

	router := gin.New()
	router.Use(SecurityHeadersMiddleware(cfg))
	router.POST("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "test")
	})
	router.PUT("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "test")
	})
	router.DELETE("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "test")
	})

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req, err := http.NewRequest(method, "/test", nil)
			assert.NoError(t, err)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
			assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
			assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
			assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
			assert.Equal(t, "max-age=31536000; includeSubDomains", w.Header().Get("Strict-Transport-Security"))
		})
	}
}
