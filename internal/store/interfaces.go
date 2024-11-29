package store

import (
    "context"
    "github.com/jackc/pgx/v4/pgxpool"
    "github.com/NomadCrew/nomad-crew-backend/types"
)

// Store provides a unified interface for all data operations
type Store interface {
    User() UserStore
    Trip() TripStore
}

// UserStore handles user-related data operations
type UserStore interface {
    GetPool() *pgxpool.Pool
    SaveUser(ctx context.Context, user *types.User) error
    GetUserByID(ctx context.Context, id int64) (*types.User, error) 
    UpdateUser(ctx context.Context, user *types.User) error
    DeleteUser(ctx context.Context, id int64) error
    AuthenticateUser(ctx context.Context, email string) (*types.User, error)
}

// TripStore handles trip-related data operations
type TripStore interface {
    GetPool() *pgxpool.Pool
    CreateTrip(ctx context.Context, trip types.Trip) (int64, error)
    GetTrip(ctx context.Context, id int64) (*types.Trip, error)
    UpdateTrip(ctx context.Context, tripID int64, update types.TripUpdate) error
    SoftDeleteTrip(ctx context.Context, tripID int64) error
}