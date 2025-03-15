package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock LocationDB
type MockLocationDB struct {
	mock.Mock
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

// Mock EventPublisher
type MockEventPublisher struct {
	mock.Mock
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

// Mock OfflineLocationService
type MockOfflineLocationService struct {
	mock.Mock
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
	service := NewLocationService(mockDB, nil)

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
				Timestamp: time.Now().Add(-2 * time.Hour).UnixMilli(),
			},
			wantErr: true,
		},
		{
			name: "invalid timestamp (future)",
			update: types.LocationUpdate{
				Latitude:  45.0,
				Longitude: -75.0,
				Accuracy:  10.0,
				Timestamp: time.Now().Add(2 * time.Minute).UnixMilli(),
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
	service := NewLocationService(mockDB, mockEventPublisher)

	validUpdate := types.LocationUpdate{
		Latitude:  45.0,
		Longitude: -75.0,
		Accuracy:  10.0,
		Timestamp: time.Now().UnixMilli(),
	}

	expectedLocation := &types.Location{
		UserID:    "test-user",
		TripID:    "test-trip",
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
	service := NewLocationService(mockDB, nil)

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

	mockDB.On("GetTripMemberLocations", ctx, "test-trip").Return(expectedLocations, nil)

	locations, err := service.GetTripMemberLocations(ctx, "test-trip")

	assert.NoError(t, err)
	assert.Equal(t, expectedLocations, locations)

	mockDB.AssertExpectations(t)
}

func TestSetOfflineService(t *testing.T) {
	mockDB := new(MockLocationDB)
	service := NewLocationService(mockDB, nil)
	mockOffline := new(MockOfflineLocationService)

	// Test setting the offline service
	service.SetOfflineService(mockOffline)

	// Test that SaveOfflineLocations uses the offline service
	ctx := context.Background()
	updates := []types.LocationUpdate{
		{
			Latitude:  45.0,
			Longitude: -75.0,
			Accuracy:  10.0,
			Timestamp: time.Now().UnixMilli(),
		},
	}

	mockOffline.On("SaveOfflineLocations", ctx, "test-user", updates, "test-device").Return(nil)
	err := service.SaveOfflineLocations(ctx, "test-user", updates, "test-device")
	assert.NoError(t, err)
	mockOffline.AssertExpectations(t)

	// Test that ProcessOfflineLocations uses the offline service
	mockOffline.On("ProcessOfflineLocations", ctx, "test-user").Return(nil)
	err = service.ProcessOfflineLocations(ctx, "test-user")
	assert.NoError(t, err)
	mockOffline.AssertExpectations(t)
}

func TestSaveOfflineLocationsWithoutService(t *testing.T) {
	mockDB := new(MockLocationDB)
	service := NewLocationService(mockDB, nil)

	// Test SaveOfflineLocations without setting the offline service
	ctx := context.Background()
	updates := []types.LocationUpdate{
		{
			Latitude:  45.0,
			Longitude: -75.0,
			Accuracy:  10.0,
			Timestamp: time.Now().UnixMilli(),
		},
	}

	err := service.SaveOfflineLocations(ctx, "test-user", updates, "test-device")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "offline service not initialized")
}

func TestProcessOfflineLocationsWithoutService(t *testing.T) {
	mockDB := new(MockLocationDB)
	service := NewLocationService(mockDB, nil)

	// Test ProcessOfflineLocations without setting the offline service
	ctx := context.Background()
	err := service.ProcessOfflineLocations(ctx, "test-user")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "offline service not initialized")
}

func TestUpdateLocationDatabaseError(t *testing.T) {
	ctx := context.Background()
	mockDB := new(MockLocationDB)
	mockEventPublisher := new(MockEventPublisher)
	service := NewLocationService(mockDB, mockEventPublisher)

	validUpdate := types.LocationUpdate{
		Latitude:  45.0,
		Longitude: -75.0,
		Accuracy:  10.0,
		Timestamp: time.Now().UnixMilli(),
	}

	expectedError := fmt.Errorf("database error")
	mockDB.On("UpdateLocation", ctx, "test-user", validUpdate).Return(nil, expectedError)

	location, err := service.UpdateLocation(ctx, "test-user", validUpdate)

	assert.Error(t, err)
	assert.Nil(t, location)
	assert.Equal(t, expectedError, err)
	mockDB.AssertExpectations(t)
	mockEventPublisher.AssertNotCalled(t, "Publish")
}

func TestUpdateLocationEventPublishError(t *testing.T) {
	ctx := context.Background()
	mockDB := new(MockLocationDB)
	mockEventPublisher := new(MockEventPublisher)
	service := NewLocationService(mockDB, mockEventPublisher)

	validUpdate := types.LocationUpdate{
		Latitude:  45.0,
		Longitude: -75.0,
		Accuracy:  10.0,
		Timestamp: time.Now().UnixMilli(),
	}

	expectedLocation := &types.Location{
		UserID:    "test-user",
		TripID:    "test-trip",
		Latitude:  validUpdate.Latitude,
		Longitude: validUpdate.Longitude,
		Accuracy:  validUpdate.Accuracy,
		Timestamp: time.UnixMilli(validUpdate.Timestamp),
	}

	mockDB.On("UpdateLocation", ctx, "test-user", validUpdate).Return(expectedLocation, nil)
	mockEventPublisher.On("Publish", ctx, expectedLocation.TripID, mock.AnythingOfType("types.Event")).Return(fmt.Errorf("event publish error"))

	location, err := service.UpdateLocation(ctx, "test-user", validUpdate)

	assert.NoError(t, err) // The function should still succeed even if event publishing fails
	assert.NotNil(t, location)
	assert.Equal(t, expectedLocation, location)
	mockDB.AssertExpectations(t)
	mockEventPublisher.AssertExpectations(t)
}
