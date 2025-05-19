package service

import (
	"context"
	"fmt"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/events"
	istore "github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/internal/utils"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

// TripManagementService handles core trip operations
type TripManagementService struct {
	store          istore.TripStore
	eventPublisher types.EventPublisher
	weatherSvc     types.WeatherServiceInterface
}

// NewTripManagementService creates a new trip management service
func NewTripManagementService(
	store istore.TripStore,
	eventPublisher types.EventPublisher,
	weatherSvc types.WeatherServiceInterface,
) *TripManagementService {
	return &TripManagementService{
		store:          store,
		eventPublisher: eventPublisher,
		weatherSvc:     weatherSvc,
	}
}

// CreateTrip creates a new trip and returns the created trip object
func (s *TripManagementService) CreateTrip(ctx context.Context, trip *types.Trip) (*types.Trip, error) {
	// Do NOT overwrite trip.CreatedBy here; it should already be set to the internal UUID by the handler
	logger.GetLogger().Infow("[DEBUG] trip.CreatedBy in service before DB call", "type", fmt.Sprintf("%T", trip.CreatedBy), "value", func() string {
		if trip.CreatedBy != nil {
			return *trip.CreatedBy
		} else {
			return "<nil>"
		}
	}())

	// Create the trip
	tripID, err := s.store.CreateTrip(ctx, *trip)
	if err != nil {
		return nil, err
	}

	// Update the trip ID
	trip.ID = tripID

	// Add creator as an owner
	membership := &types.TripMembership{
		TripID: trip.ID,
		UserID: func() string {
			if trip.CreatedBy != nil {
				return *trip.CreatedBy
			} else {
				return ""
			}
		}(),
		Role: types.MemberRoleOwner,
	}
	if err := s.store.AddMember(ctx, membership); err != nil {
		// Consider potential rollback/cleanup here if needed
		return nil, err
	}

	// Publish event
	var createdByUserID string
	if trip.CreatedBy != nil {
		createdByUserID = *trip.CreatedBy
	}
	err = events.PublishEventWithContext(
		s.eventPublisher,
		ctx,
		EventTypeTripCreated,
		trip.ID,
		createdByUserID,
		map[string]interface{}{
			"event_id": utils.GenerateEventID(),
			"tripName": trip.Name,
		},
		"trip-management-service",
	)
	if err != nil {
		// Log the error but return the created trip, as event publishing might be non-critical
		logger.GetLogger().Warnw("Failed to publish trip created event", "error", err, "tripID", trip.ID)
	}

	// Trigger weather update if the trip starts as active and has a valid destination
	if trip.Status == types.TripStatusActive && s.shouldUpdateWeather(trip) {
		if err := s.triggerWeatherUpdate(ctx, trip); err != nil {
			log := logger.GetLogger()
			log.Errorw("Failed to trigger weather update during trip creation", "error", err, "tripID", trip.ID)
			// Decide if this error should be returned or just logged. For now, just logging.
		}
	}

	// Return the created trip object
	return trip, nil
}

// GetTrip retrieves a single trip by ID, ensuring the user has permission
func (s *TripManagementService) GetTrip(ctx context.Context, id string, userID string) (*types.Trip, error) {
	// Validate Permission: Check if the user is at least a member of the trip
	role, err := s.store.GetUserRole(ctx, id, userID)
	if err != nil {
		// If error fetching role (e.g., user not found, trip not found), return appropriate error
		// Assuming GetUserRole returns a specific error type for not found / not member
		if appErr, ok := err.(*apperrors.AppError); ok && appErr.Type == apperrors.NotFoundError {
			return nil, apperrors.Forbidden("access_denied", "User is not a member of this trip or trip does not exist")
		}
		// Otherwise, return the original error (e.g., database error)
		return nil, err
	}

	// If a role is found, the user is at least a member. No need for specific role check here as Member is the minimum requirement.
	_ = role // Use role if more specific checks are needed later

	// Fetch the trip from the store
	trip, err := s.store.GetTrip(ctx, id)
	if err != nil {
		// This might indicate the trip was deleted between the role check and now, or another DB issue
		return nil, apperrors.NotFound("Trip", id)
	}
	return trip, nil
}

// getTripInternal retrieves a single trip by ID without permission checks (for internal service use)
func (s *TripManagementService) getTripInternal(ctx context.Context, id string) (*types.Trip, error) {
	trip, err := s.store.GetTrip(ctx, id)
	if err != nil {
		// Return NotFound specifically if that's the error from the store
		if appErr, ok := err.(*apperrors.AppError); ok && appErr.Type == apperrors.NotFoundError {
			return nil, apperrors.NotFound("Trip", id)
		}
		// Otherwise, wrap as a database error or return as is
		return nil, apperrors.Wrap(err, apperrors.DatabaseError, "failed to get trip from store internally")
	}
	return trip, nil
}

// UpdateTrip updates an existing trip, requiring owner permissions
// Added userID, changed return
func (s *TripManagementService) UpdateTrip(ctx context.Context, id string, userID string, updateData types.TripUpdate) (*types.Trip, error) {
	// Validate Permission: Check if the user is an owner of the trip
	role, err := s.store.GetUserRole(ctx, id, userID)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok && appErr.Type == apperrors.NotFoundError {
			return nil, apperrors.Forbidden("access_denied", "User not found or not a member of this trip")
		}
		return nil, err // Database or other error
	}
	if role != types.MemberRoleOwner {
		return nil, apperrors.Forbidden("permission_denied", "User must be an owner to update the trip")
	}

	// Check if the trip exists (redundant check if GetUserRole already confirmed trip existence, but safe)
	existingTrip, err := s.store.GetTrip(ctx, id)
	if err != nil {
		return nil, apperrors.NotFound("Trip", id)
	}

	// Store a copy for later comparison
	// originalTrip := *existingTrip // Removed as it was unused after refactor

	// Apply updates
	if updateData.Name != nil {
		existingTrip.Name = *updateData.Name
	}
	if updateData.Description != nil {
		existingTrip.Description = *updateData.Description
	}
	// Destination fields should be updated individually if present in updateData
	if updateData.DestinationPlaceID != nil {
		existingTrip.DestinationPlaceID = updateData.DestinationPlaceID
	}
	if updateData.DestinationAddress != nil {
		existingTrip.DestinationAddress = updateData.DestinationAddress
	}
	if updateData.DestinationName != nil {
		existingTrip.DestinationName = updateData.DestinationName
	}
	if updateData.DestinationLatitude != nil {
		existingTrip.DestinationLatitude = *updateData.DestinationLatitude
	}
	if updateData.DestinationLongitude != nil {
		existingTrip.DestinationLongitude = *updateData.DestinationLongitude
	}
	if updateData.StartDate != nil {
		existingTrip.StartDate = *updateData.StartDate
	}
	if updateData.EndDate != nil {
		existingTrip.EndDate = *updateData.EndDate
	}

	// Validate dates if both are being updated or one is updated to conflict with existing
	currentStartDate := existingTrip.StartDate
	currentEndDate := existingTrip.EndDate
	if updateData.StartDate != nil {
		currentStartDate = *updateData.StartDate
	}
	if updateData.EndDate != nil {
		currentEndDate = *updateData.EndDate
	}

	if !currentStartDate.IsZero() && !currentEndDate.IsZero() && currentEndDate.Before(currentStartDate) {
		return nil, apperrors.ValidationFailed("invalid_dates", "trip end date cannot be before start date")
	}

	if updateData.Status != nil { // Check if pointer is not nil
		// Further validation for status transition should happen here or before if complex
		existingTrip.Status = *updateData.Status // Dereference pointer for assignment
	}

	// Update the trip in the store using the original updateData
	// The store method is responsible for applying the partial update correctly.
	updatedTrip, err := s.store.UpdateTrip(ctx, id, updateData)
	if err != nil {
		return nil, err
	}

	// If critical weather-related fields changed, or if trip became active, trigger weather update
	if s.hasWeatherCriticalChanges(existingTrip, updatedTrip) && s.shouldUpdateWeather(updatedTrip) {
		log := logger.GetLogger()
		log.Debugw("Weather critical fields changed, triggering update", "tripID", id)
		if err := s.triggerWeatherUpdate(ctx, updatedTrip); err != nil {
			log.Errorw("Failed to trigger weather update during trip update", "error", err, "tripID", id)
			// Decide if this error should be returned or just logged. For now, just logging.
		}
	}

	// Publish event
	// Use the userID from the parameter for the event
	err = events.PublishEventWithContext(
		s.eventPublisher,
		ctx,
		EventTypeTripUpdated,
		id,
		userID, // Use the authenticated user ID
		map[string]interface{}{
			"event_id": utils.GenerateEventID(),
		},
		"trip-management-service",
	)
	if err != nil {
		// Log error but continue, return the updated trip
		logger.GetLogger().Warnw("Failed to publish trip updated event", "error", err, "tripID", id)
	}

	return updatedTrip, nil
}

