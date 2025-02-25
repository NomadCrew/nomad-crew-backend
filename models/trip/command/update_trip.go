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
	if c.Update.Status != "" {
		if err := validation.ValidateStatusTransition(c.originalTrip, c.Update.Status); err != nil {
			return nil, errors.InvalidStatusTransition(
				c.originalTrip.Status.String(),
				c.Update.Status.String(),
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

	// Emit event using the emitter (which also publishes a copy)
	emitter := shared.NewEventEmitter(c.Ctx.EventBus)
	eventErr := emitter.EmitTripEvent(ctx, c.TripID, types.EventTypeTripUpdated, updatedTrip, c.UserID)
	if eventErr != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			log.Errorw("Failed to rollback transaction after event error", "error", rollbackErr, "originalError", eventErr)
		}
		return nil, errors.NewError(errors.ServerError, "event_publish_failed", "Failed to emit trip event", http.StatusInternalServerError)
	}

	if err := tx.Commit(); err != nil {
		return nil, errors.NewDatabaseError(err)
	}

	payload, _ := json.Marshal(updatedTrip)

	return &tripinterfaces.CommandResult{
		Success: true,
		Data:    updatedTrip,
		Events: []types.Event{{
			BaseEvent: types.BaseEvent{
				ID:        uuid.NewString(),
				Type:      types.EventTypeTripUpdated,
				TripID:    c.TripID,
				UserID:    c.UserID,
				Timestamp: time.Now(),
				Version:   1,
			},
			Payload: payload,
		}},
	}, nil
}
