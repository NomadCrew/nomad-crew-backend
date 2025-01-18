package models

import (
    "context"
    "github.com/NomadCrew/nomad-crew-backend/types"
)

// TripModelInterface defines the interface for trip-related business logic
type TripModelInterface interface {
    CreateTrip(ctx context.Context, trip *types.Trip) error
    GetTripByID(ctx context.Context, id string) (*types.Trip, error)
    UpdateTrip(ctx context.Context, id string, update *types.TripUpdate) error
    DeleteTrip(ctx context.Context, id string) error
    ListUserTrips(ctx context.Context, userid string) ([]*types.Trip, error)
    SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error)
}

// Verify TripModel implements TripModelInterface at compile time
var _ TripModelInterface = (*TripModel)(nil)