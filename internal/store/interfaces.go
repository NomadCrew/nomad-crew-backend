package store

import (
    "context"
    "github.com/jackc/pgx/v4/pgxpool"
    "github.com/NomadCrew/nomad-crew-backend/types"
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
    AddMember(ctx context.Context, membership *types.TripMembership) error
    UpdateMemberRole(ctx context.Context, tripID string, userID string, role types.MemberRole) error
    RemoveMember(ctx context.Context, tripID string, userID string) error
    GetTripMembers(ctx context.Context, tripID string) ([]types.TripMembership, error)
    GetUserRole(ctx context.Context, tripID string, userID string) (types.MemberRole, error)
}