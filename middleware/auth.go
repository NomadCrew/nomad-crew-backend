package middleware

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	stderrors "errors"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/auth"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
)

// Simulator bypass constants - only used in development
const (
	simulatorBypassToken  = "simulator-dev-token-bypass" // Legacy format (deprecated)
	simulatorBypassHeader = "X-Simulator-Bypass"
	simulatorUserID       = "00000000-0000-0000-0000-000000000001" // Valid UUID for database compatibility
)

// UserResolver defines the interface for resolving Supabase User IDs to internal users.
// This interface is used by the authentication middleware to avoid import cycles.
type UserResolver interface {
	GetUserBySupabaseID(ctx context.Context, supabaseID string) (*types.User, error)
}

// isSimulatorBypassEnabled checks if simulator bypass should be allowed.
// Only enabled when SERVER_ENVIRONMENT is "development".
func isSimulatorBypassEnabled() bool {
	env := os.Getenv("SERVER_ENVIRONMENT")
	return env == "development"
}

// isSimulatorToken checks if the token is a simulator bypass token.
// Supports both the legacy format and the new JWT format.
// JWT format: {"alg":"none","typ":"JWT"}.{"sub":"00000000-0000-0000-0000-000000000001","exp":9999999999}.
func isSimulatorToken(token string) bool {
	// Check legacy format
	if token == simulatorBypassToken {
		return true
	}

	// Check JWT format by parsing the payload
	// JWT structure: header.payload.signature (base64url encoded)
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return false
	}

	// Decode the payload (second part)
	// The payload should contain {"sub":"00000000-0000-0000-0000-000000000001",...}
	// We just check if the token contains the simulator user ID in the sub claim
	payload := parts[1]

	// Base64url decode (need to handle padding)
	// Add padding if needed
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}

	// Replace URL-safe characters
	payload = strings.ReplaceAll(payload, "-", "+")
	payload = strings.ReplaceAll(payload, "_", "/")

	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return false
	}

	// Check if the decoded payload contains the simulator user ID
	return strings.Contains(string(decoded), simulatorUserID)
}

// AuthMiddleware creates a Gin middleware for authenticating requests using JWT.
// It validates the JWT token, resolves the Supabase User ID to an internal UUID,
// and stores both the Supabase ID and internal user information in the context.
func AuthMiddleware(validator Validator, userResolver UserResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		log := logger.GetLogger()
		requestPath := c.Request.URL.Path

		// DEVELOPMENT ONLY: Check for simulator bypass
		// This allows iOS simulator development without real authentication
		if isSimulatorBypassEnabled() {
			authHeader := c.GetHeader("Authorization")
			token := strings.TrimPrefix(authHeader, "Bearer ")

			if isSimulatorToken(token) {
				log.Warnw("SIMULATOR BYPASS ACTIVE - Using mock authentication (development only)",
					"path", requestPath)

				// Create a mock user for simulator development
				mockUser := &types.User{
					ID:        simulatorUserID,
					Email:     "simulator@dev.nomadcrew.local",
					Username:  "SimulatorDev",
					FirstName: "Simulator",
					LastName:  "Developer",
				}

				// Store in Gin context (simulator user is not admin by default)
				c.Set(string(UserIDKey), simulatorUserID)
				c.Set(string(AuthenticatedUserKey), mockUser)
				c.Set(string(IsAdminKey), false)

				// Store in stdlib context
				newCtx := context.WithValue(c.Request.Context(), UserIDKey, simulatorUserID)
				newCtx = context.WithValue(newCtx, AuthenticatedUserKey, mockUser)
				newCtx = context.WithValue(newCtx, IsAdminKey, false)
				c.Request = c.Request.WithContext(newCtx)

				c.Next()
				return
			}
		}

		// Step 1: Extract Token
		token, err := extractToken(c)
		if err != nil {
			_ = c.Error(err)
			c.Abort()
			return
		}

		// Step 2: Validate Token and extract claims (including admin status)
		claims, err := validator.ValidateAndGetClaims(token)
		if err != nil {

			if stderrors.Is(err, auth.ErrTokenExpired) {
				_ = c.Error(apperrors.Unauthorized("token_expired", "Invalid or expired token"))
			} else if stderrors.Is(err, auth.ErrTokenInvalid) {
				_ = c.Error(apperrors.Unauthorized("invalid_token", "Invalid or expired token"))
			} else {
				_ = c.Error(apperrors.Unauthorized("auth_failed", "Invalid or expired token")) // Generic fallback
			}

			c.Abort()
			return
		}

		// Step 3: Validate Supabase User ID from claims
		supabaseUserID := claims.UserID
		if supabaseUserID == "" {
			log.Errorw("Authentication failed: Valid token resulted in empty UserID", "path", requestPath)
			_ = c.Error(apperrors.InternalServerError("internal_error"))
			c.Abort()
			return
		}

		// Step 4: Resolve Supabase User ID to Internal User
		user, err := userResolver.GetUserBySupabaseID(c.Request.Context(), supabaseUserID)
		if err != nil || user == nil {
			log.Warnw("Valid JWT but user not found in internal system",
				"supabaseUserID", supabaseUserID, "error", err, "path", requestPath)
			_ = c.Error(apperrors.Unauthorized("user_not_onboarded", "User not found or not onboarded"))
			c.Abort()
			return
		}

		// Step 5: Store User Information in Context (single-ID era)
		// In the new schema user.ID === supabaseUserID; we no longer generate a second identifier.
		log.Debugw("Authentication successful",
			"supabaseUserID", supabaseUserID,
			"username", user.Username,
			"path", requestPath)

		// Store in Gin context
		c.Set(string(UserIDKey), supabaseUserID)
		c.Set(string(AuthenticatedUserKey), user)
		c.Set(string(IsAdminKey), claims.IsAdmin)

		// Store in stdlib context
		newCtx := context.WithValue(c.Request.Context(), UserIDKey, supabaseUserID)
		newCtx = context.WithValue(newCtx, AuthenticatedUserKey, user)
		newCtx = context.WithValue(newCtx, IsAdminKey, claims.IsAdmin)
		c.Request = c.Request.WithContext(newCtx)

		c.Next()
	}
}

