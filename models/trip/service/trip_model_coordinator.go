package service

import (
	"context"
	"sync"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/internal/utils"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/supabase-community/supabase-go"
)

// TripModelCoordinator acts as a facade over the various trip-related services
// It implements the TripModelInterface to ensure backward compatibility
type TripModelCoordinator struct {
	tripService       *TripManagementService
	memberService     *TripMemberService
	invitationService *InvitationService
	chatService       *TripChatService
	store             store.TripStore
	chatStore         store.ChatStore
	config            *config.ServerConfig
	cmdCtx            *interfaces.CommandContext // Keep for backward compatibility
}

// NewTripModelCoordinator creates a new TripModelCoordinator
func NewTripModelCoordinator(
	store store.TripStore,
	eventBus types.EventPublisher,
	weatherSvc types.WeatherServiceInterface,
	supabaseClient *supabase.Client,
	config *config.ServerConfig,
	emailSvc types.EmailService,
	chatStore store.ChatStore,
) *TripModelCoordinator {
	// Create the command context for backward compatibility
	cmdCtx := &interfaces.CommandContext{
		Store:          store,
		EventBus:       eventBus,
		WeatherSvc:     weatherSvc,
		SupabaseClient: supabaseClient,
		Config:         config,
		RequestData:    new(sync.Map),
		EmailSvc:       emailSvc,
		ChatStore:      chatStore,
	}

	// Create the individual services
	tripService := NewTripManagementService(store, eventBus, weatherSvc)
	memberService := NewTripMemberService(store, eventBus)
	invitationService := NewInvitationService(store, emailSvc, supabaseClient, config.FrontendURL, eventBus)
	chatService := NewTripChatService(chatStore, store, eventBus)

	return &TripModelCoordinator{
		tripService:       tripService,
		memberService:     memberService,
		invitationService: invitationService,
		chatService:       chatService,
		store:             store,
		chatStore:         chatStore,
		config:            config,
		cmdCtx:            cmdCtx,
	}
}

// CreateTrip delegates to TripManagementService
func (c *TripModelCoordinator) CreateTrip(ctx context.Context, trip *types.Trip) (*types.Trip, error) {
	return c.tripService.CreateTrip(ctx, trip)
}

// GetTripByID delegates to TripManagementService
func (c *TripModelCoordinator) GetTripByID(ctx context.Context, id string, userID string) (*types.Trip, error) {
	return c.tripService.GetTrip(ctx, id, userID)
}

// UpdateTrip delegates to TripManagementService
func (c *TripModelCoordinator) UpdateTrip(ctx context.Context, id string, userID string, update *types.TripUpdate) (*types.Trip, error) {
	return c.tripService.UpdateTrip(ctx, id, userID, *update)
}

// DeleteTrip delegates to TripManagementService
func (c *TripModelCoordinator) DeleteTrip(ctx context.Context, id string) error {
	return c.tripService.DeleteTrip(ctx, id)
}

// ListUserTrips delegates to TripManagementService
func (c *TripModelCoordinator) ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error) {
	return c.tripService.ListUserTrips(ctx, userID)
}

// SearchTrips delegates to TripManagementService
func (c *TripModelCoordinator) SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error) {
	return c.tripService.SearchTrips(ctx, criteria)
}

// GetUserRole delegates to TripMemberService
func (c *TripModelCoordinator) GetUserRole(ctx context.Context, tripID, userID string) (types.MemberRole, error) {
	return c.memberService.GetUserRole(ctx, tripID, userID)
}

// AddMember delegates to TripMemberService
func (c *TripModelCoordinator) AddMember(ctx context.Context, membership *types.TripMembership) error {
	return c.memberService.AddMember(ctx, membership)
}

// UpdateMemberRole delegates to TripMemberService
func (c *TripModelCoordinator) UpdateMemberRole(ctx context.Context, tripID, userID string, role types.MemberRole) (*interfaces.CommandResult, error) {
	return c.memberService.UpdateMemberRole(ctx, tripID, userID, role)
}

