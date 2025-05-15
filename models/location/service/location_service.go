package service // Changed from services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors" // Added apperrors
	"github.com/NomadCrew/nomad-crew-backend/internal/events"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/store" // Added store
	"github.com/NomadCrew/nomad-crew-backend/types"
)


// ManagementService handles location-related operations (Renamed from LocationService)
type ManagementService struct { // Renamed from LocationService
	store          store.LocationStore  // Changed from LocationDBInterface to store.LocationStore
	eventPublisher types.EventPublisher // Changed name from eventService
}

// Ensure ManagementService implements LocationManagementServiceInterface
var _ LocationManagementServiceInterface = (*ManagementService)(nil)

// NewManagementService creates a new ManagementService (Renamed from NewLocationService)
func NewManagementService(
	store store.LocationStore, // Changed from LocationDBInterface
	eventPublisher types.EventPublisher, // Changed name from eventService
) *ManagementService { // Renamed from NewLocationService
	return &ManagementService{
		store:          store,
		eventPublisher: eventPublisher,
	}
}

// UpdateLocation updates a user's location and publishes an event
func (s *ManagementService) UpdateLocation(ctx context.Context, userID string, update types.LocationUpdate) (*types.Location, error) {
	log := logger.GetLogger()

	// Validate the location data
	if err := s.validateLocationUpdate(update); err != nil {
		// Use apperrors for validation failure
		return nil, apperrors.ValidationFailed("invalid_location_data", err.Error())
	}

	// Store the location in the database
	location, err := s.store.UpdateLocation(ctx, userID, update) // Changed from s.locationDB
	if err != nil {
		log.Errorw("Failed to update location in database", "userID", userID, "error", err)
		// Use apperrors for database error
		return nil, apperrors.NewDatabaseError(err)
	}

	// Publish location update event
	if err := s.publishLocationUpdateEvent(ctx, location); err != nil {
		log.Errorw("Failed to publish location update event", "userID", userID, "error", err)
		// Non-critical error, so we don't return it, just log
	}

	return location, nil
}

// GetTripMemberLocations retrieves the latest locations for all members of a trip
// Added userID parameter and permission check
func (s *ManagementService) GetTripMemberLocations(ctx context.Context, tripID string, userID string) ([]types.MemberLocation, error) {
	log := logger.GetLogger()

	// Validate Permission: Check if the user is at least a member of the trip
	_, err := s.store.GetUserRole(ctx, tripID, userID) // Assuming store has GetUserRole
	if err != nil {
		// Handle specific errors like NotFound
		if appErr, ok := err.(*apperrors.AppError); ok && appErr.Type == apperrors.NotFoundError {
			log.Warnw("Access denied for GetTripMemberLocations", "userID", userID, "tripID", tripID)
			return nil, apperrors.Forbidden("access_denied", "User is not a member of this trip or trip does not exist")
		}
		// Handle other potential errors (e.g., database connection)
		log.Errorw("Failed to check user role for trip", "userID", userID, "tripID", tripID, "error", err)
		return nil, apperrors.Wrap(err, apperrors.ServerError, "failed to get trip member locations")
	}

	// Fetch locations if permission check passes
	locations, err := s.store.GetTripMemberLocations(ctx, tripID) // Changed from s.locationDB
	if err != nil {
		log.Errorw("Failed to get trip member locations from store", "tripID", tripID, "error", err)
		return nil, apperrors.NewDatabaseError(err)
	}
	return locations, nil
}

// validateLocationUpdate validates the location update data
func (s *ManagementService) validateLocationUpdate(update types.LocationUpdate) error {
	// Validate latitude (-90 to 90)
	if update.Latitude < -90 || update.Latitude > 90 {
		// Return specific validation error
		return fmt.Errorf("invalid latitude: %f", update.Latitude)
	}

	// Validate longitude (-180 to 180)
	if update.Longitude < -180 || update.Longitude > 180 {
		// Return specific validation error
		return fmt.Errorf("invalid longitude: %f", update.Longitude)
	}

	// Validate accuracy (must be positive)
	if update.Accuracy < 0 {
		// Return specific validation error
		return fmt.Errorf("invalid accuracy: %f", update.Accuracy)
	}

	// Validate timestamp (not too old or in the future)
	timestamp := time.UnixMilli(update.Timestamp)
	now := time.Now()

	// Allow a slightly larger window for clock skew, e.g., 2 hours past, 5 mins future
	if timestamp.Before(now.Add(-2 * time.Hour)) {
		// Return specific validation error
		return fmt.Errorf("timestamp too old: %v", timestamp)
	}

	if timestamp.After(now.Add(5 * time.Minute)) {
		// Return specific validation error
		return fmt.Errorf("timestamp in the future: %v", timestamp)
	}

	return nil
}

// publishLocationUpdateEvent publishes a location update event using the centralized helper
func (s *ManagementService) publishLocationUpdateEvent(ctx context.Context, location *types.Location) error {
	log := logger.GetLogger()

	// Convert location struct to map[string]interface{} for the payload
	var payloadMap map[string]interface{}
	locationJSON, err := json.Marshal(location)
	if err != nil {
		log.Errorw("Failed to marshal location data for event payload", "error", err, "userID", location.UserID)
		// Use apperrors for marshalling error - ServerError seems appropriate
		return apperrors.Wrap(err, apperrors.ServerError, "failed to marshal location data")
	}
	if err := json.Unmarshal(locationJSON, &payloadMap); err != nil {
		log.Errorw("Failed to unmarshal location data into map for event payload", "error", err, "userID", location.UserID)
		// Use apperrors for unmarshalling error - ServerError seems appropriate
		return apperrors.Wrap(err, apperrors.ServerError, "failed to unmarshal location payload")
	}

	// Publish using the centralized helper
	if pubErr := events.PublishEventWithContext(
		s.eventPublisher, // Changed from s.eventService
		ctx,
		string(types.EventTypeLocationUpdated),
		location.TripID,
		location.UserID,
		payloadMap,
		"location-management-service", // Updated service name
	); pubErr != nil {
		log.Warnw("Failed to publish location update event via helper", "error", pubErr, "userID", location.UserID)
		// Wrap the publishing error as ServerError
		return apperrors.Wrap(pubErr, apperrors.ServerError, "failed to publish location update event")
	}

	return nil
}
