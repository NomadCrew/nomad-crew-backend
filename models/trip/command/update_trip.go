package command

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	tripinterfaces "github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/shared"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/validation"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
)

type UpdateTripCommand struct {
	BaseCommand
	TripID       string
	Update       *types.TripUpdate
	originalTrip *types.Trip
}

func (c *UpdateTripCommand) Validate(ctx context.Context) error {
	if c.TripID == "" {
		return errors.ValidationFailed("trip_id_required", "Trip ID is required")
	}
	if c.Update == nil {
		return errors.ValidationFailed("update_required", "Update data is required")
	}

	// Store original trip for validation
	trip, err := c.Ctx.Store.GetTrip(ctx, c.TripID)
	if err != nil {
		return errors.NotFound("trip_not_found", "Trip not found")
	}
	c.originalTrip = trip

	return validation.ValidateTripUpdate(c.Update, trip)
}

func (c *UpdateTripCommand) ValidatePermissions(ctx context.Context) error {
	return c.BaseCommand.ValidatePermissions(ctx, c.TripID, types.MemberRoleOwner)
}

func (c *UpdateTripCommand) Execute(ctx context.Context) (*tripinterfaces.CommandResult, error) {
	log := logger.GetLogger()

	// Start a transaction
	tx, err := c.Ctx.Store.BeginTx(ctx)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}

	defer func() {
		if r := recover(); r != nil {
			if err := tx.Rollback(); err != nil {
				log.Errorw("Failed to rollback transaction after panic", "error", err)
			}
		}
	}()

	if err := c.Validate(ctx); err != nil {
		return nil, err
	}
	if err := c.ValidatePermissions(ctx); err != nil {
		return nil, err
	}

	// Status transition validation
	if c.Update.Status != nil {
		newStatusValue := *c.Update.Status
		if err := validation.ValidateStatusTransition(c.originalTrip, newStatusValue); err != nil {
			return nil, errors.InvalidStatusTransition(
				c.originalTrip.Status.String(),
				newStatusValue.String(),
			)
		}
	}

	updatedTrip, err := c.Ctx.Store.UpdateTrip(ctx, c.TripID, *c.Update)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			log.Errorw("Failed to rollback transaction", "error", rollbackErr, "originalError", err)
		}
		return nil, err
	}

	// Emit event using the emitter
	emitter := shared.NewEventEmitter(c.Ctx.EventBus)

	// Generate event ID for tracking
	eventID := uuid.NewString()

	// Create and publish the event through the emitter
	event := types.Event{
		BaseEvent: types.BaseEvent{
			ID:        eventID,
			Type:      types.EventTypeTripUpdated,
			TripID:    c.TripID,
			UserID:    c.UserID,
			Timestamp: time.Now().UTC(), // Ensure UTC for consistency
			Version:   1,
		},
		Metadata: types.EventMetadata{
			Source: "trip-command",
		},
	}

	// Convert trip to map to ensure proper timestamp handling
	var payloadMap map[string]interface{}
	payloadJSON, err := json.Marshal(updatedTrip)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			log.Errorw("Failed to rollback transaction after JSON marshal error", "error", rollbackErr, "originalError", err)
		}
		return nil, errors.NewError(errors.ServerError, "event_payload_marshal_failed", "Failed to marshal trip data", http.StatusInternalServerError)
	}

	if err := json.Unmarshal(payloadJSON, &payloadMap); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			log.Errorw("Failed to rollback transaction after JSON unmarshal error", "error", rollbackErr, "originalError", err)
		}
		return nil, errors.NewError(errors.ServerError, "event_payload_unmarshal_failed", "Failed to process trip data", http.StatusInternalServerError)
	}

	// Set the payload from the map to ensure proper serialization
	eventPayload, _ := json.Marshal(payloadMap)
	event.Payload = eventPayload

	// Use the emitter to publish the event
	eventErr := emitter.EmitTripEvent(ctx, c.TripID, types.EventTypeTripUpdated, payloadMap, c.UserID)
	if eventErr != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			log.Errorw("Failed to rollback transaction after event error", "error", rollbackErr, "originalError", eventErr)
		}
		return nil, errors.NewError(errors.ServerError, "event_publish_failed", "Failed to emit trip event", http.StatusInternalServerError)
	}

	if err := tx.Commit(); err != nil {
		return nil, errors.NewDatabaseError(err)
	}

	return &tripinterfaces.CommandResult{
		Success: true,
		Data:    updatedTrip,
		Events:  []types.Event{event},
	}, nil
}
