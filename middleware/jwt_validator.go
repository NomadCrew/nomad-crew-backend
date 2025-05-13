package middleware

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config" // Use project's custom errors
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

var (
	// ErrTokenExpired is returned when JWT validation fails due to expiry.
	ErrTokenExpired = errors.New("token expired")
	// ErrTokenInvalid is returned for general token validation failures (signature, format).
	ErrTokenInvalid = errors.New("token invalid")
	// ErrTokenMissingClaim is returned if a required claim (like 'sub') is missing.
	ErrTokenMissingClaim = errors.New("token missing required claim")
	// ErrValidationMethodUnavailable is returned if neither HS256 nor JWKS can be attempted.
	ErrValidationMethodUnavailable = errors.New("no validation method available for token")
	// ErrJWKSKeyNotFound is returned if the key specified by 'kid' is not found in JWKS.
	ErrJWKSKeyNotFound = errors.New("jwks key not found")
)

// Validator defines the interface for validating tokens.
type Validator interface {
	Validate(tokenString string) (string, error)
	// Add other methods used by consumers like AuthMiddleware if any
}

// JWTValidator encapsulates JWT validation logic using static secrets and JWKS.
type JWTValidator struct {
	jwksCache    *JWKSCache
	staticSecret []byte
	// Add clock skew tolerance if needed: validationClockSkew time.Duration
}

// Ensure JWTValidator implements the Validator interface
var _ Validator = (*JWTValidator)(nil)

// NewJWTValidator creates a validator instance using application configuration.
func NewJWTValidator(cfg *config.Config) (Validator, error) {
	log := logger.GetLogger()
	var staticSecret []byte
	var jwksCache *JWKSCache

	// Configure static secret (HS256) if provided
	if cfg.ExternalServices.SupabaseJWTSecret != "" {
		staticSecret = []byte(cfg.ExternalServices.SupabaseJWTSecret)
		log.Info("JWT Validator: HS256 validation enabled.")
	} else {
		log.Warn("JWT Validator: SUPABASE_JWT_SECRET not set, HS256 validation disabled.")
	}

	// Configure JWKS if URL and Anon key are provided
	if cfg.ExternalServices.SupabaseURL != "" && cfg.ExternalServices.SupabaseAnonKey != "" {
		jwksURL := fmt.Sprintf("%s/auth/v1/jwks", cfg.ExternalServices.SupabaseURL)
		// Default TTL can be configurable too, e.g., via cfg.Server.JWKSCacheTTL
		jwksCache = GetJWKSCache(jwksURL, cfg.ExternalServices.SupabaseAnonKey, 15*time.Minute)
		log.Infow("JWT Validator: JWKS validation enabled.", "url", jwksURL)
	} else {
		log.Warn("JWT Validator: SUPABASE_URL or SUPABASE_ANON_KEY not set, JWKS validation disabled.")
	}

	if staticSecret == nil && jwksCache == nil {
		return nil, fmt.Errorf("JWT validator configuration error: At least one validation method (HS256 Secret or JWKS URL+Key) must be configured")
	}

	validator := &JWTValidator{
		jwksCache:    jwksCache,
		staticSecret: staticSecret,
	}

	// Return the concrete type satisfying the interface
	return validator, nil
}

// Validate parses and validates the token using configured methods.
// It tries HS256 first (if configured), then JWKS (if configured and 'kid' is present).
// Returns userID (subject claim) and a specific error (ErrTokenExpired, ErrTokenInvalid, etc.).
func (v *JWTValidator) Validate(tokenString string) (string, error) {
	log := logger.GetLogger()

	// 1. Try HS256 validation if secret is available
	var staticErr error
	if len(v.staticSecret) > 0 {
		userID, err := v.validateHS256(tokenString)
		if err == nil {
			log.Debug("JWT validation successful using HS256")
			return userID, nil // Success!
		}
		// Don't return ErrTokenExpired immediately from HS256 if JWKS is also possible
		if !errors.Is(err, ErrTokenExpired) {
			log.Debugw("HS256 validation failed (non-expiry)", "error", err)
		}
		staticErr = err // Store HS256 error
	}

	// 2. Try JWKS validation if cache is available and token has 'kid'
	var jwksErr error
	if v.jwksCache != nil {
		// Extract kid without full validation first
		kid, alg, err := v.extractKIDAndAlg(tokenString)
		if err != nil {
			log.Warnw("Could not extract kid/alg from token header for JWKS check", "error", err)
			// If HS256 also failed, return its error or a generic invalid token error
			if staticErr != nil {
				return "", staticErr // Return the HS256 error if it occurred
			}
			return "", fmt.Errorf("%w: %w", ErrTokenInvalid, err) // Failed to parse header
		}

		if kid != "" {
			log.Debugw("Attempting JWKS validation", "kid", kid, "alg", alg)
			userID, err := v.validateJWKS(tokenString, kid, alg)
			if err == nil {
				log.Debug("JWT validation successful using JWKS")
				return userID, nil // Success!
			}
			jwksErr = err // Store JWKS error
			log.Debugw("JWKS validation failed", "kid", kid, "error", jwksErr)
		} else {
			log.Debug("No 'kid' found in token header, skipping JWKS validation.")
			jwksErr = errors.New("no kid in header for jwks") // Mark as skipped
		}
	}

	// 3. Determine final outcome
	log.Warnw("All JWT validation methods failed or were skipped", "static_error", staticErr, "jwks_error", jwksErr)

	// Prioritize returning expiry error if either method reported it
	if errors.Is(staticErr, ErrTokenExpired) || errors.Is(jwksErr, ErrTokenExpired) {
		return "", ErrTokenExpired
	}
	// Prioritize JWKS key not found error
	if errors.Is(jwksErr, ErrJWKSKeyNotFound) {
		return "", fmt.Errorf("%w: %s", ErrJWKSKeyNotFound, jwksErr)
	}

	// If HS256 was tried and failed (non-expiry), return that error
	if staticErr != nil {
		return "", fmt.Errorf("%w: %w", ErrTokenInvalid, staticErr)
	}
	// If JWKS was tried (and had kid) and failed (non-expiry, non-key-not-found), return that
	if jwksErr != nil && !errors.Is(jwksErr, errors.New("no kid in header for jwks")) {
		return "", fmt.Errorf("%w: %w", ErrTokenInvalid, jwksErr)
	}

	// If only one method was configured and failed, we already returned above.
	// If both were configured but JWKS was skipped due to no kid, and HS256 failed: already returned staticErr.
	// If neither method was configured, constructor should have failed.
	// Fallback / Should not happen state:
	return "", ErrValidationMethodUnavailable
}

