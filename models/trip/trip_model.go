package trip

import (
	"context"
	"fmt"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/command"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	store "github.com/NomadCrew/nomad-crew-backend/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/supabase-community/supabase-go"
)

type TripModel struct {
	cmdCtx *interfaces.CommandContext
	store  store.TripStore
	config *config.ServerConfig
}

var _ interfaces.TripModelInterface = (*TripModel)(nil)

func NewTripModel(
	store store.TripStore,
	eventBus types.EventPublisher,
	weatherSvc types.WeatherServiceInterface,
	supabaseClient *supabase.Client,
	config *config.ServerConfig,
	emailSvc types.EmailService,
	chatStore store.ChatStore,
) *TripModel {
	return &TripModel{
		store: store,
		cmdCtx: command.NewCommandContext(
			store,
			eventBus,
			weatherSvc,
			supabaseClient,
			config,
			emailSvc,
			chatStore,
		),
		config: config,
	}
}

func (tm *TripModel) CreateTrip(ctx context.Context, trip *types.Trip) error {
	cmd := &command.CreateTripCommand{
		BaseCommand: command.BaseCommand{
			UserID: getUserIdFromContext(ctx),
			Ctx:    tm.GetCommandContext(),
		},
		Trip: trip,
	}

	trip.CreatedBy = cmd.UserID

	result, err := cmd.Execute(ctx)
	if err != nil {
		return err
	}

	// Update trip with created data
	*trip = *(result.Data.(*types.Trip))
	return nil
}

func (tm *TripModel) UpdateTripStatus(ctx context.Context, tripID string, newStatus types.TripStatus) error {
	cmd := &command.UpdateTripStatusCommand{
		BaseCommand: command.BaseCommand{
			UserID: getUserIdFromContext(ctx),
			Ctx:    tm.cmdCtx,
		},
		TripID:    tripID,
		NewStatus: newStatus,
	}

	_, err := cmd.Execute(ctx)
	return err
}

func (tm *TripModel) InviteMember(ctx context.Context, invitation *types.TripInvitation) error {
	cmd := &command.InviteMemberCommand{
		BaseCommand: command.BaseCommand{
			UserID: invitation.InviterID,
			Ctx:    tm.cmdCtx,
		},
		Invitation: invitation,
	}

	_, err := cmd.Execute(ctx)
	return err
}

func (tm *TripModel) UpdateMemberRole(ctx context.Context, tripID, memberID string, newRole types.MemberRole) (*interfaces.CommandResult, error) {
	cmd := &command.UpdateMemberRoleCommand{
		BaseCommand: command.BaseCommand{
			UserID: getUserIdFromContext(ctx),
			Ctx:    tm.cmdCtx,
		},
		TripID:   tripID,
		MemberID: memberID,
		NewRole:  newRole,
	}

	return cmd.Execute(ctx)
}

func (tm *TripModel) RemoveMember(ctx context.Context, tripID, userID string) error {
	cmd := &command.RemoveMemberCommand{
		BaseCommand: command.BaseCommand{
			UserID: getUserIdFromContext(ctx),
			Ctx:    tm.cmdCtx,
		},
		TripID:   tripID,
		MemberID: userID,
	}

	_, err := cmd.Execute(ctx)
	return err
}

func (tm *TripModel) GetTripMembers(ctx context.Context, tripID string) ([]*types.TripMembership, error) {
	cmd := &command.GetTripMembersCommand{
		BaseCommand: command.BaseCommand{
			UserID: getUserIdFromContext(ctx),
			Ctx:    tm.cmdCtx,
		},
		TripID: tripID,
	}

	result, err := cmd.Execute(ctx)
	if err != nil {
		return nil, err
	}

	return result.Data.([]*types.TripMembership), nil
}

func (tm *TripModel) GetTripByID(ctx context.Context, id string) (*types.Trip, error) {
	cmd := &command.GetTripCommand{
		BaseCommand: command.BaseCommand{
			UserID: getUserIdFromContext(ctx),
			Ctx:    tm.cmdCtx,
		},
		TripID: id,
	}

	result, err := cmd.Execute(ctx)
	if err != nil {
		return nil, tm.tripNotFound(id)
	}
	return result.Data.(*types.Trip), nil
}

func (tm *TripModel) AddMember(ctx context.Context, membership *types.TripMembership) error {
	cmd := &command.AddMemberCommand{
		BaseCommand: command.BaseCommand{
			UserID: getUserIdFromContext(ctx),
			Ctx:    tm.cmdCtx,
		},
		TripID: membership.TripID,
		UserID: membership.UserID,
		Role:   membership.Role,
	}

	result, err := cmd.Execute(ctx)
	if err != nil {
		return err
	}

	// Update membership with generated data
	*membership = *(result.Data.(*types.TripMembership))
	return nil
}

