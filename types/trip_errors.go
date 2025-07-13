package types

// TripErrorCode represents specific error conditions in the trip module
type TripErrorCode int

const (
	// ErrTripNotFound indicates the requested trip wasn't found
	ErrTripNotFound TripErrorCode = iota + 1
	// ErrUnauthorized indicates the user doesn't have permission for the operation
	ErrUnauthorized
	// ErrInvalidData indicates validation failed on provided data
	ErrInvalidData
	// ErrDatabaseOperation indicates a database operation failed
	ErrDatabaseOperation
	// ErrInvalidStatusTransition indicates an invalid trip status transition
	ErrInvalidStatusTransition
)

// TripError encapsulates trip-related errors with specific error codes
type TripError struct {
	Code TripErrorCode
	Msg  string
	Err  error
}

// Error satisfies the error interface
func (e *TripError) Error() string {
	if e.Msg != "" {
		return e.Msg
	}

	switch e.Code {
	case ErrTripNotFound:
		return "trip not found"
	case ErrUnauthorized:
		return "unauthorized access"
	case ErrInvalidData:
		return "invalid trip data"
	case ErrDatabaseOperation:
		return "database operation failed"
	case ErrInvalidStatusTransition:
		return "invalid trip status transition"
	default:
		return "unknown trip error"
	}
}

// Unwrap returns the wrapped error
func (e *TripError) Unwrap() error {
	return e.Err
}
