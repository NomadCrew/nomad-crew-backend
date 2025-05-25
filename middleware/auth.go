package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	stderrors "errors"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/auth"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
)

// UserResolver defines the interface for resolving Supabase User IDs to internal users.
// This interface is used by the authentication middleware to avoid import cycles.
type UserResolver interface {
	GetUserBySupabaseID(ctx context.Context, supabaseID string) (*types.User, error)
}

// AuthMiddleware creates a Gin middleware for authenticating requests using JWT.
// It validates the JWT token, resolves the Supabase User ID to an internal UUID,
// and stores both the Supabase ID and internal user information in the context.
func AuthMiddleware(validator Validator, userResolver UserResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		log := logger.GetLogger()
		requestPath := c.Request.URL.Path

		// Step 1: Extract Token
		token, err := extractToken(c)
		if err != nil {
			// If extractToken returns a specific type of error (like BadRequest), handle it
			var appErr *apperrors.AppError
			if stderrors.As(err, &appErr) && appErr.GetHTTPStatus() == http.StatusBadRequest {
				log.Infow("Bad request during token extraction", "error", err, "path", requestPath)
				_ = c.Error(err) // Pass the original bad request error
			} else {
				// Handle other extraction errors (like token_missing) as Unauthorized
				log.Warnw("Authentication failed: Token extraction error", "error", err, "path", requestPath)
				_ = c.Error(apperrors.Unauthorized("token_missing", "Authorization required"))
			}

			// Set appropriate HTTP status and response in context
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": err.Error(),
			})
			c.Abort()
			return
		}

		// Step 2: Validate Token
		supabaseUserID, err := validator.Validate(token)
		if err != nil {
			// Determine appropriate error response based on validation error type
			log.Warnw("Authentication failed: Token validation error", "error", err, "path", requestPath)

			var statusCode = http.StatusUnauthorized
			var errorMsg = "Invalid or expired token"
			var errorDetails = err.Error()

			if stderrors.Is(err, auth.ErrTokenExpired) {
				_ = c.Error(apperrors.Unauthorized("token_expired", "Token has expired"))
			} else if stderrors.Is(err, auth.ErrTokenInvalid) {
				if err := c.Error(apperrors.AuthenticationFailed("invalid_token")); err != nil {
					log.Errorw("Failed to set error in context", "error", err)
				}
			} else {
				_ = c.Error(apperrors.Unauthorized("auth_failed", "Authentication failed")) // Generic fallback
			}

			// Set appropriate HTTP status and response in context
			response := gin.H{
				"code":    statusCode,
				"message": errorMsg,
				"details": errorDetails,
			}
			c.JSON(statusCode, response)
			c.Abort()
			return
		}

		// Step 3: Validate Supabase User ID
		if supabaseUserID == "" {
			log.Errorw("Authentication failed: Valid token resulted in empty UserID", "path", requestPath)
			if err := c.Error(apperrors.InternalServerError("internal_error")); err != nil {
				log.Errorw("Failed to set error in context", "error", err)
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    http.StatusInternalServerError,
				"message": "Internal server error",
			})
			c.Abort()
			return
		}

		// Step 4: Resolve Supabase User ID to Internal User
		user, err := userResolver.GetUserBySupabaseID(c.Request.Context(), supabaseUserID)
		if err != nil || user == nil {
			log.Warnw("Valid JWT but user not found in internal system",
				"supabaseUserID", supabaseUserID, "error", err, "path", requestPath)
			_ = c.Error(apperrors.Unauthorized("user_not_onboarded", "User not found or not onboarded"))
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": "User not found or not onboarded",
			})
			c.Abort()
			return
		}

		// Step 5: Store User Information in Context
		internalUserID := user.ID // user.ID is already a string in types.User

		log.Infow("Authentication successful",
			"supabaseUserID", supabaseUserID,
			"internalUserID", internalUserID,
			"username", user.Username,
			"path", requestPath)

		// Store all user information in context
		c.Set(string(UserIDKey), supabaseUserID)         // Keep for backward compatibility
		c.Set(string(InternalUserIDKey), internalUserID) // Internal UUID string
		c.Set(string(AuthenticatedUserKey), user)        // Full user object

		// Also set in the standard Go context for downstream use
		newCtx := context.WithValue(c.Request.Context(), UserIDKey, supabaseUserID)
		newCtx = context.WithValue(newCtx, InternalUserIDKey, internalUserID)
		newCtx = context.WithValue(newCtx, AuthenticatedUserKey, user)
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
