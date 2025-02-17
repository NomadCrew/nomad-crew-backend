package command

import (
	"context"
	"encoding/json"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/validation"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

type UpdateMemberRoleCommand struct {
	BaseCommand
	TripID   string
	MemberID string
	NewRole  types.MemberRole
}

func (c *UpdateMemberRoleCommand) Validate(ctx context.Context) error {
    if c.TripID == "" {
        return errors.ValidationFailed("trip_id_required", "trip ID is required")
    }
    if c.MemberID == "" {
        return errors.ValidationFailed("member_id_required", "member ID is required")
    }
    if !c.NewRole.IsValid() {
        return errors.ValidationFailed("invalid_member_role", "invalid member role")
    }

    // Validate last owner protection
    if c.NewRole != types.MemberRoleOwner {
        // Check if this is the last owner
        members, err := c.Ctx.Store.GetTripMembers(ctx, c.TripID)
        if err != nil {
            return err
        }

        ownerCount := 0
        for _, member := range members {
            if member.Role == types.MemberRoleOwner {
                ownerCount++
            }
        }

        if ownerCount <= 1 {
            return errors.ValidationFailed(
                "last_owner",
                "Cannot change role of last owner",
            )
        }
    }

    return nil
}

func (c *UpdateMemberRoleCommand) ValidatePermissions(ctx context.Context) error {
	// Requester must be owner to change roles
	return c.BaseCommand.ValidatePermissions(ctx, c.TripID, types.MemberRoleOwner)
}

func (c *UpdateMemberRoleCommand) Execute(ctx context.Context) (*interfaces.CommandResult, error) {
	if err := c.Validate(ctx); err != nil {
		return nil, err
	}
	if err := c.ValidatePermissions(ctx); err != nil {
		return nil, err
	}

	currentRole, err := c.Ctx.Store.GetUserRole(ctx, c.TripID, c.MemberID)
	if err != nil {
		return nil, errors.NotFound("member_not_found", "member not found in trip")
	}

	if err := validation.ValidateRoleTransition(currentRole, c.NewRole); err != nil {
		return nil, err
	}

	if err := c.Ctx.Store.UpdateMemberRole(ctx, c.TripID, c.MemberID, c.NewRole); err != nil {
		return nil, err
	}

	eventPayload := types.MemberRoleUpdatedEvent{
		MemberID:  c.MemberID,
		OldRole:   currentRole,
		NewRole:   c.NewRole,
		UpdatedBy: c.UserID,
	}

	// Marshal the payload to JSON
	payloadBytes, err := json.Marshal(eventPayload)
	if err != nil {
		return nil, err
	}

	return &interfaces.CommandResult{
		Success: true,
		Data:    eventPayload,
		Events:  []types.Event{createRoleUpdatedEvent(c.TripID, c.UserID, payloadBytes)},
	}, nil
}

func createRoleUpdatedEvent(tripID, userID string, payload json.RawMessage) types.Event {
	return types.Event{
		BaseEvent: types.BaseEvent{
			Type:      types.EventTypeMemberRoleUpdated,
			TripID:    tripID,
			UserID:    userID,
			Timestamp: time.Now(),
			Version:   1,
		},
		Payload: payload,
	}
}
