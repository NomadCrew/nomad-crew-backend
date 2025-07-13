package command

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
)

type GetTripMembersCommand struct {
	BaseCommand
	TripID string
}

func (c *GetTripMembersCommand) Validate() error {
	if c.TripID == "" {
		return errors.ValidationFailed("trip_id_required", "Trip ID is required")
	}
	return nil
}

func (c *GetTripMembersCommand) Execute(ctx context.Context) (*interfaces.CommandResult, error) {
	// Verify requester has access to the trip
	if _, err := c.Ctx.Store.GetUserRole(ctx, c.TripID, c.UserID); err != nil {
		return nil, errors.Forbidden("user_not_member", "User is not a member of this trip")
	}

	members, err := c.Ctx.Store.GetTripMembers(ctx, c.TripID)
	if err != nil {
		return nil, err
	}

	return &interfaces.CommandResult{
		Success: true,
		Data:    members,
	}, nil
}