// DeleteTrip deletes a trip
func (s *TripManagementService) DeleteTrip(ctx context.Context, id string) error {
	// Check if the trip exists
	_, err := s.store.GetTrip(ctx, id)
	if err != nil {
		return apperrors.NotFound("Trip", id)
	}

	// Delete the trip (using soft delete)
	if err := s.store.SoftDeleteTrip(ctx, id); err != nil {
		return err
	}

	// Publish event
	userID, err := utils.GetUserIDFromContext(ctx)
	if err != nil {
		return err
	}

	err = events.PublishEventWithContext(
		s.eventPublisher,
		ctx,
		EventTypeTripDeleted,
		id,
		userID,
		map[string]interface{}{
			"event_id": utils.GenerateEventID(),
		},
		"trip-management-service",
	)
	if err != nil {
		return err
	}

	return nil
}

// ListUserTrips lists all trips for a user
func (s *TripManagementService) ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error) {
	return s.store.ListUserTrips(ctx, userID)
}

// SearchTrips searches for trips based on criteria
func (s *TripManagementService) SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error) {
	return s.store.SearchTrips(ctx, criteria)
}

// UpdateTripStatus updates the status of a trip
func (s *TripManagementService) UpdateTripStatus(ctx context.Context, tripID, userID string, newStatus types.TripStatus) error {
	log := logger.GetLogger()

	// Validate Permission: Check if the user is an owner of the trip
	role, err := s.store.GetUserRole(ctx, tripID, userID)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok && appErr.Type == apperrors.NotFoundError {
			return apperrors.Forbidden("access_denied", "User not found or not a member of this trip")
		}
		return err // Database or other error
	}
	if role != types.MemberRoleOwner && role != types.MemberRoleAdmin { // Allow Admins too
		return apperrors.Forbidden("permission_denied", "User must be an owner or admin to update trip status")
	}

	// Get the current trip for status transition validation
	currentTrip, err := s.store.GetTrip(ctx, tripID)
	if err != nil {
		return apperrors.NotFound("Trip", tripID)
	}

	// Validate the status transition (e.g., cannot complete a trip not yet active)
	if !currentTrip.Status.IsValidTransition(newStatus) {
		return apperrors.InvalidStatusTransition(string(currentTrip.Status), string(newStatus))
	}

	// Update the trip status using the UpdateTrip store method
	update := types.TripUpdate{
		Status: &newStatus,
	}
	updatedTrip, err := s.store.UpdateTrip(ctx, tripID, update) // updatedTrip is now used
	if err != nil {
		log.Errorw("Failed to update trip status in store", "error", err, "tripID", tripID, "newStatus", newStatus)
		return apperrors.NewDatabaseError(err)
	}

	// Trigger weather update if the trip becomes active and has a valid destination
	if newStatus == types.TripStatusActive && s.shouldUpdateWeather(updatedTrip) {
		if err := s.triggerWeatherUpdate(ctx, updatedTrip); err != nil {
			log.Errorw("Failed to trigger weather update during status change to active", "error", err, "tripID", updatedTrip.ID)
			// Decide if this error should be returned or just logged. For now, just logging.
		}
	}

	// Publish event
	return events.PublishEventWithContext(
		s.eventPublisher,
		ctx,
		string(types.EventTypeTripStatusUpdated),
		tripID,
		userID,
		map[string]interface{}{
			"event_id":   utils.GenerateEventID(),
			"old_status": string(currentTrip.Status),
			"new_status": string(newStatus),
		},
		"trip-management-service",
	)
}

