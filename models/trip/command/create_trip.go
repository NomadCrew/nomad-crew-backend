package command

import (
	"context"
	"encoding/json"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/validation"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
)

type CreateTripCommand struct {
	BaseCommand
	Trip *types.Trip
}

func (c *CreateTripCommand) Validate(ctx context.Context) error {
	if c.Trip == nil {
		return errors.ValidationFailed("trip_required", "Trip data is required")
	}
	return validation.ValidateNewTrip(c.Trip)
}

func (c *CreateTripCommand) ValidatePermissions(ctx context.Context) error {
	// No existing trip to validate against
	// Creation requires valid user session only
	return nil
}

func (c *CreateTripCommand) Execute(ctx context.Context) (*interfaces.CommandResult, error) {
	if err := c.Validate(ctx); err != nil {
		return nil, err
	}

	// Set system-managed fields
	c.Trip.CreatedAt = time.Now()
	c.Trip.UpdatedAt = time.Now()

	createdID, err := c.Ctx.Store.CreateTrip(ctx, *c.Trip)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}

	// Fetch the full trip record with database-generated fields
	createdTrip, err := c.Ctx.Store.GetTrip(ctx, createdID)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	c.Trip = createdTrip

	payload, _ := json.Marshal(c.Trip)

	return &interfaces.CommandResult{
		Success: true,
		Data:    c.Trip,
		Events: []types.Event{{
			BaseEvent: types.BaseEvent{
				Type:      types.EventTypeTripCreated,
				TripID:    c.Trip.ID,
				UserID:    c.UserID,
				Timestamp: time.Now(),
				Version:   1,
			},
			Payload: payload,
		}},
	}, nil
}

func generateTripID() string {
	// Implementation from your ID generation service
	return "TRIP_" + uuid.New().String()
}
