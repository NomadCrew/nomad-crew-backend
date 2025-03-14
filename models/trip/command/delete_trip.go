package command

import (
	"context"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/shared"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
)

type DeleteTripCommand struct {
	BaseCommand
	TripID string
}

func (c *DeleteTripCommand) Validate(ctx context.Context) error {
	if c.TripID == "" {
		return errors.ValidationFailed("trip_id_required", "Trip ID is required")
	}
	return nil
}

func (c *DeleteTripCommand) ValidatePermissions(ctx context.Context) error {
	return c.BaseCommand.ValidatePermissions(ctx, c.TripID, types.MemberRoleOwner)
}

func (c *DeleteTripCommand) Execute(ctx context.Context) (*interfaces.CommandResult, error) {
	if err := c.Validate(ctx); err != nil {
		return nil, err
	}
	if err := c.ValidatePermissions(ctx); err != nil {
		return nil, err
	}

	// Verify trip exists
	if _, err := c.Ctx.Store.GetTrip(ctx, c.TripID); err != nil {
		return nil, err
	}

	// Perform soft delete
	if err := c.Ctx.Store.SoftDeleteTrip(ctx, c.TripID); err != nil {
		return nil, err
	}

	// Emit event using the shared emitter
	emitter := shared.NewEventEmitter(c.Ctx.EventBus)
	if err := emitter.EmitTripEvent(
		ctx,
		c.TripID,
		types.EventTypeTripDeleted,
		nil,
		c.UserID,
	); err != nil {
		return nil, err
	}

	return &interfaces.CommandResult{
		Success: true,
		Events: []types.Event{{
			BaseEvent: types.BaseEvent{
				ID:        uuid.NewString(),
				Type:      types.EventTypeTripDeleted,
				TripID:    c.TripID,
				UserID:    c.UserID,
				Timestamp: time.Now(),
				Version:   1,
			},
		}},
	}, nil
}
