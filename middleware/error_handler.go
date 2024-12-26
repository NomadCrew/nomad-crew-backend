package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
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

				c.JSON(appError.HTTPStatus, ErrorResponse{
					Type:    string(appError.Type),
					Message: appError.Message,
					Detail:  appError.Detail,
				})
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