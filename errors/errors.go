package errors

import (
	"fmt"
	"net/http"

	"github.com/NomadCrew/nomad-crew-backend/logger"
)

type ErrorType string

const (
	ValidationError                  ErrorType = "VALIDATION_ERROR"
	NotFoundError                    ErrorType = "NOT_FOUND"
	AuthError                        ErrorType = "AUTHENTICATION_ERROR"
	DatabaseError                    ErrorType = "DATABASE_ERROR"
	ServerError                      ErrorType = "SERVER_ERROR"
	ForbiddenError                   ErrorType = "FORBIDDEN"
	TripNotFoundError                ErrorType = "TRIP_NOT_FOUND"
	TripAccessError                  ErrorType = "TRIP_ACCESS_DENIED"
	InvalidStatusTransitionError     ErrorType = "INVALID_STATUS_TRANSITION"
	ErrorTypeTripNotFound                      = "trip_not_found"
	ErrorTypeTripAccessDenied                  = "trip_access_denied"
	ErrorTypeValidation                        = "validation_failed"
	ErrorTypeInvalidStatusTransition           = "invalid_status_transition"
	ErrorTypeConflict                          = "CONFLICT"
)

// AppError represents a structured application error
type AppError struct {
	Type       ErrorType `json:"type"`
	Code       string    `json:"code"`
	Message    string    `json:"message"`
	Detail     string    `json:"detail,omitempty"`
	HTTPStatus int       `json:"-"`
	Raw        error     `json:"-"`
}

func (e *AppError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Type, e.Message, e.Detail)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// New creates a new AppError
func New(errType ErrorType, message string, detail string) *AppError {
	httpStatus := getHTTPStatus(errType)
	return &AppError{
		Type:       errType,
		Message:    message,
		Detail:     detail,
		HTTPStatus: httpStatus,
	}
}

// Wrap wraps a raw error with AppError context
func Wrap(err error, errType ErrorType, message string) *AppError {
	if err == nil {
		return nil
	}
	return &AppError{
		Type:       errType,
		Message:    message,
		Detail:     err.Error(),
		HTTPStatus: getHTTPStatus(errType),
		Raw:        err,
	}
}

// Helper functions for common errors
func NotFound(entity string, id interface{}) *AppError {
	return &AppError{
		Type:       NotFoundError,
		Message:    fmt.Sprintf("%s not found", entity),
		Detail:     fmt.Sprintf("ID: %v", id),
		HTTPStatus: http.StatusNotFound,
	}
}

func ValidationFailed(message string, details string) *AppError {
	return &AppError{
		Type:       ValidationError,
		Message:    message,
		Detail:     details,
		HTTPStatus: http.StatusBadRequest,
	}
}

func AuthenticationFailed(message string) *AppError {
	return &AppError{
		Type:       AuthError,
		Message:    message,
		HTTPStatus: http.StatusUnauthorized,
	}
}

func NewDatabaseError(err error) *AppError {
	// Log original error but return sanitized message
	logger.GetLogger().Errorw("Database error", "error", err)
	return &AppError{
		Type:       DatabaseError,
		Message:    "Database operation failed",
		Detail:     "Please try again later",
		HTTPStatus: 500,
		Raw:        err,
	}
}

func InternalServerError(message string) *AppError {
	return &AppError{
		Type:       ServerError,
		Message:    message,
		HTTPStatus: http.StatusInternalServerError,
	}
}

func Forbidden(message string, details string) *AppError {
	return &AppError{
		Type:       ForbiddenError,
		Message:    message,
		Detail:     details,
		HTTPStatus: http.StatusForbidden,
	}
}

func TripNotFound(id string) *AppError {
	return &AppError{
		Type:       TripNotFoundError,
		Message:    "Trip not found",
		Detail:     fmt.Sprintf("Trip ID: %s", id),
		HTTPStatus: http.StatusNotFound,
	}
}

func TripAccessDenied(userID, tripID string) *AppError {
	return &AppError{
		Type:       TripAccessError,
		Message:    "Access to trip denied",
		Detail:     fmt.Sprintf("User %s cannot access trip %s", userID, tripID),
		HTTPStatus: http.StatusForbidden,
	}
}

func InvalidStatusTransition(current, new string) *AppError {
	return &AppError{
		Type:       InvalidStatusTransitionError,
		Message:    "Invalid status transition",
		Detail:     fmt.Sprintf("Cannot transition from %s to %s", current, new),
		HTTPStatus: http.StatusBadRequest,
	}
}

func NewConflictError(message string, detail string) *AppError {
	return &AppError{
		Type:       ErrorTypeConflict,
		Message:    message,
		Detail:     detail,
		HTTPStatus: http.StatusConflict,
	}
}

func Unauthorized(code, message string) error {
	return NewError(
		"unauthorized",
		code,
		message,
		http.StatusUnauthorized,
	)
}

func getHTTPStatus(errType ErrorType) int {
	switch errType {
	case ValidationError:
		return http.StatusBadRequest
	case NotFoundError:
		return http.StatusNotFound
	case AuthError:
		return http.StatusUnauthorized
	case DatabaseError:
		return http.StatusInternalServerError
	case ForbiddenError:
		return http.StatusForbidden
	case TripNotFoundError:
		return http.StatusNotFound
	case TripAccessError:
		return http.StatusForbidden
	case InvalidStatusTransitionError:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func NewError(errType ErrorType, code string, message string, status int) error {
	return &AppError{
		Type:       errType,
		Code:       code,
		Message:    message,
		HTTPStatus: status,
	}
}
