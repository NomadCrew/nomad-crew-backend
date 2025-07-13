package handlers

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/types"
)

// TripServiceInterface defines the trip service methods needed by handlers
type TripServiceInterface interface {
	IsTripMember(ctx context.Context, tripID, userID string) (bool, error)
	GetTripMember(ctx context.Context, tripID, userID string) (*types.TripMembership, error)
	GetTripForSync(ctx context.Context, tripID string) (*types.Trip, error)
}
