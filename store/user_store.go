package store

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
)

// UserStore defines the interface for user data operations.
// This is a basic definition; expand as needed.
type UserStore interface {
	// GetUserByID retrieves a user by ID from the local database
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)

	// GetUserByEmail retrieves a user by email
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)

	// GetUserBySupabaseID retrieves a user by their Supabase ID
	GetUserBySupabaseID(ctx context.Context, supabaseID string) (*models.User, error)

	// CreateUser creates a new user in the local database
	CreateUser(ctx context.Context, user *models.User) (uuid.UUID, error)

	// UpdateUser updates an existing user
	UpdateUser(ctx context.Context, id uuid.UUID, updates map[string]interface{}) (*models.User, error)

	// ListUsers retrieves a paginated list of users
	ListUsers(ctx context.Context, offset, limit int) ([]*models.User, int, error)

	// SyncUserFromSupabase fetches user data from Supabase and syncs it with the local database
	SyncUserFromSupabase(ctx context.Context, supabaseID string) (*models.User, error)

	// GetSupabaseUser gets a user directly from Supabase
	GetSupabaseUser(ctx context.Context, userID string) (*types.SupabaseUser, error)

	// ConvertToUserResponse converts a models.User to types.UserResponse
	ConvertToUserResponse(user *models.User) types.UserResponse

	// BeginTx starts a transaction
	BeginTx(ctx context.Context) (Transaction, error)
}
