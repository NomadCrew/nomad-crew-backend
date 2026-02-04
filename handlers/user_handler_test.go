package handlers

import (
	"bytes"
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

// MockUserService is now defined in mocks_test.go

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
