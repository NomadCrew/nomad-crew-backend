package middleware

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
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
	// ValidateAndGetClaims validates the token and returns full JWT claims (for onboarding)
	ValidateAndGetClaims(tokenString string) (*types.JWTClaims, error)
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
	var staticSecret []byte
	var jwksCache *JWKSCache

	// Configure static secret (HS256) if provided
	// NOTE: With new Supabase publishable/secret keys, JWKS is the primary validation method
	// HS256 is only used for legacy projects with the old JWT secret
	if cfg.ExternalServices.SupabaseJWTSecret != "" &&
		cfg.ExternalServices.SupabaseJWTSecret != "new-supabase-uses-jwks-validation-instead" {
		staticSecret = []byte(cfg.ExternalServices.SupabaseJWTSecret)
		logger.GetLogger().Infof("HS256 JWT secret configured (len=%d)",
			len(cfg.ExternalServices.SupabaseJWTSecret))
	} else {
		logger.GetLogger().Info("JWT Validator: Using JWKS validation (new Supabase API keys), HS256 disabled.")
	}

	// Configure JWKS if URL and Anon key are provided
	if cfg.ExternalServices.SupabaseURL != "" && cfg.ExternalServices.SupabaseAnonKey != "" {
		// Use the new Supabase JWKS endpoint format (.well-known/jwks.json)
		// This is required for projects using the new publishable/secret API keys
		jwksURL := fmt.Sprintf("%s/auth/v1/.well-known/jwks.json", cfg.ExternalServices.SupabaseURL)
		// Default TTL can be configurable too, e.g., via cfg.Server.JWKSCacheTTL
		jwksCache = GetJWKSCache(jwksURL, cfg.ExternalServices.SupabaseAnonKey, 15*time.Minute)
		logger.GetLogger().Infof("JWT Validator: JWKS validation enabled with URL: %s", jwksURL)
	} else {
		logger.GetLogger().Warn("JWT Validator: SUPABASE_URL or SUPABASE_ANON_KEY not set, JWKS validation disabled.")
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
		log.Debugw("Attempting HS256 validation")
		userID, err := v.validateHS256(tokenString)
		if err == nil {
			log.Debugw("HS256 validation successful", "userID", userID)
			return userID, nil // Success!
		}
		staticErr = err // Store HS256 error
		log.Debugw("HS256 validation failed", "error", err)
	}

	// 2. Try JWKS validation if cache is available and token has 'kid'
	var jwksErr error
	if v.jwksCache != nil {
		// Extract kid without full validation first
		kid, alg, err := v.extractKIDAndAlg(tokenString)
		log.Debugw("Extracted JWT header", "kid", kid, "alg", alg, "error", err)
		if err != nil {
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
				log.Debugw("JWKS validation successful", "userID", userID)
				return userID, nil // Success!
			}
			jwksErr = err // Store JWKS error
			log.Warnw("JWKS validation failed", "kid", kid, "error", err)
		} else {
			jwksErr = errors.New("no kid in header for jwks") // Mark as skipped
			log.Warnw("Token has no 'kid' in header, cannot use JWKS")
		}
	} else {
		log.Warnw("JWKS cache not configured")
	}

	// 3. Determine final outcome
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
	defer func() {
		if r := recover(); r != nil {
			logger.GetLogger().Errorw("Panic during HS256 JWT validation", "recover", r, "token", tokenString)
		}
	}()
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
		logger.GetLogger().Warnw("Token 'alg' header mismatches JWK algorithm",
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

// ValidateAndGetClaims validates the token and returns full JWT claims.
// This is used for onboarding where we need email and other claims beyond just userID.
func (v *JWTValidator) ValidateAndGetClaims(tokenString string) (*types.JWTClaims, error) {
	log := logger.GetLogger()

	// 1. Try HS256 validation if secret is available
	var staticErr error
	if len(v.staticSecret) > 0 {
		log.Debugw("ValidateAndGetClaims: Attempting HS256 validation")
		claims, err := v.validateHS256WithClaims(tokenString)
		if err == nil {
			log.Debugw("ValidateAndGetClaims: HS256 validation successful", "userID", claims.UserID)
			return claims, nil
		}
		staticErr = err
		log.Debugw("ValidateAndGetClaims: HS256 validation failed", "error", err)
	}

	// 2. Try JWKS validation if cache is available and token has 'kid'
	var jwksErr error
	if v.jwksCache != nil {
		kid, alg, err := v.extractKIDAndAlg(tokenString)
		log.Debugw("ValidateAndGetClaims: Extracted JWT header", "kid", kid, "alg", alg, "error", err)
		if err != nil {
			if staticErr != nil {
				return nil, staticErr
			}
			return nil, fmt.Errorf("%w: %w", ErrTokenInvalid, err)
		}

		if kid != "" {
			log.Debugw("ValidateAndGetClaims: Attempting JWKS validation", "kid", kid, "alg", alg)
			claims, err := v.validateJWKSWithClaims(tokenString, kid, alg)
			if err == nil {
				log.Debugw("ValidateAndGetClaims: JWKS validation successful", "userID", claims.UserID)
				return claims, nil
			}
			jwksErr = err
			log.Warnw("ValidateAndGetClaims: JWKS validation failed", "kid", kid, "error", err)
		} else {
			jwksErr = errors.New("no kid in header for jwks")
			log.Warnw("ValidateAndGetClaims: Token has no 'kid' in header, cannot use JWKS")
		}
	}

	// 3. Determine final outcome
	if errors.Is(staticErr, ErrTokenExpired) || errors.Is(jwksErr, ErrTokenExpired) {
		return nil, ErrTokenExpired
	}
	if errors.Is(jwksErr, ErrJWKSKeyNotFound) {
		return nil, fmt.Errorf("%w: %s", ErrJWKSKeyNotFound, jwksErr)
	}
	if staticErr != nil {
		return nil, fmt.Errorf("%w: %w", ErrTokenInvalid, staticErr)
	}
	if jwksErr != nil {
		return nil, fmt.Errorf("%w: %w", ErrTokenInvalid, jwksErr)
	}

	return nil, ErrValidationMethodUnavailable
}

// validateHS256WithClaims validates with HS256 and returns full claims.
func (v *JWTValidator) validateHS256WithClaims(tokenString string) (*types.JWTClaims, error) {
	defer func() {
		if r := recover(); r != nil {
			logger.GetLogger().Errorw("Panic during HS256 JWT validation with claims", "recover", r)
		}
	}()
	token, err := jwt.Parse([]byte(tokenString),
		jwt.WithKey(jwa.HS256, v.staticSecret),
		jwt.WithValidate(true),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired()) {
			return nil, fmt.Errorf("%w: %w", ErrTokenExpired, err)
		}
		return nil, fmt.Errorf("hs256 parse/validation failed: %w", err)
	}

	return extractClaimsFromToken(token)
}

