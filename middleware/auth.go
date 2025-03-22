package middleware

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
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

// JWKSCache is a cache for JWKS keys to avoid fetching on every request
type JWKSCache struct {
	keys      map[string]jwk.Key // kid -> key mapping
	expiresAt time.Time
	mutex     sync.RWMutex
	jwksURL   string
	ttl       time.Duration
}

var jwksCache *JWKSCache
var jwksCacheOnce sync.Once

// GetJWKSCache returns a singleton instance of the JWKS cache
func GetJWKSCache(jwksURL string) *JWKSCache {
	jwksCacheOnce.Do(func() {
		jwksCache = &JWKSCache{
			keys:      make(map[string]jwk.Key),
			expiresAt: time.Now(),
			jwksURL:   jwksURL,
			ttl:       15 * time.Minute, // Cache keys for 15 minutes
		}
	})

	// If URL changed, update it
	if jwksCache.jwksURL != jwksURL {
		jwksCache.mutex.Lock()
		jwksCache.jwksURL = jwksURL
		jwksCache.mutex.Unlock()
	}

	return jwksCache
}

// GetKey returns a key by its ID, fetching from remote if needed
func (c *JWKSCache) GetKey(kid string) (jwk.Key, error) {
	log := logger.GetLogger()

	// Check if key is in cache
	c.mutex.RLock()
	if key, ok := c.keys[kid]; ok && time.Now().Before(c.expiresAt) {
		c.mutex.RUnlock()
		log.Debugw("Using cached JWKS key", "kid", kid)
		return key, nil
	}
	c.mutex.RUnlock()

	// Need to refresh the cache
	return c.refreshCache(kid)
}

