package command

import (
	"context"
	"encoding/json"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
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

	actorIsInvitee := false
	if invitation.InviteeID != nil && *invitation.InviteeID != "" {
		if *invitation.InviteeID == c.UserID { // c.UserID is from BaseCommand, the authenticated user
			actorIsInvitee = true
		}
	} else {
		// InviteeID is not set, compare actor's email with InviteeEmail.
		if c.Ctx.UserStore == nil {
			logger.GetLogger().Error("UpdateInvitationStatusCommand: UserStore not available in command context for email comparison.")
			return nil, errors.InternalServerError("UserStore not available in command context")
		}
		// Assuming c.UserID in BaseCommand is the Supabase Auth ID (string)
		// and UserStore.GetUserBySupabaseID or GetUserByID expects this string.
		// Let's use GetUserByID, assuming it can handle the string ID from BaseCommand.
		currentUser, userErr := c.Ctx.UserStore.GetUserByID(ctx, c.UserID)
		if userErr != nil {
			logger.GetLogger().Errorw("UpdateInvitationStatusCommand: Failed to fetch current user details for permission check.", "userID", c.UserID, "error", userErr)
			return nil, errors.Wrap(userErr, "fetch_user_failed", "failed to fetch current user details for permission check")
		}
		if currentUser != nil && currentUser.Email == invitation.InviteeEmail {
			actorIsInvitee = true
		} else if currentUser == nil {
			logger.GetLogger().Warnw("UpdateInvitationStatusCommand: Current user not found by ID for email comparison.", "userID", c.UserID)
		}
	}

	if !actorIsInvitee {
		return nil, errors.Forbidden("update_forbidden", "User cannot update this invitation. Actor is not the invitee.")
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
