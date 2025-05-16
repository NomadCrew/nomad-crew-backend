package models

import (
	"context"
	"fmt"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/validation"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

// TripModelFacade wraps the real trip model and adapts it to the interface expected by tests
type TripModelFacade struct {
	store          store.TripStore
	weatherSvc     types.WeatherServiceInterface
	eventPublisher types.EventPublisher
}

// NewTripModel creates a new trip model for use in tests
func NewTripModel(store store.TripStore, weatherSvc types.WeatherServiceInterface, eventPublisher types.EventPublisher) *TripModelFacade {
	return &TripModelFacade{
		store:          store,
		weatherSvc:     weatherSvc,
		eventPublisher: eventPublisher,
	}
}

// CreateTrip implements a facade method for tests
func (tm *TripModelFacade) CreateTrip(ctx context.Context, trip *types.Trip) error {
	if err := validation.ValidateNewTrip(trip); err != nil {
		return err
	}
	id, err := tm.store.CreateTrip(ctx, *trip)
	if err != nil {
		return err
	}
	trip.ID = id

	// Start weather updates for the trip
	if tm.weatherSvc != nil {
		tm.weatherSvc.StartWeatherUpdates(ctx, id, trip.DestinationLatitude, trip.DestinationLongitude)
	}

	// Simulate publishing an event
	if tm.eventPublisher != nil {
		if err := tm.eventPublisher.Publish(ctx, id, types.Event{
			BaseEvent: types.BaseEvent{
				Type:      types.EventTypeTripCreated,
				TripID:    id,
				Timestamp: time.Now(),
				Version:   1,
			},
			Metadata: types.EventMetadata{
				Source: "trip_model",
			},
		}); err != nil {
			log := logger.GetLogger()
			log.Warnw("Failed to publish trip created event", "error", err)
		}
	}
	return nil
}

// GetTripByID implements a facade method for tests
func (tm *TripModelFacade) GetTripByID(ctx context.Context, id string) (*types.Trip, error) {
	return tm.store.GetTrip(ctx, id)
}

// UpdateTrip implements a facade method for tests
func (tm *TripModelFacade) UpdateTrip(ctx context.Context, id string, update *types.TripUpdate) error {
	// First check if the trip exists
	existingTrip, err := tm.store.GetTrip(ctx, id)
	if err != nil {
		return err
	}

	// Validate the update payload against the existing trip
	if err := validation.ValidateTripUpdate(update, existingTrip); err != nil {
		return err // Return validation error before attempting to update in store
	}

	// If trip exists and validation passes, update it
	_, err = tm.store.UpdateTrip(ctx, id, *update)
	// Simulate publishing an event
	if err == nil && tm.eventPublisher != nil {
		if err := tm.eventPublisher.Publish(ctx, id, types.Event{
			BaseEvent: types.BaseEvent{
				Type:      types.EventTypeTripUpdated,
				TripID:    id,
				Timestamp: time.Now(),
				Version:   1,
			},
			Metadata: types.EventMetadata{
				Source: "trip_model",
			},
		}); err != nil {
			log := logger.GetLogger()
			log.Warnw("Failed to publish trip updated event", "error", err)
		}
	}
	return err
}

// DeleteTrip implements a facade method for tests
func (tm *TripModelFacade) DeleteTrip(ctx context.Context, id string) error {
	// First check if the trip exists
	_, err := tm.store.GetTrip(ctx, id)
	if err != nil {
		return err
	}

	// If trip exists, delete it
	err = tm.store.SoftDeleteTrip(ctx, id)
	if err != nil {
		return err
	}

	// Publish event if deletion was successful
	if tm.eventPublisher != nil {
		if err := tm.eventPublisher.Publish(ctx, id, types.Event{
			BaseEvent: types.BaseEvent{
				Type:      types.EventTypeTripDeleted,
				TripID:    id,
				Timestamp: time.Now(),
				Version:   1,
			},
			Metadata: types.EventMetadata{
				Source: "trip_model",
			},
		}); err != nil {
			log := logger.GetLogger()
			log.Warnw("Failed to publish trip deleted event", "error", err)
		}
	}

	return nil
}

// ListUserTrips implements a facade method for tests
func (tm *TripModelFacade) ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error) {
	return tm.store.ListUserTrips(ctx, userID)
}

// SearchTrips implements a facade method for tests
func (tm *TripModelFacade) SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error) {
	return tm.store.SearchTrips(ctx, criteria)
}

// GetUserRole implements a facade method for tests
func (tm *TripModelFacade) GetUserRole(ctx context.Context, tripID, userID string) (types.MemberRole, error) {
	return tm.store.GetUserRole(ctx, tripID, userID)
}

