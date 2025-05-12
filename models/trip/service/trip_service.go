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
	// Set creator as the creator
	userID, err := utils.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	trip.CreatedBy = userID

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
		UserID: userID,
		Role:   types.MemberRoleOwner,
	}
	if err := s.store.AddMember(ctx, membership); err != nil {
		// Consider potential rollback/cleanup here if needed
		return nil, err
	}

	// Publish event
	err = events.PublishEventWithContext(
		s.eventPublisher,
		ctx,
		EventTypeTripCreated,
		trip.ID,
		trip.CreatedBy,
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

	// Trigger weather update if applicable
	if s.shouldUpdateWeather(trip) {
		s.triggerWeatherUpdate(ctx, trip)
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
	originalTrip := *existingTrip

	// Apply updates (example: update name if provided)
	if updateData.Name != nil {
		existingTrip.Name = *updateData.Name
	}
	if updateData.Description != nil {
		existingTrip.Description = *updateData.Description
	}
	if updateData.Destination != nil {
		existingTrip.Destination = *updateData.Destination
	}
	if updateData.StartDate != nil {
		existingTrip.StartDate = *updateData.StartDate
	}
	if updateData.EndDate != nil {
		existingTrip.EndDate = *updateData.EndDate
	}
	if updateData.Status != "" {
		existingTrip.Status = updateData.Status
	}

	// Update the trip in the store
	updatedTrip, err := s.store.UpdateTrip(ctx, id, updateData)
	if err != nil {
		return nil, err
	}

	// Check if weather update is needed based on changes
	if s.hasWeatherCriticalChanges(&originalTrip, updatedTrip) {
		if s.shouldUpdateWeather(updatedTrip) { // Double check if the *new* state warrants updates
			s.triggerWeatherUpdate(ctx, updatedTrip)
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

	// Fetch the trip to get current status and validate existence
	trip, err := s.store.GetTrip(ctx, tripID)
	if err != nil {
		log.Errorw("Failed to get trip for status update", "error", err, "tripID", tripID)
		if appErr, ok := err.(*apperrors.AppError); ok && appErr.Type == apperrors.NotFoundError {
			return apperrors.NotFound("trip", tripID)
		}
		return apperrors.Wrap(err, apperrors.DatabaseError, "Failed to get trip for status update")
	}

	// Validate the status transition using the method on the type
	if !trip.Status.IsValidTransition(newStatus) { // Updated call
		log.Warnw("Invalid status transition attempt", "tripID", tripID, "currentStatus", trip.Status, "newStatus", newStatus)
		return apperrors.ValidationFailed("invalid_status_transition", fmt.Sprintf("Cannot transition from %s to %s", trip.Status, newStatus))
	}

	// Update the trip status using the UpdateTrip store method
	updateData := types.TripUpdate{
		Status: newStatus,
	}
	if _, err := s.store.UpdateTrip(ctx, tripID, updateData); err != nil {
		log.Errorw("Failed to update trip status in store", "error", err, "tripID", tripID, "newStatus", newStatus)
		return apperrors.NewDatabaseError(err)
	}

	// Publish event
	err = events.PublishEventWithContext(
		s.eventPublisher,
		ctx,
		EventTypeTripStatusChanged,
		tripID,
		userID,
		map[string]interface{}{
			"event_id":   utils.GenerateEventID(),
			"old_status": string(trip.Status),
			"new_status": string(newStatus),
		},
		"trip-management-service",
	)
	if err != nil {
		log.Warnw("Failed to publish trip status changed event", "error", err, "tripID", tripID)
	}

	return nil
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
		s.triggerWeatherUpdate(ctx, trip)
		log.Infow("Manually triggered weather update", "tripID", tripID)
		return nil
	}
	log.Infow("Skipped manual weather trigger", "tripID", tripID, "reason", "shouldUpdateWeather returned false")
	return apperrors.ValidationFailed("weather_update_skipped", "Trip conditions not met for weather update")
}

// shouldUpdateWeather determines if a weather update should be performed based on trip status and data
func (s *TripManagementService) shouldUpdateWeather(trip *types.Trip) bool {
	if trip == nil {
		return false
	}
	// Only update for active or planning trips with valid destination and dates
	return (trip.Status == types.TripStatusActive || trip.Status == types.TripStatusPlanning) &&
		!trip.StartDate.IsZero() &&
		!trip.EndDate.IsZero() &&
		IsDestinationValid(trip.Destination)
}

// hasWeatherCriticalChanges checks if trip updates require a weather update
func (s *TripManagementService) hasWeatherCriticalChanges(oldTrip, newTrip *types.Trip) bool {
	if oldTrip == nil || newTrip == nil {
		return false // Cannot compare if one is nil
	}
	// Check if destination changed significantly
	destinationChanged := !AreDestinationsEqual(oldTrip.Destination, newTrip.Destination)

	// Check if dates changed
	datesChanged := !oldTrip.StartDate.Equal(newTrip.StartDate) || !oldTrip.EndDate.Equal(newTrip.EndDate)

	// Check if status changed to active/planning (from something else)
	statusBecameRelevant := (oldTrip.Status != types.TripStatusActive && oldTrip.Status != types.TripStatusPlanning) &&
		(newTrip.Status == types.TripStatusActive || newTrip.Status == types.TripStatusPlanning)

	return (destinationChanged || datesChanged || statusBecameRelevant) && IsDestinationValid(newTrip.Destination)
}

// triggerWeatherUpdate calls the weather service to start/trigger updates
func (s *TripManagementService) triggerWeatherUpdate(ctx context.Context, trip *types.Trip) {
	log := logger.GetLogger()
	if s.weatherSvc != nil {
		log.Infow("Triggering weather service update", "tripID", trip.ID)
		s.weatherSvc.TriggerImmediateUpdate(ctx, trip.ID, trip.Destination)
	} else {
		log.Warnw("Weather service not available, cannot trigger update", "tripID", trip.ID)
	}
}

// GetWeatherForTrip retrieves the weather forecast for a trip
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
