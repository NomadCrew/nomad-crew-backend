package errors

import (
	"fmt"
	"net/http"

	"github.com/NomadCrew/nomad-crew-backend/logger"
)

type ErrorType string

const (
	ValidationError ErrorType = "VALIDATION_ERROR"
	NotFoundError   ErrorType = "NOT_FOUND"
	AuthError       ErrorType = "AUTHENTICATION_ERROR"
	DatabaseError   ErrorType = "DATABASE_ERROR"
	ServerError     ErrorType = "SERVER_ERROR"
)

// AppError represents a structured application error
type AppError struct {
	Type       ErrorType `json:"type"`
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
	default:
		return http.StatusInternalServerError
	}
}
