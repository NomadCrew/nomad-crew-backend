package errors

import (
	"fmt"

	"github.com/NomadCrew/nomad-crew-backend/errors"
)

// Error codes for trip-related operations.
// NOTE: These error codes are trip-domain specific by design.
// Generic error codes exist in errors/errors.go for cross-domain use.
const (
	// Authorization errors
	ErrUserNotFound = "USER_NOT_FOUND"
	ErrUnauthorized = "UNAUTHORIZED"
	ErrForbidden    = "FORBIDDEN"
	ErrInvalidRole  = "INVALID_ROLE"

	// Trip errors
	ErrTripNotFound    = "TRIP_NOT_FOUND"
	ErrInvalidTripData = "INVALID_TRIP_DATA"

	// Member errors
	ErrMemberNotFound = "MEMBER_NOT_FOUND"
	ErrMemberExists   = "MEMBER_ALREADY_EXISTS"

	// Invitation errors
	ErrInvitationNotFound = "INVITATION_NOT_FOUND"
	ErrInvalidInvitation  = "INVALID_INVITATION"

	// Chat errors
	ErrChatMessageNotFound = "CHAT_MESSAGE_NOT_FOUND"

	// Operation errors
	ErrOperationFailed = "OPERATION_FAILED"
)

// NewUnauthorizedError creates a new unauthorized error
func NewUnauthorizedError(message string) *errors.AppError {
	return &errors.AppError{
		Type:    errors.AuthError,
		Message: message,
	}
}

// NewForbiddenError creates a new forbidden error
func NewForbiddenError(message string) *errors.AppError {
	return &errors.AppError{
		Type:    errors.AuthorizationError,
		Message: message,
	}
}

// NewNotFoundError creates a new not found error
func NewNotFoundError(entity, id string) *errors.AppError {
	return &errors.AppError{
		Type:    errors.NotFoundError,
		Message: fmt.Sprintf("%s not found", entity),
		Details: id,
	}
}

// NewInvalidDataError creates a new invalid data error
func NewInvalidDataError(message string) *errors.AppError {
	return &errors.AppError{
		Type:    errors.ValidationError,
		Message: message,
	}
}

// NewOperationFailedError creates a new operation failed error
func NewOperationFailedError(message string, err error) *errors.AppError {
	detail := ""
	if err != nil {
		detail = err.Error()
	}

	return &errors.AppError{
		Type:    errors.ServerError,
		Message: message,
		Details: detail,
	}
}
