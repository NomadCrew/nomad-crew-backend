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

		// Step 2: Decode the payload manually (which contains the claims)
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

		// Step 4: Try to validate the token with our secret
		_, validationErr := jwt.Parse([]byte(token),
			jwt.WithVerify(true),
			jwt.WithKey(jwa.HS256, []byte(cfg.ExternalServices.SupabaseJWTSecret)),
			jwt.WithValidate(true),
			jwt.WithAcceptableSkew(30*time.Second), // Allow 30 seconds of clock skew
		)

		if validationErr != nil {
			response["validation_success"] = false
			response["validation_error"] = validationErr.Error()

			// Check specifically for expiration error
			if strings.Contains(validationErr.Error(), "exp not satisfied") ||
				strings.Contains(validationErr.Error(), "token expired") {
				response["error_type"] = "token_expired"
				response["recommendation"] = "The token has expired. Request a new token from Supabase."
			}
		} else {
			response["validation_success"] = true
		}

		// Step 5: Try to validate without checking expiration
		_, signatureErr := jwt.Parse([]byte(token),
			jwt.WithVerify(true),
			jwt.WithKey(jwa.HS256, []byte(cfg.ExternalServices.SupabaseJWTSecret)),
			jwt.WithValidate(false), // Don't validate standard claims like expiration
		)

		if signatureErr != nil {
			response["signature_valid"] = false
			response["signature_error"] = signatureErr.Error()
		} else {
			response["signature_valid"] = true
			response["recommendation"] = "Signature is valid, but token may be expired. Check the expiration time."
		}

		c.JSON(http.StatusOK, response)
	}
}
