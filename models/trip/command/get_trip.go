package command

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

type GetTripCommand struct {
	BaseCommand
	TripID string
}

func (c *GetTripCommand) Validate(ctx context.Context) error {
	if c.TripID == "" {
		return errors.ValidationFailed("trip_id_required", "Trip ID is required")
	}
	return nil
}

func (c *GetTripCommand) ValidatePermissions(ctx context.Context) error {
	// Require at least member access
	return c.BaseCommand.ValidatePermissions(ctx, c.TripID, types.MemberRoleMember)
}

func (c *GetTripCommand) Execute(ctx context.Context) (*interfaces.CommandResult, error) {
	if err := c.Validate(ctx); err != nil {
		return nil, err
	}
	if err := c.ValidatePermissions(ctx); err != nil {
		return nil, err
	}

	trip, err := c.Ctx.Store.GetTrip(ctx, c.TripID)
	if err != nil {
		return nil, errors.NotFound("trip_not_found", "Trip not found")
	}

	return &interfaces.CommandResult{
		Success: true,
		Data:    trip,
	}, nil
}
