package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	// Use the interface defined in the same package
	// locationSvc "github.com/NomadCrew/nomad-crew-backend/models/location/service"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	// Import store interface for MockLocationDB if needed
	"github.com/NomadCrew/nomad-crew-backend/store"
)

// Mock LocationDB (implements store.LocationStore)
// Note: Update methods if store.LocationStore interface changes
type MockLocationDB struct {
	mock.Mock
	// Ensure this mock implements store.LocationStore
	_ store.LocationStore // Embed to satisfy interface, methods are mocked below
}

func (m *MockLocationDB) UpdateLocation(ctx context.Context, userID string, update types.LocationUpdate) (*types.Location, error) {
	args := m.Called(ctx, userID, update)
	if loc, ok := args.Get(0).(*types.Location); ok {
		return loc, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockLocationDB) GetTripMemberLocations(ctx context.Context, tripID string) ([]types.MemberLocation, error) {
	args := m.Called(ctx, tripID)
	if locs, ok := args.Get(0).([]types.MemberLocation); ok {
		return locs, args.Error(1)
	}
	return nil, args.Error(1)
}

// Add missing methods from store.LocationStore (example)
func (m *MockLocationDB) GetUserRole(ctx context.Context, tripID, userID string) (types.MemberRole, error) {
	args := m.Called(ctx, tripID, userID)
	// Simplify return for mock - assume string conversion is fine for test
	return types.MemberRole(args.String(0)), args.Error(1)
}

// Mock EventPublisher (implements types.EventPublisher)
type MockEventPublisher struct {
	mock.Mock
	_ types.EventPublisher // Embed to satisfy interface
}

func (m *MockEventPublisher) Publish(ctx context.Context, topic string, event types.Event) error {
	args := m.Called(ctx, topic, event)
	return args.Error(0)
}

func (m *MockEventPublisher) PublishBatch(ctx context.Context, topic string, events []types.Event) error {
	args := m.Called(ctx, topic, events)
	return args.Error(0)
}

func (m *MockEventPublisher) Subscribe(ctx context.Context, tripID string, userID string, filters ...types.EventType) (<-chan types.Event, error) {
	args := m.Called(ctx, tripID, userID, filters)
	if ch, ok := args.Get(0).(<-chan types.Event); ok {
		return ch, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockEventPublisher) Unsubscribe(ctx context.Context, tripID string, userID string) error {
	args := m.Called(ctx, tripID, userID)
	return args.Error(0)
}

// Mock OfflineLocationService (implements OfflineLocationServiceInterface from this package)
type MockOfflineLocationService struct {
	mock.Mock
	_ OfflineLocationServiceInterface // Embed interface from current package
}

func (m *MockOfflineLocationService) SaveOfflineLocations(ctx context.Context, userID string, updates []types.LocationUpdate, deviceID string) error {
	args := m.Called(ctx, userID, updates, deviceID)
	return args.Error(0)
}

func (m *MockOfflineLocationService) ProcessOfflineLocations(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func TestValidateLocationUpdate(t *testing.T) {
	mockDB := new(MockLocationDB)
	mockOffline := new(MockOfflineLocationService) // Need to provide this dependency
	// service := NewLocationService(mockDB, nil)
	service := NewManagementService(mockDB, nil, mockOffline) // Use renamed constructor

	tests := []struct {
		name    string
		update  types.LocationUpdate
		wantErr bool
	}{
		{
			name: "valid location update",
			update: types.LocationUpdate{
				Latitude:  45.0,
				Longitude: -75.0,
				Accuracy:  10.0,
				Timestamp: time.Now().UnixMilli(),
			},
			wantErr: false,
		},
		{
			name: "invalid latitude (too high)",
			update: types.LocationUpdate{
				Latitude:  91.0,
				Longitude: -75.0,
				Accuracy:  10.0,
				Timestamp: time.Now().UnixMilli(),
			},
			wantErr: true,
		},
		{
			name: "invalid latitude (too low)",
			update: types.LocationUpdate{
				Latitude:  -91.0,
				Longitude: -75.0,
				Accuracy:  10.0,
				Timestamp: time.Now().UnixMilli(),
			},
			wantErr: true,
		},
		{
			name: "invalid longitude (too high)",
			update: types.LocationUpdate{
				Latitude:  45.0,
				Longitude: 181.0,
				Accuracy:  10.0,
				Timestamp: time.Now().UnixMilli(),
			},
			wantErr: true,
		},
		{
			name: "invalid longitude (too low)",
			update: types.LocationUpdate{
				Latitude:  45.0,
				Longitude: -181.0,
				Accuracy:  10.0,
				Timestamp: time.Now().UnixMilli(),
			},
			wantErr: true,
		},
		{
			name: "invalid accuracy (negative)",
			update: types.LocationUpdate{
				Latitude:  45.0,
				Longitude: -75.0,
				Accuracy:  -1.0,
				Timestamp: time.Now().UnixMilli(),
			},
			wantErr: true,
		},
		{
			name: "invalid timestamp (too old)",
			update: types.LocationUpdate{
				Latitude:  45.0,
				Longitude: -75.0,
				Accuracy:  10.0,
				Timestamp: time.Now().Add(-3 * time.Hour).UnixMilli(), // Adjusted test case
			},
			wantErr: true,
		},
		{
			name: "invalid timestamp (future)",
			update: types.LocationUpdate{
				Latitude:  45.0,
				Longitude: -75.0,
				Accuracy:  10.0,
				Timestamp: time.Now().Add(6 * time.Minute).UnixMilli(), // Adjusted test case
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateLocationUpdate(tt.update)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdateLocation(t *testing.T) {
	ctx := context.Background()
	mockDB := new(MockLocationDB)
	mockEventPublisher := new(MockEventPublisher)
	mockOffline := new(MockOfflineLocationService)
	service := NewManagementService(mockDB, mockEventPublisher, mockOffline) // Use renamed constructor

	validUpdate := types.LocationUpdate{
		Latitude:  45.0,
		Longitude: -75.0,
		Accuracy:  10.0,
		Timestamp: time.Now().UnixMilli(),
	}

	expectedLocation := &types.Location{
		UserID:    "test-user",
		TripID:    "test-trip", // Assuming store provides TripID based on UserID?
		Latitude:  validUpdate.Latitude,
		Longitude: validUpdate.Longitude,
		Accuracy:  validUpdate.Accuracy,
		Timestamp: time.UnixMilli(validUpdate.Timestamp),
	}

	mockDB.On("UpdateLocation", ctx, "test-user", validUpdate).Return(expectedLocation, nil)
	mockEventPublisher.On("Publish", ctx, expectedLocation.TripID, mock.AnythingOfType("types.Event")).Return(nil)

	location, err := service.UpdateLocation(ctx, "test-user", validUpdate)

	assert.NoError(t, err)
	assert.NotNil(t, location)
	assert.Equal(t, expectedLocation, location)

	mockDB.AssertExpectations(t)
	mockEventPublisher.AssertExpectations(t)
}

func TestGetTripMemberLocations(t *testing.T) {
	ctx := context.Background()
	mockDB := new(MockLocationDB)
	mockOffline := new(MockOfflineLocationService)
	service := NewManagementService(mockDB, nil, mockOffline) // Use renamed constructor

	now := time.Now()
	expectedLocations := []types.MemberLocation{
		{
			Location: types.Location{
				UserID:    "user-1",
				Latitude:  45.0,
				Longitude: -75.0,
				Accuracy:  10.0,
				Timestamp: now,
			},
			UserName: "User One",
			UserRole: "member",
		},
		{
			Location: types.Location{
				UserID:    "user-2",
				Latitude:  46.0,
				Longitude: -76.0,
				Accuracy:  15.0,
				Timestamp: now,
			},
			UserName: "User Two",
			UserRole: "admin",
		},
	}

	// Mock the permission check (GetUserRole)
	mockDB.On("GetUserRole", ctx, "test-trip", "requesting-user").Return(types.MemberRoleMember, nil)
	mockDB.On("GetTripMemberLocations", ctx, "test-trip").Return(expectedLocations, nil)

	locations, err := service.GetTripMemberLocations(ctx, "test-trip", "requesting-user")

	assert.NoError(t, err)
	assert.Equal(t, expectedLocations, locations)

	mockDB.AssertExpectations(t)
}

// TestSetOfflineService is removed as SetOfflineService is removed

func TestSaveOfflineLocationsWithoutService(t *testing.T) {
	ctx := context.Background()
	mockDB := new(MockLocationDB)
	// Initialize ManagementService with nil offline service
	service := NewManagementService(mockDB, nil, nil)

	updates := []types.LocationUpdate{{ /* ... */ }}
	err := service.SaveOfflineLocations(ctx, "user", updates, "device")

	assert.Error(t, err)
	// Check for specific AppError if desired
}

// TestProcessOfflineLocationsWithoutService is similar

func TestUpdateLocationDatabaseError(t *testing.T) {
	ctx := context.Background()
	mockDB := new(MockLocationDB)
	mockOffline := new(MockOfflineLocationService)
	service := NewManagementService(mockDB, nil, mockOffline)

	update := types.LocationUpdate{ /* ... valid update ... */ }

	mockDB.On("UpdateLocation", ctx, "user", update).Return(nil, fmt.Errorf("db error"))

	_, err := service.UpdateLocation(ctx, "user", update)

	assert.Error(t, err)
	// assert.IsType(t, &apperrors.AppError{}, err) // Check if it's an AppError
	// appErr := err.(*apperrors.AppError)
	// assert.Equal(t, apperrors.DatabaseError, appErr.Type)
	mockDB.AssertExpectations(t)
}

func TestUpdateLocationEventPublishError(t *testing.T) {
	ctx := context.Background()
	mockDB := new(MockLocationDB)
	mockEventPublisher := new(MockEventPublisher)
	mockOffline := new(MockOfflineLocationService)
	service := NewManagementService(mockDB, mockEventPublisher, mockOffline)

	update := types.LocationUpdate{ /* ... */ }
	expectedLocation := &types.Location{ /* ... */ }

	mockDB.On("UpdateLocation", ctx, "user", update).Return(expectedLocation, nil)
	mockEventPublisher.On("Publish", ctx, mock.Anything, mock.Anything).Return(fmt.Errorf("publish error"))

	_, err := service.UpdateLocation(ctx, "user", update)

	assert.NoError(t, err) // Service currently logs warning, doesn't return error on publish failure
	// If service behavior changes to return error, assert error here:
	// assert.Error(t, err)
	// assert.IsType(t, &apperrors.AppError{}, err)
	// appErr := err.(*apperrors.AppError)
	// assert.Equal(t, apperrors.ServerError, appErr.Type) // Or specific EventPublishError
	mockDB.AssertExpectations(t)
	mockEventPublisher.AssertExpectations(t)
}
