package store

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store provides a unified interface for all data operations
type Store interface {
	Trip() TripStore
	Location() LocationStore
	Todo() TodoStore
	Chat() ChatStore
	User() UserStore
	Notification() NotificationStore
	BeginTx(ctx context.Context) (types.DatabaseTransaction, error)
}

// TripStore handles trip-related data operations including trips, memberships, and invitations.
type TripStore interface {
	// GetPool returns the underlying database connection pool.
	GetPool() *pgxpool.Pool

	// Trip CRUD operations
	CreateTrip(ctx context.Context, trip types.Trip) (string, error)
	GetTrip(ctx context.Context, id string) (*types.Trip, error)
	UpdateTrip(ctx context.Context, id string, update types.TripUpdate) (*types.Trip, error)
	SoftDeleteTrip(ctx context.Context, id string) error
	ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error)
	SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error)

	// Membership operations
	AddMember(ctx context.Context, membership *types.TripMembership) error
	UpdateMemberRole(ctx context.Context, tripID string, userID string, role types.MemberRole) error
	RemoveMember(ctx context.Context, tripID string, userID string) error
	GetTripMembers(ctx context.Context, tripID string) ([]types.TripMembership, error)
	GetUserRole(ctx context.Context, tripID string, userID string) (types.MemberRole, error)

	// User lookup (for invitations)
	LookupUserByEmail(ctx context.Context, email string) (*types.SupabaseUser, error)

	// Invitation operations
	CreateInvitation(ctx context.Context, invitation *types.TripInvitation) error
	GetInvitation(ctx context.Context, invitationID string) (*types.TripInvitation, error)
	GetInvitationsByTripID(ctx context.Context, tripID string) ([]*types.TripInvitation, error)
	UpdateInvitationStatus(ctx context.Context, invitationID string, status types.InvitationStatus) error

	// Transaction support
	// BeginTx starts a new database transaction. Use the returned DatabaseTransaction
	// for Commit() and Rollback() operations.
	BeginTx(ctx context.Context) (types.DatabaseTransaction, error)

	// Deprecated: Commit is deprecated. Use BeginTx() to start a transaction and call
	// Commit() on the returned DatabaseTransaction instead.
	Commit() error

	// Deprecated: Rollback is deprecated. Use BeginTx() to start a transaction and call
	// Rollback() on the returned DatabaseTransaction instead.
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

	// SearchUsers searches for users by query (username, email, contact_email, first_name, last_name)
	SearchUsers(ctx context.Context, query string, limit int) ([]*types.UserSearchResult, error)

	// UpdateContactEmail updates the user's contact email
	UpdateContactEmail(ctx context.Context, userID string, email string) error

	// GetUserByContactEmail retrieves a user by their contact email
	GetUserByContactEmail(ctx context.Context, contactEmail string) (*types.User, error)
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

// NotificationStore defines the interface for notification operations
type NotificationStore interface {
	// Create inserts a new notification into the database
	Create(ctx context.Context, notification *types.NotificationCreate) (string, error)

	// GetByID retrieves a notification by its ID
	GetByID(ctx context.Context, id string) (*types.Notification, error)

	// GetByUser retrieves notifications for a specific user with pagination and optional status filtering
	// status: nil = all, true = read only, false = unread only
	GetByUser(ctx context.Context, userID string, limit, offset int32, status *bool) ([]*types.Notification, error)

	// MarkRead marks a single notification as read for a specific user
	MarkRead(ctx context.Context, id string, userID string) error

	// MarkAllReadByUser marks all unread notifications as read for a specific user
	// Returns the number of notifications marked as read
	MarkAllReadByUser(ctx context.Context, userID string) (int64, error)

	// GetUnreadCount retrieves the count of unread notifications for a specific user
	GetUnreadCount(ctx context.Context, userID string) (int64, error)

	// Delete removes a notification by its ID, ensuring the operation is performed by the owner
	Delete(ctx context.Context, id string, userID string) error
}

// PushTokenStore defines the interface for push token operations
type PushTokenStore interface {
	// RegisterToken registers or updates a push token for a user
	RegisterToken(ctx context.Context, userID, token, deviceType string) (*types.PushToken, error)

	// DeactivateToken deactivates a specific token for a user (e.g., on logout)
	DeactivateToken(ctx context.Context, userID, token string) error

	// DeactivateAllUserTokens deactivates all tokens for a user
	DeactivateAllUserTokens(ctx context.Context, userID string) error

	// GetActiveTokensForUser retrieves all active push tokens for a user
	GetActiveTokensForUser(ctx context.Context, userID string) ([]*types.PushToken, error)

	// GetActiveTokensForUsers retrieves all active push tokens for multiple users (batch)
	GetActiveTokensForUsers(ctx context.Context, userIDs []string) ([]*types.PushToken, error)

	// InvalidateToken marks a token as invalid (when Expo reports delivery failure)
	InvalidateToken(ctx context.Context, token string) error

	// UpdateTokenLastUsed updates the last_used_at timestamp for a token
	UpdateTokenLastUsed(ctx context.Context, token string) error
}
