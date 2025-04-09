package service

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/types"
)

// OfflineLocationServiceInterface defines the interface for offline location operations
type OfflineLocationServiceInterface interface {
	SaveOfflineLocations(ctx context.Context, userID string, updates []types.LocationUpdate, deviceID string) error
	ProcessOfflineLocations(ctx context.Context, userID string) error
}
