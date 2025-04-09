package store

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/types"
)

// LocationStore defines the interface for location database operations.
// It includes methods needed by the location service.
type LocationStore interface {
	// UpdateLocation updates or inserts a user's location and returns the updated location.
	UpdateLocation(ctx context.Context, userID string, update types.LocationUpdate) (*types.Location, error)

	// GetTripMemberLocations retrieves the latest location for each member of a given trip.
	GetTripMemberLocations(ctx context.Context, tripID string) ([]types.MemberLocation, error)

	// GetUserRole retrieves the role of a specific user within a specific trip.
	// This is used for permission checks.
	GetUserRole(ctx context.Context, tripID string, userID string) (types.MemberRole, error)
}
