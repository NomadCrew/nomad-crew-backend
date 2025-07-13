// Package store provides re-exports of core store interfaces
package store

import (
	internalstore "github.com/NomadCrew/nomad-crew-backend/internal/store"
)

// Re-export internal store interfaces
type (
	// TripStore handles trip-related data operations
	TripStore = internalstore.TripStore
	// ChatStore defines the interface for chat operations
	ChatStore = internalstore.ChatStore
	// Transaction defines an interface for database transactions
	Transaction = internalstore.Transaction
	// TodoStore defines the interface for todo operations
	TodoStore = internalstore.TodoStore
)
