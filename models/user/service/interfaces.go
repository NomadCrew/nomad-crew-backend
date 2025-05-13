package service

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
)

// UserServiceInterface defines the contract for user operations
type UserServiceInterface interface {
	// GetUserByID retrieves a user by internal ID
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)

	// GetUserByEmail retrieves a user by email
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)

	// GetUserBySupabaseID retrieves a user by Supabase ID
	GetUserBySupabaseID(ctx context.Context, supabaseID string) (*models.User, error)

	// CreateUser creates a new user
	CreateUser(ctx context.Context, user *models.User) (uuid.UUID, error)

	// UpdateUser updates an existing user
	UpdateUser(ctx context.Context, id uuid.UUID, updates models.UserUpdateRequest) (*models.User, error)

	// UpdateUserProfile handles profile updates specifically with validation
	UpdateUserProfile(ctx context.Context, id uuid.UUID, currentUserID uuid.UUID, isAdmin bool, updates models.UserUpdateRequest) (*models.User, error)

	// UpdateUserPreferencesWithValidation validates and updates user preferences
	UpdateUserPreferencesWithValidation(ctx context.Context, id uuid.UUID, currentUserID uuid.UUID, isAdmin bool, preferences map[string]interface{}) error

	// ListUsers retrieves a paginated list of users
	ListUsers(ctx context.Context, offset, limit int) ([]*models.User, int, error)

	// SyncWithSupabase syncs a user with Supabase
	SyncWithSupabase(ctx context.Context, supabaseID string) (*models.User, error)

	// GetUserProfile gets a user profile for API responses
	GetUserProfile(ctx context.Context, id uuid.UUID) (*types.UserProfile, error)

	// GetUserProfiles gets multiple user profiles for API responses
	GetUserProfiles(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*types.UserProfile, error)

	// UpdateLastSeen updates a user's last seen timestamp
	UpdateLastSeen(ctx context.Context, id uuid.UUID) error

	// SetOnlineStatus sets a user's online status
	SetOnlineStatus(ctx context.Context, id uuid.UUID, isOnline bool) error

	// UpdateUserPreferences updates a user's preferences
	UpdateUserPreferences(ctx context.Context, id uuid.UUID, preferences map[string]interface{}) error

	// ValidateUserUpdateRequest validates a user update request
	ValidateUserUpdateRequest(update models.UserUpdateRequest) error
}
