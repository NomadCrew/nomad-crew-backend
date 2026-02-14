package command

import (
	"context"
	"encoding/json"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/shared"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
)

type RemoveMemberCommand struct {
	BaseCommand
	TripID           string
	MemberID         string
	RequestingUserID string
}

func (c *RemoveMemberCommand) Validate(ctx context.Context) error {
	if c.TripID == "" || c.MemberID == "" {
		return errors.ValidationFailed("trip_id_and_member_required", "trip ID and member ID are required")
	}
	return nil
}

func (c *RemoveMemberCommand) ValidatePermissions(ctx context.Context) error {
	// Special case: Allow removal if user is removing themselves
	if c.MemberID == c.BaseCommand.UserID {
		return nil
	}
	return c.BaseCommand.ValidatePermissions(ctx, c.TripID, types.MemberRoleOwner)
}

func (c *RemoveMemberCommand) Execute(ctx context.Context) (*interfaces.CommandResult, error) {
	if err := c.Validate(ctx); err != nil {
		return nil, err
	}
	if err := c.ValidatePermissions(ctx); err != nil {
		return nil, err
	}

	// Use the locked method that wraps owner check + removal in a transaction with FOR UPDATE
	// This prevents TOCTOU race conditions on concurrent owner removals
	if err := c.Ctx.Store.RemoveMemberWithOwnerLock(ctx, c.TripID, c.MemberID); err != nil {
		return nil, err
	}

	// Emit event via the shared emitter
	emitter := shared.NewEventEmitter(c.Ctx.EventBus)
	payload, _ := json.Marshal(types.MemberRemovedEvent{
		RemovedUserID: c.MemberID,
		RemovedBy:     c.BaseCommand.UserID,
	})
	if err := emitter.EmitTripEvent(
		ctx,
		c.TripID,
		types.EventTypeMemberRemoved,
		map[string]string{"user_id": c.MemberID},
		c.RequestingUserID,
	); err != nil {
		return nil, err
	}

	return &interfaces.CommandResult{
		Success: true,
		Events: []types.Event{{
			BaseEvent: types.BaseEvent{
				ID:        uuid.NewString(),
				Type:      types.EventTypeMemberRemoved,
				TripID:    c.TripID,
				UserID:    c.MemberID,
				Timestamp: time.Now(),
				Version:   1,
			},
			Payload: payload,
		}},
	}, nil
}
