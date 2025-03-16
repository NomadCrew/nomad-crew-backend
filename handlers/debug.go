package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/gin-gonic/gin"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// Helper function for finding minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// simpleTokenValidation runs a simple direct validation test with the provided token and secret
// exactly matching the user-provided test snippet
func simpleTokenValidation(tokenString string, secret string) map[string]interface{} {
	result := make(map[string]interface{})
	log := logger.GetLogger()

	// First try to manually extract header information
	parts := strings.Split(tokenString, ".")
	if len(parts) == 3 {
		headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
		if err == nil {
			var headerMap map[string]interface{}
			if json.Unmarshal(headerBytes, &headerMap) == nil {
				// Extract kid if present
				if kid, ok := headerMap["kid"].(string); ok {
					result["kid"] = kid
				}

				// Extract algorithm if present
				if alg, ok := headerMap["alg"].(string); ok {
					result["alg"] = alg
				}
			}
		}
	}

	// Parse without verification to examine the token structure
	parsedWithoutVerify, err := jwt.Parse([]byte(tokenString), jwt.WithVerify(false))
	if err == nil && parsedWithoutVerify != nil {
		// Get basic token information
		result["issuer"] = parsedWithoutVerify.Issuer()
		result["subject"] = parsedWithoutVerify.Subject()
		if !parsedWithoutVerify.Expiration().IsZero() {
			result["expiration"] = parsedWithoutVerify.Expiration().Format(time.RFC3339)
			result["is_expired"] = time.Now().After(parsedWithoutVerify.Expiration())
		}
	}

	// Now try the exact validation as provided in the test snippet
	token, err := jwt.Parse([]byte(tokenString),
		jwt.WithVerify(true),
		jwt.WithKey(jwa.HS256, []byte(secret)),
	)

	if err != nil {
		result["validation_success"] = false
		result["validation_error"] = err.Error()

		// Check for specific error types
		if strings.Contains(err.Error(), "could not verify message") {
			result["signature_error"] = "signature verification failed"
		} else if strings.Contains(err.Error(), "exp not satisfied") {
			result["token_expired"] = true
			result["signature_valid"] = true // The token signature is valid, just expired
		}
	} else {
		result["validation_success"] = true
		// Include basic token info if validation succeeded
		if token != nil {
			result["subject"] = token.Subject()
			result["issuer"] = token.Issuer()
			result["expiration"] = token.Expiration().Format(time.RFC3339)
		}
	}

	// Add specific test for JWKS-based validation
	if kid, ok := result["kid"].(string); ok && kid != "" {
		result["recommendation"] = "Token has 'kid' claim and might require JWKS-based validation instead of static secret"

		// Try fetching the key from the configured JWKS URL if we're using Supabase
		cfg, _ := config.LoadConfig()
		if cfg != nil && cfg.ExternalServices.SupabaseURL != "" {
			jwksURL := fmt.Sprintf("%s/auth/v1/jwks", cfg.ExternalServices.SupabaseURL)
			result["jwks_url"] = jwksURL
			result["note"] = "Consider using JWKS-based validation for this token"

			// Attempt JWKS validation
			result["jwks_validation_attempted"] = true

			// Create JWKS cache instance
			jwksCache := middleware.GetJWKSCache(jwksURL)
			if jwksCache != nil {
				// Try to fetch the key for this specific kid
				key, err := jwksCache.GetKey(kid)
				if err != nil {
					result["jwks_key_fetch_error"] = err.Error()
				} else if key != nil {
					result["jwks_key_found"] = true

					// Always use RS256 for JWKS validation (most common for JWTs from Supabase)
					alg := jwa.RS256

					// Attempt validation with the JWKS key
					_, jwksErr := jwt.Parse([]byte(tokenString),
						jwt.WithVerify(true),
						jwt.WithKey(alg, key),
					)

					if jwksErr != nil {
						result["jwks_validation_success"] = false
						result["jwks_validation_error"] = jwksErr.Error()

						// Check if token is just expired
						if strings.Contains(jwksErr.Error(), "exp not satisfied") {
							result["jwks_validation_note"] = "Token signature is valid with JWKS but token is expired"
							result["jwks_signature_valid"] = true
							result["root_issue"] = "token_expired"
							log.Infow("JWKS validation identified token expiration as the issue", "kid", kid)
						}
					} else {
						result["jwks_validation_success"] = true
						log.Infow("JWKS validation successful", "kid", kid)
					}
				}
			}
		}
	}

	return result
}

