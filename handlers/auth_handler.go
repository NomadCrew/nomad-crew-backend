package handlers

import (
	"net/http"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/gin-gonic/gin"
	"github.com/supabase-community/supabase-go"
)

// AuthHandler handles authentication-related endpoints
type AuthHandler struct {
	supabase *supabase.Client
	config   *config.Config
}

// NewAuthHandler creates a new AuthHandler
func NewAuthHandler(supabaseClient *supabase.Client, config *config.Config) *AuthHandler {
	return &AuthHandler{
		supabase: supabaseClient,
		config:   config,
	}
}

// RefreshTokenHandler handles token refresh requests
func (h *AuthHandler) RefreshTokenHandler(c *gin.Context) {
	log := logger.GetLogger()

	// Get the refresh token from the request
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		if err := c.Error(errors.ValidationFailed("invalid_request", "Invalid request format")); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
		return
	}

	log.Debugw("Attempting to refresh token")

	// Use the Supabase client to refresh the token
	session, err := h.supabase.Auth.RefreshToken(req.RefreshToken)
	if err != nil {
		log.Warnw("Failed to refresh token", "error", err)
		if err := c.Error(errors.Unauthorized("refresh_failed", "Failed to refresh token")); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
		return
	}

	// Return the new tokens
	c.JSON(http.StatusOK, gin.H{
		"access_token":  session.AccessToken,
		"refresh_token": session.RefreshToken,
		"expires_in":    session.ExpiresIn,
		"token_type":    "bearer",
	})
}
