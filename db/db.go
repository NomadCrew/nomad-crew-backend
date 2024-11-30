package db

import (
    "context"
    "github.com/jackc/pgx/v4/pgxpool"
    "github.com/NomadCrew/nomad-crew-backend/types"
)

// Store provides access to all database operations
type Store struct {
    pool *pgxpool.Pool
    Users UserRepository
    Trips TripRepository
}

// NewStore creates a new database store
func NewStore(pool *pgxpool.Pool) *Store {
    store := &Store{
        pool: pool,
    }
    store.Users = &UserRepo{store}
    store.Trips = &TripRepo{store}
    return store
}

// UserRepository defines the interface for user operations
type UserRepository interface {
    Create(ctx context.Context, user *types.User) error
    GetByID(ctx context.Context, id int64) (*types.User, error)
    Update(ctx context.Context, user *types.User) error
    Delete(ctx context.Context, id int64) error
    GetByEmail(ctx context.Context, email string) (*types.User, error)
}

// TripRepository defines the interface for trip operations
type TripRepository interface {
    Create(ctx context.Context, trip *types.Trip) (int64, error)
    GetByID(ctx context.Context, id int64) (*types.Trip, error)
    Update(ctx context.Context, id int64, update *types.TripUpdate) error
    Delete(ctx context.Context, id int64) error
}
