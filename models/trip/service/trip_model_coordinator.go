package service

import (
	"context"
	"sync"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/internal/utils"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/services"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/supabase-community/supabase-go"
)

// TripModelCoordinator acts as a facade over the various trip-related services
// It implements the TripModelInterface to ensure backward compatibility
type TripModelCoordinator struct {
	// Use interfaces for dependencies
	TripService       TripManagementServiceInterface
	MemberService     TripMemberServiceInterface
	InvitationService InvitationServiceInterface
	ChatService       TripChatServiceInterface
	// Keep internal fields unexported
	store     store.TripStore // Keep concrete store for GetTripStore method
	chatStore store.ChatStore // Keep concrete store for GetChatStore method
	config    *config.ServerConfig
	cmdCtx    *interfaces.CommandContext // Keep for backward compatibility
}

// NewTripModelCoordinator creates a new TripModelCoordinator
// Accepts interfaces, allowing real or mock implementations
func NewTripModelCoordinator(
	tripStore store.TripStore, // Now internal/store.TripStore
	chatStore store.ChatStore, // Now internal/store.ChatStore
	userStore store.UserStore, // Now internal/store.UserStore
	eventBus types.EventPublisher,
	weatherSvc types.WeatherServiceInterface,
	supabaseClient *supabase.Client,
	config *config.ServerConfig,
	emailSvc types.EmailService, // Assuming EmailService is an interface
	supabaseService *services.SupabaseService,
) *TripModelCoordinator {
	// Create the command context for backward compatibility
	cmdCtx := &interfaces.CommandContext{
		Store:          tripStore,
		UserStore:      userStore,
		EventBus:       eventBus,
		WeatherSvc:     weatherSvc,
		SupabaseClient: supabaseClient,
		Config:         config,
		RequestData:    new(sync.Map),
		EmailSvc:       emailSvc,
		ChatStore:      chatStore,
	}

	// Create the concrete service instances
	tripServiceInstance := NewTripManagementService(tripStore, userStore, eventBus, weatherSvc, supabaseService)
	memberServiceInstance := NewTripMemberService(tripStore, eventBus, supabaseService)
	invitationServiceInstance := NewInvitationService(tripStore, emailSvc, supabaseClient, config.FrontendURL, eventBus)

	// Note: TripChatService has been deprecated.
	// For full chat functionality, use models/chat/service.ChatService instead.
	// This nil assignment is temporary - TripModelCoordinator should be refactored
	// to not depend on chat service directly.
	var chatServiceInstance TripChatServiceInterface = nil

	return &TripModelCoordinator{
		// Assign concrete instances to interface fields
		TripService:       tripServiceInstance,
		MemberService:     memberServiceInstance,
		InvitationService: invitationServiceInstance,
		ChatService:       chatServiceInstance,
		// Assign other dependencies
		store:     tripStore,
		chatStore: chatStore,
		config:    config,
		cmdCtx:    cmdCtx,
	}
}

// CreateTrip delegates to TripManagementService
func (c *TripModelCoordinator) CreateTrip(ctx context.Context, trip *types.Trip) (*types.Trip, error) {
	return c.TripService.CreateTrip(ctx, trip)
}

// GetTripByID delegates to TripManagementService
func (c *TripModelCoordinator) GetTripByID(ctx context.Context, id string, userID string) (*types.Trip, error) {
	return c.TripService.GetTrip(ctx, id, userID)
}

// UpdateTrip delegates to TripManagementService
func (c *TripModelCoordinator) UpdateTrip(ctx context.Context, id string, userID string, update *types.TripUpdate) (*types.Trip, error) {
	return c.TripService.UpdateTrip(ctx, id, userID, *update)
}

// DeleteTrip delegates to TripManagementService
func (c *TripModelCoordinator) DeleteTrip(ctx context.Context, id string) error {
	return c.TripService.DeleteTrip(ctx, id)
}

// ListUserTrips delegates to TripManagementService
func (c *TripModelCoordinator) ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error) {
	return c.TripService.ListUserTrips(ctx, userID)
}

// SearchTrips delegates to TripManagementService
func (c *TripModelCoordinator) SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error) {
	return c.TripService.SearchTrips(ctx, criteria)
}

// GetUserRole delegates to TripMemberService
func (c *TripModelCoordinator) GetUserRole(ctx context.Context, tripID, userID string) (types.MemberRole, error) {
	return c.MemberService.GetUserRole(ctx, tripID, userID)
}

