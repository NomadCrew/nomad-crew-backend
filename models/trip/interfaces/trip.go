package interfaces

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/types"
)

// TripModelInterface defines the interface for trip-related business logic
type TripModelInterface interface {
	CreateTrip(ctx context.Context, trip *types.Trip) error
	GetTripByID(ctx context.Context, id string) (*types.Trip, error)
	UpdateTrip(ctx context.Context, id string, update *types.TripUpdate) error
	DeleteTrip(ctx context.Context, id string) error
	ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error)
	SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error)
	GetUserRole(ctx context.Context, tripID, userID string) (types.MemberRole, error)
	AddMember(ctx context.Context, membership *types.TripMembership) error
	UpdateMemberRole(ctx context.Context, tripID, userID string, role types.MemberRole) (*CommandResult, error)
	RemoveMember(ctx context.Context, tripID, userID string) error
	CreateInvitation(ctx context.Context, invitation *types.TripInvitation) error
	GetInvitation(ctx context.Context, invitationID string) (*types.TripInvitation, error)
	UpdateInvitationStatus(ctx context.Context, invitationID string, status types.InvitationStatus) error
	LookupUserByEmail(ctx context.Context, email string) (*types.SupabaseUser, error)
}
