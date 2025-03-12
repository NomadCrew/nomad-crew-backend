package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"encoding/base64"

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
		log := logger.GetLogger()

		log.Debugw("Processing auth middleware",
			"path", c.Request.URL.Path,
			"method", c.Request.Method,
			"headers", func() map[string]string {
				headers := make(map[string]string)
				for k, v := range c.Request.Header {
					if k != "Authorization" && k != "Cookie" { // Skip sensitive headers
						headers[k] = strings.Join(v, ",")
					}
				}
				return headers
			}())

		// Extract token from Authorization header
		var token string
		authHeader := c.GetHeader("Authorization")

		log.Debugw("Auth header inspection",
			"header_present", authHeader != "",
			"header_length", len(authHeader),
			"starts_with_bearer", strings.HasPrefix(authHeader, "Bearer "))

		if authHeader != "" {
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
				log.Debugw("Bearer token extracted",
					"token_length", len(token),
					"token_format", func() string {
						parts := strings.Split(token, ".")
						return fmt.Sprintf("parts: %d", len(parts))
					}())
			}
		}

		if token == "" {
			log.Warn("No token provided in request")

			// Check if this is a WebSocket upgrade request
			isWebSocketUpgrade := strings.ToLower(c.GetHeader("Connection")) == "upgrade" &&
				strings.ToLower(c.GetHeader("Upgrade")) == "websocket"

			// For WebSocket connections, we'll check if there's a token in the query parameters
			if isWebSocketUpgrade {
				// Try to get token from query parameters for WebSocket connections
				tokenFromQuery := c.Query("token")
				if tokenFromQuery != "" {
					log.Debugw("Found token in query parameters for WebSocket connection",
						"token_length", len(tokenFromQuery))
					token = tokenFromQuery
				} else {
					log.Warnw("No token in query parameters for WebSocket connection",
						"path", c.Request.URL.Path)
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
						"error": "Authorization required",
					})
					return
				}
			} else {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "Authorization required",
				})
				return
			}
		}

		// Validate JWT token
		log.Debugw("Starting JWT validation",
			"token_length", len(token),
			"request_path", c.Request.URL.Path)

		userID, err := validateJWT(token)
		if err != nil {
			// Enhanced error logging
			log.Warnw("Invalid JWT token",
				"error", err,
				"token_length", len(token),
				"request_path", c.Request.URL.Path,
				"request_method", c.Request.Method,
				"client_ip", c.ClientIP())

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
				"token_length", len(token))
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Authentication system error",
			})
			return
		}

		log.Debugw("Authentication successful",
			"userID", userID,
			"path", c.Request.URL.Path)
		c.Set("user_id", userID)
		c.Next()
	}
}

