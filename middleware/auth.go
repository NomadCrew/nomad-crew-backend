package middleware

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
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

	log.Debugw("Refreshing JWKS cache", "url", c.jwksURL)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Create a new request to add headers
	req, err := http.NewRequest("GET", c.jwksURL, nil)
	if err != nil {
		log.Errorw("Failed to create request", "error", err, "url", c.jwksURL)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Get the Supabase anon key from config
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Errorw("Failed to load config for JWKS fetch", "error", err)
		return nil, fmt.Errorf("failed to load config for JWKS fetch: %w", err)
	}

	// Add the API key header
	anonKey := cfg.ExternalServices.SupabaseAnonKey
	if anonKey == "" {
		log.Errorw("SUPABASE_ANON_KEY is empty", "error", "JWKS fetch cannot proceed")
		return nil, fmt.Errorf("SUPABASE_ANON_KEY environment variable is not set")
	}

	req.Header.Add("apikey", anonKey)
	log.Debugw("Added API key to JWKS request", "key_length", len(anonKey))

	// Fetch the JWKS
	resp, err := client.Do(req)
	if err != nil {
		log.Errorw("Failed to fetch JWKS", "error", err, "url", c.jwksURL)
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Errorw("JWKS endpoint returned non-200 status",
			"status", resp.StatusCode,
			"url", c.jwksURL)
		return nil, fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	// Parse the JWKS response
	var jwksResp struct {
		Keys []jwk.Key `json:"keys"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&jwksResp); err != nil {
		log.Errorw("Failed to decode JWKS response", "error", err)
		return nil, fmt.Errorf("failed to decode JWKS response: %w", err)
	}

	// Update cache with new keys
	newKeys := make(map[string]jwk.Key)
	var targetKey jwk.Key

	for _, key := range jwksResp.Keys {
		kid := key.KeyID()
		newKeys[kid] = key

		if kid == targetKid {
			targetKey = key
		}
	}

	// Update the cache
	c.keys = newKeys
	c.expiresAt = time.Now().Add(c.ttl)

	// Check if we found the target key
	if targetKey == nil {
		log.Errorw("No matching key found in JWKS", "kid", targetKid)
		return nil, fmt.Errorf("no matching key found in JWKS for kid: %s", targetKid)
	}

	log.Debugw("JWKS cache refreshed successfully", "key_count", len(newKeys))
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

	// Get the Supabase configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Errorw("Failed to load config for JWT validation", "error", err)
		return "", fmt.Errorf("failed to load config for JWT validation: %w", err)
	}

	// Extract the kid from the token header
	kid, ok := tokenObj.PrivateClaims()["kid"].(string)
	if !ok {
		log.Debugw("No kid found in token header, trying to validate with static secret")

		// Fallback to static secret-based verification (for backward compatibility)
		rawSecret := cfg.ExternalServices.SupabaseJWTSecret

		// Check for empty JWT secret
		if rawSecret == "" {
			log.Errorw("SUPABASE_JWT_SECRET is empty",
				"error", "JWT validation cannot proceed with empty secret")
			return "", fmt.Errorf("SUPABASE_JWT_SECRET environment variable is not set")
		}

		// Try with raw secret
		validToken, err := jwt.Parse([]byte(tokenString),
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

		// Try with base64 decoded secret (if it looks like base64)
		if isBase64(rawSecret) {
			decodedSecret, err := base64.StdEncoding.DecodeString(rawSecret)
			if err == nil {
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
			}
		}

		// Try with URL-safe base64 decoded secret
		decodedSecret, err := base64.RawURLEncoding.DecodeString(rawSecret)
		if err == nil {
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
		}

		log.Errorw("All static secret validation approaches failed", "error", err)
		return "", fmt.Errorf("failed to validate token with static secret: %w", err)
	}

	// If kid is present, fetch and use the corresponding JWK
	supabaseURL := cfg.ExternalServices.SupabaseURL
	if supabaseURL == "" {
		log.Error("SUPABASE_URL is not configured")
		return "", fmt.Errorf("SUPABASE_URL is not configured")
	}

	// Build the JWKS URL
	jwksURL := fmt.Sprintf("%s/auth/v1/jwks", strings.TrimSuffix(supabaseURL, "/"))

	// Get the key from cache or fetch from JWKS endpoint
	cache := GetJWKSCache(jwksURL)
	matchingKey, err := cache.GetKey(kid)
	if err != nil {
		log.Errorw("Failed to get key from JWKS", "error", err, "kid", kid)
		return "", err
	}

	// Get the algorithm from the token header
	algStr, ok := tokenObj.PrivateClaims()["alg"].(string)
	if !ok {
		log.Errorw("No alg found in token header", "kid", kid)
		return "", fmt.Errorf("no algorithm specified in token header")
	}

	// Map the string algorithm to jwa algorithm
	var alg jwa.SignatureAlgorithm
	switch algStr {
	case "HS256":
		alg = jwa.HS256
	case "HS384":
		alg = jwa.HS384
	case "HS512":
		alg = jwa.HS512
	case "RS256":
		alg = jwa.RS256
	case "RS384":
		alg = jwa.RS384
	case "RS512":
		alg = jwa.RS512
	default:
		log.Errorw("Unsupported algorithm in token", "alg", algStr)
		return "", fmt.Errorf("unsupported algorithm in token: %s", algStr)
	}

	log.Debugw("Using algorithm for verification", "alg", alg)

	// Verify the token with the matching key
	validToken, err := jwt.Parse([]byte(tokenString),
		jwt.WithVerify(true),
		jwt.WithKey(alg, matchingKey),
		jwt.WithValidate(true),
		jwt.WithAcceptableSkew(30*time.Second),
	)

	if err != nil {
		log.Errorw("Token validation failed with JWKS key", "error", err, "kid", kid)
		return "", fmt.Errorf("token validation failed with JWKS key: %w", err)
	}

	log.Debug("Token validation successful with JWKS key")
	sub := validToken.Subject()
	if sub == "" {
		log.Error("Token validation failed: missing subject claim")
		return "", fmt.Errorf("missing subject claim in token")
	}
	return sub, nil
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
