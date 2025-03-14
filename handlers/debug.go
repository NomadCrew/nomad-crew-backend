package handlers

import (
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

		// Try to parse without verification to inspect
		tokenObj, err := jwt.Parse([]byte(token), jwt.WithVerify(false))
		if err != nil {
			log.Errorw("Failed to parse token without verification", "error", err)
			response["parse_error"] = err.Error()
			response["parse_success"] = false
			c.JSON(http.StatusOK, response)
			return
		}

		// Token parsed successfully, add claims to response
		response["parse_success"] = true

		// Extract claims
		claims := make(map[string]interface{})
		for k, v := range tokenObj.PrivateClaims() {
			claims[k] = v
		}

		// Add registered claims
		if sub := tokenObj.Subject(); sub != "" {
			claims["sub"] = sub
		}
		if iss := tokenObj.Issuer(); iss != "" {
			claims["iss"] = iss
		}
		if aud := tokenObj.Audience(); len(aud) > 0 {
			claims["aud"] = aud
		}
		if exp := tokenObj.Expiration(); !exp.IsZero() {
			claims["exp"] = exp
			claims["expires_at"] = exp.Format(time.RFC3339)
			claims["is_expired"] = time.Now().After(exp)
			claims["time_until_expiry"] = time.Until(exp).String()
		}
		if iat := tokenObj.IssuedAt(); !iat.IsZero() {
			claims["iat"] = iat
			claims["issued_at"] = iat.Format(time.RFC3339)
		}

		response["claims"] = claims

		// Extract the kid from the token header
		kid, ok := tokenObj.PrivateClaims()["kid"].(string)
		response["has_kid"] = ok
		if ok {
			response["kid"] = kid

			// Test JWKS validation
			supabaseURL := cfg.ExternalServices.SupabaseURL
			if supabaseURL != "" {
				jwksURL := fmt.Sprintf("%s/auth/v1/jwks", strings.TrimSuffix(supabaseURL, "/"))
				response["jwks_url"] = jwksURL

				// Get the JWKS cache
				cache := middleware.GetJWKSCache(jwksURL)
				matchingKey, jwksErr := cache.GetKey(kid)

				if jwksErr != nil {
					log.Errorw("Failed to get key from JWKS", "error", jwksErr, "kid", kid)
					response["jwks_error"] = jwksErr.Error()
					response["jwks_key_found"] = false
				} else {
					response["jwks_key_found"] = true
					response["jwks_key_algorithm"] = matchingKey.Algorithm()

					// Try to verify with the key
					algStr, ok := tokenObj.PrivateClaims()["alg"].(string)
					if !ok {
						response["jwks_validation_error"] = "No alg found in token header"
					} else {
						response["token_alg"] = algStr

						// Map the algorithm
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
							response["jwks_validation_error"] = fmt.Sprintf("Unsupported algorithm: %s", algStr)
							c.JSON(http.StatusOK, response)
							return
						}

						// Verify with the key
						_, jwksValidationErr := jwt.Parse([]byte(token),
							jwt.WithVerify(true),
							jwt.WithKey(alg, matchingKey),
							jwt.WithValidate(true),
							jwt.WithAcceptableSkew(30*time.Second),
						)

						if jwksValidationErr != nil {
							response["jwks_validation_success"] = false
							response["jwks_validation_error"] = jwksValidationErr.Error()
						} else {
							response["jwks_validation_success"] = true
						}
					}
				}
			} else {
				response["jwks_error"] = "SUPABASE_URL is not configured"
			}
		} else {
			// Try static secret validation
			rawSecret := cfg.ExternalServices.SupabaseJWTSecret
			if rawSecret == "" {
				response["static_validation_error"] = "SUPABASE_JWT_SECRET is not set"
			} else {
				// Try with raw secret
				_, err := jwt.Parse([]byte(token),
					jwt.WithVerify(true),
					jwt.WithKey(jwa.HS256, []byte(rawSecret)),
					jwt.WithValidate(true),
					jwt.WithAcceptableSkew(30*time.Second),
				)

				if err != nil {
					response["raw_secret_validation_success"] = false
					response["raw_secret_validation_error"] = err.Error()
				} else {
					response["raw_secret_validation_success"] = true
				}

				// Try with base64 decoded secret
				if middleware.IsBase64(rawSecret) {
					decodedSecret, err := middleware.DecodeBase64(rawSecret)
					if err != nil {
						response["base64_decode_error"] = err.Error()
					} else {
						_, err := jwt.Parse([]byte(token),
							jwt.WithVerify(true),
							jwt.WithKey(jwa.HS256, decodedSecret),
							jwt.WithValidate(true),
							jwt.WithAcceptableSkew(30*time.Second),
						)

						if err != nil {
							response["base64_validation_success"] = false
							response["base64_validation_error"] = err.Error()
						} else {
							response["base64_validation_success"] = true
						}
					}
				}
			}
		}

		c.JSON(http.StatusOK, response)
	}
}
