package command

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
)

type ListTripsCommand struct {
	BaseCommand
	UserID string
}

func (c *ListTripsCommand) Validate(ctx context.Context) error {
    if c.UserID == "" {
        return errors.ValidationFailed("user_id_required", "User ID is required")
    }
    return nil
}

func (c *ListTripsCommand) ValidatePermissions(ctx context.Context) error {
	// No specific permissions needed as users can list their own trips
	return nil
}

func (c *ListTripsCommand) Execute(ctx context.Context) (*interfaces.CommandResult, error) {
	if err := c.Validate(ctx); err != nil {
		return nil, err
	}

	trips, err := c.Ctx.Store.ListUserTrips(ctx, c.UserID)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}

	return &interfaces.CommandResult{
		Success: true,
		Data:    trips,
	}, nil
}
