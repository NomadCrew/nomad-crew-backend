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
		isOptions      bool   // Simulate OPTIONS preflight request
	}{
		{
			name:           "Allowed Origin - Simple Request",
			requestOrigin:  "http://localhost:3000",
			expectedOrigin: "http://localhost:3000",
			isOptions:      false,
		},
		{
			name:           "Another Allowed Origin - Simple Request",
			requestOrigin:  "https://nomadcrew.uk",
			expectedOrigin: "https://nomadcrew.uk",
			isOptions:      false,
		},
		{
			name:           "Disallowed Origin - Simple Request",
			requestOrigin:  "http://malicious.com",
			expectedOrigin: "", // Should not set the header for disallowed origins
			isOptions:      false,
		},
		{
			name:           "No Origin Header - Simple Request",
			requestOrigin:  "",  // No Origin header sent
			expectedOrigin: "*", // Should allow any origin if none is specified by client? Depending on desired strictness. Let's assume '*' for now. Revisit if needed.
			// Alternatively, if strict, expectedOrigin should be "" or the first allowed origin. Let's stick with '*' for broader compatibility if no origin is sent.
			isOptions: false,
		},
		{
			name:           "Allowed Origin - Preflight Request",
			requestOrigin:  "http://localhost:3000",
			expectedOrigin: "http://localhost:3000",
			isOptions:      true,
		},
		{
			name:           "Disallowed Origin - Preflight Request",
			requestOrigin:  "http://malicious.com",
			expectedOrigin: "", // Should not allow disallowed origin in preflight
			isOptions:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := gin.Default() // Use Default to include logger/recovery for realistic testing if needed, or New()
			r.Use(middleware)

			// Add appropriate route based on whether it's an OPTIONS request or not
			if tc.isOptions {
				r.OPTIONS("/test", testHandler) // Need a route for OPTIONS
			} else {
				r.GET("/test", testHandler) // Need a route for GET
			}

			method := http.MethodGet
			if tc.isOptions {
				method = http.MethodOptions
			}

			req, _ := http.NewRequest(method, "/test", nil)
			if tc.requestOrigin != "" {
				req.Header.Set("Origin", tc.requestOrigin)
			}
			// For OPTIONS preflight, browsers also send Access-Control-Request-Method
			if tc.isOptions {
				req.Header.Set("Access-Control-Request-Method", "GET")
			}

			r.ServeHTTP(w, req)

			// Assertions
			if tc.isOptions {
				// Preflight requests should return 204 No Content if allowed
				if tc.expectedOrigin != "" {
					assert.Equal(t, http.StatusNoContent, w.Code)
					assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Methods"))
					assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Headers"))
					assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
				} else {
					// If origin is disallowed, the default Gin behavior might handle it,
					// or the middleware might simply not add headers and let the request proceed
					// resulting in a 404 if no handler matches, or 200 if testHandler runs.
					// Let's assume for a disallowed preflight, we expect no CORS headers set.
					// The status code might depend on whether the testHandler runs or not.
					// If CORSMiddleware aborts, it should be 204/403, if not, 200 from testHandler.
					// The current CORSMiddleware doesn't explicitly abort for disallowed origins on OPTIONS, it just doesn't set headers.
					// Let's refine this test if specific abort behavior is added. For now, check headers.
					assert.Equal(t, tc.expectedOrigin, w.Header().Get("Access-Control-Allow-Origin"))
					assert.Empty(t, w.Header().Get("Access-Control-Allow-Methods")) // Or check if it's empty / not the expected values
				}

			} else {
				// Simple requests should return 200 OK if allowed
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, "OK", w.Body.String())
			}

			// Always check the Allow-Origin header
			assert.Equal(t, tc.expectedOrigin, w.Header().Get("Access-Control-Allow-Origin"))

			// Check Vary header is set correctly
			if tc.expectedOrigin != "*" && tc.expectedOrigin != "" {
				assert.Equal(t, "Origin", w.Header().Get("Vary"))
			} else {
				assert.Empty(t, w.Header().Get("Vary"))
			}
		})
	}
}
