package handlers

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// MockUserService implements the user service interface for handler tests.
// This is the CANONICAL definition - do not redeclare in other test files.
type MockUserService struct {
	mock.Mock
}

// Ensure MockUserService implements UserServiceInterface at compile time
// var _ userservice.UserServiceInterface = (*MockUserService)(nil)

// GetUserByID retrieves a user by internal ID
func (m *MockUserService) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

// GetUserByEmail retrieves a user by email
func (m *MockUserService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

// GetUserBySupabaseID retrieves a user by Supabase ID
func (m *MockUserService) GetUserBySupabaseID(ctx context.Context, supabaseID string) (*models.User, error) {
	args := m.Called(ctx, supabaseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

// CreateUser creates a new user
func (m *MockUserService) CreateUser(ctx context.Context, user *models.User) (uuid.UUID, error) {
	args := m.Called(ctx, user)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

// UpdateUser updates an existing user
func (m *MockUserService) UpdateUser(ctx context.Context, id uuid.UUID, updates models.UserUpdateRequest) (*models.User, error) {
	args := m.Called(ctx, id, updates)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

// UpdateUserProfile handles profile updates specifically with validation
func (m *MockUserService) UpdateUserProfile(ctx context.Context, id uuid.UUID, currentUserID uuid.UUID, isAdmin bool, updates models.UserUpdateRequest) (*models.User, error) {
	args := m.Called(ctx, id, currentUserID, isAdmin, updates)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

// UpdateUserPreferencesWithValidation validates and updates user preferences
func (m *MockUserService) UpdateUserPreferencesWithValidation(ctx context.Context, id uuid.UUID, currentUserID uuid.UUID, isAdmin bool, preferences map[string]interface{}) error {
	args := m.Called(ctx, id, currentUserID, isAdmin, preferences)
	return args.Error(0)
}

// ListUsers retrieves a paginated list of users
func (m *MockUserService) ListUsers(ctx context.Context, offset, limit int) ([]*models.User, int, error) {
	args := m.Called(ctx, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*models.User), args.Int(1), args.Error(2)
}

// SyncWithSupabase syncs a user with Supabase
func (m *MockUserService) SyncWithSupabase(ctx context.Context, supabaseID string) (*models.User, error) {
	args := m.Called(ctx, supabaseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

// GetUserProfile gets a user profile for API responses
func (m *MockUserService) GetUserProfile(ctx context.Context, id uuid.UUID) (*types.UserProfile, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.UserProfile), args.Error(1)
}

// GetUserProfiles gets multiple user profiles for API responses
func (m *MockUserService) GetUserProfiles(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*types.UserProfile, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[uuid.UUID]*types.UserProfile), args.Error(1)
}

// UpdateLastSeen updates a user's last seen timestamp
func (m *MockUserService) UpdateLastSeen(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// SetOnlineStatus sets a user's online status
func (m *MockUserService) SetOnlineStatus(ctx context.Context, id uuid.UUID, isOnline bool) error {
	args := m.Called(ctx, id, isOnline)
	return args.Error(0)
}

// UpdateUserPreferences updates a user's preferences
func (m *MockUserService) UpdateUserPreferences(ctx context.Context, id uuid.UUID, preferences map[string]interface{}) error {
	args := m.Called(ctx, id, preferences)
	return args.Error(0)
}

// ValidateUserUpdateRequest validates a user update request
func (m *MockUserService) ValidateUserUpdateRequest(update models.UserUpdateRequest) error {
	args := m.Called(update)
	return args.Error(0)
}

// ValidateAndExtractClaims validates JWT token and extracts claims
func (m *MockUserService) ValidateAndExtractClaims(tokenString string) (*types.JWTClaims, error) {
	args := m.Called(tokenString)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.JWTClaims), args.Error(1)
}

// OnboardUserFromJWTClaims handles user onboarding from JWT claims
func (m *MockUserService) OnboardUserFromJWTClaims(ctx context.Context, claims *types.JWTClaims) (*types.UserProfile, error) {
	args := m.Called(ctx, claims)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.UserProfile), args.Error(1)
}
