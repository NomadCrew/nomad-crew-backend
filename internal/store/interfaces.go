package store

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Store provides a unified interface for all data operations
type Store interface {
	Trip() TripStore
	Location() LocationStore
	Todo() TodoStore
	Chat() ChatStore
	User() UserStore
	BeginTx(ctx context.Context) (types.DatabaseTransaction, error)
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
	BeginTx(ctx context.Context) (types.DatabaseTransaction, error)
	Commit() error
	Rollback() error
}

type TodoStore interface {
	// CreateTodo creates a new todo item in the database.
	CreateTodo(ctx context.Context, todo *types.Todo) (string, error)

	// GetTodo retrieves a todo item by its ID.
	GetTodo(ctx context.Context, id string) (*types.Todo, error)

	// ListTodos retrieves all todos for a specific trip.
	ListTodos(ctx context.Context, tripID string) ([]*types.Todo, error)

	// UpdateTodo updates an existing todo item.
	UpdateTodo(ctx context.Context, id string, update *types.TodoUpdate) (*types.Todo, error)

	// DeleteTodo removes a todo item from the database.
	DeleteTodo(ctx context.Context, id string) error

	// BeginTx starts a new database transaction.
	BeginTx(ctx context.Context) (types.DatabaseTransaction, error)
}

// ChatStore defines the interface for chat operations
type ChatStore interface {
	// Chat Group operations
	CreateChatGroup(ctx context.Context, group types.ChatGroup) (string, error)
	GetChatGroup(ctx context.Context, groupID string) (*types.ChatGroup, error)
	UpdateChatGroup(ctx context.Context, groupID string, update types.ChatGroupUpdateRequest) error
	DeleteChatGroup(ctx context.Context, groupID string) error
	ListChatGroupsByTrip(ctx context.Context, tripID string, limit, offset int) (*types.ChatGroupPaginatedResponse, error)

	// Chat Message operations
	CreateChatMessage(ctx context.Context, message types.ChatMessage) (string, error)
	GetChatMessageByID(ctx context.Context, messageID string) (*types.ChatMessage, error)
	UpdateChatMessage(ctx context.Context, messageID string, content string) error
	DeleteChatMessage(ctx context.Context, messageID string) error
	ListChatMessages(ctx context.Context, groupID string, params types.PaginationParams) ([]types.ChatMessage, int, error)

	// Chat Group Member operations
	AddChatGroupMember(ctx context.Context, groupID, userID string) error
	RemoveChatGroupMember(ctx context.Context, groupID, userID string) error
	ListChatGroupMembers(ctx context.Context, groupID string) ([]types.UserResponse, error)
	UpdateLastReadMessage(ctx context.Context, groupID, userID, messageID string) error

	// Chat Message Reaction operations
	AddReaction(ctx context.Context, messageID, userID, reaction string) error
	RemoveReaction(ctx context.Context, messageID, userID, reaction string) error
	ListChatMessageReactions(ctx context.Context, messageID string) ([]types.ChatMessageReaction, error)

	// User operations - moved to UserStore
	// GetUserByID(ctx context.Context, userID string) (*types.SupabaseUser, error)
}

// UserStore defines the interface for user data operations
type UserStore interface {
	// GetUserByID retrieves a user by ID
	GetUserByID(ctx context.Context, userID string) (*types.User, error)

	// GetUserByEmail retrieves a user by email
	GetUserByEmail(ctx context.Context, email string) (*types.User, error)

	// GetUserBySupabaseID retrieves a user by their Supabase ID
	GetUserBySupabaseID(ctx context.Context, supabaseID string) (*types.User, error)

	// CreateUser creates a new user
	CreateUser(ctx context.Context, user *types.User) (string, error)

	// UpdateUser updates an existing user
	UpdateUser(ctx context.Context, userID string, updates map[string]interface{}) (*types.User, error)

	// ListUsers retrieves a paginated list of users
	ListUsers(ctx context.Context, offset, limit int) ([]*types.User, int, error)

	// SyncUserFromSupabase fetches user data from Supabase and syncs it with the local database
	SyncUserFromSupabase(ctx context.Context, supabaseID string) (*types.User, error)

	// GetSupabaseUser gets a user directly from Supabase
	GetSupabaseUser(ctx context.Context, userID string) (*types.SupabaseUser, error)

	// ConvertToUserResponse converts a user to UserResponse type
	ConvertToUserResponse(user *types.User) (types.UserResponse, error)

	// GetUserProfile retrieves a user profile for API responses
	GetUserProfile(ctx context.Context, userID string) (*types.UserProfile, error)

	// GetUserProfiles retrieves multiple user profiles for API responses
	GetUserProfiles(ctx context.Context, userIDs []string) (map[string]*types.UserProfile, error)

	// UpdateLastSeen updates a user's last seen timestamp
	UpdateLastSeen(ctx context.Context, userID string) error

	// SetOnlineStatus sets a user's online status
	SetOnlineStatus(ctx context.Context, userID string, isOnline bool) error

	// UpdateUserPreferences updates a user's preferences
	UpdateUserPreferences(ctx context.Context, userID string, preferences map[string]interface{}) error

	// BeginTx starts a transaction
	BeginTx(ctx context.Context) (types.DatabaseTransaction, error)

	// GetUserByUsername retrieves a user by their username
	GetUserByUsername(ctx context.Context, username string) (*types.User, error)
}

// Use types.DatabaseTransaction here instead of a local interface
type Transaction = types.DatabaseTransaction

// LocationStore defines the interface for location-related operations
type LocationStore interface {
	// CreateLocation creates a new location record
	CreateLocation(ctx context.Context, location *types.Location) (string, error)

	// GetLocation retrieves a location by its ID
	GetLocation(ctx context.Context, id string) (*types.Location, error)

	// UpdateLocation updates an existing location
	UpdateLocation(ctx context.Context, id string, update *types.LocationUpdate) (*types.Location, error)

	// DeleteLocation removes a location record
	DeleteLocation(ctx context.Context, id string) error

	// ListTripMemberLocations retrieves all locations for members of a specific trip
	ListTripMemberLocations(ctx context.Context, tripID string) ([]*types.MemberLocation, error)

	// BeginTx starts a new database transaction
	BeginTx(ctx context.Context) (types.DatabaseTransaction, error)
}