// GetTripWithMembers gets a trip with its members
func (s *TripManagementService) GetTripWithMembers(ctx context.Context, tripID string, userID string) (*types.TripWithMembers, error) {
	// Get basic trip details, performing permission check
	trip, err := s.GetTrip(ctx, tripID, userID)
	if err != nil {
		return nil, err
	}

	// Get members
	members, err := s.store.GetTripMembers(ctx, tripID)
	if err != nil {
		return nil, err
	}

	// Convert []types.TripMembership to []*types.TripMembership
	memberPtrs := make([]*types.TripMembership, len(members))
	for i := range members {
		memberPtrs[i] = &members[i]
	}

	return &types.TripWithMembers{
		Trip:    *trip,
		Members: memberPtrs, // Use the slice of pointers
	}, nil
}

// TriggerWeatherUpdate explicitly triggers a weather update for a trip
func (s *TripManagementService) TriggerWeatherUpdate(ctx context.Context, tripID string) error {
	log := logger.GetLogger()
	// Use internal getter without permission checks
	trip, err := s.getTripInternal(ctx, tripID)
	if err != nil {
		log.Errorw("Failed to get trip for weather trigger", "error", err, "tripID", tripID)
		return err
	}
	if s.shouldUpdateWeather(trip) {
		if err := s.triggerWeatherUpdate(ctx, trip); err != nil {
			log.Errorw("Failed to manually trigger weather update", "error", err, "tripID", tripID)
			return err // Propagate the error
		}
		log.Infow("Manually triggered weather update", "tripID", tripID)
		return nil
	}
	log.Infow("Skipped manual weather trigger", "tripID", tripID, "reason", "shouldUpdateWeather returned false")
	return apperrors.ValidationFailed("weather_update_skipped", "Trip conditions not met for weather update")
}