// AddMember implements a facade method for tests
func (tm *TripModelFacade) AddMember(ctx context.Context, membership *types.TripMembership) error {
	return tm.store.AddMember(ctx, membership)
}

// UpdateMemberRole implements a facade method for tests
func (tm *TripModelFacade) UpdateMemberRole(ctx context.Context, tripID, userID string, role types.MemberRole) (*interfaces.CommandResult, error) {
	err := tm.store.UpdateMemberRole(ctx, tripID, userID, role)
	if err != nil {
		return nil, err
	}
	return &interfaces.CommandResult{Success: true}, nil
}

// RemoveMember implements a facade method for tests
func (tm *TripModelFacade) RemoveMember(ctx context.Context, tripID string, userID string) error {
	return tm.store.RemoveMember(ctx, tripID, userID)
}

// CreateInvitation implements a facade method for tests
func (tm *TripModelFacade) CreateInvitation(ctx context.Context, invitation *types.TripInvitation) error {
	return tm.store.CreateInvitation(ctx, invitation)
}

// GetInvitation implements a facade method for tests
func (tm *TripModelFacade) GetInvitation(ctx context.Context, invitationID string) (*types.TripInvitation, error) {
	return tm.store.GetInvitation(ctx, invitationID)
}

// UpdateInvitationStatus implements a facade method for tests
func (tm *TripModelFacade) UpdateInvitationStatus(ctx context.Context, invitationID string, status types.InvitationStatus) error {
	return tm.store.UpdateInvitationStatus(ctx, invitationID, status)
}

// LookupUserByEmail implements a facade method for tests
func (tm *TripModelFacade) LookupUserByEmail(ctx context.Context, email string) (*types.SupabaseUser, error) {
	// This would be a mock implementation for tests
	return tm.store.LookupUserByEmail(ctx, email)
}

// UpdateTripStatus implements a facade method for tests
func (tm *TripModelFacade) UpdateTripStatus(ctx context.Context, tripID string, newStatus types.TripStatus) error {
	// First, get the current trip to validate the status transition
	trip, err := tm.store.GetTrip(ctx, tripID)
	if err != nil {
		return err
	}

	// Validate the status transition
	if !trip.Status.IsValidTransition(newStatus) {
		return &errors.AppError{
			Type:    errors.ValidationError,
			Message: fmt.Sprintf("Cannot transition from %s to %s", trip.Status, newStatus),
		}
	}

	// Validate time-based constraints
	switch newStatus {
	case types.TripStatusActive:
		if trip.EndDate.Before(time.Now()) {
			return &errors.AppError{
				Type:    errors.ValidationError,
				Message: "cannot activate a trip that has already ended",
			}
		}
	case types.TripStatusCompleted:
		if trip.EndDate.After(time.Now()) {
			return &errors.AppError{
				Type:    errors.ValidationError,
				Message: "cannot complete a trip before its end date",
			}
		}
	}

	update := types.TripUpdate{
		Status: &newStatus,
	}
	_, err = tm.store.UpdateTrip(ctx, tripID, update)

	// Simulate publishing an event
	if err == nil && tm.eventPublisher != nil {
		if err := tm.eventPublisher.Publish(ctx, tripID, types.Event{
			BaseEvent: types.BaseEvent{
				Type:      types.EventTypeTripStatusUpdated,
				TripID:    tripID,
				Timestamp: time.Now(),
				Version:   1,
			},
			Metadata: types.EventMetadata{
				Source: "trip_model",
			},
		}); err != nil {
			log := logger.GetLogger()
			log.Warnw("Failed to publish trip status updated event", "error", err)
		}
	}

	// Start weather updates if the trip is now active
	if err == nil && newStatus == types.TripStatusActive && tm.weatherSvc != nil {
		tm.weatherSvc.StartWeatherUpdates(ctx, tripID, trip.DestinationLatitude, trip.DestinationLongitude)
	}

	return err
}

// TodoModelFacade wraps a todo model for testing
type TodoModelFacade struct {
	todoStore interface{}
	tripModel *TripModelFacade
}

// NewTodoModelFacade creates a facade for the todo model
func NewTodoModelFacade(todoStore interface{}, tripModel *TripModelFacade) *TodoModelFacade {
	return &TodoModelFacade{
		todoStore: todoStore,
		tripModel: tripModel,
	}
}

// CreateTodo implements a facade method for tests
func (tm *TodoModelFacade) CreateTodo(ctx context.Context, todo *types.Todo) (string, error) {
	// Implementation will depend on the todoStore interface
	return "", nil
}

// ListTripTodos implements a facade method for tests
func (tm *TodoModelFacade) ListTripTodos(ctx context.Context, tripID string, userID string, limit int, offset int) (*types.PaginatedResponse, error) {
	// Implementation will depend on the todoStore interface
	return nil, nil
}