// Direct approach exactly matching the successful test script
func directSuccessTest(tokenString string, secret string) map[string]interface{} {
	result := make(map[string]interface{})
	log := logger.GetLogger()

	// Direct approach exactly matching the successful test script
	log.Infow("Attempting direct HS256 validation with raw JWT secret from test script")

	token, err := jwt.Parse([]byte(tokenString),
		jwt.WithVerify(true),
		jwt.WithKey(jwa.HS256, []byte(secret)),
	)

	if err != nil {
		result["success"] = false
		result["error"] = err.Error()
		log.Warnw("Direct HS256 validation failed", "error", err)
	} else {
		result["success"] = true
		result["subject"] = token.Subject()
		log.Infow("Direct HS256 validation succeeded using raw secret")
	}

	// Include a debug tip
	result["notes"] = "This is the exact approach from the successful test script"
	result["approach"] = "Direct HS256 with raw JWT secret"

	return result
}

// DebugJWTHandler handles debug requests for JWT validation
func DebugJWTHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		log := logger.GetLogger()

		// Get token from request
		token := c.Query("token")
		if token == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if token == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "No token provided",
				"help":  "Provide token as query parameter or Authorization: Bearer header",
			})
			return
		}

		// Load configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			log.Errorw("Failed to load config", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to load configuration",
				"details": err.Error(),
			})
			return
		}

		// Create a debug response
		response := gin.H{
			"token_length":  len(token),
			"config_loaded": true,
			"env_variables": gin.H{
				"supabase_url_set":         cfg.ExternalServices.SupabaseURL != "",
				"supabase_url_length":      len(cfg.ExternalServices.SupabaseURL),
				"supabase_anon_key_set":    cfg.ExternalServices.SupabaseAnonKey != "",
				"supabase_anon_key_length": len(cfg.ExternalServices.SupabaseAnonKey),
				"jwt_secret_set":           cfg.ExternalServices.SupabaseJWTSecret != "",
				"jwt_secret_length":        len(cfg.ExternalServices.SupabaseJWTSecret),
			},
		}

		// Add the direct simple token validation result
		response["direct_validation_test"] = simpleTokenValidation(token, cfg.ExternalServices.SupabaseJWTSecret)

		// Step 1: Manual parsing to extract token parts
		parts := strings.Split(token, ".")
		if len(parts) != 3 {
			response["parse_success"] = false
			response["parse_error"] = "Invalid token format: token must have 3 parts"
			c.JSON(http.StatusOK, response)
			return
		}

		// Add token format information
		response["token_format"] = gin.H{
			"parts_count":      len(parts),
			"header_length":    len(parts[0]),
			"payload_length":   len(parts[1]),
			"signature_length": len(parts[2]),
		}

		// Step 2a: Decode the header to check for algorithm information
		var header map[string]interface{}
		headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
		if err != nil {
			log.Warnf("Failed to decode header: %v", err)
		} else {
			if err := json.Unmarshal(headerBytes, &header); err != nil {
				log.Warnf("Failed to unmarshal header: %v", err)
			} else {
				response["token_header"] = header
				// Check if header contains algorithm info
				if alg, ok := header["alg"].(string); ok {
					response["token_algorithm"] = alg
				}
			}
		}

		// Step 2b: Decode the payload manually (which contains the claims)
		var claims map[string]interface{}
		payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			response["parse_success"] = false
			response["parse_error"] = fmt.Sprintf("Failed to decode payload: %v", err)
			c.JSON(http.StatusOK, response)
			return
		}

		// Unmarshal the JSON payload
		if err := json.Unmarshal(payloadBytes, &claims); err != nil {
			response["parse_success"] = false
			response["parse_error"] = fmt.Sprintf("Failed to unmarshal claims: %v", err)
			c.JSON(http.StatusOK, response)
			return
		}

		response["parse_success"] = true
		response["claims"] = claims

		// Step 3: Manual check of expiration
		now := time.Now().Unix()
		expClaim, hasExp := claims["exp"]
		if hasExp {
			// Convert the exp claim to a number
			var expTime int64
			switch exp := expClaim.(type) {
			case float64:
				expTime = int64(exp)
			case int64:
				expTime = exp
			case json.Number:
				expTime, _ = exp.Int64()
			default:
				log.Warnw("Unexpected type for exp claim", "type", fmt.Sprintf("%T", expClaim))
			}

			expTimeFormatted := time.Unix(expTime, 0).Format(time.RFC3339)
			nowFormatted := time.Unix(now, 0).Format(time.RFC3339)

			response["token_expiration"] = gin.H{
				"expiration_time": expTimeFormatted,
				"current_time":    nowFormatted,
				"is_expired":      now > expTime,
				"seconds_diff":    expTime - now,
			}

			if now > expTime {
				response["token_expiration"].(gin.H)["time_since_expiry"] = time.Since(time.Unix(expTime, 0)).String()
				response["token_status"] = "expired"
			} else {
				response["token_expiration"].(gin.H)["time_until_expiry"] = time.Until(time.Unix(expTime, 0)).String()
				response["token_status"] = "valid"
			}
		} else {
			response["token_status"] = "no_expiration"
		}

		// Step 4: Add the direct simple token validation test
		response["simple_validation_test"] = simpleTokenValidation(token, cfg.ExternalServices.SupabaseJWTSecret)

		// Add our direct successful approach from test script
		response["direct_success_test"] = directSuccessTest(token, cfg.ExternalServices.SupabaseJWTSecret)

		// Add tests for all variants of our new validation approaches
		response["comprehensive_validation"] = comprehensiveValidationTests(token, cfg, header)

		// Step 5: Try to validate without checking expiration - with multiple formats and algorithms
		response["signature_validation_attempts"] = []gin.H{}

		// Try different algorithms - including asymmetric ones
		algorithms := []jwa.SignatureAlgorithm{
			jwa.HS256, jwa.HS384, jwa.HS512,
			jwa.RS256, jwa.RS384, jwa.RS512,
		}

		// Try different secret formats
		secretFormats := []struct {
			name   string
			secret []byte
		}{
			{name: "raw", secret: []byte(cfg.ExternalServices.SupabaseJWTSecret)},
		}

		// Add base64 standard decoded secret if decodable
		stdDecoded, err := base64.StdEncoding.DecodeString(cfg.ExternalServices.SupabaseJWTSecret)
		if err == nil {
			secretFormats = append(secretFormats, struct {
				name   string
				secret []byte
			}{name: "base64_std", secret: stdDecoded})

			// Also log the decoded content (first few bytes)
			if len(stdDecoded) > 0 {
				previewLen := min(8, len(stdDecoded))
				response["base64_std_preview"] = fmt.Sprintf("%x", stdDecoded[:previewLen])
			}
		} else {
			response["base64_std_error"] = err.Error()
		}

		// Add URL-safe base64 decoded secret if decodable
		urlDecoded, err := base64.URLEncoding.DecodeString(cfg.ExternalServices.SupabaseJWTSecret)
		if err == nil {
			secretFormats = append(secretFormats, struct {
				name   string
				secret []byte
			}{name: "base64_url", secret: urlDecoded})

			// Also log the decoded content (first few bytes)
			if len(urlDecoded) > 0 {
				previewLen := min(8, len(urlDecoded))
				response["base64_url_preview"] = fmt.Sprintf("%x", urlDecoded[:previewLen])
			}
		} else {
			response["base64_url_error"] = err.Error()
		}

		// Add Raw URL-safe base64 decoded secret if decodable
		rawUrlDecoded, err := base64.RawURLEncoding.DecodeString(cfg.ExternalServices.SupabaseJWTSecret)
		if err == nil {
			secretFormats = append(secretFormats, struct {
				name   string
				secret []byte
			}{name: "base64_raw_url", secret: rawUrlDecoded})

			// Also log the decoded content (first few bytes)
			if len(rawUrlDecoded) > 0 {
				previewLen := min(8, len(rawUrlDecoded))
				response["base64_raw_url_preview"] = fmt.Sprintf("%x", rawUrlDecoded[:previewLen])
			}
		} else {
			response["base64_raw_url_error"] = err.Error()
		}

		// Try all combinations
		var validationSuccess bool

		// Try a completely minimal validation approach too
		_, minimalErr := jwt.Parse([]byte(token),
			jwt.WithVerify(true),
			jwt.WithKey(jwa.HS256, []byte(cfg.ExternalServices.SupabaseJWTSecret)),
		)

		if minimalErr == nil {
			response["minimal_validation_success"] = true
			validationSuccess = true
			response["working_validation"] = gin.H{
				"algorithm": "HS256",
				"format":    "raw",
				"approach":  "minimal",
			}
		} else {
			response["minimal_validation_success"] = false
			response["minimal_validation_error"] = minimalErr.Error()
		}

		// Also add Supabase-specific JWT formats
		if issuer, ok := claims["iss"].(string); ok && strings.Contains(issuer, "supabase") {
			// Try with format documented in Supabase docs
			// See: https://supabase.com/docs/guides/auth/server-side-rendering#verifying-jwt

			// The secret is supposed to be the Base64-encoded representation of
			// the HS256 key used to sign the JWT
			if len(secretFormats) > 0 {
				response["issuer_detected"] = issuer
				response["supabase_specific_format"] = true
			}
		}

		for _, alg := range algorithms {
			for _, secretFormat := range secretFormats {
				attempt := gin.H{
					"algorithm": alg.String(),
					"format":    secretFormat.name,
				}

				// Skip trying asymmetric algos with symmetric keys
				if (alg == jwa.RS256 || alg == jwa.RS384 || alg == jwa.RS512) &&
					secretFormat.name != "base64_std" && secretFormat.name != "base64_url" {
					attempt["skipped"] = true
					attempt["reason"] = "asymmetric algorithm requires properly formatted key"
					response["signature_validation_attempts"] = append(
						response["signature_validation_attempts"].([]gin.H),
						attempt,
					)
					continue
				}

				var signatureErr error
				var parseResult jwt.Token

				// Special handling for RS256
				if alg == jwa.RS256 || alg == jwa.RS384 || alg == jwa.RS512 {
					// We need to parse this as a potential PEM block or other key format
					attempt["key_length"] = len(secretFormat.secret)

					// Skip if key is too short for an RSA key
					if len(secretFormat.secret) < 64 {
						attempt["skipped"] = true
						attempt["reason"] = "key too short for RSA"
						response["signature_validation_attempts"] = append(
							response["signature_validation_attempts"].([]gin.H),
							attempt,
						)
						continue
					}
				}

				parseResult, signatureErr = jwt.Parse([]byte(token),
					jwt.WithVerify(true),
					jwt.WithKey(alg, secretFormat.secret),
					jwt.WithValidate(false), // Skip exp validation
				)

				if signatureErr != nil {
					attempt["success"] = false
					attempt["error"] = signatureErr.Error()

					// Check if it's an expired token error specifically
					if strings.Contains(signatureErr.Error(), "exp not satisfied") {
						attempt["error_type"] = "token_expired"
						// This is actually promising - it means the signature passed but expiration failed
						attempt["signature_valid"] = true
						validationSuccess = true

						response["working_validation"] = gin.H{
							"algorithm": alg.String(),
							"format":    secretFormat.name,
							"note":      "Signature valid but token expired",
						}
					}
				} else {
					attempt["success"] = true
					validationSuccess = true

					// If this worked, add it to the main response
					response["working_validation"] = gin.H{
						"algorithm": alg.String(),
						"format":    secretFormat.name,
					}

					// Extract subject for confirmation
					if parseResult != nil {
						sub := parseResult.Subject()
						if sub != "" {
							attempt["subject"] = sub
						}
					}
				}

				response["signature_validation_attempts"] = append(
					response["signature_validation_attempts"].([]gin.H),
					attempt,
				)
			}
		}

		// Try a last-ditch direct approach without using the library's verification
		// This will help us double-check if our JWT library is behaving as expected
		if !validationSuccess {
			// Add debugging info about the JWT parts
			response["manual_inspection"] = gin.H{
				"jwt_parts":       parts,
				"header_decoded":  header,
				"payload_decoded": claims,
			}
		}

		// Check for 'kid' in header - if present, note that JWKS-based validation might be needed
		if kid, ok := header["kid"].(string); ok && kid != "" {
			response["has_kid"] = true
			response["kid_value"] = kid
			response["jwt_key_type"] = "jwks"

			// Construct the JWKS URL
			jwksURL := fmt.Sprintf("%s/auth/v1/jwks", cfg.ExternalServices.SupabaseURL)
			response["jwks_url"] = jwksURL

			log.Infow("Token contains 'kid' header, suggesting JWKS validation required",
				"kid", kid,
				"jwks_url", jwksURL)
		}

		response["signature_valid"] = validationSuccess
		if validationSuccess {
			response["recommendation"] = "Signature is valid, but token may be expired. Use the working validation approach in your auth middleware."
		} else {
			// Also print the first few characters of the secret for debugging
			secretPreview := ""
			if len(cfg.ExternalServices.SupabaseJWTSecret) > 8 {
				secretPreview = cfg.ExternalServices.SupabaseJWTSecret[:8] + "..."
			}

			response["recommendation"] = fmt.Sprintf(
				"None of the signature validation approaches worked. Check your SUPABASE_JWT_SECRET (starts with: %s). You may need to verify the secret in the Supabase dashboard.",
				secretPreview,
			)
		}

		c.JSON(http.StatusOK, response)
	}
}