// AddMember delegates to TripMemberService
func (c *TripModelCoordinator) AddMember(ctx context.Context, membership *types.TripMembership) error {
	return c.MemberService.AddMember(ctx, membership)
}

// UpdateMemberRole delegates to TripMemberService
func (c *TripModelCoordinator) UpdateMemberRole(ctx context.Context, tripID, userID string, role types.MemberRole) (*interfaces.CommandResult, error) {
	return c.MemberService.UpdateMemberRole(ctx, tripID, userID, role)
}

// RemoveMember delegates to TripMemberService
func (c *TripModelCoordinator) RemoveMember(ctx context.Context, tripID, userID string) error {
	return c.MemberService.RemoveMember(ctx, tripID, userID)
}

// CreateInvitation delegates to InvitationService
func (c *TripModelCoordinator) CreateInvitation(ctx context.Context, invitation *types.TripInvitation) error {
	return c.InvitationService.CreateInvitation(ctx, invitation)
}

// GetInvitation delegates to InvitationService
func (c *TripModelCoordinator) GetInvitation(ctx context.Context, invitationID string) (*types.TripInvitation, error) {
	return c.InvitationService.GetInvitation(ctx, invitationID)
}

// UpdateInvitationStatus delegates to InvitationService
func (c *TripModelCoordinator) UpdateInvitationStatus(ctx context.Context, invitationID string, status types.InvitationStatus) error {
	return c.InvitationService.UpdateInvitationStatus(ctx, invitationID, status)
}

// LookupUserByEmail delegates to InvitationService
func (c *TripModelCoordinator) LookupUserByEmail(ctx context.Context, email string) (*types.SupabaseUser, error) {
	return c.InvitationService.LookupUserByEmail(ctx, email)
}

// UpdateTripStatus delegates to TripManagementService
func (c *TripModelCoordinator) UpdateTripStatus(ctx context.Context, tripID string, newStatus types.TripStatus) error {
	userID, err := utils.GetUserIDFromContext(ctx)
	if err != nil {
		return err
	}
	return c.TripService.UpdateTripStatus(ctx, tripID, userID, newStatus)
}

// GetTripWithMembers delegates to TripManagementService
func (c *TripModelCoordinator) GetTripWithMembers(ctx context.Context, tripID string, userID string) (*types.TripWithMembers, error) {
	return c.TripService.GetTripWithMembers(ctx, tripID, userID)
}

// FindInvitationByTripAndEmail delegates to InvitationService
func (c *TripModelCoordinator) FindInvitationByTripAndEmail(ctx context.Context, tripID, email string) (*types.TripInvitation, error) {
	return c.InvitationService.FindInvitationByTripAndEmail(ctx, tripID, email)
}

// InviteMember delegates to InvitationService (legacy method)
func (c *TripModelCoordinator) InviteMember(ctx context.Context, invitation *types.TripInvitation) error {
	return c.InvitationService.CreateInvitation(ctx, invitation)
}

// GetTripMembers delegates to TripMemberService
func (c *TripModelCoordinator) GetTripMembers(ctx context.Context, tripID string) ([]types.TripMembership, error) {
	return c.MemberService.GetTripMembers(ctx, tripID)
}

// GetTrip retrieves a trip by ID without user auth, delegating to the underlying store.
func (c *TripModelCoordinator) GetTrip(ctx context.Context, tripID string) (*types.Trip, error) {
	// Assuming c.store is the TripStore which has GetTrip(ctx, tripID)
	return c.store.GetTrip(ctx, tripID)
}

// ListMessages is deprecated - chat functionality has moved to models/chat/service
// This method returns empty results for backward compatibility
func (c *TripModelCoordinator) ListMessages(ctx context.Context, tripID string, limit int, before string) ([]*types.ChatMessage, error) {
	// Chat functionality has moved to models/chat/service.ChatService
	// Return empty results for backward compatibility
	return []*types.ChatMessage{}, nil
}

// UpdateLastReadMessage is deprecated - chat functionality has moved to models/chat/service
// This method is a no-op for backward compatibility
func (c *TripModelCoordinator) UpdateLastReadMessage(ctx context.Context, tripID string, messageID string) error {
	// Chat functionality has moved to models/chat/service.ChatService
	// No-op for backward compatibility
	return nil
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
