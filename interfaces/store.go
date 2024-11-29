package interfaces

import (
    "context"
    "github.com/jackc/pgx/v4/pgxpool"
    "github.com/NomadCrew/nomad-crew-backend/types"
)

// UserStore defines the interface for user storage operations
type UserStore interface {
    SaveUser(ctx context.Context, user *types.User) error
    GetUserByID(ctx context.Context, id int64) (*types.User, error)
    UpdateUser(ctx context.Context, user *types.User) error
    DeleteUser(ctx context.Context, id int64) error
    AuthenticateUser(ctx context.Context, email string) (*types.User, error)
}

// TripStore defines the interface for trip storage operations
type TripStore interface {
    GetPool() *pgxpool.Pool
    CreateTrip(ctx context.Context, trip types.Trip) (int64, error)
    GetTrip(ctx context.Context, id int64) (*types.Trip, error)
    UpdateTrip(ctx context.Context, tripID int64, update types.TripUpdate) error
    SoftDeleteTrip(ctx context.Context, tripID int64) error
}