// RemoveMember delegates to TripMemberService
func (c *TripModelCoordinator) RemoveMember(ctx context.Context, tripID, userID string) error {
	return c.memberService.RemoveMember(ctx, tripID, userID)
}

// CreateInvitation delegates to InvitationService
func (c *TripModelCoordinator) CreateInvitation(ctx context.Context, invitation *types.TripInvitation) error {
	return c.invitationService.CreateInvitation(ctx, invitation)
}

// GetInvitation delegates to InvitationService
func (c *TripModelCoordinator) GetInvitation(ctx context.Context, invitationID string) (*types.TripInvitation, error) {
	return c.invitationService.GetInvitation(ctx, invitationID)
}

// UpdateInvitationStatus delegates to InvitationService
func (c *TripModelCoordinator) UpdateInvitationStatus(ctx context.Context, invitationID string, status types.InvitationStatus) error {
	return c.invitationService.UpdateInvitationStatus(ctx, invitationID, status)
}

// LookupUserByEmail delegates to InvitationService
func (c *TripModelCoordinator) LookupUserByEmail(ctx context.Context, email string) (*types.SupabaseUser, error) {
	return c.invitationService.LookupUserByEmail(ctx, email)
}

// UpdateTripStatus delegates to TripManagementService
func (c *TripModelCoordinator) UpdateTripStatus(ctx context.Context, tripID string, newStatus types.TripStatus) error {
	userID, err := utils.GetUserIDFromContext(ctx)
	if err != nil {
		return err
	}
	return c.tripService.UpdateTripStatus(ctx, tripID, userID, newStatus)
}

// GetTripWithMembers delegates to TripManagementService
func (c *TripModelCoordinator) GetTripWithMembers(ctx context.Context, tripID string, userID string) (*types.TripWithMembers, error) {
	return c.tripService.GetTripWithMembers(ctx, tripID, userID)
}

// FindInvitationByTripAndEmail delegates to InvitationService
func (c *TripModelCoordinator) FindInvitationByTripAndEmail(ctx context.Context, tripID, email string) (*types.TripInvitation, error) {
	return c.invitationService.FindInvitationByTripAndEmail(ctx, tripID, email)
}

// InviteMember delegates to InvitationService (legacy method)
func (c *TripModelCoordinator) InviteMember(ctx context.Context, invitation *types.TripInvitation) error {
	return c.invitationService.CreateInvitation(ctx, invitation)
}

// GetTripMembers delegates to TripMemberService
func (c *TripModelCoordinator) GetTripMembers(ctx context.Context, tripID string) ([]*types.TripMembership, error) {
	return c.memberService.GetTripMembers(ctx, tripID)
}

// ListMessages delegates to TripChatService
func (c *TripModelCoordinator) ListMessages(ctx context.Context, tripID string, limit int, before string) ([]*types.ChatMessage, error) {
	userID, err := utils.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return c.chatService.ListMessages(ctx, tripID, userID, limit, before)
}

// UpdateLastReadMessage delegates to TripChatService
func (c *TripModelCoordinator) UpdateLastReadMessage(ctx context.Context, tripID string, messageID string) error {
	userID, err := utils.GetUserIDFromContext(ctx)
	if err != nil {
		return err
	}
	return c.chatService.UpdateLastReadMessage(ctx, tripID, userID, messageID)
}

// GetCommandContext returns the command context (for backward compatibility)
func (c *TripModelCoordinator) GetCommandContext() *interfaces.CommandContext {
	return c.cmdCtx
}

// GetTripStore returns the trip store
func (c *TripModelCoordinator) GetTripStore() store.TripStore {
	return c.store
}

// GetChatStore returns the chat store
func (c *TripModelCoordinator) GetChatStore() store.ChatStore {
	return c.chatStore
}