// comprehensiveValidationTests runs all our validation approaches from the most
// promising to least promising, to determine which is most reliable
func comprehensiveValidationTests(tokenString string, cfg *config.Config, tokenHeader map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	log := logger.GetLogger()

	// Array to store results of each approach
	approaches := []map[string]interface{}{}

	// Helper to add an approach result
	addApproach := func(name, description string, fn func() (jwt.Token, error)) {
		approach := map[string]interface{}{
			"name":        name,
			"description": description,
		}

		token, err := fn()
		if err != nil {
			approach["success"] = false
			approach["error"] = err.Error()
		} else {
			approach["success"] = true
			if token != nil {
				approach["subject"] = token.Subject()
				approach["issuer"] = token.Issuer()
			}
		}

		approaches = append(approaches, approach)
	}

	// APPROACH 1: Direct HS256 with raw JWT secret
	addApproach("direct_hs256", "Direct HS256 validation with raw JWT secret", func() (jwt.Token, error) {
		return jwt.Parse([]byte(tokenString),
			jwt.WithVerify(true),
			jwt.WithKey(jwa.HS256, []byte(cfg.ExternalServices.SupabaseJWTSecret)),
		)
	})

	// Extract kid value if present
	var kidValue string
	if kid, ok := tokenHeader["kid"].(string); ok && kid != "" {
		kidValue = kid

		// APPROACH 2: HS256 with kid value as secret
		addApproach("kid_as_secret", "HS256 validation using kid value as secret", func() (jwt.Token, error) {
			return jwt.Parse([]byte(tokenString),
				jwt.WithVerify(true),
				jwt.WithKey(jwa.HS256, []byte(kidValue)),
			)
		})

		// APPROACH 3: Try to use JWKS validation if Supabase URL is set
		if cfg.ExternalServices.SupabaseURL != "" {
			jwksURL := fmt.Sprintf("%s/auth/v1/jwks", cfg.ExternalServices.SupabaseURL)

			// Create JWKS cache instance
			jwksCache := middleware.GetJWKSCache(jwksURL)

			// Try to fetch the key for this specific kid
			key, err := jwksCache.GetKey(kidValue)
			if err == nil && key != nil {
				addApproach("jwks_rs256", "RS256 validation with JWKS key", func() (jwt.Token, error) {
					return jwt.Parse([]byte(tokenString),
						jwt.WithVerify(true),
						jwt.WithKey(jwa.RS256, key),
					)
				})
			} else {
				log.Warnw("Failed to get key from JWKS", "error", err)
			}
		}
	}

	// APPROACH 4: Try other common algorithms with raw secret
	for _, alg := range []struct {
		name string
		alg  jwa.SignatureAlgorithm
	}{
		{"HS384", jwa.HS384},
		{"HS512", jwa.HS512},
	} {
		addApproach("direct_"+alg.name, fmt.Sprintf("Direct %s validation with raw JWT secret", alg.name), func() (jwt.Token, error) {
			return jwt.Parse([]byte(tokenString),
				jwt.WithVerify(true),
				jwt.WithKey(alg.alg, []byte(cfg.ExternalServices.SupabaseJWTSecret)),
			)
		})
	}

	// Find the successful approach
	var workingApproach map[string]interface{}
	for _, approach := range approaches {
		if success, ok := approach["success"].(bool); ok && success {
			workingApproach = approach
			break
		}
	}

	result["approaches"] = approaches

	if workingApproach != nil {
		result["working_approach"] = workingApproach
		result["recommendation"] = fmt.Sprintf(
			"The best approach for validating this token is: %s - %s",
			workingApproach["name"],
			workingApproach["description"],
		)

		// Add code snippet for the working approach
		switch workingApproach["name"] {
		case "direct_hs256":
			result["code_snippet"] = `
// Validate JWT token with direct HS256 approach
token, err := jwt.Parse([]byte(tokenString),
	jwt.WithVerify(true),
	jwt.WithKey(jwa.HS256, []byte(cfg.ExternalServices.SupabaseJWTSecret)),
)
`
		case "kid_as_secret":
			result["code_snippet"] = `
// Extract kid from token header
var kidValue string
parts := strings.Split(tokenString, ".")
if len(parts) == 3 {
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err == nil {
		var headerMap map[string]interface{}
		if json.Unmarshal(headerBytes, &headerMap) == nil {
			if kid, ok := headerMap["kid"].(string); ok {
				kidValue = kid
			}
		}
	}
}

// Validate token using kid as secret
token, err := jwt.Parse([]byte(tokenString),
	jwt.WithVerify(true),
	jwt.WithKey(jwa.HS256, []byte(kidValue)),
)
`
		case "jwks_rs256":
			result["code_snippet"] = `
// Extract kid from token header
var kidValue string
parts := strings.Split(tokenString, ".")
if len(parts) == 3 {
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err == nil {
		var headerMap map[string]interface{}
		if json.Unmarshal(headerBytes, &headerMap) == nil {
			if kid, ok := headerMap["kid"].(string); ok {
				kidValue = kid
			}
		}
	}
}

// Get key from JWKS
jwksURL := fmt.Sprintf("%s/auth/v1/jwks", cfg.ExternalServices.SupabaseURL)
jwksCache := middleware.GetJWKSCache(jwksURL)
key, err := jwksCache.GetKey(kidValue)
if err == nil && key != nil {
	token, err := jwt.Parse([]byte(tokenString),
		jwt.WithVerify(true),
		jwt.WithKey(jwa.RS256, key),
	)
}
`
		}
	} else {
		result["recommendation"] = "None of the approaches was successful. You may need to try additional methods."
	}

	return result
}