// refreshCache fetches the latest keys from the JWKS endpoint
func (c *JWKSCache) refreshCache(targetKid string) (jwk.Key, error) {
	log := logger.GetLogger()

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Double-check if another goroutine has already refreshed the cache
	if key, ok := c.keys[targetKid]; ok && time.Now().Before(c.expiresAt) {
		log.Debugw("Key found in cache after lock", "kid", targetKid)
		return key, nil
	}

	log.Infow("Refreshing JWKS cache",
		"url", c.jwksURL,
		"target_kid", targetKid,
		"cached_keys_count", len(c.keys))

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Create a new request to add headers
	req, err := http.NewRequest("GET", c.jwksURL, nil)
	if err != nil {
		log.Errorw("Failed to create request",
			"error", err,
			"error_type", fmt.Sprintf("%T", err),
			"url", c.jwksURL)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Get the Supabase configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Errorw("Failed to load config for JWKS fetch",
			"error", err,
			"error_details", fmt.Sprintf("%+v", err))
		return nil, fmt.Errorf("failed to load config for JWKS fetch: %w", err)
	}

	// Add the API key header
	anonKey := cfg.ExternalServices.SupabaseAnonKey
	if anonKey == "" {
		log.Errorw("SUPABASE_ANON_KEY is empty", "error", "JWKS fetch cannot proceed")
		return nil, fmt.Errorf("SUPABASE_ANON_KEY environment variable is not set")
	}

	// Enhanced logging for Supabase configuration
	log.Infow("Supabase configuration",
		"supabase_url", cfg.ExternalServices.SupabaseURL,
		"anon_key_present", anonKey != "",
		"anon_key_length", len(anonKey),
		"jwks_url", c.jwksURL)

	// Normalize the anon key to ensure it doesn't have any problematic characters for headers
	// Use two approaches to maximize compatibility with different Supabase configurations

	// Approach 1: Use the "apikey" header (Supabase standard)
	req.Header.Set("apikey", anonKey) // Use Set instead of Add to avoid duplicates

	// Approach 2: Also use the "Authorization: Bearer" format as a fallback
	req.Header.Set("Authorization", "Bearer "+anonKey)

	log.Infow("Added API key headers to JWKS request",
		"key_length", len(anonKey),
		"key_first_chars", func() string {
			if len(anonKey) > 5 {
				return anonKey[:5] + "..."
			}
			return ""
		}(),
		"headers_used", []string{"apikey", "Authorization"})

	// Log request details for debugging
	log.Debugw("JWKS request details",
		"method", req.Method,
		"url", req.URL.String(),
		"headers", func() map[string]string {
			headers := make(map[string]string)
			for k, v := range req.Header {
				if k != "Authorization" { // Skip sensitive headers
					headers[k] = strings.Join(v, ",")
				}
			}
			return headers
		}())

	// Fetch the JWKS
	log.Infow("Sending JWKS request now", "url", c.jwksURL)
	resp, err := client.Do(req)
	if err != nil {
		log.Errorw("Failed to fetch JWKS",
			"error", err,
			"error_type", fmt.Sprintf("%T", err),
			"error_details", err.Error(),
			"url", c.jwksURL)
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	log.Infow("JWKS response received",
		"status", resp.StatusCode,
		"status_text", resp.Status,
		"content_type", resp.Header.Get("Content-Type"),
		"content_length", resp.ContentLength)

	if resp.StatusCode != http.StatusOK {
		// Try to read response body for more details
		bodyBytes, _ := io.ReadAll(resp.Body)
		responseBody := string(bodyBytes)

		log.Errorw("JWKS endpoint returned non-200 status",
			"status", resp.StatusCode,
			"url", c.jwksURL,
			"response_body", func() string {
				if len(responseBody) > 500 {
					return responseBody[:500] + "..." // Truncate long responses
				}
				return responseBody
			}())
		return nil, fmt.Errorf("JWKS endpoint returned status %d: %s", resp.StatusCode, responseBody)
	}

	// Parse the JWKS response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorw("Failed to read JWKS response body",
			"error", err,
			"error_details", err.Error())
		return nil, fmt.Errorf("failed to read JWKS response body: %w", err)
	}

	// Log response body for debugging (but truncate if too long)
	log.Debugw("JWKS response body",
		"body", func() string {
			if len(bodyBytes) > 500 {
				return string(bodyBytes[:500]) + "..." // Truncate long responses
			}
			return string(bodyBytes)
		}())

	var jwksResp struct {
		Keys []jwk.Key `json:"keys"`
	}

	if err := json.Unmarshal(bodyBytes, &jwksResp); err != nil {
		log.Errorw("Failed to decode JWKS response",
			"error", err,
			"error_details", err.Error(),
			"response_body_length", len(bodyBytes))
		return nil, fmt.Errorf("failed to decode JWKS response: %w", err)
	}

	// Update cache with new keys
	newKeys := make(map[string]jwk.Key)
	var targetKey jwk.Key

	log.Infow("JWKS keys received",
		"key_count", len(jwksResp.Keys),
		"keys", func() []string {
			var kids []string
			for _, k := range jwksResp.Keys {
				kids = append(kids, k.KeyID())
			}
			return kids
		}())

	for _, key := range jwksResp.Keys {
		kid := key.KeyID()
		newKeys[kid] = key

		log.Debugw("JWKS key details",
			"kid", kid,
			"alg", key.Algorithm(),
			"key_type", fmt.Sprintf("%T", key))

		if kid == targetKid {
			targetKey = key
			log.Infow("Found matching key in JWKS response", "kid", kid)
		}
	}

	// Update the cache
	c.keys = newKeys
	c.expiresAt = time.Now().Add(c.ttl)

	// Check if we found the target key
	if targetKey == nil {
		log.Errorw("No matching key found in JWKS",
			"kid", targetKid,
			"available_kids", func() []string {
				var kids []string
				for k := range newKeys {
					kids = append(kids, k)
				}
				return kids
			}())
		return nil, fmt.Errorf("no matching key found in JWKS for kid: %s", targetKid)
	}

	log.Infow("JWKS cache refreshed successfully",
		"key_count", len(newKeys),
		"target_kid_found", targetKey != nil,
		"cache_expires_at", c.expiresAt)
	return targetKey, nil
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

// tryHS256Validation attempts to validate a token using HS256 with the static secret
func tryHS256Validation(tokenString string) (string, error) {
	log := logger.GetLogger()

	// Get the Supabase configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}

	// Check for empty JWT secret
	if cfg.ExternalServices.SupabaseJWTSecret == "" {
		return "", fmt.Errorf("SUPABASE_JWT_SECRET environment variable is not set")
	}

	// Parse and validate the token using static secret
	token, err := jwt.Parse([]byte(tokenString),
		jwt.WithVerify(true),
		jwt.WithKey(jwa.HS256, []byte(cfg.ExternalServices.SupabaseJWTSecret)),
	)
	if err != nil {
		return "", fmt.Errorf("HS256 validation failed: %w", err)
	}

	// Get the subject (user ID)
	sub := token.Subject()
	if sub == "" {
		return "", fmt.Errorf("missing subject claim in token")
	}

	log.Infow("Static secret (HS256) token validation successful")
	return sub, nil
}

