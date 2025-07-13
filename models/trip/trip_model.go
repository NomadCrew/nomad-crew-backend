package trip

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	istore "github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/service"
	"github.com/NomadCrew/nomad-crew-backend/services"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/supabase-community/supabase-go"
)

// TripModel is now a thin wrapper around TripModelCoordinator
// This maintains backward compatibility while using the new service architecture
type TripModel struct {
	coordinator *service.TripModelCoordinator
	store       store.TripStore
	config      *config.ServerConfig
}

var _ interfaces.TripModelInterface = (*TripModel)(nil)

// NewTripModel creates a new TripModel using the coordinator
func NewTripModel(
	tripStoreApp store.TripStore,
	chatStoreApp store.ChatStore,
	userStoreInternal store.UserStore,
	eventBus types.EventPublisher,
	weatherSvc types.WeatherServiceInterface,
	supabaseClient *supabase.Client,
	config *config.ServerConfig,
	emailSvc types.EmailService,
	supabaseService *services.SupabaseService,
) *TripModel {
	// Create the coordinator
	var internalTripStore istore.TripStore = tripStoreApp
	var internalChatStore istore.ChatStore = chatStoreApp

	coordinator := service.NewTripModelCoordinator(
		internalTripStore,
		internalChatStore,
		userStoreInternal,
		eventBus,
		weatherSvc,
		supabaseClient,
		config,
		emailSvc,
		supabaseService,
	)

	return &TripModel{
		coordinator: coordinator,
		store:       tripStoreApp,
		config:      config,
	}
}

// All methods now delegate to the coordinator

func (tm *TripModel) CreateTrip(ctx context.Context, trip *types.Trip) (*types.Trip, error) {
	return tm.coordinator.CreateTrip(ctx, trip)
}

func (tm *TripModel) UpdateTripStatus(ctx context.Context, tripID string, newStatus types.TripStatus) error {
	return tm.coordinator.UpdateTripStatus(ctx, tripID, newStatus)
}

func (tm *TripModel) InviteMember(ctx context.Context, invitation *types.TripInvitation) error {
	return tm.coordinator.InviteMember(ctx, invitation)
}

func (tm *TripModel) UpdateMemberRole(ctx context.Context, tripID, memberID string, newRole types.MemberRole) (*interfaces.CommandResult, error) {
	return tm.coordinator.UpdateMemberRole(ctx, tripID, memberID, newRole)
}

func (tm *TripModel) RemoveMember(ctx context.Context, tripID, userID string) error {
	return tm.coordinator.RemoveMember(ctx, tripID, userID)
}

func (tm *TripModel) GetTripMembers(ctx context.Context, tripID string) ([]types.TripMembership, error) {
	return tm.coordinator.GetTripMembers(ctx, tripID)
}

func (tm *TripModel) GetTripByID(ctx context.Context, id string, userID string) (*types.Trip, error) {
	trip, err := tm.coordinator.GetTripByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	return trip, nil
}

func (tm *TripModel) AddMember(ctx context.Context, membership *types.TripMembership) error {
	return tm.coordinator.AddMember(ctx, membership)
}

func (tm *TripModel) UpdateInvitationStatus(ctx context.Context, invitationID string, status types.InvitationStatus) error {
	return tm.coordinator.UpdateInvitationStatus(ctx, invitationID, status)
}

func (tm *TripModel) SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error) {
	return tm.coordinator.SearchTrips(ctx, criteria)
}

func (tm *TripModel) GetTripWithMembers(ctx context.Context, tripID string, userID string) (*types.TripWithMembers, error) {
	return tm.coordinator.GetTripWithMembers(ctx, tripID, userID)
}

// GetTripStore returns the trip store
func (tm *TripModel) GetTripStore() store.TripStore {
	return tm.store
}

/*
func (tm *TripModel) tripNotFound(tripID string) error {
	return &errors.AppError{
		Type:    errors.TripNotFoundError,
		Message: "Trip not found",
	}
}
*/

func (tm *TripModel) CreateInvitation(ctx context.Context, invitation *types.TripInvitation) error {
	return tm.coordinator.CreateInvitation(ctx, invitation)
}

func (tm *TripModel) DeleteTrip(ctx context.Context, id string) error {
	return tm.coordinator.DeleteTrip(ctx, id)
}

func (tm *TripModel) ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error) {
	return tm.coordinator.ListUserTrips(ctx, userID)
}

func (tm *TripModel) UpdateTrip(ctx context.Context, id string, userID string, update *types.TripUpdate) (*types.Trip, error) {
	return tm.coordinator.UpdateTrip(ctx, id, userID, update)
}

func (tm *TripModel) GetInvitation(ctx context.Context, invitationID string) (*types.TripInvitation, error) {
	return tm.coordinator.GetInvitation(ctx, invitationID)
}

func (tm *TripModel) FindInvitationByTripAndEmail(ctx context.Context, tripID, email string) (*types.TripInvitation, error) {
	return tm.coordinator.FindInvitationByTripAndEmail(ctx, tripID, email)
}

func (tm *TripModel) GetUserRole(ctx context.Context, tripID, userID string) (types.MemberRole, error) {
	return tm.coordinator.GetUserRole(ctx, tripID, userID)
}

func (tm *TripModel) LookupUserByEmail(ctx context.Context, email string) (*types.SupabaseUser, error) {
	return tm.coordinator.LookupUserByEmail(ctx, email)
}

// GetTrip retrieves a trip by its ID, without user-specific checks (used for general trip info like for invitations)
func (tm *TripModel) GetTrip(ctx context.Context, tripID string) (*types.Trip, error) {
	return tm.coordinator.GetTrip(ctx, tripID)
}

// GetCommandContext returns the command context from the coordinator
func (tm *TripModel) GetCommandContext() *interfaces.CommandContext {
	return tm.coordinator.GetCommandContext() // Delegate to coordinator
}
