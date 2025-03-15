package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
)

// LocationDBInterface defines the interface for location database operations
type LocationDBInterface interface {
	UpdateLocation(ctx context.Context, userID string, update types.LocationUpdate) (*types.Location, error)
	GetTripMemberLocations(ctx context.Context, tripID string) ([]types.MemberLocation, error)
}

// OfflineLocationServiceInterface defines the interface for offline location operations
type OfflineLocationServiceInterface interface {
	SaveOfflineLocations(ctx context.Context, userID string, updates []types.LocationUpdate, deviceID string) error
	ProcessOfflineLocations(ctx context.Context, userID string) error
}

// LocationService handles location-related operations
type LocationService struct {
	locationDB     LocationDBInterface
	eventService   types.EventPublisher
	offlineService OfflineLocationServiceInterface
}

// NewLocationService creates a new LocationService
func NewLocationService(locationDB LocationDBInterface, eventService types.EventPublisher) *LocationService {
	service := &LocationService{
		locationDB:   locationDB,
		eventService: eventService,
	}

	return service
}

// SetOfflineService sets the offline location service
// This is needed to avoid circular dependencies
func (s *LocationService) SetOfflineService(offlineService OfflineLocationServiceInterface) {
	s.offlineService = offlineService
}

// UpdateLocation updates a user's location and publishes an event
func (s *LocationService) UpdateLocation(ctx context.Context, userID string, update types.LocationUpdate) (*types.Location, error) {
	log := logger.GetLogger()

	// Validate the location data
	if err := s.validateLocationUpdate(update); err != nil {
		return nil, err
	}

	// Store the location in the database
	location, err := s.locationDB.UpdateLocation(ctx, userID, update)
	if err != nil {
		log.Errorw("Failed to update location in database", "userID", userID, "error", err)
		return nil, err
	}

	// Publish location update event
	if err := s.publishLocationUpdateEvent(ctx, location); err != nil {
		log.Warnw("Failed to publish location update event", "userID", userID, "error", err)
		// Continue even if event publishing fails
	}

	return location, nil
}

// GetTripMemberLocations retrieves the latest locations for all members of a trip
func (s *LocationService) GetTripMemberLocations(ctx context.Context, tripID string) ([]types.MemberLocation, error) {
	return s.locationDB.GetTripMemberLocations(ctx, tripID)
}

// SaveOfflineLocations saves a batch of location updates to be processed later
func (s *LocationService) SaveOfflineLocations(ctx context.Context, userID string, updates []types.LocationUpdate, deviceID string) error {
	if s.offlineService == nil {
		return fmt.Errorf("offline service not initialized")
	}
	return s.offlineService.SaveOfflineLocations(ctx, userID, updates, deviceID)
}

// ProcessOfflineLocations processes all offline location updates for a user
func (s *LocationService) ProcessOfflineLocations(ctx context.Context, userID string) error {
	if s.offlineService == nil {
		return fmt.Errorf("offline service not initialized")
	}
	return s.offlineService.ProcessOfflineLocations(ctx, userID)
}

// validateLocationUpdate validates the location update data
func (s *LocationService) validateLocationUpdate(update types.LocationUpdate) error {
	// Validate latitude (-90 to 90)
	if update.Latitude < -90 || update.Latitude > 90 {
		return fmt.Errorf("invalid latitude: %f", update.Latitude)
	}

	// Validate longitude (-180 to 180)
	if update.Longitude < -180 || update.Longitude > 180 {
		return fmt.Errorf("invalid longitude: %f", update.Longitude)
	}

	// Validate accuracy (must be positive)
	if update.Accuracy < 0 {
		return fmt.Errorf("invalid accuracy: %f", update.Accuracy)
	}

	// Validate timestamp (not too old or in the future)
	timestamp := time.UnixMilli(update.Timestamp)
	now := time.Now()

	// Check if timestamp is more than 1 hour in the past
	if timestamp.Before(now.Add(-1 * time.Hour)) {
		return fmt.Errorf("timestamp too old: %v", timestamp)
	}

	// Check if timestamp is more than 1 minute in the future
	if timestamp.After(now.Add(1 * time.Minute)) {
		return fmt.Errorf("timestamp in the future: %v", timestamp)
	}

	return nil
}

// publishLocationUpdateEvent publishes a location update event
func (s *LocationService) publishLocationUpdateEvent(ctx context.Context, location *types.Location) error {
	// Create event payload
	payload, err := json.Marshal(location)
	if err != nil {
		return fmt.Errorf("failed to marshal location: %w", err)
	}

	// Create and publish the event
	event := types.Event{
		BaseEvent: types.BaseEvent{
			ID:        uuid.New().String(),
			Type:      types.EventTypeLocationUpdated,
			TripID:    location.TripID,
			UserID:    location.UserID,
			Timestamp: time.Now(),
			Version:   1,
		},
		Metadata: types.EventMetadata{
			Source: "location_service",
		},
		Payload: payload,
	}

	return s.eventService.Publish(ctx, location.TripID, event)
}
