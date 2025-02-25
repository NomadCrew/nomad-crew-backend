package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// Add at package level
type contextKey string

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

			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "Invalid authentication token",
				"details": err.Error(),
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

// validateSupabaseToken parses the token without signature verification and without
// automatic claim validation, then performs manual checks for required claims.
func validateSupabaseToken(tokenString, secret string) (*SupabaseClaims, error) {
	token, err := jwt.Parse([]byte(tokenString),
		jwt.WithVerify(true),
		jwt.WithKey(jwa.HS256, []byte(secret)),
		jwt.WithAcceptableSkew(30*time.Second),
		jwt.WithValidator(jwt.ValidatorFunc(func(_ context.Context, t jwt.Token) jwt.ValidationError {
			if time.Now().Add(-30 * time.Second).Before(t.Expiration()) {
				return nil
			}
			return jwt.ErrTokenExpired()
		})),
	)

	if err != nil {
		return nil, fmt.Errorf("invalid token: %v", err)
	}

	// Map standard claims
	claims := &SupabaseClaims{
		Subject: token.Subject(),
		Exp:     token.Expiration().Unix(),
	}

	// Extract custom claims
	if email, ok := token.PrivateClaims()["email"].(string); ok {
		claims.Email = email
	}
	if role, ok := token.PrivateClaims()["role"].(string); ok {
		claims.Role = role
	}
	if metadata, ok := token.PrivateClaims()["user_metadata"].(map[string]interface{}); ok {
		claims.UserMetadata = types.UserMetadata{
			Username:       getStringValue(metadata, "username"),
			FirstName:      getStringValue(metadata, "firstName"),
			LastName:       getStringValue(metadata, "lastName"),
			ProfilePicture: getStringValue(metadata, "avatar_url"),
		}
	}

	return claims, nil
}

// getStringValue safely extracts a string value from a map.
func getStringValue(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

// Modify your JWT validation logic
func validateJWT(tokenString string) (string, error) {
	log := logger.GetLogger()

	// First parse without verification to inspect the token
	tokenObj, err := jwt.Parse([]byte(tokenString), jwt.WithVerify(false))
	if err == nil {
		log.Debugw("JWT header inspection", "header", tokenObj.PrivateClaims())
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

func maskSecret(secret string) string {
	if len(secret) < 8 {
		return "***"
	}
	return secret[:2] + "***" + secret[len(secret)-2:]
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

func validateToken(token string) bool {
	if token == "" {
		return false
	}

	// Reuse existing JWT validation
	_, err := validateJWT(token)
	if err != nil {
		return false
	}

	// Additional check for token expiration
	claims, ok := getJWTClaims(token).(map[string]interface{})
	if !ok {
		return false
	}

	exp, ok := claims["exp"].(float64)
	if !ok {
		return false
	}

	return time.Now().Unix() < int64(exp)
}
