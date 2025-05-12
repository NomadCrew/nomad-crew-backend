// Package db provides implementations for data access interfaces defined in internal/store.
package db

import (
	"context"
	"encoding/json"

	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/internal/store/postgres"
	"github.com/NomadCrew/nomad-crew-backend/models"
	oldstore "github.com/NomadCrew/nomad-crew-backend/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
)

// UserStore interface is defined in internal/store/interfaces.go

// PostgresUserStore is an adapter that connects the existing code with the new UserStore implementation
// It implements both internal/store.UserStore and store.UserStore interfaces
type PostgresUserStore struct {
	internalStore store.UserStore
}

// NewPostgresUserStore creates a new instance of PostgresUserStore
func NewPostgresUserStore(pool *pgxpool.Pool, supabaseURL, supabaseKey string) oldstore.UserStore {
	// Create an instance of the internal UserStore
	internalStore := postgres.NewUserStore(pool, supabaseURL, supabaseKey)

	return &PostgresUserStore{
		internalStore: internalStore,
	}
}

// Adapters for the old interface (store.UserStore)

// GetUserByID retrieves a user by ID from the local database (old interface)
func (s *PostgresUserStore) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	// Call the internal implementation
	user, err := s.internalStore.GetUserByID(ctx, id.String())
	if err != nil {
		return nil, err
	}

	// Convert from types.User to models.User
	return convertToModelsUser(user), nil
}

// GetUserByEmail retrieves a user by email (old interface)
func (s *PostgresUserStore) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	// Call the internal implementation
	user, err := s.internalStore.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	// Convert from types.User to models.User
	return convertToModelsUser(user), nil
}

// GetUserBySupabaseID retrieves a user by their Supabase ID (old interface)
func (s *PostgresUserStore) GetUserBySupabaseID(ctx context.Context, supabaseID string) (*models.User, error) {
	// Call the internal implementation
	user, err := s.internalStore.GetUserBySupabaseID(ctx, supabaseID)
	if err != nil {
		return nil, err
	}

	// Convert from types.User to models.User
	return convertToModelsUser(user), nil
}

// CreateUser creates a new user in the local database (old interface)
func (s *PostgresUserStore) CreateUser(ctx context.Context, user *models.User) (uuid.UUID, error) {
	// Convert from models.User to types.User
	typesUser := convertToTypesUser(user)

	// Call the internal implementation
	id, err := s.internalStore.CreateUser(ctx, typesUser)
	if err != nil {
		return uuid.Nil, err
	}

	// Convert the string ID to uuid.UUID
	return uuid.Parse(id)
}

// UpdateUser updates an existing user (old interface)
func (s *PostgresUserStore) UpdateUser(ctx context.Context, id uuid.UUID, updates map[string]interface{}) (*models.User, error) {
	// Call the internal implementation
	user, err := s.internalStore.UpdateUser(ctx, id.String(), updates)
	if err != nil {
		return nil, err
	}

	// Convert from types.User to models.User
	return convertToModelsUser(user), nil
}

// ListUsers retrieves a paginated list of users (old interface)
func (s *PostgresUserStore) ListUsers(ctx context.Context, offset, limit int) ([]*models.User, int, error) {
	// Call the internal implementation
	typesUsers, total, err := s.internalStore.ListUsers(ctx, offset, limit)
	if err != nil {
		return nil, 0, err
	}

	// Convert from []*types.User to []*models.User
	modelsUsers := make([]*models.User, len(typesUsers))
	for i, user := range typesUsers {
		modelsUsers[i] = convertToModelsUser(user)
	}

	return modelsUsers, total, nil
}

// SyncUserFromSupabase fetches user data from Supabase and syncs it with the local database (old interface)
func (s *PostgresUserStore) SyncUserFromSupabase(ctx context.Context, supabaseID string) (*models.User, error) {
	// Call the internal implementation
	user, err := s.internalStore.SyncUserFromSupabase(ctx, supabaseID)
	if err != nil {
		return nil, err
	}

	// Convert from types.User to models.User
	return convertToModelsUser(user), nil
}

// GetSupabaseUser gets a user directly from Supabase (old interface)
func (s *PostgresUserStore) GetSupabaseUser(ctx context.Context, userID string) (*types.SupabaseUser, error) {
	// Direct call to the internal implementation (no conversion needed)
	return s.internalStore.GetSupabaseUser(ctx, userID)
}

