package errors

import (
	"fmt"
	"net/http"
)

// Error types
const (
	NotFoundError      = "NOT_FOUND"
	ValidationError    = "VALIDATION"
	DatabaseError      = "DATABASE"
	AuthError          = "AUTHENTICATION"
	AuthorizationError = "AUTHORIZATION"
	ServerError        = "SERVER"
	ExternalAPIError   = "EXTERNAL_API"
	TripNotFoundError  = "TRIP_NOT_FOUND"
)

// AppError is a structured error type for the application
type AppError struct {
	Type       string
	Message    string
	Details    string
	Detail     string // Alias for Details to maintain compatibility
	Raw        error  // Original error if this is a wrapper
	HTTPStatus int    // HTTP status code to return
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s_ERROR: %s (%s)", e.Type, e.Message, e.Detail)
	}
	return fmt.Sprintf("%s_ERROR: %s", e.Type, e.Message)
}

// getHTTPStatus returns the appropriate HTTP status code for the error type
func getHTTPStatus(errType string) int {
	switch errType {
	case NotFoundError:
		return http.StatusNotFound
	case ValidationError:
		return http.StatusBadRequest
	case AuthError:
		return http.StatusUnauthorized
	case DatabaseError:
		return http.StatusInternalServerError
	case AuthorizationError:
		return http.StatusForbidden
	case TripNotFoundError:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

// New creates a new AppError with the given type, message, and detail
func New(errType string, message string, detail string) *AppError {
	return &AppError{
		Type:       errType,
		Message:    message,
		Details:    detail,
		Detail:     detail,
		HTTPStatus: getHTTPStatus(errType),
	}
}

// NotFound creates a new not found error
func NotFound(resource string, id interface{}) *AppError {
	return &AppError{
		Type:       NotFoundError,
		Message:    fmt.Sprintf("%s not found", resource),
		Details:    fmt.Sprintf("ID: %v", id),
		Detail:     fmt.Sprintf("ID: %v", id),
		HTTPStatus: http.StatusNotFound,
	}
}

// ValidationFailed creates a new validation error
func ValidationFailed(message string, details string) *AppError {
	return &AppError{
		Type:       ValidationError,
		Message:    message,
		Details:    details,
		Detail:     details,
		HTTPStatus: http.StatusBadRequest,
	}
}

// NewDatabaseError creates a new database error
func NewDatabaseError(err error) *AppError {
	return &AppError{
		Type:       DatabaseError,
		Message:    "Database operation failed",
		Details:    err.Error(),
		Detail:     err.Error(),
		Raw:        err,
		HTTPStatus: http.StatusInternalServerError,
	}
}

// AuthenticationFailed creates a new authentication error
func AuthenticationFailed(message string) *AppError {
	return &AppError{
		Type:       AuthError,
		Message:    message,
		HTTPStatus: http.StatusUnauthorized,
	}
}

// AuthorizationFailed creates a new authorization error
func AuthorizationFailed(code string, message string) *AppError {
	return &AppError{
		Type:       AuthorizationError,
		Message:    message,
		Details:    code,
		Detail:     code,
		HTTPStatus: http.StatusForbidden,
	}
}

// InternalServerError creates a new server error
func InternalServerError(message string) *AppError {
	return &AppError{
		Type:       ServerError,
		Message:    message,
		HTTPStatus: http.StatusInternalServerError,
	}
}

// Wrap wraps an existing error with additional context
func Wrap(err error, errType string, message string) *AppError {
	return &AppError{
		Type:       errType,
		Message:    message,
		Details:    err.Error(),
		Detail:     err.Error(),
		Raw:        err,
		HTTPStatus: getHTTPStatus(errType),
	}
}

// Forbidden creates a new authorization error for forbidden resources
func Forbidden(code string, message string) *AppError {
	return &AppError{
		Type:       AuthorizationError,
		Message:    message,
		Details:    code,
		Detail:     code,
		HTTPStatus: http.StatusForbidden,
	}
}

// GetHTTPStatus returns the HTTP status code to use
func (e *AppError) GetHTTPStatus() int {
	return e.HTTPStatus
}

func NewError(errType string, code string, message string, status int) error {
	return &AppError{
		Type:    errType,
		Message: message,
		Details: code,
	}
}

// NewExternalServiceError creates a new error for external service failures
func NewExternalServiceError(err error) *AppError {
	if err == nil {
		return nil
	}
	return &AppError{
		Type:    ExternalAPIError,
		Message: "External service error",
		Details: err.Error(),
	}
}

// InvalidStatusTransition creates a new validation error for invalid status transitions
func InvalidStatusTransition(currentStatus string, newStatus string) *AppError {
	return &AppError{
		Type:    ValidationError,
		Message: "Invalid status transition",
		Details: fmt.Sprintf("Cannot transition from %s to %s", currentStatus, newStatus),
	}
}

// NewConflictError creates an error for conflict situations like duplicate resources
func NewConflictError(code string, message string) *AppError {
	return &AppError{
		Type:       "CONFLICT",
		Message:    message,
		Details:    code,
		Detail:     code,
		HTTPStatus: http.StatusConflict,
	}
}

// Unauthorized creates a new authentication error
func Unauthorized(code string, message string) *AppError {
	return &AppError{
		Type:       AuthError,
		Message:    message,
		Details:    code,
		Detail:     code,
		HTTPStatus: http.StatusUnauthorized,
	}
}
