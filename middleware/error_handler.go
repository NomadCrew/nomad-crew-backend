package middleware

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/gin-gonic/gin"
)

type ErrorResponse struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
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

			// Log request details for all errors
			log.Infow("Request details for error",
				"path", c.Request.URL.Path,
				"method", c.Request.Method,
				"client_ip", c.ClientIP(),
				"user_agent", c.Request.UserAgent(),
				"headers", func() map[string]string {
					headers := make(map[string]string)
					for k, v := range c.Request.Header {
						if k != "Authorization" && k != "Cookie" { // Skip sensitive headers
							headers[k] = strings.Join(v, ",")
						}
					}
					return headers
				}())

			// Handle AppError
			if appError, ok := err.(*errors.AppError); ok {
				// Extra logging for auth errors
				if appError.Type == errors.AuthError {
					log.Errorw("Authentication error",
						"type", appError.Type,
						"message", appError.Message,
						"detail", appError.Detail,
						"path", c.Request.URL.Path,
						"method", c.Request.Method,
						"stack_trace", string(stackTrace),
					)

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
					log.Errorw("Application error",
						"type", appError.Type,
						"message", appError.Message,
						"detail", appError.Detail,
						"path", c.Request.URL.Path,
						"method", c.Request.Method,
					)
				}

				// Create the response
				response := ErrorResponse{
					Type:    string(appError.Type),
					Message: appError.Message,
					Detail:  appError.Detail,
				}

				// For auth errors (particularly token expiration), preserve any additional fields
				// that might have been set in the middleware
				if appError.Type == errors.AuthError && appError.Message == "Your session has expired" {
					// Check if we have a response already set with extra fields
					if v, exists := c.Get("auth_error_response"); exists {
						if authError, ok := v.(gin.H); ok {
							// Just set the status code but use the existing response
							c.JSON(appError.HTTPStatus, authError)
							return
						}
					}
				}

				c.JSON(appError.HTTPStatus, response)
				return
			}

			// Handle unknown errors
			log.Errorw("Unexpected error",
				"error", err,
				"error_type", fmt.Sprintf("%T", err),
				"error_details", fmt.Sprintf("%+v", err),
				"path", c.Request.URL.Path,
				"method", c.Request.Method,
				"stack_trace", string(stackTrace),
			)

			c.JSON(500, ErrorResponse{
				Type:    string(errors.ServerError),
				Message: "Internal server error",
				Detail:  "An unexpected error occurred",
			})
		}
	}
}
