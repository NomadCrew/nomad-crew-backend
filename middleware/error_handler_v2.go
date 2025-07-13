package middleware

import (
	"fmt"
	"runtime/debug"
	"strconv"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
)

// ErrorHandlerV2 provides standardized error handling using the new response format
func ErrorHandlerV2() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Process the request
		c.Next()

		// If there's no error, return
		if len(c.Errors) == 0 {
			return
		}

		// Get the last error
		lastError := c.Errors.Last()
		err := lastError.Err

		// Get request ID
		requestID := c.GetString(string(RequestIDKey))

		// Get the logger
		log := logger.GetLogger()

		// Process the error
		var errorInfo *types.ErrorInfo
		var statusCode int
		var errorCode string

		// Handle different error types
		switch e := err.(type) {
		case *errors.AppError:
			statusCode = e.GetHTTPStatus()
			errorCode = mapErrorTypeToCode(e.Type)
			errorInfo = &types.ErrorInfo{
				Code:    errorCode,
				Message: e.Message,
				TraceID: requestID,
			}

			// Add details for certain error types or in debug mode
			if e.Details != nil && (gin.IsDebugging() ||
				e.Type == errors.ValidationError ||
				e.Type == errors.NotFoundError) {
				errorInfo.Details = e.Details
			} else if e.Detail != "" && (gin.IsDebugging() ||
				e.Type == errors.ValidationError ||
				e.Type == errors.NotFoundError) {
				errorInfo.Details = map[string]interface{}{"detail": e.Detail}
			}

			// Log the error
			logger.LogHTTPError(c, err, statusCode, fmt.Sprintf("%s error", e.Type))

			// Special handling for auth errors
			if e.Type == errors.AuthError {
				handleAuthError(c, log)
			}

		default:
			// Handle Gin binding errors
			if lastError.Type == gin.ErrorTypeBind {
				statusCode = 400
				errorCode = types.ErrCodeValidationFailed
				errorInfo = &types.ErrorInfo{
					Code:    errorCode,
					Message: "Failed to bind request",
					TraceID: requestID,
				}

				if gin.IsDebugging() {
					errorInfo.Details = map[string]interface{}{"error": err.Error()}
				}

				logger.LogHTTPError(c, err, statusCode, "Request binding error")

			} else if lastError.Type == gin.ErrorTypePublic {
				// Handle Gin public errors
				statusCode = 400
				errorCode = types.ErrCodeBadRequest
				errorInfo = &types.ErrorInfo{
					Code:    errorCode,
					Message: err.Error(),
					TraceID: requestID,
				}

				logger.LogHTTPError(c, err, statusCode, "Public error")

			} else {
				// Handle unknown errors
				statusCode = 500
				errorCode = types.ErrCodeInternalError
				errorInfo = &types.ErrorInfo{
					Code:    errorCode,
					Message: "Internal Server Error",
					TraceID: requestID,
				}

				// Include error details and stack trace in debug mode
				if gin.IsDebugging() {
					errorInfo.Details = map[string]interface{}{
						"error": err.Error(),
						"stack": string(debug.Stack()),
					}
				}

				logger.LogHTTPError(c, err, statusCode, "Unexpected server error")
			}
		}

		// Build standardized response
		response := types.StandardResponse{
			Success: false,
			Error:   errorInfo,
			Meta: &types.MetaInfo{
				RequestID: requestID,
			},
		}

		// Set status code header for monitoring
		c.Header("X-Error-Code", errorCode)
		c.Header("X-Status-Code", strconv.Itoa(statusCode))

		// Send response
		c.JSON(statusCode, response)
	}
}

// mapErrorTypeToCode maps legacy error types to new error codes
func mapErrorTypeToCode(errorType errors.ErrorType) string {
	switch errorType {
	case errors.NotFoundError:
		return types.ErrCodeNotFound
	case errors.ValidationError:
		return types.ErrCodeValidationFailed
	case errors.AuthError:
		return types.ErrCodeUnauthorized
	case errors.AuthorizationError:
		return types.ErrCodeForbidden
	case errors.ConflictError:
		return types.ErrCodeConflict
	case errors.DatabaseError:
		return types.ErrCodeDatabaseError
	case errors.ExternalServiceError:
		return types.ErrCodeExternalServiceError
	case errors.ServerError:
		return types.ErrCodeInternalError
	default:
		return types.ErrCodeInternalError
	}
}

// handleAuthError provides special handling for authentication errors
func handleAuthError(c *gin.Context, log *logger.Logger) {
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
}