package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockJWTValidator is a mock type for the Validator interface
type MockJWTValidator struct {
	mock.Mock
}

// Mock the Validate method signature: Validate(tokenString string) (string, error)
func (m *MockJWTValidator) Validate(tokenString string) (string, error) {
	args := m.Called(tokenString)
	return args.String(0), args.Error(1)
}

// Ensure MockJWTValidator implements the Validator interface
var _ Validator = (*MockJWTValidator)(nil)

// MockUserResolver is a mock type for the UserResolver interface
type MockUserResolver struct {
	mock.Mock
}

// Mock the GetUserBySupabaseID method
func (m *MockUserResolver) GetUserBySupabaseID(ctx context.Context, supabaseID string) (*types.User, error) {
	args := m.Called(ctx, supabaseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}

// Ensure MockUserResolver implements the UserResolver interface
var _ UserResolver = (*MockUserResolver)(nil)

func setupAuthTestRouter(validator Validator, userResolver UserResolver) (*gin.Engine, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	r := gin.New()

	// Add the ErrorHandler middleware to process errors added by AuthMiddleware
	r.Use(ErrorHandler())

	// Pass both the mock validator and user resolver to the middleware
	authMiddleware := AuthMiddleware(validator, userResolver)
	r.Use(authMiddleware)

	// Define a test route that requires authentication
	r.GET("/protected", func(c *gin.Context) {
		// Retrieve canonical user ID from context
		supabaseUserID, exists := c.Get(string(UserIDKey))
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "UserID not found in context"})
			return
		}

		// Check for authenticated user object
		userObj, exists := c.Get(string(AuthenticatedUserKey))
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Authenticated user not found in context"})
			return
		}

		user := userObj.(*types.User)
		c.JSON(http.StatusOK, gin.H{
			"message":          "Success",
			"supabase_user_id": supabaseUserID,
			"user_email":       user.Email,
		})
	})

	return r, w
}

func TestAuthMiddleware(t *testing.T) {
	mockValidator := new(MockJWTValidator)
	mockUserResolver := new(MockUserResolver)
	router, w := setupAuthTestRouter(mockValidator, mockUserResolver)

	testSupabaseUserID := "supabase-user-123"
	validTokenString := "valid.token.string"

	testUser := &types.User{
		ID:       testSupabaseUserID,
		Email:    "test@example.com",
		Username: "testuser",
	}

	testCases := []struct {
		name              string
		tokenHeader       string
		mockSetup         func() // Setup expectations on the mocks
		expectedStatus    int
		expectedBodyCheck func(body string) bool
	}{
		{
			name:           "No Authorization Header",
			tokenHeader:    "",
			mockSetup:      func() {}, // No validation call expected
			expectedStatus: http.StatusUnauthorized,
			expectedBodyCheck: func(body string) bool {
				return assert.Contains(t, body, "Authorization header missing or token not found")
			},
		},
		{
			name:           "Invalid Authorization Header Format - No Bearer",
			tokenHeader:    "InvalidToken",
			mockSetup:      func() {}, // No validation call expected
			expectedStatus: http.StatusUnauthorized,
			expectedBodyCheck: func(body string) bool {
				return assert.Contains(t, body, "Invalid authorization header format")
			},
		},
		{
			name:        "Token Validation Fails",
			tokenHeader: fmt.Sprintf("Bearer %s", validTokenString),
			mockSetup: func() {
				// Mock Validate to return an empty string and an error
				mockValidator.On("Validate", validTokenString).Return("", errors.New("validation failed")).Once()
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBodyCheck: func(body string) bool {
				return assert.Contains(t, body, "Invalid or expired token")
			},
		},
		{
			name:        "User Not Found in Internal System",
			tokenHeader: fmt.Sprintf("Bearer %s", validTokenString),
			mockSetup: func() {
				// Mock successful token validation
				mockValidator.On("Validate", validTokenString).Return(testSupabaseUserID, nil).Once()
				// Mock user resolver to return user not found
				mockUserResolver.On("GetUserBySupabaseID", mock.Anything, testSupabaseUserID).Return(nil, errors.New("user not found")).Once()
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBodyCheck: func(body string) bool {
				return assert.Contains(t, body, "User not found or not onboarded")
			},
		},
		{
			name:        "Successful Authentication",
			tokenHeader: fmt.Sprintf("Bearer %s", validTokenString),
			mockSetup: func() {
				// Mock successful token validation
				mockValidator.On("Validate", validTokenString).Return(testSupabaseUserID, nil).Once()
				// Mock successful user resolution
				mockUserResolver.On("GetUserBySupabaseID", mock.Anything, testSupabaseUserID).Return(testUser, nil).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBodyCheck: func(body string) bool {
				return assert.Contains(t, body, "Success") &&
					assert.Contains(t, body, testSupabaseUserID) &&
					assert.Contains(t, body, testUser.Email)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset recorder and mock expectations for each test case
			*w = *httptest.NewRecorder()
			mockValidator.ExpectedCalls = nil
			mockValidator.Calls = nil
			mockUserResolver.ExpectedCalls = nil
			mockUserResolver.Calls = nil
			tc.mockSetup()

			req, _ := http.NewRequest("GET", "/protected", nil)
			if tc.tokenHeader != "" {
				req.Header.Set("Authorization", tc.tokenHeader)
			}

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)
			assert.True(t, tc.expectedBodyCheck(w.Body.String()), "Body check failed for: %s", w.Body.String())

			// Verify mock expectations
			mockValidator.AssertExpectations(t)
			mockUserResolver.AssertExpectations(t)
		})
	}
}
