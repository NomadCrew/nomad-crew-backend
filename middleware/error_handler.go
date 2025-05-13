package middleware

import (
	"fmt"
	"runtime/debug"
	"strconv"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/gin-gonic/gin"
)

type ErrorResponse struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
	Code    string `json:"code,omitempty"` // For HTTP status code as string
}

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Capture stack trace before Next() to preserve the full call stack
		stackTrace := debug.Stack()

		c.Next()

		// Check if there are any errors
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			log := logger.GetLogger()

			// Basic metadata for all errors
			metadata := map[string]interface{}{
				"path":        c.Request.URL.Path,
				"method":      c.Request.Method,
				"client_ip":   c.ClientIP(),
				"user_agent":  c.Request.UserAgent(),
				"stack_trace": string(stackTrace),
			}

			// Handle AppError
			if appError, ok := err.(*errors.AppError); ok {
				statusCode := appError.GetHTTPStatus()

				// Use enhanced error logging for application errors
				metadata["error_type"] = string(appError.Type)
				metadata["error_message"] = appError.Message

				if appError.Detail != "" {
					metadata["error_detail"] = appError.Detail
				}

				// Extra logging for auth errors
				if appError.Type == errors.AuthError {
					logger.LogHTTPError(c, err, statusCode, "Authentication error")

					// Try to load config for environment checks
					cfg, err := config.LoadConfig()
					if err == nil {
						log.Infow("Auth error environment check",
							"SUPABASE_URL_SET", cfg.ExternalServices.SupabaseURL != "",
							"SUPABASE_URL_LENGTH", len(cfg.ExternalServices.SupabaseURL),
							"SUPABASE_ANON_KEY_SET", cfg.ExternalServices.SupabaseAnonKey != "",
							"SUPABASE_ANON_KEY_LENGTH", len(cfg.ExternalServices.SupabaseAnonKey),
							"SUPABASE_JWT_SECRET_SET", cfg.ExternalServices.SupabaseJWTSecret != "",
							"SUPABASE_JWT_SECRET_LENGTH", len(cfg.ExternalServices.SupabaseJWTSecret))
					} else {
						log.Warnw("Failed to load config for environment check", "error", err)
					}
				} else {
					logger.LogHTTPError(c, err, statusCode, fmt.Sprintf("%s error", appError.Type))
				}

				// Create the response
				response := map[string]interface{}{
					"type":    string(appError.Type),
					"message": appError.Message,
					"code":    strconv.Itoa(statusCode),
				}

				// Only include details for validation and not-found errors or in debug mode
				if appError.Detail != "" && (gin.IsDebugging() ||
					appError.Type == errors.ValidationError ||
					appError.Type == errors.NotFoundError) {
					response["details"] = appError.Detail
				}

				// For auth errors (particularly token expiration), preserve any additional fields
				// that might have been set in the middleware
				if appError.Type == errors.AuthError && appError.Message == "Your session has expired" {
					// Check if we have a response already set with extra fields
					if v, exists := c.Get("auth_error_response"); exists {
						if authError, ok := v.(gin.H); ok {
							// Just set the status code but use the existing response
							c.JSON(statusCode, authError)
							return
						}
					}
				}

				c.JSON(statusCode, response)
				return
			}

			// Handle Gin binding errors - which come as public errors
			if c.Errors.Last().Type == gin.ErrorTypeBind {
				logger.LogHTTPError(c, err, 400, "Request binding error")

				response := map[string]interface{}{
					"type":    string(errors.ValidationError),
					"message": "Failed to bind request",
					"code":    "400",
				}

				// Include details in debug mode
				if gin.IsDebugging() {
					response["details"] = err.Error()
				}

				c.JSON(400, response)
				return
			}

			// Handle Gin public errors
			if c.Errors.Last().Type == gin.ErrorTypePublic {
				logger.LogHTTPError(c, err, 400, "Public error")

				c.JSON(400, map[string]interface{}{
					"type":    string(errors.ValidationError),
					"message": err.Error(),
					"code":    "400",
				})
				return
			}

			// Handle unknown errors
			logger.LogHTTPError(c, err, 500, "Unexpected server error")

			response := map[string]interface{}{
				"type":    string(errors.ServerError),
				"message": "Internal Server Error",
				"code":    "500",
			}

			// In debug mode, include error details
			if gin.IsDebugging() {
				response["details"] = err.Error()
			}

			c.JSON(500, response)
		}
	}
}
