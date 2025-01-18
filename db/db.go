package db

import (
    "context"
    "github.com/jackc/pgx/v4/pgxpool"
    "github.com/NomadCrew/nomad-crew-backend/types"
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

// TripRepository defines the interface for trip operations
type TripRepository interface {
    Create(ctx context.Context, trip *types.Trip) (string, error)
    GetByID(ctx context.Context, id string) (*types.Trip, error)
    Update(ctx context.Context, id string, update *types.TripUpdate) error
    Delete(ctx context.Context, id string) error
}