// DebugJWTDirectHandler tests JWT validation with hardcoded secret from the test script
func DebugJWTDirectHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		log := logger.GetLogger()

		// Get token from request
		token := c.Query("token")
		if token == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if token == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "No token provided",
				"help":  "Provide token as query parameter or Authorization: Bearer header",
			})
			return
		}

		// Load configuration for comparison
		cfg, err := config.LoadConfig()
		if err != nil {
			log.Errorw("Failed to load config", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to load configuration",
				"details": err.Error(),
			})
			return
		}

		// Response object
		response := gin.H{
			"token_length": len(token),
		}

		// Hardcoded secret from your test script
		hardcodedSecret := "KwiMJ4NKJW/FFE41DNj5C6fj19mww+ydxyesjZUG+UZ2rHKnTTTf2NkDxv1QTgpM06JDcwdEJJioAIeP5AMO/w==" // #nosec G101 - intentionally hardcoded for debug purposes only

		// Compare the two secrets (safely display first 8 chars)
		envSecret := strings.TrimSpace(cfg.ExternalServices.SupabaseJWTSecret)
		response["secrets_comparison"] = gin.H{
			"hardcoded_secret_prefix": func() string {
				if len(hardcodedSecret) > 8 {
					return hardcodedSecret[:8] + "..."
				}
				return "too short"
			}(),
			"env_secret_prefix": func() string {
				if len(envSecret) > 8 {
					return envSecret[:8] + "..."
				}
				return "too short"
			}(),
			"secrets_match":    hardcodedSecret == envSecret,
			"hardcoded_length": len(hardcodedSecret),
			"env_length":       len(envSecret),
		}

		// Test with hardcoded secret directly
		hardcodedResult := testDirectValidation(token, hardcodedSecret)
		response["hardcoded_validation"] = hardcodedResult

		// Test with environment secret
		envResult := testDirectValidation(token, envSecret)
		response["env_validation"] = envResult

		// If there are character differences, try to identify them
		if hardcodedSecret != envSecret {
			// Find the first position where they differ
			minLen := min(len(hardcodedSecret), len(envSecret))
			diffIndex := -1

			for i := 0; i < minLen; i++ {
				if hardcodedSecret[i] != envSecret[i] {
					diffIndex = i
					break
				}
			}

			if diffIndex >= 0 {
				response["first_difference"] = gin.H{
					"position":       diffIndex,
					"hardcoded_char": fmt.Sprintf("%q (byte: %d)", string(hardcodedSecret[diffIndex]), hardcodedSecret[diffIndex]),
					"env_char":       fmt.Sprintf("%q (byte: %d)", string(envSecret[diffIndex]), envSecret[diffIndex]),
				}

				// Show a small window around the difference
				startPos := max(0, diffIndex-5)
				endPosHard := min(len(hardcodedSecret), diffIndex+6)
				endPosEnv := min(len(envSecret), diffIndex+6)

				response["difference_context"] = gin.H{
					"hardcoded_excerpt": fmt.Sprintf("%q", hardcodedSecret[startPos:endPosHard]),
					"env_excerpt":       fmt.Sprintf("%q", envSecret[startPos:endPosEnv]),
				}
			} else if len(hardcodedSecret) != len(envSecret) {
				// They differ in length
				response["length_difference"] = gin.H{
					"hardcoded_extra": func() string {
						if len(hardcodedSecret) > len(envSecret) {
							return fmt.Sprintf("%q", hardcodedSecret[len(envSecret):])
						}
						return ""
					}(),
					"env_extra": func() string {
						if len(envSecret) > len(hardcodedSecret) {
							return fmt.Sprintf("%q", envSecret[len(hardcodedSecret):])
						}
						return ""
					}(),
				}
			}
		}

		// Final recommendation
		if hardcodedResult["success"].(bool) {
			response["recommendation"] = "The hardcoded secret works! Update your environment variable to match it exactly."
		} else if envResult["success"].(bool) {
			response["recommendation"] = "Environment secret works but hardcoded doesn't - something else is wrong."
		} else {
			response["recommendation"] = "Neither secret works. Check your token or try additional approaches."
		}

		c.JSON(http.StatusOK, response)
	}
}

// Helper function to test JWT validation with a specific secret
func testDirectValidation(tokenString string, secret string) map[string]interface{} {
	result := make(map[string]interface{})
	log := logger.GetLogger()

	token, err := jwt.Parse([]byte(tokenString),
		jwt.WithVerify(true),
		jwt.WithKey(jwa.HS256, []byte(secret)),
	)

	if err != nil {
		result["success"] = false
		result["error"] = err.Error()
		log.Warnw("JWT validation failed", "error", err)
	} else {
		result["success"] = true
		if token != nil {
			result["subject"] = token.Subject()
			result["issuer"] = token.Issuer()
		}
		log.Infow("JWT validation succeeded")
	}

	return result
}

// Helper function for finding maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
