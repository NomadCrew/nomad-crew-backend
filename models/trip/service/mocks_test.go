package service_test

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/stretchr/testify/mock"
)

// MockWeatherService is a consolidated mock for WeatherServiceInterface
// Used by both trip_service_test.go and trip_service_notification_test.go
type MockWeatherService struct {
	mock.Mock
}

func (m *MockWeatherService) StartWeatherUpdates(ctx context.Context, tripID string, lat float64, lon float64) {
	m.Called(ctx, tripID, lat, lon)
}

func (m *MockWeatherService) IncrementSubscribers(tripID string, lat float64, lon float64) {
	m.Called(tripID, lat, lon)
}

func (m *MockWeatherService) DecrementSubscribers(tripID string) {
	m.Called(tripID)
}

func (m *MockWeatherService) TriggerImmediateUpdate(ctx context.Context, tripID string, lat float64, lon float64) error {
	args := m.Called(ctx, tripID, lat, lon)
	return args.Error(0)
}

func (m *MockWeatherService) GetWeather(ctx context.Context, tripID string) (*types.WeatherInfo, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.WeatherInfo), args.Error(1)
}

func (m *MockWeatherService) GetWeatherByCoords(ctx context.Context, tripID string, lat, lon float64) (*types.WeatherInfo, error) {
	args := m.Called(ctx, tripID, lat, lon)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.WeatherInfo), args.Error(1)
}

// MockUserStore is a consolidated mock for types.UserStore
// Used by both trip_service_test.go and trip_service_notification_test.go
type MockUserStore struct {
	mock.Mock
}

func (m *MockUserStore) GetUserByID(ctx context.Context, userID string) (*types.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}

func (m *MockUserStore) GetUserByEmail(ctx context.Context, email string) (*types.User, error) {
	args := m.Called(ctx, email)
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

func (m *MockUserStore) GetUserByUsername(ctx context.Context, username string) (*types.User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}

func (m *MockUserStore) ListUsers(ctx context.Context, offset, limit int) ([]*types.User, int, error) {
	args := m.Called(ctx, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*types.User), args.Int(1), args.Error(2)
}

func (m *MockUserStore) SyncUserFromSupabase(ctx context.Context, supabaseID string) (*types.User, error) {
	args := m.Called(ctx, supabaseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}

func (m *MockUserStore) GetSupabaseUser(ctx context.Context, userID string) (*types.SupabaseUser, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SupabaseUser), args.Error(1)
}

func (m *MockUserStore) ConvertToUserResponse(user *types.User) (types.UserResponse, error) {
	args := m.Called(user)
	return args.Get(0).(types.UserResponse), args.Error(1)
}

func (m *MockUserStore) GetUserProfile(ctx context.Context, userID string) (*types.UserProfile, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.UserProfile), args.Error(1)
}

func (m *MockUserStore) GetUserProfiles(ctx context.Context, userIDs []string) (map[string]*types.UserProfile, error) {
	args := m.Called(ctx, userIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]*types.UserProfile), args.Error(1)
}

func (m *MockUserStore) UpdateLastSeen(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockUserStore) SetOnlineStatus(ctx context.Context, userID string, isOnline bool) error {
	args := m.Called(ctx, userID, isOnline)
	return args.Error(0)
}

func (m *MockUserStore) UpdateUserPreferences(ctx context.Context, userID string, preferences map[string]interface{}) error {
	args := m.Called(ctx, userID, preferences)
	return args.Error(0)
}

func (m *MockUserStore) BeginTx(ctx context.Context) (types.DatabaseTransaction, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(types.DatabaseTransaction), args.Error(1)
}

func (m *MockUserStore) GetUserByContactEmail(ctx context.Context, contactEmail string) (*types.User, error) {
	args := m.Called(ctx, contactEmail)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}

func (m *MockUserStore) SearchUsers(ctx context.Context, query string, limit int) ([]*types.UserSearchResult, error) {
	args := m.Called(ctx, query, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.UserSearchResult), args.Error(1)
}

func (m *MockUserStore) UpdateContactEmail(ctx context.Context, userID string, email string) error {
	args := m.Called(ctx, userID, email)
	return args.Error(0)
}