func (tm *TripModel) UpdateInvitationStatus(ctx context.Context, invitationID string, status types.InvitationStatus) error {
	cmd := &command.UpdateInvitationStatusCommand{
		BaseCommand: command.BaseCommand{
			UserID: getUserIdFromContext(ctx),
			Ctx:    tm.cmdCtx,
		},
		InvitationID: invitationID,
		NewStatus:    status,
	}

	_, err := cmd.Execute(ctx)
	return err
}

func (tm *TripModel) SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error) {
	cmd := &command.SearchTripsCommand{
		BaseCommand: command.BaseCommand{
			UserID: getUserIdFromContext(ctx),
			Ctx:    tm.cmdCtx,
		},
		Criteria: criteria,
	}

	result, err := cmd.Execute(ctx)
	if err != nil {
		return nil, err
	}
	return result.Data.([]*types.Trip), nil
}

func (tm *TripModel) GetTripWithMembers(ctx context.Context, tripID string) (*types.TripWithMembers, error) {
	// Get basic trip details
	trip, err := tm.GetTripByID(ctx, tripID)
	if err != nil {
		return nil, tm.tripNotFound(tripID)
	}

	// Get members using existing command pattern
	members, err := tm.GetTripMembers(ctx, tripID)
	if err != nil {
		return nil, err
	}

	return &types.TripWithMembers{
		Trip:    *trip,
		Members: members,
	}, nil
}

// GetCommandContext returns the command context for dependency injection
func (tm *TripModel) GetCommandContext() *interfaces.CommandContext {
	return tm.cmdCtx
}

// GetTripStore returns the trip store
func (tm *TripModel) GetTripStore() store.TripStore {
	return tm.store
}

// GetChatStore returns the chat store from the command context
func (tm *TripModel) GetChatStore() store.ChatStore {
	return tm.cmdCtx.ChatStore
}

func getUserIdFromContext(ctx context.Context) string {
	if userID, ok := ctx.Value("user_id").(string); ok {
		return userID
	}
	return ""
}

func (tm *TripModel) tripNotFound(tripID string) error {
	return &errors.AppError{
		Type:    errors.TripNotFoundError,
		Message: "Trip not found",
		Detail:  fmt.Sprintf("Trip with ID %s does not exist", tripID),
	}
}

func (tm *TripModel) CreateInvitation(ctx context.Context, invitation *types.TripInvitation) error {
	invitation.InviterID = getUserIdFromContext(ctx)
	return tm.store.CreateInvitation(ctx, invitation)
}

func (tm *TripModel) DeleteTrip(ctx context.Context, id string) error {
	cmd := &command.DeleteTripCommand{
		BaseCommand: command.BaseCommand{
			UserID: getUserIdFromContext(ctx),
			Ctx:    tm.cmdCtx,
		},
		TripID: id,
	}
	_, err := cmd.Execute(ctx)
	return err
}

func (tm *TripModel) ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error) {
	cmd := &command.ListTripsCommand{
		BaseCommand: command.BaseCommand{
			UserID: userID,
			Ctx:    tm.cmdCtx,
		},
	}

	result, err := cmd.Execute(ctx)
	if err != nil {
		return nil, err
	}

	return result.Data.([]*types.Trip), nil
}

func (tm *TripModel) UpdateTrip(ctx context.Context, id string, update *types.TripUpdate) error {
	cmd := &command.UpdateTripCommand{
		BaseCommand: command.BaseCommand{
			UserID: getUserIdFromContext(ctx),
			Ctx:    tm.cmdCtx,
		},
		TripID: id,
		Update: update,
	}

	_, err := cmd.Execute(ctx)
	return err
}

func (tm *TripModel) GetInvitation(ctx context.Context, invitationID string) (*types.TripInvitation, error) {
	invitation, err := tm.store.GetInvitation(ctx, invitationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get invitation: %w", err)
	}
	return invitation, nil
}

// FindInvitationByTripAndEmail finds an invitation by trip ID and invitee email
func (tm *TripModel) FindInvitationByTripAndEmail(ctx context.Context, tripID, email string) (*types.TripInvitation, error) {
	invitations, err := tm.store.GetInvitationsByTripID(ctx, tripID)
	if err != nil {
		return nil, fmt.Errorf("failed to get invitations for trip: %w", err)
	}

	for _, invitation := range invitations {
		if invitation.InviteeEmail == email && invitation.Status == types.InvitationStatusPending {
			return invitation, nil
		}
	}

	return nil, errors.NotFound("invitation_not_found", "No pending invitation found for this email and trip")
}

func (tm *TripModel) GetUserRole(ctx context.Context, tripID, userID string) (types.MemberRole, error) {
	// Add validation to prevent empty ID errors
	if tripID == "" {
		return "", fmt.Errorf("cannot get user role with empty trip ID")
	}
	if userID == "" {
		return "", fmt.Errorf("cannot get user role with empty user ID")
	}

	return tm.store.GetUserRole(ctx, tripID, userID)
}

func (tm *TripModel) LookupUserByEmail(ctx context.Context, email string) (*types.SupabaseUser, error) {
	return tm.store.LookupUserByEmail(ctx, email)
}