// tryJWKSValidation attempts to validate a token using JWKS
func tryJWKSValidation(tokenString string, kidValue string, algValue string) (string, error) {
	log := logger.GetLogger()

	// Get the Supabase configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}

	// Check for Supabase URL
	if cfg.ExternalServices.SupabaseURL == "" {
		return "", fmt.Errorf("SUPABASE_URL environment variable is not set")
	}

	// Form JWKS URL
	jwksURL := fmt.Sprintf("%s/auth/v1/jwks", cfg.ExternalServices.SupabaseURL)
	log.Infow("Attempting JWKS validation",
		"jwks_url", jwksURL,
		"kid", kidValue,
		"alg", algValue)

	// Create JWKS cache instance
	jwksCache := GetJWKSCache(jwksURL)

	// Try to fetch the key
	key, err := jwksCache.GetKey(kidValue)
	if err != nil || key == nil {
		return "", fmt.Errorf("failed to get key from JWKS: %w", err)
	}

	// Try to validate with RS256 (most common)
	token, err := jwt.Parse([]byte(tokenString),
		jwt.WithVerify(true),
		jwt.WithKey(jwa.RS256, key),
	)
	if err != nil {
		return "", fmt.Errorf("RS256 validation failed: %w", err)
	}

	// Get the subject
	sub := token.Subject()
	if sub == "" {
		return "", fmt.Errorf("missing subject claim in token")
	}

	log.Infow("JWKS (RS256) token validation successful")
	return sub, nil
}

// validateJWT validates a JWT token and returns the subject (user ID)
func validateJWT(tokenString string) (string, error) {
	log := logger.GetLogger()

	// Get the Supabase configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Errorw("Failed to load config for JWT validation",
			"error", err,
			"error_details", fmt.Sprintf("%+v", err))
		return "", fmt.Errorf("failed to load config for JWT validation: %w", err)
	}

	// Check for empty JWT secret
	if cfg.ExternalServices.SupabaseJWTSecret == "" {
		log.Errorw("SUPABASE_JWT_SECRET is empty",
			"error", "JWT validation cannot proceed with empty secret")
		return "", fmt.Errorf("SUPABASE_JWT_SECRET environment variable is not set")
	}

	// First, try to extract the kid from token header by manual parsing
	var kidValue string
	var algValue string
	parts := strings.Split(tokenString, ".")
	if len(parts) == 3 {
		headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
		if err == nil {
			var headerMap map[string]interface{}
			if json.Unmarshal(headerBytes, &headerMap) == nil {
				// Extract kid if present
				if kid, ok := headerMap["kid"].(string); ok && kid != "" {
					kidValue = kid
					log.Infow("JWT token contains kid claim", "kid", kidValue)
				}

				// Extract algorithm if present
				if alg, ok := headerMap["alg"].(string); ok && alg != "" {
					algValue = alg
					log.Infow("JWT token uses algorithm", "alg", algValue)
				}
			}
		}
	}

	// Enhanced debug logging for token information
	log.Infow("Token debugging information",
		"token_algorithm", algValue,
		"token_kid", kidValue,
		"token_format", fmt.Sprintf("%d parts", len(parts)),
		"token_length", len(tokenString))

	// WORKAROUND: Try HS256 validation first (static secret)
	staticUserID, staticErr := tryHS256Validation(tokenString)
	if staticErr == nil {
		// Static secret validation succeeded
		return staticUserID, nil
	}

	log.Infow("Static secret validation failed, trying JWKS next",
		"error", staticErr)

	// If token has a kid claim, try JWKS validation
	if kidValue != "" {
		jwksUserID, jwksErr := tryJWKSValidation(tokenString, kidValue, algValue)
		if jwksErr == nil {
			// JWKS validation succeeded
			return jwksUserID, nil
		}

		log.Errorw("Both validation methods failed",
			"static_error", staticErr,
			"jwks_error", jwksErr)

		return "", fmt.Errorf("token validation failed with both methods: %w", jwksErr)
	}

	// If we got here, static validation failed and there was no kid
	return "", fmt.Errorf("failed to validate token: %w", staticErr)
}

// Helper function to check if a string is base64 encoded
func isBase64(s string) bool {
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
}

// IsBase64 checks if a string is base64 encoded (exported version)
func IsBase64(s string) bool {
	return isBase64(s)
}

// DecodeBase64 decodes a base64-encoded string
func DecodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
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
