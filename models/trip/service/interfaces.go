package service

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces" // For CommandResult if needed
	"github.com/NomadCrew/nomad-crew-backend/types"
)

// Interfaces defining the methods used by TripModelCoordinator

type TripManagementServiceInterface interface {
	CreateTrip(ctx context.Context, trip *types.Trip) (*types.Trip, error)
	GetTrip(ctx context.Context, id string, userID string) (*types.Trip, error)
	UpdateTrip(ctx context.Context, id string, userID string, updateData types.TripUpdate) (*types.Trip, error)
	DeleteTrip(ctx context.Context, id string) error
	ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error)
	SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error)
	UpdateTripStatus(ctx context.Context, tripID, userID string, newStatus types.TripStatus) error
	GetTripWithMembers(ctx context.Context, tripID string, userID string) (*types.TripWithMembers, error)
	TriggerWeatherUpdate(ctx context.Context, tripID string) error
	GetWeatherForTrip(ctx context.Context, tripID string) (*types.WeatherInfo, error)
	// Add internal methods ONLY if called by coordinator
}

type TripMemberServiceInterface interface {
	GetUserRole(ctx context.Context, tripID, userID string) (types.MemberRole, error)
	AddMember(ctx context.Context, membership *types.TripMembership) error
	UpdateMemberRole(ctx context.Context, tripID, userID string, role types.MemberRole) (*interfaces.CommandResult, error)
	RemoveMember(ctx context.Context, tripID, userID string) error
	GetTripMembers(ctx context.Context, tripID string) ([]types.TripMembership, error)
}

type InvitationServiceInterface interface {
	CreateInvitation(ctx context.Context, invitation *types.TripInvitation) error
	GetInvitation(ctx context.Context, invitationID string) (*types.TripInvitation, error)
	UpdateInvitationStatus(ctx context.Context, invitationID string, status types.InvitationStatus) error
	LookupUserByEmail(ctx context.Context, email string) (*types.SupabaseUser, error)
	FindInvitationByTripAndEmail(ctx context.Context, tripID, email string) (*types.TripInvitation, error)
}

type TripChatServiceInterface interface {
	ListMessages(ctx context.Context, tripID string, userID string, limit int, before string) ([]*types.ChatMessage, error)
	UpdateLastReadMessage(ctx context.Context, tripID string, userID string, messageID string) error
}
