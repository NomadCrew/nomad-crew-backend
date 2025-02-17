package command

import (
	"context"
	"encoding/json"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

type AcceptInvitationCommand struct {
	BaseCommand
	InvitationID string
}

func (c *AcceptInvitationCommand) Validate(ctx context.Context) error {
	if c.InvitationID == "" {
		return errors.ValidationFailed("invitation_id_required", "Invitation ID is required")
	}
	return nil
}

func (c *AcceptInvitationCommand) Execute(ctx context.Context) (*interfaces.CommandResult, error) {
	invitation, err := c.Ctx.Store.GetInvitation(ctx, c.InvitationID)
	if err != nil {
		return nil, errors.NotFound("invitation_not_found", "Invitation not found")
	}

	if invitation.Status != types.InvitationStatusPending || time.Now().After(invitation.ExpiresAt) {
		return nil, errors.NewConflictError(
			"Invitation is no longer valid",
			"Invitation has either been used or expired",
		)
	}

	membership := &types.TripMembership{
		TripID: invitation.TripID,
		UserID: c.UserID,
		Role:   invitation.Role,
		Status: types.MembershipStatusActive,
	}

	if err := c.Ctx.Store.AddMember(ctx, membership); err != nil {
		return nil, err
	}

	// 4. Update invitation status
	if err := c.Ctx.Store.UpdateInvitationStatus(ctx, c.InvitationID, types.InvitationStatusAccepted); err != nil {
		return nil, err
	}

	payload, _ := json.Marshal(membership)

	return &interfaces.CommandResult{
		Success: true,
		Data:    membership,
		Events: []types.Event{{
			BaseEvent: types.BaseEvent{
				Type:      types.EventTypeInvitationAccepted,
				TripID:    invitation.TripID,
				UserID:    c.UserID,
				Timestamp: time.Now(),
				Version:   1,
			},
			Payload: payload,
		}},
	}, nil
}
