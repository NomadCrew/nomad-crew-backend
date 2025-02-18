package command

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/golang-jwt/jwt/v5"
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

	// Generate invitation token before creation
	token := c.generateInvitationJWT()
	c.Invitation.Token = token

	if err := c.Ctx.Store.CreateInvitation(ctx, c.Invitation); err != nil {
		return nil, err
	}

	// Build the acceptance URL to include in the email
	acceptanceURL := fmt.Sprintf("%s/accept-invite/%s",
		c.Ctx.Config.FrontendURL,
		token,
	)

	payload, _ := json.Marshal(types.InvitationCreatedEvent{
		InvitationID:  c.Invitation.ID,
		InviteeEmail:  c.Invitation.InviteeEmail,
		ExpiresAt:     c.Invitation.ExpiresAt,
		AcceptanceURL: acceptanceURL,
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

func (c *InviteMemberCommand) generateInvitationJWT() string {
	claims := &types.InvitationClaims{
		InvitationID: c.Invitation.ID,
		TripID:       c.Invitation.TripID,
		InviteeEmail: c.Invitation.InviteeEmail,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(c.Invitation.ExpiresAt),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, _ := token.SignedString([]byte(c.Ctx.Config.JwtSecretKey))
	return signedToken
}
