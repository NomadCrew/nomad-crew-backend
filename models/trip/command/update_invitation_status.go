package command

import (
	"context"
	"encoding/json"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
)

type UpdateInvitationStatusCommand struct {
	BaseCommand
	InvitationID string
	NewStatus    types.InvitationStatus
}

func (c *UpdateInvitationStatusCommand) Validate(ctx context.Context) error {
	if c.InvitationID == "" {
		return errors.ValidationFailed("invitation_required", "invitation ID is required")
	}
	if !c.NewStatus.IsValid() {
		return errors.ValidationFailed("invalid_invitation_status", "invalid invitation status")
	}
	return nil
}

func (c *UpdateInvitationStatusCommand) ValidatePermissions(ctx context.Context) error {
	// No role-based permissions, handled in Execute
	return nil
}

func (c *UpdateInvitationStatusCommand) Execute(ctx context.Context) (*interfaces.CommandResult, error) {
	if err := c.Validate(ctx); err != nil {
		return nil, err
	}

	invitation, err := c.Ctx.Store.GetInvitation(ctx, c.InvitationID)
	if err != nil {
		return nil, err
	}

	// Verify user is the invitee
	if invitation.InviteeEmail != c.UserID {
		return nil, errors.Forbidden("update_forbidden", "user cannot update this invitation")
	}

	if err := c.Ctx.Store.UpdateInvitationStatus(ctx, c.InvitationID, c.NewStatus); err != nil {
		return nil, err
	}

	return &interfaces.CommandResult{
		Success: true,
		Events: []types.Event{{
			BaseEvent: types.BaseEvent{
				ID:        uuid.NewString(),
				Type:      types.EventTypeInvitationStatusUpdated,
				TripID:    invitation.TripID,
				UserID:    c.UserID,
				Timestamp: time.Now(),
				Version:   1,
			},
			Payload: json.RawMessage(mustMarshal(types.InvitationStatusUpdatedEvent{
				InvitationID: c.InvitationID,
				NewStatus:    c.NewStatus,
			})),
		}},
	}, nil
}

func mustMarshal(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
