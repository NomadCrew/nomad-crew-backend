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
    
    // New validation for past start date
    if !trip.StartDate.IsZero() && trip.StartDate.Before(time.Now()) {
        validationErrors = append(validationErrors, "start date cannot be in the past")
    }
    
    if trip.EndDate.Before(trip.StartDate) {
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

	if !update.StartDate.IsZero() && !update.EndDate.IsZero() &&
		update.EndDate.Before(update.StartDate) {
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
    return tm.store.UpdateTrip(ctx, id, update)
}