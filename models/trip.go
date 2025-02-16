package models

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	ErrTripValidation          = "TRIP_VALIDATION"
	ErrTripNotFound            = "TRIP_NOT_FOUND"
	ErrTripStatusConflict      = "TRIP_STATUS_CONFLICT"
	ErrTripAccess              = "TRIP_ACCESS_DENIED"
	ErrTripMembership          = "TRIP_MEMBERSHIP"
	ErrInvalidStatusTransition = "INVALID_STATUS_TRANSITION"
)

type TripError struct {
	Code      string
	Message   string
	Details   string
	TripID    string
	UserID    string
	Operation string
	Timestamp time.Time
}

func (e *TripError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Code, e.Message, e.Details)
}

func (tm *TripModel) newTripError(code string, msg string, details string, tripID string, userID string, op string) error {
	return &TripError{
		Code:      code,
		Message:   msg,
		Details:   details,
		TripID:    tripID,
		UserID:    userID,
		Operation: op,
		Timestamp: time.Now(),
	}
}

// Helper methods for common trip errors
func (tm *TripModel) tripNotFound(tripID string) error {
	return tm.newTripError(
		ErrTripNotFound,
		"Trip not found",
		fmt.Sprintf("Trip with ID %s does not exist", tripID),
		tripID,
		"",
		"lookup",
	)
}

func (tm *TripModel) TripAccessDenied(tripID string, userID string, requiredRole types.MemberRole) error {
	return tm.newTripError(
		ErrTripAccess,
		"Access denied",
		fmt.Sprintf("User %s requires role %s for trip %s", userID, requiredRole, tripID),
		tripID,
		userID,
		"access_check",
	)
}

type TripModel struct {
	store          store.TripStore
	WeatherService types.WeatherServiceInterface
	eventService   types.EventPublisher
	log            *zap.SugaredLogger
}

func NewTripModel(store store.TripStore, weatherService types.WeatherServiceInterface, eventService types.EventPublisher) *TripModel {
	return &TripModel{
		store:          store,
		WeatherService: weatherService,
		eventService:   eventService,
		log:            logger.GetLogger(),
	}
}

type EventContext struct {
	CorrelationID string
	CausationID   string
	UserID        string
}

func (tm *TripModel) emitEvent(ctx context.Context, tripID string, eventType types.EventType, payload interface{}, eventCtx EventContext) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal event payload: %w", err)
	}

	event := types.Event{
		BaseEvent: types.BaseEvent{
			ID:        uuid.New().String(),
			Type:      eventType,
			TripID:    tripID,
			UserID:    eventCtx.UserID,
			Timestamp: time.Now(),
			Version:   1,
		},
		Metadata: types.EventMetadata{
			Source:        "trip_model",
			CorrelationID: eventCtx.CorrelationID,
			CausationID:   eventCtx.CausationID,
		},
		Payload: data,
	}

	return tm.eventService.Publish(ctx, tripID, event)
}

func (tm *TripModel) CreateTrip(ctx context.Context, trip *types.Trip) error {
	log := logger.GetLogger()
	if err := validateTrip(trip); err != nil {
		return err
	}

	id, err := tm.store.CreateTrip(ctx, *trip)

	if err != nil {
		log.Debug("Generated trip ID: %s (length: %d)", id, len(id))
		return errors.NewDatabaseError(err)
	}

	trip.ID = id

	if err := tm.emitEvent(ctx, id, types.EventTypeTripCreated, trip, EventContext{UserID: trip.CreatedBy}); err != nil {
		tm.log.Errorw("Failed to emit trip created event",
			"error", err,
			"tripId", id,
		)
	}

	if tm.WeatherService != nil &&
		(trip.Status == types.TripStatusActive || trip.Status == types.TripStatusPlanning) {
		tm.WeatherService.StartWeatherUpdates(context.Background(), trip.ID, trip.Destination)
	}

	return nil
}

func (tm *TripModel) GetTripByID(ctx context.Context, id string) (*types.Trip, error) {
	trip, err := tm.store.GetTrip(ctx, id)
	if err != nil {
		return nil, tm.tripNotFound(id)
	}
	return trip, nil
}

type StatusTransition struct {
	FromStatus types.TripStatus
	ToStatus   types.TripStatus
	Validation func(trip *types.Trip) error
}

