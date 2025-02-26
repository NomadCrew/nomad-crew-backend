package middleware

import (
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
		c.Next()

		// Check if there are any errors
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			log := logger.GetLogger()

			// Handle AppError
			if appError, ok := err.(*errors.AppError); ok {
				log.Errorw("Application error",
					"type", appError.Type,
					"message", appError.Message,
					"detail", appError.Detail,
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
				)

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
				"path", c.Request.URL.Path,
				"method", c.Request.Method,
			)

			c.JSON(500, ErrorResponse{
				Type:    string(errors.ServerError),
				Message: "Internal server error",
				Detail:  "An unexpected error occurred",
			})
		}
	}
}
