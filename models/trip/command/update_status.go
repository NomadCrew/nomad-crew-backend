package command

import (
	"context"
	"fmt"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/shared"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/validation"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

type UpdateTripStatusCommand struct {
	BaseCommand
	TripID    string
	NewStatus types.TripStatus
}

func (c *UpdateTripStatusCommand) Validate() error {
	if !c.NewStatus.IsValid() {
		return errors.ValidationFailed(
			"invalid status",
			fmt.Sprintf("status %s is not valid", c.NewStatus),
		)
	}
	return nil
}

func (c *UpdateTripStatusCommand) Execute(ctx context.Context) (*interfaces.CommandResult, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	trip, err := c.Ctx.Store.GetTrip(ctx, c.TripID)
	if err != nil {
		return nil, err
	}

	if err := validation.ValidateStatusTransition(trip, c.NewStatus); err != nil {
		return nil, err
	}

	update := types.TripUpdate{Status: c.NewStatus}
	updatedTrip, err := c.Ctx.Store.UpdateTrip(ctx, c.TripID, update)
	if err != nil {
		return nil, err
	}

	emitter := shared.NewEventEmitter(c.Ctx.EventBus)
	if err := emitter.EmitTripEvent(
		ctx,
		c.TripID,
		types.EventTypeTripStatusUpdated,
		updatedTrip,
		c.UserID,
	); err != nil {
		return nil, err
	}

	// Handle weather updates based on new status
	if c.NewStatus == types.TripStatusActive || c.NewStatus == types.TripStatusPlanning {
		c.Ctx.WeatherSvc.StartWeatherUpdates(ctx, c.TripID, trip.Destination)
	}

	return &interfaces.CommandResult{
		Success: true,
		Data:    updatedTrip,
	}, nil
}