func getAllowedTransitions() []StatusTransition {
	return []StatusTransition{
		{
			FromStatus: types.TripStatusPlanning,
			ToStatus:   types.TripStatusActive,
			Validation: func(trip *types.Trip) error {
				if trip.EndDate.Before(time.Now()) {
					return errors.ValidationFailed(
						"invalid status transition",
						"cannot activate a trip that has already ended",
					)
				}
				return nil
			},
		},
		{
			FromStatus: types.TripStatusActive,
			ToStatus:   types.TripStatusCompleted,
			Validation: func(trip *types.Trip) error {
				if trip.EndDate.After(time.Now()) {
					return errors.ValidationFailed(
						"invalid status transition",
						"cannot complete a trip before its end date",
					)
				}
				return nil
			},
		},
		// Add other valid transitions here
	}
}

func (tm *TripModel) validateStatusTransition(trip *types.Trip, newStatus types.TripStatus) error {
	if !trip.Status.IsValidTransition(newStatus) {
		return tm.newTripError(
			ErrInvalidStatusTransition,
			"Invalid status transition",
			fmt.Sprintf("Cannot transition from %s to %s", trip.Status, newStatus),
			trip.ID,
			"",
			"status_validation",
		)
	}

	for _, transition := range getAllowedTransitions() {
		if transition.FromStatus == trip.Status && transition.ToStatus == newStatus {
			if err := transition.Validation(trip); err != nil {
				return tm.newTripError(
					ErrTripValidation,
					"Transition validation failed",
					err.Error(),
					trip.ID,
					"",
					"status_validation",
				)
			}
		}
	}
	return nil
}

func (tm *TripModel) UpdateTrip(ctx context.Context, id string, update *types.TripUpdate) error {
	if err := validateTripUpdate(update); err != nil {
		return err
	}

	currentTrip, err := tm.GetTripByID(ctx, id)
	if err != nil {
		return err
	}

	if update.Status != "" {
		if err := tm.validateStatusTransition(currentTrip, update.Status); err != nil {
			return err
		}
	}

	updatedTrip, err := tm.store.UpdateTrip(ctx, id, *update)
	if err != nil {
		return errors.NewDatabaseError(err)
	}

	if err := tm.emitEvent(ctx, id, types.EventTypeTripUpdated, updatedTrip, EventContext{UserID: ""}); err != nil {
		tm.log.Errorw("Failed to emit trip updated event", "error", err, "tripId", id)
	}

	return nil
}

func (tm *TripModel) DeleteTrip(ctx context.Context, id string) error {
	currentTrip, err := tm.GetTripByID(ctx, id)
	if err != nil {
		return err
	}

	err = tm.store.SoftDeleteTrip(ctx, id)
	if err != nil {
		return errors.NewDatabaseError(err)
	}

	if err := tm.emitEvent(ctx, id, types.EventTypeTripDeleted, currentTrip, EventContext{UserID: currentTrip.CreatedBy}); err != nil {
		tm.log.Errorw("Failed to emit trip deleted event",
			"error", err,
			"tripId", id,
		)
	}

	return nil
}

func (tm *TripModel) ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error) {
	trips, err := tm.store.ListUserTrips(ctx, userID)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return trips, nil
}

func (tm *TripModel) SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error) {
	trips, err := tm.store.SearchTrips(ctx, criteria)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return trips, nil
}

// Helper functions for validation
func validateTrip(trip *types.Trip) error {
	var validationErrors []string
	now := time.Now().UTC()

	if trip.Name == "" {
		validationErrors = append(validationErrors, "trip name is required")
	}
	if trip.Destination.Address == "" {
		validationErrors = append(validationErrors, "trip destination is required")
	}
	if trip.StartDate.IsZero() {
		validationErrors = append(validationErrors, "trip start date is required")
	}
	if trip.EndDate.IsZero() {
		validationErrors = append(validationErrors, "trip end date is required")
	}

	// Only validate start date being in past for new trips (where ID is empty)
	if trip.ID == "" && !trip.StartDate.IsZero() {
		startDate := trip.StartDate.Truncate(24 * time.Hour)
		if startDate.Before(now) {
			validationErrors = append(validationErrors, "start date cannot be in the past")
		}
	}

	if !trip.StartDate.IsZero() && !trip.EndDate.IsZero() && trip.EndDate.Before(trip.StartDate) {
		validationErrors = append(validationErrors, "trip end date cannot be before start date")
	}

	if trip.CreatedBy == "" {
		validationErrors = append(validationErrors, "trip creator ID is required")
	}

	if trip.Status == "" {
		trip.Status = types.TripStatusPlanning
	} else if !trip.Status.IsValid() {
		validationErrors = append(validationErrors, "invalid trip status")
	}

	if len(validationErrors) > 0 {
		return errors.ValidationFailed(
			"Invalid trip data",
			strings.Join(validationErrors, "; "),
		)
	}
	return nil
}