// shouldUpdateWeather checks if a trip is in a state that warrants weather updates (e.g., active and has a destination)
func (s *TripManagementService) shouldUpdateWeather(trip *types.Trip) bool {
	if trip == nil {
		return false
	}
	// Only update weather for active or planning trips with valid destination coordinates
	isValidDest := trip.DestinationLatitude != 0 || trip.DestinationLongitude != 0
	return (trip.Status == types.TripStatusActive || trip.Status == types.TripStatusPlanning) && isValidDest
}

// hasWeatherCriticalChanges checks if destination or status relevant to weather has changed.
func (s *TripManagementService) hasWeatherCriticalChanges(oldTrip, newTrip *types.Trip) bool {
	if oldTrip == nil || newTrip == nil {
		return oldTrip != newTrip // If one is nil and other isn't, it's a change
	}
	// Check for changes in coordinates
	if oldTrip.DestinationLatitude != newTrip.DestinationLatitude ||
		oldTrip.DestinationLongitude != newTrip.DestinationLongitude {
		return true
	}
	// Check for changes in PlaceID (important for geocoding if lat/lon are not primary)
	if (oldTrip.DestinationPlaceID == nil && newTrip.DestinationPlaceID != nil) ||
		(oldTrip.DestinationPlaceID != nil && newTrip.DestinationPlaceID == nil) ||
		(oldTrip.DestinationPlaceID != nil && newTrip.DestinationPlaceID != nil && *oldTrip.DestinationPlaceID != *newTrip.DestinationPlaceID) {
		return true
	}
	// Check if status changed to/from a weather-relevant state
	isOldRelevant := (oldTrip.Status == types.TripStatusActive || oldTrip.Status == types.TripStatusPlanning)
	isNewRelevant := (newTrip.Status == types.TripStatusActive || newTrip.Status == types.TripStatusPlanning)
	return isOldRelevant != isNewRelevant
}

// triggerWeatherUpdate triggers an immediate weather update for the trip.
func (s *TripManagementService) triggerWeatherUpdate(ctx context.Context, trip *types.Trip) error {
	if s.weatherSvc == nil || trip == nil {
		return nil // Or an error indicating service/trip not available
	}
	log := logger.GetLogger()
	// Ensure destination coordinates are valid before triggering
	if trip.DestinationLatitude != 0 || trip.DestinationLongitude != 0 {
		log.Infow("Triggering immediate weather update for trip", "tripID", trip.ID, "lat", trip.DestinationLatitude, "lon", trip.DestinationLongitude)
		if err := s.weatherSvc.TriggerImmediateUpdate(ctx, trip.ID, trip.DestinationLatitude, trip.DestinationLongitude); err != nil {
			log.Errorw("Failed to trigger weather update via weather service", "error", err, "tripID", trip.ID)
			return err // Propagate error
		}
	} else {
		log.Warnw("Skipping weather update trigger due to invalid/missing destination coordinates", "tripID", trip.ID)
		// Optionally return a specific error here if this case should be an error
		// return apperrors.ValidationFailed("missing_destination_coords", "Cannot trigger weather update without destination coordinates.")
	}
	return nil
}

// GetWeatherForTrip retrieves weather information for a trip.
func (s *TripManagementService) GetWeatherForTrip(ctx context.Context, tripID string) (*types.WeatherInfo, error) {
	// Check if the trip exists
	trip, err := s.store.GetTrip(ctx, tripID)
	if err != nil {
		return nil, apperrors.NotFound("Trip", tripID)
	}

	// Check if the trip has the necessary data for a weather update
	if !s.shouldUpdateWeather(trip) {
		return nil, apperrors.ValidationFailed("incomplete_trip_data", "Trip is missing destination or dates required for weather forecast")
	}

	// Call the weather service to get the data
	if s.weatherSvc == nil {
		// Handle case where weather service might not be configured
		return nil, fmt.Errorf("weather service is not available") // Or return a specific AppError
	}

	weatherInfo, err := s.weatherSvc.GetWeather(ctx, tripID)
	if err != nil {
		// Handle errors from the weather service (e.g., API error, not found in cache)
		// Consider wrapping the error or returning specific AppErrors based on the error type
		logger.GetLogger().Errorw("Failed to get weather from weather service", "error", err, "tripID", tripID)
		// Use ServerError for dependency failures
		return nil, apperrors.Wrap(err, apperrors.ServerError, "failed to retrieve weather information")
	}

	return weatherInfo, nil
}
