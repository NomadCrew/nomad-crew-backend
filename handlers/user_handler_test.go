package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockUserService struct {
	mock.Mock
}

func (m *MockUserService) ValidateAndExtractClaims(token string) (*types.JWTClaims, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.JWTClaims), args.Error(1)
}
func (m *MockUserService) OnboardUserFromJWTClaims(ctx context.Context, claims *types.JWTClaims) (*types.UserProfile, error) {
	args := m.Called(ctx, claims)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.UserProfile), args.Error(1)
}

// ... other methods can panic or be no-ops for now ...

func TestOnboardUser_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := new(MockUserService)
	handler := &UserHandler{userService: mockSvc}

	claims := &types.JWTClaims{
		UserID:   "supabase-123",
		Email:    "test@example.com",
		Username: "uniqueuser",
	}
	profile := &types.UserProfile{
		ID:       "uuid-1",
		Username: "uniqueuser",
		Email:    "test@example.com",
	}

	mockSvc.On("ValidateAndExtractClaims", "validtoken").Return(claims, nil)
	mockSvc.On("OnboardUserFromJWTClaims", mock.Anything, claims).Return(profile, nil)

	router := gin.New()
	router.POST("/users/onboard", handler.OnboardUser)

	body := map[string]interface{}{"username": "uniqueuser"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/users/onboard", bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer validtoken")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp types.UserProfile
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "uniqueuser", resp.Username)
}

func TestOnboardUser_UsernameTaken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := new(MockUserService)
	handler := &UserHandler{userService: mockSvc}

	claims := &types.JWTClaims{
		UserID:   "supabase-123",
		Email:    "test@example.com",
		Username: "takenuser",
	}

	mockSvc.On("ValidateAndExtractClaims", "validtoken").Return(claims, nil)
	mockSvc.On("OnboardUserFromJWTClaims", mock.Anything, claims).Return((*types.UserProfile)(nil), errors.New("username is already taken"))

	router := gin.New()
	router.POST("/users/onboard", handler.OnboardUser)

	body := map[string]interface{}{"username": "takenuser"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/users/onboard", bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer validtoken")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Username is already taken")
}

func TestOnboardUser_UsernameMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := new(MockUserService)
	handler := &UserHandler{userService: mockSvc}

	claims := &types.JWTClaims{
		UserID:   "supabase-123",
		Email:    "test@example.com",
		Username: "",
	}

	mockSvc.On("ValidateAndExtractClaims", "validtoken").Return(claims, nil)
	mockSvc.On("OnboardUserFromJWTClaims", mock.Anything, claims).Return((*types.UserProfile)(nil), errors.New("username is required and cannot be empty"))

	router := gin.New()
	router.POST("/users/onboard", handler.OnboardUser)

	body := map[string]interface{}{}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/users/onboard", bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer validtoken")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Username is required")
}

func TestOnboardUser_InvalidJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := new(MockUserService)
	handler := &UserHandler{userService: mockSvc}

	mockSvc.On("ValidateAndExtractClaims", "invalidtoken").Return((*types.JWTClaims)(nil), errors.New("invalid or expired token"))

	router := gin.New()
	router.POST("/users/onboard", handler.OnboardUser)

	body := map[string]interface{}{"username": "uniqueuser"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/users/onboard", bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer invalidtoken")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid or expired token")
}
