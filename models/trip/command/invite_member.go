package command

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type InviteMemberCommand struct {
	BaseCommand
	Invitation *types.TripInvitation
}

func (c *InviteMemberCommand) Validate(ctx context.Context) error {
	logger.GetLogger().Debugw("Validating invitation",
		"inviteeEmail", c.Invitation.InviteeEmail,
		"tripID", c.Invitation.TripID,
		"expiresAt", c.Invitation.ExpiresAt)
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

	// Get trip details for email
	trip, err := c.Ctx.Store.GetTrip(ctx, c.Invitation.TripID)
	if err != nil {
		return nil, err
	}

	// Ensure invitation has an ID before generating the token
	if c.Invitation.ID == "" {
		c.Invitation.ID = uuid.NewString()
	}

	// Generate JWT token
	token, err := c.generateInvitationJWT()
	if err != nil {
		return nil, err
	}
	c.Invitation.Token = token

	if err := c.Ctx.Store.CreateInvitation(ctx, c.Invitation); err != nil {
		return nil, err
	}

	// Build acceptance URL - use the new format
	acceptanceURL := fmt.Sprintf("nomadcrew://invite/accept/%s", token)

	log := logger.GetLogger()
	log.Infow("Generated invitation acceptance URL",
		"url", acceptanceURL,
		"inviteeEmail", c.Invitation.InviteeEmail,
		"tripId", c.Invitation.TripID)

	// Get API host from config for fallback
	apiHost := "https://nomadcrew.uk/api/v1"
	if c.Ctx.Config != nil && c.Ctx.Config.FrontendURL != "" {
		// Extract host from frontend URL
		apiHost = strings.TrimSuffix(c.Ctx.Config.FrontendURL, "/") + "/api/v1"
	}

	// Add protocol if missing
	if !strings.HasPrefix(apiHost, "http") {
		apiHost = "https://" + apiHost
	}

	// Build web API URL
	webAPIURL := fmt.Sprintf("%s/trips/invitations/accept/%s", apiHost, token)

	// Send invitation email
	emailData := types.EmailData{
		To:      c.Invitation.InviteeEmail,
		Subject: fmt.Sprintf("You're invited to join %s on NomadCrew", trip.Name),
		TemplateData: map[string]interface{}{
			"UserEmail":       c.Invitation.InviteeEmail,
			"TripName":        trip.Name,
			"AcceptanceURL":   webAPIURL,     // Use the web API URL for the main button
			"AppDeepLink":     acceptanceURL, // App deep link for direct app access
			"InvitationToken": token,
		},
	}

	if err := c.Ctx.EmailSvc.SendInvitationEmail(ctx, emailData); err != nil {
		// Log error but don't fail the command - we can retry email sending
		log := logger.GetLogger()
		log.Errorw("Failed to send invitation email",
			"error", err,
			"inviteeEmail", c.Invitation.InviteeEmail,
			"tripId", c.Invitation.TripID)
	}

	// Create event payload
	payload, _ := json.Marshal(types.InvitationCreatedEvent{
		EventID:       uuid.NewString(),
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
				ID:        uuid.NewString(),
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

func (c *InviteMemberCommand) generateInvitationJWT() (string, error) {
	if c.Ctx.Config == nil || c.Ctx.Config.JwtSecretKey == "" {
		logger.GetLogger().Error("Missing JWT secret configuration")
		return "", errors.New("configuration_error", "missing JWT secret configuration", "JwtSecretKey is not configured")
	}

	claims := &types.InvitationClaims{
		InvitationID: c.Invitation.ID,
		TripID:       c.Invitation.TripID,
		InviteeEmail: c.Invitation.InviteeEmail,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(c.Invitation.ExpiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(c.Ctx.Config.JwtSecretKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signedToken, nil
}