func validateTripUpdate(update *types.TripUpdate) error {
	var validationErrors []string

	if update.StartDate != nil && update.EndDate != nil &&
		!(*update.StartDate).IsZero() && !(*update.EndDate).IsZero() &&
		(*update.EndDate).Before(*update.StartDate) {
		validationErrors = append(validationErrors, "trip end date cannot be before start date")
	}

	if update.Status != "" && !update.Status.IsValid() {
		validationErrors = append(validationErrors, "invalid trip status")
	}

	if len(validationErrors) > 0 {
		return errors.ValidationFailed(
			"Invalid trip update data",
			strings.Join(validationErrors, "; "),
		)
	}
	return nil
}

func (tm *TripModel) UpdateTripStatus(ctx context.Context, id string, newStatus types.TripStatus) error {
	// First get current trip to check current status
	currentTrip, err := tm.GetTripByID(ctx, id)
	if err != nil {
		return err
	}

	// Replace validation block with:
	if err := tm.validateStatusTransition(currentTrip, newStatus); err != nil {
		return err
	}

	// Update the status in the database
	update := types.TripUpdate{Status: newStatus}
	if _, err := tm.store.UpdateTrip(ctx, id, update); err != nil {
		return err
	}

	// Fetch updated trip for event and weather updates
	trip, err := tm.GetTripByID(ctx, id)
	if err != nil {
		return err
	}

	// Emit status updated event
	if err := tm.emitEvent(ctx, id, types.EventTypeTripStatusUpdated, trip, EventContext{UserID: ""}); err != nil {
		tm.log.Errorw("Failed to emit trip status updated event",
			"error", err,
			"tripId", id,
		)
	}

	// Start weather updates if needed
	if newStatus == types.TripStatusActive || newStatus == types.TripStatusPlanning {
		tm.WeatherService.StartWeatherUpdates(context.Background(), id, trip.Destination)
	}

	return nil
}

// AddMember adds a new member to a trip with role validation
func (tm *TripModel) AddMember(ctx context.Context, tripID string, userID string, role types.MemberRole) error {
	// First verify that the trip exists
	trip, err := tm.GetTripByID(ctx, tripID)
	log := logger.GetLogger()
	log.Infof("Trip: %v", trip)
	if err != nil {
		return err
	}

	// Verify the role is valid
	if !isValidRole(role) {
		return tm.newTripError(
			ErrTripValidation,
			"Invalid role",
			fmt.Sprintf("Role %s is not valid", role),
			tripID,
			userID,
			"add_member",
		)
	}

	membership := &types.TripMembership{
		TripID: tripID,
		UserID: userID,
		Role:   role,
		Status: types.MembershipStatusActive,
	}

	if err := tm.store.AddMember(ctx, membership); err != nil {
		return errors.NewDatabaseError(err)
	}

	if err := tm.emitEvent(ctx, tripID, types.EventTypeMemberAdded, membership, EventContext{UserID: userID}); err != nil {
		tm.log.Errorw("Failed to emit member added event",
			"error", err,
			"tripId", tripID,
			"userId", userID,
		)
	}

	return nil
}

func (tm *TripModel) GetUserRole(ctx context.Context, tripID, userID string) (types.MemberRole, error) {
	return tm.store.GetUserRole(ctx, tripID, userID)
}

