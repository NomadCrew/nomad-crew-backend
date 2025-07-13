package store

import "errors"

// Predefined errors for the store layer.
var (
	// ErrNotFound indicates that a requested resource was not found.
	ErrNotFound = errors.New("resource not found")

	// ErrForbidden indicates that the user is not authorized to perform the requested action.
	ErrForbidden = errors.New("forbidden")

	// ErrConflict indicates a conflict, e.g., trying to create a resource that already exists.
	ErrConflict = errors.New("conflict")

	// Add other common store-level errors here as needed.
)