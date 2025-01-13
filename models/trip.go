package models

import (
	"context"
	"strings"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

type TripModel struct {
	store store.TripStore
}

func NewTripModel(store store.TripStore) *TripModel {
	return &TripModel{store: store}
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
	if err := validateTripUpdate(update); err != nil {
		return err
	}

	// First check if trip exists
	_, err := tm.GetTripByID(ctx, id)
	if err != nil {
		return err
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

	if trip.Name == "" {
		validationErrors = append(validationErrors, "trip name is required")
	}
	if trip.Destination == "" {
		validationErrors = append(validationErrors, "trip destination is required")
	}
	if trip.StartDate.IsZero() {
		validationErrors = append(validationErrors, "trip start date is required")
	}
	if trip.EndDate.IsZero() {
		validationErrors = append(validationErrors, "trip end date is required")
	}
	if trip.EndDate.Before(trip.StartDate) {
		validationErrors = append(validationErrors, "trip end date cannot be before start date")
	}
	if trip.CreatedBy == "" {
		validationErrors = append(validationErrors, "trip creator ID is required")
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

	if !update.StartDate.IsZero() && !update.EndDate.IsZero() &&
		update.EndDate.Before(update.StartDate) {
		validationErrors = append(validationErrors, "trip end date cannot be before start date")
	}

	if len(validationErrors) > 0 {
		return errors.ValidationFailed(
			"Invalid trip update data",
			strings.Join(validationErrors, "; "),
		)
	}
	return nil
}
