package store

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Store provides a unified interface for all data operations
type Store interface {
	Trip() TripStore
}

// TripStore handles trip-related data operations
type TripStore interface {
	GetPool() *pgxpool.Pool
	CreateTrip(ctx context.Context, trip types.Trip) (string, error)
	GetTrip(ctx context.Context, id string) (*types.Trip, error)
	UpdateTrip(ctx context.Context, id string, update types.TripUpdate) (*types.Trip, error)
	SoftDeleteTrip(ctx context.Context, id string) error
	ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error)
	SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error)
	AddMember(ctx context.Context, membership *types.TripMembership) error
	UpdateMemberRole(ctx context.Context, tripID string, userID string, role types.MemberRole) error
	RemoveMember(ctx context.Context, tripID string, userID string) error
	GetTripMembers(ctx context.Context, tripID string) ([]types.TripMembership, error)
	GetUserRole(ctx context.Context, tripID string, userID string) (types.MemberRole, error)
	LookupUserByEmail(ctx context.Context, email string) (*types.SupabaseUser, error)
	CreateInvitation(ctx context.Context, invitation *types.TripInvitation) error
	GetInvitation(ctx context.Context, invitationID string) (*types.TripInvitation, error)
	GetInvitationsByTripID(ctx context.Context, tripID string) ([]*types.TripInvitation, error)
	UpdateInvitationStatus(ctx context.Context, invitationID string, status types.InvitationStatus) error
	BeginTx(ctx context.Context) (Transaction, error)
	Commit() error
	Rollback() error
}

type TodoStore interface {
	CreateTodo(ctx context.Context, todo *types.Todo) error
	GetTodo(ctx context.Context, id string) (*types.Todo, error)
	UpdateTodo(ctx context.Context, id string, update *types.TodoUpdate) error
	ListTodos(ctx context.Context, tripID string, limit int, offset int) ([]*types.Todo, int, error)
	DeleteTodo(ctx context.Context, id string, userID string) error
}

type Transaction interface {
	Commit() error
	Rollback() error
	// Include other transaction methods used in your codebase
}