// Modify your JWT validation logic
func validateJWT(tokenString string) (string, error) {
	log := logger.GetLogger()

	// First parse without verification to inspect the token
	tokenObj, err := jwt.Parse([]byte(tokenString), jwt.WithVerify(false))
	if err != nil {
		log.Errorw("Failed to parse token without verification", "error", err)
		return "", err
	}

	// Log detailed token information
	log.Debugw("JWT header inspection",
		"alg", tokenObj.PrivateClaims()["alg"],
		"kid", tokenObj.PrivateClaims()["kid"],
		"typ", tokenObj.PrivateClaims()["typ"],
		"iss", tokenObj.Issuer(),
		"sub", tokenObj.Subject(),
		"aud", tokenObj.Audience(),
		"exp", tokenObj.Expiration(),
		"iat", tokenObj.IssuedAt())

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

	// Get the Supabase JWT secret from config
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Errorw("Failed to load config for JWT validation", "error", err)
		return "", fmt.Errorf("failed to load config for JWT validation: %w", err)
	}

	// Get the raw secret
	rawSecret := cfg.ExternalServices.SupabaseJWTSecret

	// Check for empty JWT secret
	if rawSecret == "" {
		log.Errorw("SUPABASE_JWT_SECRET is empty",
			"error", "JWT validation cannot proceed with empty secret")
		return "", fmt.Errorf("SUPABASE_JWT_SECRET environment variable is not set")
	}

	// Log JWT secret info (safely)
	log.Debugw("JWT secret info",
		"secret_length", len(rawSecret),
		"first_chars", func() string {
			if len(rawSecret) > 3 {
				return rawSecret[:3] + "..."
			}
			return ""
		}(),
		"is_base64", func() bool {
			_, err := base64.StdEncoding.DecodeString(rawSecret)
			return err == nil
		}())

	// Try multiple approaches to validate the token
	var validationErr error
	var validToken jwt.Token

	// Approach 1: Try with raw secret
	log.Debug("Attempting to verify token with raw JWT secret")
	validToken, err = jwt.Parse([]byte(tokenString),
		jwt.WithVerify(true),
		jwt.WithKey(jwa.HS256, []byte(rawSecret)),
		jwt.WithValidate(true),
		jwt.WithAcceptableSkew(30*time.Second),
	)

	if err == nil {
		log.Debug("Token validation successful with raw secret")
		sub := validToken.Subject()
		if sub == "" {
			log.Error("Token validation failed: missing subject claim")
			return "", fmt.Errorf("missing subject claim in token")
		}
		return sub, nil
	}

	validationErr = err
	log.Debugw("Token validation with raw secret failed", "error", err)

	// Approach 2: Try with base64 decoded secret (if it looks like base64)
	if isBase64(rawSecret) {
		decodedSecret, err := base64.StdEncoding.DecodeString(rawSecret)
		if err == nil {
			log.Debug("Attempting to verify token with base64 decoded JWT secret")
			validToken, err = jwt.Parse([]byte(tokenString),
				jwt.WithVerify(true),
				jwt.WithKey(jwa.HS256, decodedSecret),
				jwt.WithValidate(true),
				jwt.WithAcceptableSkew(30*time.Second),
			)

			if err == nil {
				log.Debug("Token validation successful with decoded secret")
				sub := validToken.Subject()
				if sub == "" {
					log.Error("Token validation failed: missing subject claim")
					return "", fmt.Errorf("missing subject claim in token")
				}
				return sub, nil
			}

			log.Debugw("Token validation with decoded secret failed", "error", err)
		}
	}

	// Approach 3: Try with URL-safe base64 decoded secret
	decodedSecret, err := base64.RawURLEncoding.DecodeString(rawSecret)
	if err == nil {
		log.Debug("Attempting to verify token with URL-safe base64 decoded JWT secret")
		validToken, err = jwt.Parse([]byte(tokenString),
			jwt.WithVerify(true),
			jwt.WithKey(jwa.HS256, decodedSecret),
			jwt.WithValidate(true),
			jwt.WithAcceptableSkew(30*time.Second),
		)

		if err == nil {
			log.Debug("Token validation successful with URL-safe decoded secret")
			sub := validToken.Subject()
			if sub == "" {
				log.Error("Token validation failed: missing subject claim")
				return "", fmt.Errorf("missing subject claim in token")
			}
			return sub, nil
		}

		log.Debugw("Token validation with URL-safe decoded secret failed", "error", err)
	}

	// If all approaches failed, return the original error
	log.Errorw("All token validation approaches failed",
		"error", validationErr,
		"error_type", fmt.Sprintf("%T", validationErr),
		"token_length", len(tokenString))

	return "", validationErr
}

// Helper function to check if a string is base64 encoded
func isBase64(s string) bool {
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
}

// nolint:unused
func maskToken(token string) string {
	return logger.MaskJWT(token)
}

// nolint:unused
func getJWTClaims(token string) interface{} {
	// Parse the token without validation to extract claims
	tokenObj, err := jwt.Parse([]byte(token), jwt.WithVerify(false))
	if err != nil {
		return map[string]interface{}{
			"error": "failed to parse token",
		}
	}

	// Build a map with only non-sensitive claims
	claims := make(map[string]interface{})

	// Add only non-sensitive standard claims
	if sub := tokenObj.Subject(); sub != "" {
		claims["sub"] = logger.MaskSensitiveString(sub, 3, 3)
	}
	claims["iss"] = tokenObj.Issuer()
	if !tokenObj.Expiration().IsZero() {
		claims["exp"] = tokenObj.Expiration().Unix()
	}
	if !tokenObj.IssuedAt().IsZero() {
		claims["iat"] = tokenObj.IssuedAt().Unix()
	}

	// Don't include private claims as they might contain sensitive information

	return claims
}

// Helper function to mask potentially sensitive string values
// nolint:unused
func maskString(s string) string {
	return logger.MaskSensitiveString(s, 3, 3)
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
