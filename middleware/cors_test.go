package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCORSMiddleware(t *testing.T) {
	// Setup Gin router in test mode
	gin.SetMode(gin.TestMode)

	// Define allowed origins for the test
	allowedOrigins := []string{"http://localhost:3000", "https://nomadcrew.uk"}
	// Create a mock config
	mockConfig := &config.ServerConfig{
		AllowedOrigins: allowedOrigins,
	}
	middleware := CORSMiddleware(mockConfig)

	// Create a test handler that the middleware should call next
	testHandler := func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	}

	testCases := []struct {
		name           string
		requestOrigin  string
		expectedOrigin string // Expected Access-Control-Allow-Origin header
		expectedStatus int    // Expected HTTP status code
		isOptions      bool   // Simulate OPTIONS preflight request
	}{
		{
			name:           "Allowed Origin - Simple Request",
			requestOrigin:  "http://localhost:3000",
			expectedOrigin: "http://localhost:3000",
			expectedStatus: http.StatusOK,
			isOptions:      false,
		},
		{
			name:           "Another Allowed Origin - Simple Request",
			requestOrigin:  "https://nomadcrew.uk",
			expectedOrigin: "https://nomadcrew.uk",
			expectedStatus: http.StatusOK,
			isOptions:      false,
		},
		{
			name:           "Disallowed Origin - Simple Request",
			requestOrigin:  "http://malicious.com",
			expectedOrigin: "",
			expectedStatus: http.StatusForbidden,
			isOptions:      false,
		},
		{
			name:           "No Origin Header - Simple Request",
			requestOrigin:  "",  // No Origin header sent — same-origin or non-browser request
			expectedOrigin: "",  // No CORS header set
			expectedStatus: http.StatusOK, // Passes through (CORS only applies to cross-origin)
			isOptions:      false,
		},
		{
			name:           "Allowed Origin - Preflight Request",
			requestOrigin:  "http://localhost:3000",
			expectedOrigin: "http://localhost:3000",
			expectedStatus: http.StatusNoContent,
			isOptions:      true,
		},
		{
			name:           "Disallowed Origin - Preflight Request",
			requestOrigin:  "http://malicious.com",
			expectedOrigin: "",
			expectedStatus: http.StatusForbidden,
			isOptions:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := gin.Default()
			r.Use(middleware)

			if tc.isOptions {
				r.OPTIONS("/test", testHandler)
			} else {
				r.GET("/test", testHandler)
			}

			method := http.MethodGet
			if tc.isOptions {
				method = http.MethodOptions
			}

			req, _ := http.NewRequest(method, "/test", nil)
			if tc.requestOrigin != "" {
				req.Header.Set("Origin", tc.requestOrigin)
			}
			if tc.isOptions {
				req.Header.Set("Access-Control-Request-Method", "GET")
			}

			r.ServeHTTP(w, req)

			// Assertions
			assert.Equal(t, tc.expectedStatus, w.Code)
			assert.Equal(t, tc.expectedOrigin, w.Header().Get("Access-Control-Allow-Origin"))

			if tc.isOptions && tc.expectedOrigin != "" {
				assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Methods"))
				assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Headers"))
				assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
			}
			if tc.expectedStatus == http.StatusOK && !tc.isOptions {
				assert.Equal(t, "OK", w.Body.String())
			}
		})
	}
}
