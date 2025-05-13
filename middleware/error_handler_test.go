package middleware

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	// Adjust this path based on your go.mod module name and package location
	customErrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:unused
type publicError struct {
	Err    error
	Type   gin.ErrorType
	Meta   interface{}
	Status int // Custom field to map error to status code
}

//nolint:unused
func (p *publicError) Error() string {
	return p.Err.Error()
}

//nolint:unused
func (p *publicError) Unwrap() error {
	return p.Err
}

//nolint:unused
type internalError struct {
	Err error
}

//nolint:unused
func (i *internalError) Error() string {
	return i.Err.Error()
}

func TestErrorHandler(t *testing.T) {
	// Setup Gin router in test mode
	gin.SetMode(gin.TestMode)

	// Define test cases
	testCases := []struct {
		name               string
		err                error          // The error to simulate
		ginErrorType       gin.ErrorType  // Type for gin.Error
		expectedStatusCode int            // Expected HTTP status code
		expectedBody       map[string]any // Expected JSON body structure
		debugMode          bool           // Simulate gin.IsDebugging()
	}{
		{
			name:               "Standard Go Error - Debug Mode",
			err:                errors.New("internal processing error"),
			ginErrorType:       gin.ErrorTypePrivate, // Standard errors are treated as private
			expectedStatusCode: http.StatusInternalServerError,
			expectedBody: map[string]any{
				"code":    http.StatusInternalServerError,
				"message": "Internal Server Error",
				"details": "internal processing error", // Details shown in debug mode
			},
			debugMode: true,
		},
		{
			name:               "Standard Go Error - Production Mode",
			err:                errors.New("internal processing error"),
			ginErrorType:       gin.ErrorTypePrivate,
			expectedStatusCode: http.StatusInternalServerError,
			expectedBody: map[string]any{
				"code":    http.StatusInternalServerError,
				"message": "Internal Server Error",
				// Details omitted in production mode
			},
			debugMode: false,
		},
		{
			name:               "Gin Public Error",
			err:                errors.New("invalid input provided"),
			ginErrorType:       gin.ErrorTypePublic,
			expectedStatusCode: http.StatusBadRequest, // Default for public errors, can be overridden by custom error
			expectedBody: map[string]any{
				"code":    http.StatusBadRequest,
				"message": "invalid input provided", // Public message shown
			},
			debugMode: false,
		},
		{
			name:               "Gin Bind Error",
			err:                errors.New("failed to bind JSON"),
			ginErrorType:       gin.ErrorTypeBind,
			expectedStatusCode: http.StatusBadRequest,
			expectedBody: map[string]any{
				"code":    http.StatusBadRequest,
				"message": "Failed to bind request", // Generic message for bind errors
				"details": "failed to bind JSON",    // Details shown in debug mode
			},
			debugMode: true,
		},
		{
			name:               "Custom Not Found Error",
			err:                customErrors.NotFound("User", "user-id-123"),
			ginErrorType:       gin.ErrorTypePublic,
			expectedStatusCode: http.StatusNotFound,
			expectedBody: map[string]any{
				"code":    http.StatusNotFound,
				"message": "User not found",
				"details": "ID: user-id-123",
			},
			debugMode: false,
		},
		{
			name:               "Custom Validation Error",
			err:                customErrors.ValidationFailed("Validation Error", "email is required"),
			ginErrorType:       gin.ErrorTypePublic,
			expectedStatusCode: http.StatusBadRequest,
			expectedBody: map[string]any{
				"code":    http.StatusBadRequest,
				"message": "Validation Error",
				"details": "email is required",
			},
			debugMode: false,
		},
		{
			name:               "Custom Internal Error",
			err:                customErrors.Wrap(errors.New("database connection failed"), customErrors.DatabaseError, "Internal Server Error"),
			ginErrorType:       gin.ErrorTypePrivate,
			expectedStatusCode: http.StatusInternalServerError,
			expectedBody: map[string]any{
				"code":    http.StatusInternalServerError,
				"message": "Internal Server Error",
			},
			debugMode: false,
		},
		{
			name:               "Custom Internal Error - Debug Mode",
			err:                customErrors.Wrap(errors.New("database connection failed"), customErrors.DatabaseError, "Internal Server Error"),
			ginErrorType:       gin.ErrorTypePrivate,
			expectedStatusCode: http.StatusInternalServerError,
			expectedBody: map[string]any{
				"code":    http.StatusInternalServerError,
				"message": "Internal Server Error",
				"details": "database connection failed",
			},
			debugMode: true,
		},
		{
			name:               "Nil Error", // Should not panic, should proceed normally
			err:                nil,
			ginErrorType:       0, // Doesn't matter
			expectedStatusCode: http.StatusOK,
			expectedBody:       nil, // No error JSON body expected
			debugMode:          false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set debug mode for Gin based on test case
			if tc.debugMode {
				gin.SetMode(gin.DebugMode)
			} else {
				gin.SetMode(gin.ReleaseMode)
			}
			defer gin.SetMode(gin.TestMode) // Reset after test

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w) // Use CreateTestContext for direct middleware testing

			// Simulate request
			req, _ := http.NewRequest("GET", "/test", nil)
			c.Request = req // Assign request to context

			// Add the error handler and a dummy handler that adds the error
			r.Use(ErrorHandler())
			r.GET("/test", func(ctx *gin.Context) {
				if tc.err != nil {
					_ = ctx.Error(tc.err).SetType(tc.ginErrorType) // Add error to context
					// Abort to ensure ErrorHandler runs, mimicking real Gin flow
					// Although ErrorHandler is often used as the *last* middleware,
					// in testing directly, calling Abort isn't strictly necessary
					// as we manually call the handler chain (or just the middleware).
					// However, if other middleware rely on abort, it's safer.
					// In this direct call scenario, it might not matter.
				} else {
					ctx.String(http.StatusOK, "OK") // Simulate success if no error
				}
			})

			// Serve the request
			r.ServeHTTP(w, req)

			// Assertions
			assert.Equal(t, tc.expectedStatusCode, w.Code)

			if tc.expectedBody != nil {
				var responseBody map[string]any
				err := json.Unmarshal(w.Body.Bytes(), &responseBody)
				require.NoError(t, err, "Failed to unmarshal response body")

				// Compare relevant fields, allow for extra fields if necessary
				for key, expectedValue := range tc.expectedBody {
					assert.Contains(t, responseBody, key)
					// Use fmt.Sprintf for consistent comparison, esp. for numeric types
					assert.Equal(t, fmt.Sprintf("%v", expectedValue), fmt.Sprintf("%v", responseBody[key]), "Field mismatch: %s", key)
				}
				// Ensure 'details' is absent if not expected (e.g., prod mode internal errors)
				if _, exists := tc.expectedBody["details"]; !exists {
					assert.NotContains(t, responseBody, "details")
				}
			} else {
				// If no error was expected, the body should not be the error JSON
				assert.NotContains(t, w.Body.String(), `"code":`)
				assert.NotContains(t, w.Body.String(), `"message":`)
				if tc.err == nil {
					assert.Equal(t, "OK", w.Body.String()) // Check for success response body
				}
			}
		})
	}
	// Reset Gin mode after all tests in the suite
	gin.SetMode(gin.TestMode)
}
