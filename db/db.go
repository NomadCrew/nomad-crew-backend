package db

import (
    "context"
    "fmt"
    "github.com/jackc/pgx/v4/pgxpool"
    "github.com/jackc/pgx/v4"
    "github.com/NomadCrew/nomad-crew-backend/types"
    "github.com/NomadCrew/nomad-crew-backend/logger"
)

// Store provides access to all database operations
type Store struct {
    pool *pgxpool.Pool
    Trips TripRepository
}

// NewStore creates a new database store
func NewStore(pool *pgxpool.Pool) *Store {
    store := &Store{
        pool: pool,
    }
    store.Trips = &TripRepo{store}
    return store
}

func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
    tx, err := pool.Begin(ctx)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }

    var committed bool
    defer func() {
        if !committed {
            if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
                log := logger.GetLogger()
                log.Errorw("Failed to rollback transaction", 
                    "error", rollbackErr,
                    "originalError", err)
            }
        }
    }()

    if err := fn(tx); err != nil {
        return err
    }

    if err := tx.Commit(ctx); err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }
    committed = true
    return nil
}

// TripRepository defines the interface for trip operations
type TripRepository interface {
    Create(ctx context.Context, trip *types.Trip) (string, error)
    GetByID(ctx context.Context, id string) (*types.Trip, error)
    Update(ctx context.Context, id string, update *types.TripUpdate) error
    Delete(ctx context.Context, id string) error
}