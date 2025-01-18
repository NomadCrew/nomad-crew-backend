package store

import (
    "context"
    "github.com/NomadCrew/nomad-crew-backend/types"
    "github.com/jackc/pgx/v4/pgxpool"
)

// Store provides a unified interface for all data operations
type Store interface {
    Trip() TripStore
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