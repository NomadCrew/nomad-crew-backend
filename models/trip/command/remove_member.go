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

	targetRole, err := c.Ctx.Store.GetUserRole(ctx, c.TripID, c.MemberID)
	if err != nil {
		return nil, errors.NotFound("member_not_found", "member not found in trip")
	}

	// The ValidateRoleTransition check with MemberRoleNone is removed as it's redundant
	// and MemberRoleNone is deprecated. GetUserRole confirms the member exists.

	// Last owner protection
	if targetRole == types.MemberRoleOwner {
		members, err := c.Ctx.Store.GetTripMembers(ctx, c.TripID)
		if err != nil {
			return nil, err
		}

		ownerCount := 0
		for _, member := range members {
			if member.Role == types.MemberRoleOwner {
				ownerCount++
			}
		}

		if ownerCount <= 1 {
			return nil, errors.ValidationFailed(
				"Cannot remove last owner",
				"There must be at least one owner remaining in the trip",
			)
		}
	}

	if err := c.Ctx.Store.RemoveMember(ctx, c.TripID, c.MemberID); err != nil {
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
