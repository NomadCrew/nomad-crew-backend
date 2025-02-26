package command

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

type SearchTripsCommand struct {
	BaseCommand
	Criteria types.TripSearchCriteria
}

func (c *SearchTripsCommand) Validate(ctx context.Context) error {
	// Set defaults
	if c.Criteria.Limit == 0 {
		c.Criteria.Limit = 50
	} else if c.Criteria.Limit > 100 {
		return errors.ValidationFailed("max_limit", "max limit is 100")
	}

	if c.Criteria.Offset < 0 {
		c.Criteria.Offset = 0
	}

	// Validate date ranges
	if !c.Criteria.StartDate.IsZero() && !c.Criteria.EndDate.IsZero() &&
		c.Criteria.StartDate.After(c.Criteria.EndDate) {
		return errors.ValidationFailed("invalid_date_range", "invalid date range")
	}

	return nil
}

func (c *SearchTripsCommand) ValidatePermissions(ctx context.Context) error {
	// Basic permission check - user must be authenticated
	if c.UserID == "" {
		return errors.Unauthorized("auth_required", "authentication required for search")
	}
	return nil
}

func (c *SearchTripsCommand) Execute(ctx context.Context) (*interfaces.CommandResult, error) {
	if err := c.Validate(ctx); err != nil {
		return nil, err
	}

	if err := c.ValidatePermissions(ctx); err != nil {
		return nil, err
	}

	// Enforce user-specific filtering
	c.Criteria.UserID = c.UserID

	trips, err := c.Ctx.Store.SearchTrips(ctx, c.Criteria)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}

	return &interfaces.CommandResult{
		Success: true,
		Data:    trips,
	}, nil
}
