package store

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v4/pgxpool"
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
	GetUserByID(ctx context.Context, id string) (*types.User, error)
	UpdateUser(ctx context.Context, user *types.User) error
	DeleteUser(ctx context.Context, id string) error
	AuthenticateUser(ctx context.Context, email string) (*types.User, error)
}

// TripStore handles trip-related data operations
type TripStore interface {
	GetPool() *pgxpool.Pool
	CreateTrip(ctx context.Context, trip types.Trip) (string, error)
	GetTrip(ctx context.Context, id string) (*types.Trip, error)
	UpdateTrip(ctx context.Context, id string, update types.TripUpdate) error
	SoftDeleteTrip(ctx context.Context, id string) error
	ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error)
	SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error)
}