// UpdateMemberRole updates a member's role with validation
func (tm *TripModel) UpdateMemberRole(ctx context.Context, tripID string, userID string, newRole types.MemberRole, requestingUserID string) error {
	// Check if requesting user is an admin
	requestingUserRole, err := tm.store.GetUserRole(ctx, tripID, requestingUserID)
	if err != nil {
		return err
	}

	if requestingUserRole != types.MemberRoleOwner {
		return errors.ValidationFailed(
			"Unauthorized",
			"Only admins can update member roles",
		)
	}

	// Prevent removing the last admin
	if newRole != types.MemberRoleOwner {
		members, err := tm.store.GetTripMembers(ctx, tripID)
		if err != nil {
			return err
		}

		adminCount := 0
		for _, member := range members {
			if member.Role == types.MemberRoleOwner {
				adminCount++
			}
		}

		if adminCount <= 1 {
			return errors.ValidationFailed(
				"Invalid operation",
				"Cannot remove the last admin from the trip",
			)
		}
	}

	if err := tm.store.UpdateMemberRole(ctx, tripID, userID, newRole); err != nil {
		return errors.NewDatabaseError(err)
	}

	// Emit event
	payload := map[string]interface{}{
		"userId":  userID,
		"newRole": newRole,
	}
	if err := tm.emitEvent(ctx, tripID, types.EventTypeMemberRoleUpdated, payload, EventContext{UserID: requestingUserID}); err != nil {
		tm.log.Errorw("Failed to emit member role updated event",
			"error", err,
			"tripId", tripID,
			"userId", userID,
		)
	}

	return nil
}

// RemoveMember removes a member from a trip
func (tm *TripModel) RemoveMember(ctx context.Context, tripID string, userID string, requestingUserID string) error {
	// Check if requesting user is an admin
	requestingUserRole, err := tm.store.GetUserRole(ctx, tripID, requestingUserID)
	if err != nil {
		return err
	}

	if requestingUserRole != types.MemberRoleOwner && requestingUserID != userID {
		return tm.TripAccessDenied(tripID, requestingUserID, types.MemberRoleOwner)
	}

	// If removing an admin, check if they're the last one
	currentRole, err := tm.store.GetUserRole(ctx, tripID, userID)
	if err != nil {
		return err
	}

	if currentRole == types.MemberRoleOwner {
		if rule, ok := tm.getMembershipRules()["remove_owner"]; ok {
			if err := rule.Validate(ctx, tm, tripID, userID, requestingUserID); err != nil {
				return err
			}
		}
	}

	if err := tm.store.RemoveMember(ctx, tripID, userID); err != nil {
		return errors.NewDatabaseError(err)
	}

	payload := map[string]interface{}{
		"removedUserId": userID,
	}
	if err := tm.emitEvent(ctx, tripID, types.EventTypeMemberRemoved, payload, EventContext{UserID: requestingUserID}); err != nil {
		tm.log.Errorw("Failed to emit member removed event",
			"error", err,
			"tripId", tripID,
			"userId", userID,
		)
	}

	return nil
}

// GetTripMembers gets all active members of a trip
func (tm *TripModel) GetTripMembers(ctx context.Context, tripID string) ([]types.TripMembership, error) {
	members, err := tm.store.GetTripMembers(ctx, tripID)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return members, nil
}

// Helper function to validate roles
func isValidRole(role types.MemberRole) bool {
	return role == types.MemberRoleOwner || role == types.MemberRoleMember
}

// GetTripWithMembers gets a trip with its members
func (tm *TripModel) GetTripWithMembers(ctx context.Context, id string) (*types.TripWithMembers, error) {
	trip, err := tm.GetTripByID(ctx, id)
	if err != nil {
		return nil, err
	}

	members, err := tm.GetTripMembers(ctx, id)
	if err != nil {
		return nil, err
	}

	return &types.TripWithMembers{
		Trip:    *trip,
		Members: members,
	}, nil
}

type MembershipRule struct {
	Operation string
	Validate  func(ctx context.Context, tm *TripModel, tripID, userID, requestingUserID string) error
}

func (tm *TripModel) getMembershipRules() map[string]MembershipRule {
	return map[string]MembershipRule{
		"remove_owner": {
			Operation: "remove_owner",
			Validate: func(ctx context.Context, tm *TripModel, tripID, userID, requestingUserID string) error {
				members, err := tm.store.GetTripMembers(ctx, tripID)
				if err != nil {
					return err
				}

				adminCount := 0
				for _, member := range members {
					if member.Role == types.MemberRoleOwner {
						adminCount++
					}
				}

				if adminCount <= 1 {
					return errors.ValidationFailed(
						"Invalid operation",
						"Cannot remove the last admin from the trip",
					)
				}
				return nil
			},
		},
	}
}
