package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// SupabaseClaims represents the expected claims in a Supabase JWT.
type SupabaseClaims struct {
	Subject     string `json:"sub"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	Exp         int64  `json:"exp"`
	AppMetadata struct {
		Provider  string   `json:"provider"`
		Providers []string `json:"providers"`
	} `json:"app_metadata"`
	UserMetadata types.UserMetadata `json:"user_metadata"`
}

// CustomClaims represents the custom claims structure for JWT validation.
type CustomClaims struct {
	Subject string `json:"sub"`
	// Add other claims as needed
}

// AuthMiddleware verifies the API key and validates the Bearer token.
func AuthMiddleware(config *config.ServerConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		var token string
		log := logger.GetLogger()

		// WebSocket-specific token handling
		if IsWebSocket(c) {
			token = c.Query("token")
			log.Debugw("WebSocket token extraction",
				"tokenPresent", token != "",
				"queryParams", c.Request.URL.Query(),
				"path", c.Request.URL.Path)
			if token == "" {
				log.Warn("WebSocket connection missing token parameter")
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "WebSocket connections require ?token parameter",
				})
				return
			}
		} else {
			// Standard Bearer token handling
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization required",
			})
			return
		}

		// Validate JWT token
		log.Debugw("JWT validation attempt", "token", maskToken(token))
		userID, err := validateJWT(token)
		if err != nil {
			// Enhanced error logging
			log.Warnw("Invalid JWT token",
				"error", err,
				"token", maskToken(token),
				"tokenLength", len(token),
				"requestPath", c.Request.URL.Path,
				"requestMethod", c.Request.Method,
				"clientIP", c.ClientIP())

			// Return a more user-friendly message if token is expired
			errorMessage := "Invalid authentication token"
			errorDetails := err.Error()

			if strings.Contains(errorDetails, "token expired") || strings.Contains(errorDetails, "exp not satisfied") {
				errorMessage = "Your session has expired"
				errorDetails = "Please use your refresh token to obtain a new access token via the /v1/auth/refresh endpoint"

				// Create enhanced response with additional info
				enhancedResponse := gin.H{
					"error":            errorMessage,
					"details":          errorDetails,
					"code":             "token_expired",
					"refresh_endpoint": "/v1/auth/refresh",
					"refresh_required": true,
				}

				// Store the enhanced response for the error handler
				c.Set("auth_error_response", enhancedResponse)

				// Also set the standard error for consistent error handling
				if err := c.Error(apperrors.Unauthorized("token_expired", errorMessage)); err != nil {
					log.Errorw("Failed to set error in context", "error", err)
				}
				c.Abort()
				return
			}

			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   errorMessage,
				"details": errorDetails,
			})
			return
		}

		if userID == "" {
			log.Errorw("Empty userID from valid JWT",
				"tokenClaims", getJWTClaims(token),
				"maskedToken", maskToken(token))
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Authentication system error",
			})
			return
		}

		log.Debugw("Authentication successful",
			"userID", userID,
			"isWebSocket", IsWebSocket(c))
		c.Set("user_id", userID)
		c.Next()
	}
}

// Modify your JWT validation logic
func validateJWT(tokenString string) (string, error) {
	log := logger.GetLogger()

	// First parse without verification to inspect the token
	tokenObj, err := jwt.Parse([]byte(tokenString), jwt.WithVerify(false))
	if err == nil {
		log.Debugw("JWT header inspection", "header", tokenObj.PrivateClaims())

		// Log expiration time to help with debugging
		if !tokenObj.Expiration().IsZero() {
			expiresAt := tokenObj.Expiration()
			now := time.Now()
			log.Debugw("Token expiration details",
				"expires_at", expiresAt,
				"current_time", now,
				"is_expired", now.After(expiresAt),
				"time_until_expiry", expiresAt.Sub(now).String())
		}
	}

	// Get the Supabase JWT secret from config
	cfg, err := config.LoadConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load config for JWT validation: %w", err)
	}

	// Decode the base64 secret if needed
	secret := cfg.ExternalServices.SupabaseJWTSecret
	log.Debugw("Using Supabase JWT secret for validation", "secret_length", len(secret))

	// Now parse with verification using the appropriate settings for Supabase tokens
	tokenObj, err = jwt.Parse([]byte(tokenString),
		jwt.WithVerify(true),
		jwt.WithKey(jwa.HS256, []byte(secret)),
		jwt.WithValidate(true),
		jwt.WithAcceptableSkew(30*time.Second),
	)

	if err != nil {
		// Check specifically for token expiration
		if strings.Contains(err.Error(), "exp not satisfied") {
			expiryInfo := "Token has expired"

			// Try to parse the token without validation to get expiry details
			if expiredToken, parseErr := jwt.Parse([]byte(tokenString), jwt.WithVerify(false)); parseErr == nil {
				if !expiredToken.Expiration().IsZero() {
					expiredAt := expiredToken.Expiration()
					now := time.Now()
					expiryInfo = fmt.Sprintf("Token expired at %s (expired %s ago)",
						expiredAt.Format(time.RFC3339),
						now.Sub(expiredAt).String())

					log.Debugw("Token expiration details",
						"expired_at", expiredAt,
						"current_time", now,
						"expired_by", now.Sub(expiredAt).String())
				}
			}

			return "", fmt.Errorf("token expired: %s", expiryInfo)
		}
		return "", fmt.Errorf("invalid token: %w", err)
	}

	// Extract the subject claim (user ID)
	sub := tokenObj.Subject()
	if sub == "" {
		// If subject is empty, try to get from private claims
		if subClaim, ok := tokenObj.PrivateClaims()["sub"].(string); ok {
			sub = subClaim
		}
	}

	return sub, nil
}

func maskToken(token string) string {
	if len(token) < 8 {
		return "***"
	}
	return token[:4] + "***" + token[len(token)-4:]
}

func getJWTClaims(token string) interface{} {
	// Parse the token without validation to extract claims
	tokenObj, err := jwt.Parse([]byte(token), jwt.WithVerify(false))
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to parse token: %v", err),
		}
	}

	// Build a map of all claims
	claims := make(map[string]interface{})

	// Add standard claims
	claims["sub"] = tokenObj.Subject()
	claims["iss"] = tokenObj.Issuer()
	if !tokenObj.Expiration().IsZero() {
		claims["exp"] = tokenObj.Expiration().Unix()
	}
	if !tokenObj.IssuedAt().IsZero() {
		claims["iat"] = tokenObj.IssuedAt().Unix()
	}

	// Add all private claims
	for k, v := range tokenObj.PrivateClaims() {
		claims[k] = v
	}

	return claims
}

func ValidateTokenWithoutAbort(token string) (string, error) {
	if token == "" {
		return "", fmt.Errorf("empty token")
	}

	// Reuse your existing JWT validation logic
	userID, err := validateJWT(token)
	if err != nil {
		return "", fmt.Errorf("invalid token: %w", err)
	}

	return userID, nil
}