// extractToken extracts the JWT token from header or query param (for WS).
// Returns the token string or an error if not found.
func extractToken(c *gin.Context) (string, error) {
	log := logger.GetLogger() // Use logger instance

	// 1. Check Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token != "" {
				return token, nil
			}
			// If Bearer prefix exists but no token, it's an invalid format
			return "", apperrors.Unauthorized("invalid_auth_format", "Invalid authorization header format")
		}
		// Authorization header exists but doesn't have Bearer prefix
		return "", apperrors.Unauthorized("invalid_auth_format", "Invalid authorization header format")
	}

	// 2. Check WebSocket upgrade headers and query parameter
	isWebSocketUpgrade := strings.EqualFold(strings.TrimSpace(c.GetHeader("Connection")), "upgrade") &&
		strings.EqualFold(strings.TrimSpace(c.GetHeader("Upgrade")), "websocket")

	if isWebSocketUpgrade {
		tokenFromQuery := c.Query("token")
		if tokenFromQuery != "" {
			return tokenFromQuery, nil
		}
		log.Warnw("WebSocket upgrade request missing 'token' in query parameter", "path", c.Request.URL.Path)
		// Use ValidationError type constant for bad request input
		return "", apperrors.ValidationFailed("websocket_token_missing", "WebSocket token required in query parameter")
	}

	// 3. No token found
	return "", apperrors.Unauthorized("token_missing", "Authorization header missing or token not found")
}

// Update ValidateTokenWithoutAbort to accept the interface as well
func ValidateTokenWithoutAbort(validator Validator, token string) (string, error) {
	if token == "" {
		// Use ValidationError type constant for bad request input
		return "", apperrors.ValidationFailed("token_empty", "Token cannot be empty")
	}
	if validator == nil {
		return "", apperrors.InternalServerError("validator_nil")
	}

	userID, err := validator.Validate(token)
	if err != nil {
		// Map validation errors to appropriate app errors if needed, or return raw validation error
		if stderrors.Is(err, auth.ErrTokenExpired) {
			return "", apperrors.Unauthorized("token_expired", err.Error())
		}
		return "", apperrors.Unauthorized("invalid_token", fmt.Sprintf("Invalid token: %v", err))
	}
	if userID == "" { // Should not happen if Validate returns nil error, but defensive check
		return "", apperrors.InternalServerError("auth_internal_error")
	}

	return userID, nil
}