// ConvertToUserResponse converts a models.User to types.UserResponse (old interface)
func (s *PostgresUserStore) ConvertToUserResponse(user *models.User) types.UserResponse {
	return types.UserResponse{
		ID:          user.ID.String(),
		Email:       user.Email,
		Username:    user.Username,
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		AvatarURL:   user.ProfilePictureURL,
		DisplayName: user.GetDisplayName(),
	}
}

// Interface for the new internal/store.UserStore methods

// GetUserProfile retrieves a user profile for API responses
func (s *PostgresUserStore) GetUserProfile(ctx context.Context, userID string) (*types.UserProfile, error) {
	return s.internalStore.GetUserProfile(ctx, userID)
}

// GetUserProfiles retrieves multiple user profiles for API responses
func (s *PostgresUserStore) GetUserProfiles(ctx context.Context, userIDs []string) (map[string]*types.UserProfile, error) {
	return s.internalStore.GetUserProfiles(ctx, userIDs)
}

// UpdateLastSeen updates a user's last seen timestamp
func (s *PostgresUserStore) UpdateLastSeen(ctx context.Context, userID string) error {
	return s.internalStore.UpdateLastSeen(ctx, userID)
}

// SetOnlineStatus sets a user's online status
func (s *PostgresUserStore) SetOnlineStatus(ctx context.Context, userID string, isOnline bool) error {
	return s.internalStore.SetOnlineStatus(ctx, userID, isOnline)
}

// UpdateUserPreferences updates a user's preferences
func (s *PostgresUserStore) UpdateUserPreferences(ctx context.Context, userID string, preferences map[string]interface{}) error {
	return s.internalStore.UpdateUserPreferences(ctx, userID, preferences)
}

// BeginTx starts a transaction
func (s *PostgresUserStore) BeginTx(ctx context.Context) (types.DatabaseTransaction, error) {
	return s.internalStore.BeginTx(ctx)
}

// Commit commits the transaction
func (s *PostgresUserStore) Commit() error {
	if txStore, ok := s.internalStore.(types.DatabaseTransaction); ok {
		return txStore.Commit()
	}
	return nil
}

// Rollback aborts the transaction
func (s *PostgresUserStore) Rollback() error {
	if txStore, ok := s.internalStore.(types.DatabaseTransaction); ok {
		return txStore.Rollback()
	}
	return nil
}

// Helper functions for conversion between models.User and types.User

// convertToTypesUser converts a models.User to types.User
func convertToTypesUser(user *models.User) *types.User {
	if user == nil {
		return nil
	}

	// Convert preferences from []byte to map[string]interface{}
	var preferencesMap map[string]interface{}
	if len(user.Preferences) > 0 {
		// Unmarshal preferences if they exist
		if err := json.Unmarshal(user.Preferences, &preferencesMap); err != nil {
			preferencesMap = make(map[string]interface{})
		}
	} else {
		preferencesMap = make(map[string]interface{})
	}

	return &types.User{
		ID:                user.ID.String(),
		SupabaseID:        user.SupabaseID,
		Username:          user.Username,
		FirstName:         user.FirstName,
		LastName:          user.LastName,
		Email:             user.Email,
		CreatedAt:         user.CreatedAt,
		UpdatedAt:         user.UpdatedAt,
		ProfilePictureURL: user.ProfilePictureURL,
		RawUserMetaData:   user.RawUserMetaData,
		LastSeenAt:        user.LastSeenAt,
		IsOnline:          user.IsOnline,
		Preferences:       preferencesMap,
	}
}

// convertToModelsUser converts a types.User to models.User
func convertToModelsUser(user *types.User) *models.User {
	if user == nil {
		return nil
	}

	id, err := uuid.Parse(user.ID)
	if err != nil {
		// Fall back to a new UUID if parsing fails
		id = uuid.New()
	}

	// Convert preferences from map[string]interface{} to []byte
	var preferencesBytes []byte
	if user.Preferences != nil {
		// Marshal preferences to JSON
		bytes, err := json.Marshal(user.Preferences)
		if err == nil {
			preferencesBytes = bytes
		}
	}

	return &models.User{
		ID:                id,
		SupabaseID:        user.SupabaseID,
		Username:          user.Username,
		FirstName:         user.FirstName,
		LastName:          user.LastName,
		Email:             user.Email,
		CreatedAt:         user.CreatedAt,
		UpdatedAt:         user.UpdatedAt,
		ProfilePictureURL: user.ProfilePictureURL,
		RawUserMetaData:   user.RawUserMetaData,
		LastSeenAt:        user.LastSeenAt,
		IsOnline:          user.IsOnline,
		Preferences:       preferencesBytes,
	}
}