// extractKIDAndAlg parses the JWT header without validation to get key ID and algorithm.
func (v *JWTValidator) extractKIDAndAlg(tokenString string) (kid string, alg string, err error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return "", "", fmt.Errorf("invalid token format, expected 3 parts, got %d", len(parts))
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", "", fmt.Errorf("failed to decode token header: %w", err)
	}

	var headerMap map[string]interface{}
	if err := json.Unmarshal(headerBytes, &headerMap); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal token header JSON: %w", err)
	}

	if k, ok := headerMap["kid"].(string); ok {
		kid = k
	}
	if a, ok := headerMap["alg"].(string); ok {
		alg = a
	}
	return kid, alg, nil
}

// validateHS256 attempts validation using the static secret.
func (v *JWTValidator) validateHS256(tokenString string) (string, error) {
	token, err := jwt.Parse([]byte(tokenString),
		jwt.WithKey(jwa.HS256, v.staticSecret),
		jwt.WithValidate(true), // Enable clock skew etc.
		// jwt.WithClock(jwt.ClockFunc(time.Now)), // Default clock
		// jwt.WithAcceptableSkew(1*time.Minute), // Example: Add skew if needed
	)
	if err != nil {
		// Check for expiration explicitly
		if errors.Is(err, jwt.ErrTokenExpired()) {
			return "", fmt.Errorf("%w: %w", ErrTokenExpired, err)
		}
		return "", fmt.Errorf("hs256 parse/validation failed: %w", err)
	}

	sub := token.Subject()
	if sub == "" {
		return "", ErrTokenMissingClaim // Missing subject claim
	}
	return sub, nil
}

// validateJWKS attempts validation using a key fetched from the JWKS cache.
func (v *JWTValidator) validateJWKS(tokenString string, kid string, alg string) (string, error) {
	log := logger.GetLogger()

	// Fetch key from cache
	key, err := v.jwksCache.GetKey(kid)
	if err != nil {
		// Distinguish key-not-found error
		if strings.Contains(err.Error(), "key with kid") || strings.Contains(err.Error(), "not found in JWKS") {
			return "", fmt.Errorf("%w: %w", ErrJWKSKeyNotFound, err)
		}
		return "", fmt.Errorf("failed to get key '%s' from jwks cache: %w", kid, err)
	}
	if key == nil { // Should be caught by err check above, but defensive check
		return "", fmt.Errorf("%w: key '%s' is nil", ErrJWKSKeyNotFound, kid)
	}

	parseOptions := []jwt.ParseOption{
		jwt.WithKey(key.Algorithm(), key),
		jwt.WithValidate(true),
		// jwt.WithAcceptableSkew(1*time.Minute),
	}

	keyAlg := key.Algorithm()
	headerAlg := jwa.SignatureAlgorithm(alg)

	// Compare string representations directly in the 'if' condition
	if alg != "" && keyAlg != jwa.NoSignature && headerAlg.String() != keyAlg.String() { // Compare strings
		log.Warnw("Token 'alg' header mismatches JWK algorithm",
			"header_alg", headerAlg.String(),
			"key_alg", keyAlg.String(),
			"kid", kid)
	}

	// Parse and validate
	token, err := jwt.Parse([]byte(tokenString), parseOptions...)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired()) {
			return "", fmt.Errorf("%w: %w", ErrTokenExpired, err)
		}
		return "", fmt.Errorf("jwks validation failed for kid '%s': %w", kid, err)
	}

	sub := token.Subject()
	if sub == "" {
		return "", ErrTokenMissingClaim // Missing subject claim
	}
	return sub, nil
}
