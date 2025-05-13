package service

import (
	"context"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockLocationDB is a mock type for the LocationDBInterface type
type MockLocationDB struct {
	mock.Mock
	store.LocationStore // Embed the interface
}

// Implement methods of store.LocationStore for MockLocationDB
func (m *MockLocationDB) CreateLocation(ctx context.Context, location *types.Location) (string, error) {
	args := m.Called(ctx, location)
	return args.String(0), args.Error(1)
}

func (m *MockLocationDB) GetLocation(ctx context.Context, id string) (*types.Location, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Location), args.Error(1)
}

func (m *MockLocationDB) UpdateLocation(ctx context.Context, id string, update types.LocationUpdate) (*types.Location, error) {
	args := m.Called(ctx, id, update)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Location), args.Error(1)
}

func (m *MockLocationDB) DeleteLocation(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockLocationDB) ListTripMemberLocations(ctx context.Context, tripID string) ([]*types.MemberLocation, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.MemberLocation), args.Error(1)
}

func (m *MockLocationDB) GetTripMemberLocations(ctx context.Context, tripID string) ([]types.MemberLocation, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.MemberLocation), args.Error(1)
}

func (m *MockLocationDB) GetUserRole(ctx context.Context, tripID string, userID string) (types.MemberRole, error) {
	args := m.Called(ctx, tripID, userID)
	return args.Get(0).(types.MemberRole), args.Error(1)
}

func (m *MockLocationDB) BeginTx(ctx context.Context) (types.DatabaseTransaction, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(types.DatabaseTransaction), args.Error(1)
}

// MockEventPublisher is a mock for types.EventPublisher
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

func TestValidateLocationUpdate(t *testing.T) {
	mockDB := new(MockLocationDB)
	service := NewManagementService(mockDB, nil) // Use renamed constructor

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
				Timestamp: time.Now().Add(-25 * time.Hour).UnixMilli(), // More than 24 hours old
			},
			wantErr: true,
		},
		{
			name: "invalid timestamp (in the future)",
			update: types.LocationUpdate{
				Latitude:  45.0,
				Longitude: -75.0,
				Accuracy:  10.0,
				Timestamp: time.Now().Add(10 * time.Minute).UnixMilli(), // timestamp in the future
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateLocationUpdate(tt.update) // Call as a method of the service
			if (err != nil) != tt.wantErr {
				t.Errorf("validateLocationUpdate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpdateLocation(t *testing.T) {
	ctx := context.Background()
	mockDB := new(MockLocationDB)
	mockEventPublisher := new(MockEventPublisher)               // Create a mock event publisher
	service := NewManagementService(mockDB, mockEventPublisher) // Pass mock event publisher

	userID := "test-user"
	validUpdate := types.LocationUpdate{
		Latitude:  45.0,
		Longitude: -75.0,
		Accuracy:  10.0,
		Timestamp: time.Now().UnixMilli(),
	}

	expectedLocation := &types.Location{
		UserID:    userID,
		TripID:    "test-trip", // Assuming store provides TripID based on UserID?
		Latitude:  validUpdate.Latitude,
		Longitude: validUpdate.Longitude,
		Accuracy:  validUpdate.Accuracy,
		Timestamp: time.UnixMilli(validUpdate.Timestamp), // Corrected timestamp conversion
	}

	mockDB.On("UpdateLocation", ctx, userID, validUpdate).Return(expectedLocation, nil)
	mockEventPublisher.On("Publish", ctx, expectedLocation.TripID, mock.AnythingOfType("types.Event")).Return(nil)

	location, err := service.UpdateLocation(ctx, userID, validUpdate)

	assert.NoError(t, err)
	assert.NotNil(t, location)
	mockDB.AssertExpectations(t)
	mockEventPublisher.AssertExpectations(t) // Assert expectations for event publisher
}

func TestGetTripMemberLocations(t *testing.T) {
	ctx := context.Background()
	mockDB := new(MockLocationDB)
	mockEventPublisher := new(MockEventPublisher) // Add event publisher mock
	service := NewManagementService(mockDB, mockEventPublisher)

	tripID := "test-trip"
	userID := "requesting-user"
	now := time.Now()

	// Changed from pointers to match the service implementation return type
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

	// Setup the mock expectations BEFORE calling the method
	// Mock the permission check (GetUserRole)
	mockDB.On("GetUserRole", ctx, tripID, userID).Return(types.MemberRoleMember, nil)

	// Update to use GetTripMemberLocations instead of ListTripMemberLocations to match the service implementation
	mockDB.On("GetTripMemberLocations", ctx, tripID).Return(expectedLocations, nil)

	// Call the method under test
	locations, err := service.GetTripMemberLocations(ctx, tripID, userID)

	// Verify results
	assert.NoError(t, err)
	assert.Equal(t, expectedLocations, locations)
	mockDB.AssertExpectations(t)
}
