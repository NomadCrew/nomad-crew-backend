package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockJWTValidator is a mock type for the *middleware*.JWTValidator struct's methods
// We need an interface that middleware.AuthMiddleware accepts, or mock the concrete type's methods.
// AuthMiddleware takes *middleware.JWTValidator directly. We mock its Validate method.
type MockJWTValidator struct {
	mock.Mock
	// Embed the actual validator if needed to satisfy interface, or just mock methods called.
	// middleware.JWTValidator // Embedding might pull in dependencies. Let's just mock Validate.
}

// Mock the Validate method signature: Validate(tokenString string) (string, error)
func (m *MockJWTValidator) Validate(tokenString string) (string, error) {
	args := m.Called(tokenString)
	// Return the mocked UserID (string) and error
	return args.String(0), args.Error(1)
}

// Ensure MockJWTValidator implements the interface
var _ Validator = (*MockJWTValidator)(nil)

func setupAuthTestRouter(validator Validator) (*gin.Engine, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	r := gin.New()

	// Pass the mock validator to the actual middleware function
	authMiddleware := AuthMiddleware(validator) // AuthMiddleware expects *JWTValidator
	r.Use(authMiddleware)

	// Define a test route that requires authentication
	r.GET("/protected", func(c *gin.Context) {
		userID, exists := c.Get(string(UserIDKey))
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "UserID not found in context"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Success", "user_id": userID})
	})

	// Define a public route that should not be affected directly by auth (but middleware runs)
	r.GET("/public", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Public OK"})
	})

	return r, w
}

func TestAuthMiddleware(t *testing.T) {
	mockValidator := new(MockJWTValidator) // Use the mock struct
	router, w := setupAuthTestRouter(mockValidator)
	testUserID := uuid.New()
	testUserIDString := testUserID.String()

	validTokenString := "valid.token.string"

	testCases := []struct {
		name              string
		tokenHeader       string
		mockSetup         func() // Setup expectations on the mock
		expectedStatus    int
		expectedBody      string
		checkContextValue bool
		expectedUserID    string
	}{
		{
			name:           "No Authorization Header",
			tokenHeader:    "",
			mockSetup:      func() {}, // No validation call expected
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"code":401,"message":"AUTHENTICATION_ERROR: Authorization header missing or token not found"}`,
		},
		{
			name:           "Invalid Authorization Header Format - No Bearer",
			tokenHeader:    "InvalidToken",
			mockSetup:      func() {}, // No validation call expected
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"code":401,"message":"AUTHENTICATION_ERROR: Invalid authorization header format"}`,
		},
		{
			name:           "Invalid Authorization Header Format - Only Bearer",
			tokenHeader:    "Bearer ",
			mockSetup:      func() {}, // No validation call expected
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"code":401,"message":"AUTHENTICATION_ERROR: Invalid authorization header format"}`,
		},
		{
			name:        "Token Validation Fails",
			tokenHeader: fmt.Sprintf("Bearer %s", validTokenString),
			mockSetup: func() {
				// Mock Validate to return an empty string and an error
				mockValidator.On("Validate", validTokenString).Return("", errors.New("validation failed")).Once()
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"code":401,"message":"Invalid or expired token","details":"validation failed"}`,
		},
		{
			name:        "Token Validation Succeeds",
			tokenHeader: fmt.Sprintf("Bearer %s", validTokenString),
			mockSetup: func() {
				// Mock Validate to return the UserID string and nil error
				mockValidator.On("Validate", validTokenString).Return(testUserIDString, nil).Once()
			},
			expectedStatus:    http.StatusOK,
			expectedBody:      fmt.Sprintf(`{"message":"Success","user_id":"%s"}`, testUserIDString),
			checkContextValue: true,
			expectedUserID:    testUserIDString,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset recorder and mock expectations for each test case
			*w = *httptest.NewRecorder()      // Create a new recorder
			mockValidator.ExpectedCalls = nil // Clear previous expectations
			mockValidator.Calls = nil
			tc.mockSetup()

			req, _ := http.NewRequest("GET", "/protected", nil)
			if tc.tokenHeader != "" {
				req.Header.Set("Authorization", tc.tokenHeader)
			}

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)
			assert.JSONEq(t, tc.expectedBody, w.Body.String())

			// Optionally verify mock expectations
			mockValidator.AssertExpectations(t)

			// Check context value only if validation was expected to succeed
			// Need a way to access context *after* middleware but *before* handler fully responds in test
			// This is tricky without modifying the test handler. The current test handler checks it.
			// If checkContextValue is true, the handler success implies context was set.
		})
	}
}
