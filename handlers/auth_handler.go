// Package handlers contains the HTTP handlers for the application's API endpoints.
package handlers

import (
	"net/http"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/gin-gonic/gin"
	"github.com/supabase-community/supabase-go"
)

// AuthHandler provides handlers for authentication-related operations,
// primarily interacting with the Supabase authentication client.
type AuthHandler struct {
	supabase *supabase.Client
	config   *config.Config
}

// NewAuthHandler creates a new instance of AuthHandler with dependencies.
func NewAuthHandler(supabaseClient *supabase.Client, config *config.Config) *AuthHandler {
	return &AuthHandler{
		supabase: supabaseClient,
		config:   config,
	}
}

// RefreshTokenHandler handles requests to refresh an expired JWT access token
// using a valid refresh token provided in the request body.
// It interacts with the Supabase client to perform the refresh operation.
// Input: JSON body with "refresh_token" field.
// Output: JSON body with new "access_token", "refresh_token", "expires_in", "token_type".
func (h *AuthHandler) RefreshTokenHandler(c *gin.Context) {
	log := logger.GetLogger()

	// Bind the refresh token from the request body.
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnw("RefreshTokenHandler: Invalid request body", "error", err)
		// Use c.Error to attach the application error for the middleware to handle.
		_ = c.Error(errors.ValidationFailed("invalid_request", "Invalid request format: refresh_token is required"))
		return
	}

	log.Debugw("Attempting to refresh token")

	// Use the Supabase client to refresh the session.
	session, err := h.supabase.Auth.RefreshToken(req.RefreshToken)
	if err != nil {
		log.Warnw("Failed to refresh token via Supabase", "error", err)
		_ = c.Error(errors.Unauthorized("refresh_failed", "Failed to refresh token, likely invalid or expired"))
		return
	}

	// Return the new session details.
	c.JSON(http.StatusOK, gin.H{
		"access_token":  session.AccessToken,
		"refresh_token": session.RefreshToken,
		"expires_in":    session.ExpiresIn,
		"token_type":    "bearer",
	})
}
