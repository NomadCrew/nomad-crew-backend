package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

type TripModel struct {
    store          store.TripStore
    WeatherService types.WeatherServiceInterface
}

func NewTripModel(store store.TripStore, weatherService types.WeatherServiceInterface) *TripModel {
    return &TripModel{
        store:          store,
        WeatherService: weatherService,
    }
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

	if trip.Status == types.TripStatusActive || trip.Status == types.TripStatusPlanning {
		tm.WeatherService.StartWeatherUpdates(context.Background(), trip.ID, trip.Destination)
	}

	return nil
}

func (tm *TripModel) GetTripByID(ctx context.Context, id string) (*types.Trip, error) {
	trip, err := tm.store.GetTrip(ctx, id)
	if err != nil {
		return nil, errors.NotFound("Trip", id)
	}
	return trip, nil
}

func (tm *TripModel) UpdateTrip(ctx context.Context, id string, update *types.TripUpdate) error {
	// Validate the update fields
	if err := validateTripUpdate(update); err != nil {
		return err
	}

	// First check if trip exists
	currentTrip, err := tm.GetTripByID(ctx, id)
	if err != nil {
		return err
	}

	// If updating status, validate status transition
	if update.Status != "" {
		if !currentTrip.Status.IsValidTransition(update.Status) {
			return errors.ValidationFailed(
				"Invalid status transition",
				fmt.Sprintf("Cannot transition from %s to %s", currentTrip.Status, update.Status),
			)
		}
		// Additional business rule validations
		now := time.Now()
		switch update.Status {
		case types.TripStatusActive:
			if currentTrip.EndDate.Before(now) {
				return errors.ValidationFailed(
					"Invalid status transition",
					"Cannot activate a trip that has already ended",
				)
			}
		case types.TripStatusCompleted:
			if currentTrip.EndDate.After(now) {
				return errors.ValidationFailed(
					"Invalid status transition",
					"Cannot complete a trip before its end date",
				)
			}
		}
	}

	err = tm.store.UpdateTrip(ctx, id, *update)
	if err != nil {
		return errors.NewDatabaseError(err)
	}

	return nil
}

func (tm *TripModel) DeleteTrip(ctx context.Context, id string) error {
	// First check if trip exists
	_, err := tm.GetTripByID(ctx, id)
	if err != nil {
		return err
	}

	err = tm.store.SoftDeleteTrip(ctx, id)
	if err != nil {
		return errors.NewDatabaseError(err)
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

	// Validate status transition
	if !currentTrip.Status.IsValidTransition(newStatus) {
		return errors.ValidationFailed(
			"invalid status transition",
			fmt.Sprintf("cannot transition from %s to %s", currentTrip.Status, newStatus),
		)
	}

	// Additional business rules
	now := time.Now()
	switch newStatus {
	case types.TripStatusActive:
		// Can only activate future or ongoing trips
		if currentTrip.EndDate.Before(now) {
			return errors.ValidationFailed(
				"invalid status transition",
				"cannot activate a trip that has already ended",
			)
		}
	case types.TripStatusCompleted:
		// Can only complete trips that have ended
		if currentTrip.EndDate.After(now) {
			return errors.ValidationFailed(
				"invalid status transition",
				"cannot complete a trip before its end date",
			)
		}
	}

	// Update the status in the database
	update := types.TripUpdate{Status: newStatus}
	if err := tm.store.UpdateTrip(ctx, id, update); err != nil {
		return err
	}
	if newStatus == types.TripStatusActive || newStatus == types.TripStatusPlanning {
		trip, err := tm.GetTripByID(ctx, id)
		if err != nil {
			return err
		}
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
		return errors.ValidationFailed(
			"Invalid role",
			fmt.Sprintf("Role %s is not valid", role),
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
		return errors.ValidationFailed(
			"Unauthorized",
			"Only admins can remove other members",
		)
	}

	// If removing an admin, check if they're the last one
	currentRole, err := tm.store.GetUserRole(ctx, tripID, userID)
	if err != nil {
		return err
	}

	if currentRole == types.MemberRoleOwner {
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

	if err := tm.store.RemoveMember(ctx, tripID, userID); err != nil {
		return errors.NewDatabaseError(err)
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
