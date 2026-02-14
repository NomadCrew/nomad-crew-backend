package service

import (
	"context"
	"fmt"

	internal_errors "github.com/NomadCrew/nomad-crew-backend/internal/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/events"
	"github.com/NomadCrew/nomad-crew-backend/internal/utils"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/supabase-community/supabase-go"
)

// InvitationService handles trip invitation operations
type InvitationService struct {
	store          store.TripStore
	emailSvc       types.EmailService
	supabaseClient *supabase.Client
	frontendURL    string
	eventPublisher types.EventPublisher
}

// NewInvitationService creates a new invitation service
func NewInvitationService(
	store store.TripStore,
	emailSvc types.EmailService,
	supabaseClient *supabase.Client,
	frontendURL string,
	eventPublisher types.EventPublisher,
) *InvitationService {
	return &InvitationService{
		store:          store,
		emailSvc:       emailSvc,
		supabaseClient: supabaseClient,
		frontendURL:    frontendURL,
		eventPublisher: eventPublisher,
	}
}

// CreateInvitation creates a new trip invitation
func (s *InvitationService) CreateInvitation(ctx context.Context, invitation *types.TripInvitation) error {
	log := logger.GetLogger()

	// Check if the invitation already exists for this email and trip
	existingInvitation, err := s.FindInvitationByTripAndEmail(ctx, invitation.TripID, invitation.InviteeEmail)
	if err == nil && existingInvitation != nil {
		log.Infow("Found existing invitation for email, will resend",
			"invitationID", existingInvitation.ID,
			"email", invitation.InviteeEmail,
			"status", existingInvitation.Status)

		// Update the existing invitation's status to pending if not already
		if existingInvitation.Status != types.InvitationStatusPending {
			if updateErr := s.UpdateInvitationStatus(ctx, existingInvitation.ID, types.InvitationStatusPending); updateErr != nil {
				return updateErr
			}
		}

		// Always resend the email for existing pending invitations
		// This allows users to "resend" invitations
		log.Infow("Resending invitation email", "invitationID", existingInvitation.ID, "email", invitation.InviteeEmail)
		return s.sendInvitationEmail(ctx, existingInvitation)
	}

	// Create a new invitation
	if err := s.store.CreateInvitation(ctx, invitation); err != nil {
		return err
	}

	// Publish event using the standardized function
	err = events.PublishEventWithContext(
		s.eventPublisher,
		ctx,
		string(types.EventTypeInvitationCreated), // Use canonical event type from types/events.go
		invitation.TripID,
		invitation.InviterID,
		map[string]interface{}{
			"event_id":      utils.GenerateEventID(), // Use utils func
			"invitation_id": invitation.ID,
			"invitee_email": invitation.InviteeEmail,
			"role":          string(invitation.Role),
		},
		"invitation-service",
	)
	if err != nil {
		// Log or handle publish error, but continue to send email
		logger.GetLogger().Errorw("Failed to publish invitation created event", "error", err, "invitationID", invitation.ID)
	}

	// Send invitation email
	return s.sendInvitationEmail(ctx, invitation)
}

// GetInvitation retrieves an invitation by ID
func (s *InvitationService) GetInvitation(ctx context.Context, invitationID string) (*types.TripInvitation, error) {
	invitation, err := s.store.GetInvitation(ctx, invitationID)
	if err != nil {
		return nil, internal_errors.NewNotFoundError("Invitation", invitationID) // Use internal error
	}
	return invitation, nil
}

// UpdateInvitationStatus updates the status of an invitation
func (s *InvitationService) UpdateInvitationStatus(ctx context.Context, invitationID string, status types.InvitationStatus) error {
	return s.store.UpdateInvitationStatus(ctx, invitationID, status)
}

// FindInvitationByTripAndEmail finds an invitation by trip ID and email
func (s *InvitationService) FindInvitationByTripAndEmail(ctx context.Context, tripID, email string) (*types.TripInvitation, error) {
	invitations, err := s.store.GetInvitationsByTripID(ctx, tripID)
	if err != nil {
		return nil, err
	}

	for _, inv := range invitations {
		if inv.InviteeEmail == email && inv.Status == types.InvitationStatusPending {
			return inv, nil
		}
	}

	return nil, internal_errors.NewNotFoundError("Invitation", email) // Use internal error
}

// GetInvitationsByTripID retrieves all invitations for a trip
func (s *InvitationService) GetInvitationsByTripID(ctx context.Context, tripID string) ([]*types.TripInvitation, error) {
	return s.store.GetInvitationsByTripID(ctx, tripID)
}

// LookupUserByEmail looks up a user by email using Supabase
func (s *InvitationService) LookupUserByEmail(ctx context.Context, email string) (*types.SupabaseUser, error) {
	// Use the store's implementation
	return s.store.LookupUserByEmail(ctx, email)
}

// Helper method to send an invitation email
func (s *InvitationService) sendInvitationEmail(ctx context.Context, invitation *types.TripInvitation) error {
	// Get the trip to get more details
	trip, err := s.store.GetTrip(ctx, invitation.TripID)
	if err != nil {
		return err
	}

	// Build the full invitation URL (pre-compute to avoid html/template URL ambiguity)
	baseURL := s.frontendURL
	if baseURL == "" {
		baseURL = "https://nomadcrew.uk"
	}
	invitationURL := fmt.Sprintf("%s/invite/accept/%s", baseURL, invitation.Token.String)

	// Prepare email data using the correct struct
	// Email service requires: UserEmail, TripName, InvitationURL
	emailData := types.EmailData{
		To:      invitation.InviteeEmail,
		Subject: "You're invited to join a trip on NomadCrew!",
		TemplateData: map[string]interface{}{
			"UserEmail":     invitation.InviteeEmail,
			"TripName":      trip.Name,
			"InvitationURL": invitationURL,
		},
	}

	// Send the email
	return s.emailSvc.SendInvitationEmail(ctx, emailData)
}
