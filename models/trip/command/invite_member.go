package command

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
    "strings"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/shared"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
	"github.com/supabase-community/supabase-go"
)

type InviteMemberCommand struct {
	BaseCommand
	Invitation *types.TripInvitation
}

func (c *InviteMemberCommand) Validate(ctx context.Context) error {
    if c.Invitation.TripID == "" {
        return errors.ValidationFailed("trip_id_required", "Trip ID is required")
    }
    if c.Invitation.InviteeEmail == "" {
        return errors.ValidationFailed("invitee_email_required", "Invitee email is required")
    }
    
    // Email format validation
    if !strings.Contains(c.Invitation.InviteeEmail, "@") {
        return errors.ValidationFailed("invalid_email", "Invalid email format")
    }

    // Expiration validation
    if c.Invitation.ExpiresAt.IsZero() {
        c.Invitation.ExpiresAt = time.Now().Add(7 * 24 * time.Hour) // Default 7 days
    } else if c.Invitation.ExpiresAt.Before(time.Now()) {
        return errors.ValidationFailed("invalid_expiration", "Expiration time cannot be in the past")
    }

    // Role validation
    if !c.Invitation.Role.IsValid() {
        return errors.ValidationFailed("invalid_role", "Invalid member role")
    }

    return nil
}

func (c *InviteMemberCommand) ValidatePermissions(ctx context.Context) error {
	return c.BaseCommand.ValidatePermissions(ctx, c.Invitation.TripID, types.MemberRoleOwner)
}

func (c *InviteMemberCommand) Execute(ctx context.Context) (*interfaces.CommandResult, error) {
	if err := c.Validate(ctx); err != nil {
		return nil, err
	}
	if err := c.ValidatePermissions(ctx); err != nil {
		return nil, err
	}

	// Set default expiration (7 days)
	if c.Invitation.ExpiresAt.IsZero() {
		c.Invitation.ExpiresAt = time.Now().Add(7 * 24 * time.Hour)
	}

	if err := c.Ctx.Store.CreateInvitation(ctx, c.Invitation); err != nil {
		return nil, err
	}

	payload, _ := json.Marshal(types.InvitationCreatedEvent{
		InvitationID: c.Invitation.ID,
		InviteeEmail: c.Invitation.InviteeEmail,
		ExpiresAt:    c.Invitation.ExpiresAt,
	})

	return &interfaces.CommandResult{
		Success: true,
		Data:    c.Invitation,
		Events: []types.Event{{
			BaseEvent: types.BaseEvent{
				Type:      types.EventTypeInvitationCreated,
				TripID:    c.Invitation.TripID,
				UserID:    c.UserID,
				Timestamp: time.Now(),
				Version:   1,
			},
			Payload: payload,
		}},
	}, nil
}

func (c *InviteMemberCommand) handleExistingUserInvite(ctx context.Context, user *types.SupabaseUser) error {
	payload, _ := json.Marshal(map[string]interface{}{
		"tripId":       c.Invitation.TripID,
		"invitationId": c.Invitation.ID,
	})

	emitter := shared.NewEventEmitter(c.Ctx.EventBus)
	return emitter.EmitTripEvent(
		ctx,
		c.Invitation.TripID,
		types.EventTypeInvitationCreated,
		payload,
		user.ID,
	)
}

func (c *InviteMemberCommand) handleNewUserInvite(ctx context.Context) error {
	signUpData := map[string]interface{}{
		"email": c.Invitation.InviteeEmail,
		"redirect_to": fmt.Sprintf("%s/accept-invite/%s",
			c.Ctx.Config.FrontendURL,
			c.Invitation.ID),
	}

	_, err := c.Ctx.SupabaseClient.Auth.Signup(c.Request.Context(), supabase.SignupRequest{
		Email:    c.Invitation.InviteeEmail,
		Password: uuid.New().String(),
		Data:     signUpData,
	})
	return err
}
