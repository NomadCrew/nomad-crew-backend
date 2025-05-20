package service

import (
	"context"
	"errors"
	"testing"

	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockUserStore implements the UserStore interface for testing
// Only methods needed for these tests are implemented

type MockUserStore struct {
	mock.Mock
}

func (m *MockUserStore) GetUserByUsername(ctx context.Context, username string) (*types.User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}
func (m *MockUserStore) GetUserBySupabaseID(ctx context.Context, supabaseID string) (*types.User, error) {
	args := m.Called(ctx, supabaseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}
func (m *MockUserStore) CreateUser(ctx context.Context, user *types.User) (string, error) {
	args := m.Called(ctx, user)
	return args.String(0), args.Error(1)
}
func (m *MockUserStore) UpdateUser(ctx context.Context, userID string, updates map[string]interface{}) (*types.User, error) {
	args := m.Called(ctx, userID, updates)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}

// Implement all other methods as no-ops or panics for interface compliance
func (m *MockUserStore) GetUserByID(ctx context.Context, userID string) (*types.User, error) {
	panic("not implemented")
}
func (m *MockUserStore) GetUserByEmail(ctx context.Context, email string) (*types.User, error) {
	panic("not implemented")
}
func (m *MockUserStore) ListUsers(ctx context.Context, offset, limit int) ([]*types.User, int, error) {
	panic("not implemented")
}
func (m *MockUserStore) SyncUserFromSupabase(ctx context.Context, supabaseID string) (*types.User, error) {
	panic("not implemented")
}
func (m *MockUserStore) GetSupabaseUser(ctx context.Context, userID string) (*types.SupabaseUser, error) {
	panic("not implemented")
}
func (m *MockUserStore) ConvertToUserResponse(user *types.User) (types.UserResponse, error) {
	panic("not implemented")
}
func (m *MockUserStore) GetUserProfile(ctx context.Context, userID string) (*types.UserProfile, error) {
	panic("not implemented")
}
func (m *MockUserStore) GetUserProfiles(ctx context.Context, userIDs []string) (map[string]*types.UserProfile, error) {
	panic("not implemented")
}
func (m *MockUserStore) UpdateLastSeen(ctx context.Context, userID string) error {
	panic("not implemented")
}
func (m *MockUserStore) SetOnlineStatus(ctx context.Context, userID string, isOnline bool) error {
	panic("not implemented")
}
func (m *MockUserStore) UpdateUserPreferences(ctx context.Context, userID string, preferences map[string]interface{}) error {
	panic("not implemented")
}
func (m *MockUserStore) BeginTx(ctx context.Context) (types.DatabaseTransaction, error) {
	panic("not implemented")
}

// testUserService embeds UserService and overrides GetUserProfile for testing

type testUserService struct {
	*UserService
	getUserProfileFunc func(ctx context.Context, id interface{}) (*types.UserProfile, error)
}

func (s *testUserService) GetUserProfile(ctx context.Context, id interface{}) (*types.UserProfile, error) {
	return s.getUserProfileFunc(ctx, id)
}

func TestOnboardUserFromJWTClaims_Success(t *testing.T) {
	mockStore := new(MockUserStore)
	svc := &testUserService{
		UserService: &UserService{userStore: mockStore},
		getUserProfileFunc: func(ctx context.Context, id interface{}) (*types.UserProfile, error) {
			return &types.UserProfile{ID: "uuid-1", Username: "uniqueuser", Email: "test@example.com"}, nil
		},
	}

	claims := &types.JWTClaims{
		UserID:   "supabase-123",
		Email:    "test@example.com",
		Username: "uniqueuser",
	}

	mockStore.On("GetUserByUsername", mock.Anything, "uniqueuser").Return(nil, nil)
	mockStore.On("GetUserBySupabaseID", mock.Anything, "supabase-123").Return(nil, errors.New("user not found: no rows in result set"))
	mockStore.On("CreateUser", mock.Anything, mock.Anything).Return("uuid-1", nil)

	profile, err := svc.OnboardUserFromJWTClaims(context.Background(), claims)
	assert.NoError(t, err)
	assert.NotNil(t, profile)
	assert.Equal(t, "uniqueuser", profile.Username)
}

func TestOnboardUserFromJWTClaims_UsernameTaken(t *testing.T) {
	mockStore := new(MockUserStore)
	svc := &UserService{userStore: mockStore}

	claims := &types.JWTClaims{
		UserID:   "supabase-123",
		Email:    "test@example.com",
		Username: "takenuser",
	}

	mockStore.On("GetUserByUsername", mock.Anything, "takenuser").Return(&types.User{SupabaseID: "other-user"}, nil)

	profile, err := svc.OnboardUserFromJWTClaims(context.Background(), claims)
	assert.Error(t, err)
	assert.Nil(t, profile)
	assert.Contains(t, err.Error(), "username is already taken")
}

func TestOnboardUserFromJWTClaims_UsernameMissing(t *testing.T) {
	mockStore := new(MockUserStore)
	svc := &UserService{userStore: mockStore}

	claims := &types.JWTClaims{
		UserID:   "supabase-123",
		Email:    "test@example.com",
		Username: "",
	}

	profile, err := svc.OnboardUserFromJWTClaims(context.Background(), claims)
	assert.Error(t, err)
	assert.Nil(t, profile)
	assert.Contains(t, err.Error(), "username is required")
}
