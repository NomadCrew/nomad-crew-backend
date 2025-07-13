package service

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/types"
)

// LocationUpdateProcessor defines the interface for processing location updates
type LocationUpdateProcessor interface {
	// UpdateLocation updates a user's location and publishes an event
	UpdateLocation(ctx context.Context, userID string, update types.LocationUpdate) (*types.Location, error)
}

// LocationManagementServiceInterface defines the full interface for the location management service
// This interface extends LocationUpdateProcessor with additional methods
type LocationManagementServiceInterface interface {
	LocationUpdateProcessor

	// GetTripMemberLocations retrieves the latest locations for all members of a trip
	GetTripMemberLocations(ctx context.Context, tripID string, userID string) ([]types.MemberLocation, error)
}