// validateJWKSWithClaims validates with JWKS and returns full claims.
func (v *JWTValidator) validateJWKSWithClaims(tokenString string, kid string, alg string) (*types.JWTClaims, error) {
	key, err := v.jwksCache.GetKey(kid)
	if err != nil {
		if strings.Contains(err.Error(), "key with kid") || strings.Contains(err.Error(), "not found in JWKS") {
			return nil, fmt.Errorf("%w: %w", ErrJWKSKeyNotFound, err)
		}
		return nil, fmt.Errorf("failed to get key '%s' from jwks cache: %w", kid, err)
	}
	if key == nil {
		return nil, fmt.Errorf("%w: key '%s' is nil", ErrJWKSKeyNotFound, kid)
	}

	parseOptions := []jwt.ParseOption{
		jwt.WithKey(key.Algorithm(), key),
		jwt.WithValidate(true),
	}

	token, err := jwt.Parse([]byte(tokenString), parseOptions...)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired()) {
			return nil, fmt.Errorf("%w: %w", ErrTokenExpired, err)
		}
		return nil, fmt.Errorf("jwks validation failed for kid '%s': %w", kid, err)
	}

	return extractClaimsFromToken(token)
}

// extractClaimsFromToken extracts JWTClaims from a validated jwt.Token.
func extractClaimsFromToken(token jwt.Token) (*types.JWTClaims, error) {
	sub := token.Subject()
	if sub == "" {
		return nil, ErrTokenMissingClaim
	}

	claims := &types.JWTClaims{
		UserID: sub,
	}

	// Extract email from claims (Supabase stores it in the "email" claim)
	if emailVal, ok := token.Get("email"); ok {
		if email, ok := emailVal.(string); ok {
			claims.Email = email
		}
	}

	// Extract username if present (Supabase may store it in user_metadata or raw_user_meta_data)
	if usernameVal, ok := token.Get("username"); ok {
		if username, ok := usernameVal.(string); ok {
			claims.Username = username
		}
	}

	// Also check user_metadata for additional fields
	if userMetaVal, ok := token.Get("user_metadata"); ok {
		if userMeta, ok := userMetaVal.(map[string]interface{}); ok {
			if username, ok := userMeta["username"].(string); ok && claims.Username == "" {
				claims.Username = username
			}
			if email, ok := userMeta["email"].(string); ok && claims.Email == "" {
				claims.Email = email
			}
		}
	}

	logger.GetLogger().Debugw("Extracted claims from token",
		"userID", claims.UserID,
		"email", claims.Email,
		"username", claims.Username)

	return claims, nil
}
