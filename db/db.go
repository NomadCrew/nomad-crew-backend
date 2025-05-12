// Package db provides database access functionality.
// Note: This package is being phased out in favor of the store/postgres package.
// Please use the appropriate store implementation from store/postgres instead.
package db

import (
	"context"
	"fmt"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Store provides access to database repositories.
// Deprecated: This Store struct seems partially implemented or outdated.
// Consider using individual store implementations (like PostgresChatStore, TripDB, etc.)
// injected directly where needed, rather than this aggregate Store.
type Store struct {
	pool  *pgxpool.Pool
	Trips TripRepository // Example repository; add others if this Store is used.
}

// NewStore creates a new database store.
// Deprecated: See note on the Store struct.
func NewStore(pool *pgxpool.Pool) *Store {
	store := &Store{
		pool: pool,
	}
	// Initialize repositories and assign them to the store fields.
	// Ensure TripRepo is defined elsewhere or update this initialization.
	// store.Trips = &TripRepo{store}
	return store
}

// WithTx executes the provided function `fn` within a database transaction.
// It begins a transaction, executes `fn` with the transaction object, and commits
// the transaction if `fn` returns nil. If `fn` returns an error or if the commit
// fails, it rolls back the transaction.
// It ensures rollback happens even if `fn` panics (though panics should be avoided).
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	committed := false // Track commit status for deferred rollback
	defer func() {
		if !committed {
			// Use a separate context for rollback in case the original ctx is canceled.
			rollbackCtx := context.Background()
			if rollbackErr := tx.Rollback(rollbackCtx); rollbackErr != nil {
				// Log rollback error, especially if it masks the original function error.
				logger.GetLogger().Errorw("Failed to rollback transaction",
					"rollbackError", rollbackErr,
					"originalError", err) // Log original error from fn if available
			}
		}
	}()

	// Execute the provided function with the transaction.
	err = fn(tx)
	if err != nil {
		return err // Error occurred in fn, defer will rollback
	}

	// Commit the transaction if fn was successful.
	if commitErr := tx.Commit(ctx); commitErr != nil {
		return fmt.Errorf("failed to commit transaction: %w", commitErr)
	}
	committed = true // Mark as committed so defer doesn't rollback
	return nil
}

// TripRepository defines the interface for database operations related to trips.
// Deprecated: Interfaces for specific data stores (like TripStore) are typically
// defined in the `internal/store` package.
// Consider using store.TripStore instead.
type TripRepository interface {
	Create(ctx context.Context, trip *types.Trip) (string, error)
	GetByID(ctx context.Context, id string) (*types.Trip, error)
	Update(ctx context.Context, id string, update *types.TripUpdate) (*types.Trip, error)
	Delete(ctx context.Context, id string) error
}
