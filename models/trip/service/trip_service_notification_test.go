package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/internal/notification"
	storemocks "github.com/NomadCrew/nomad-crew-backend/internal/store/mocks"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	tripservice "github.com/NomadCrew/nomad-crew-backend/models/trip/service"
	"github.com/NomadCrew/nomad-crew-backend/types"
	typesMocks "github.com/NomadCrew/nomad-crew-backend/types/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// NOTE: MockWeatherService and MockUserStore are defined in mocks_test.go

// MockNotificationService is a mock implementation of NotificationService
type MockNotificationService struct {
	mock.Mock
	enabled bool
}

func (m *MockNotificationService) IsEnabled() bool {
	return m.enabled
}

func (m *MockNotificationService) SendTripUpdate(ctx context.Context, userIDs []string, data notification.TripUpdateData, priority notification.Priority) error {
	args := m.Called(ctx, userIDs, data, priority)
	return args.Error(0)
}

func (m *MockNotificationService) SendTripUpdateAsync(ctx context.Context, userIDs []string, data notification.TripUpdateData, priority notification.Priority) {
	m.Called(ctx, userIDs, data, priority)
}

func (m *MockNotificationService) SendChatMessage(ctx context.Context, recipientIDs []string, data notification.ChatMessageData) error {
	args := m.Called(ctx, recipientIDs, data)
	return args.Error(0)
}

func (m *MockNotificationService) SendChatMessageAsync(ctx context.Context, recipientIDs []string, data notification.ChatMessageData) {
	m.Called(ctx, recipientIDs, data)
}

func (m *MockNotificationService) SendWeatherAlert(ctx context.Context, userIDs []string, data notification.WeatherAlertData, priority notification.Priority) error {
	args := m.Called(ctx, userIDs, data, priority)
	return args.Error(0)
}

func (m *MockNotificationService) SendLocationUpdate(ctx context.Context, recipientIDs []string, data notification.LocationUpdateData) error {
	args := m.Called(ctx, recipientIDs, data)
	return args.Error(0)
}

func (m *MockNotificationService) SendSystemAlert(ctx context.Context, userID string, data notification.SystemAlertData, priority notification.Priority) error {
	args := m.Called(ctx, userID, data, priority)
	return args.Error(0)
}

func (m *MockNotificationService) SendCustomNotification(ctx context.Context, userID string, eventType notification.EventType, priority notification.Priority, data map[string]interface{}) error {
	args := m.Called(ctx, userID, eventType, priority, data)
	return args.Error(0)
}

func TestTripServiceNotifications(t *testing.T) {
	t.Run("CreateTrip sends notification", func(t *testing.T) {
		// Setup mocks
		mockStore := new(storemocks.TripStore)
		mockUserStore := new(MockUserStore)
		mockEventPublisher := new(typesMocks.EventPublisher)
		mockWeatherSvc := new(MockWeatherService)
		mockNotificationSvc := &MockNotificationService{enabled: true}

		// Create service with notification service
		service := tripservice.NewTripManagementService(
			mockStore,
			mockUserStore,
			mockEventPublisher,
			mockWeatherSvc,
			nil, // supabaseService
		)

		// Setup test data
		creatorID := "user-123"
		trip := &types.Trip{
			Name:        "Test Trip",
			Description: "A test trip",
			CreatedBy:   &creatorID,
			StartDate:   time.Now().Add(24 * time.Hour),
			EndDate:     time.Now().Add(48 * time.Hour),
			Status:      types.TripStatusPlanning,
		}

		// Setup expectations
		mockStore.On("CreateTrip", mock.Anything, mock.AnythingOfType("types.Trip")).Return("trip-123", nil)
		mockEventPublisher.On("PublishEvent", mock.Anything, mock.Anything).Return(nil)
		
		// Expect notification to be sent
		mockNotificationSvc.On("SendTripUpdate", 
			mock.Anything, 
			[]string{creatorID},
			mock.MatchedBy(func(data notification.TripUpdateData) bool {
				return data.TripID == "trip-123" &&
					data.TripName == "Test Trip" &&
					data.UpdateType == "trip_created"
			}),
			notification.PriorityMedium,
		).Return(nil)

		// Execute
		createdTrip, err := service.CreateTrip(context.Background(), trip)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, createdTrip)
		assert.Equal(t, "trip-123", createdTrip.ID)
		
		// Wait a bit for async notification
		time.Sleep(100 * time.Millisecond)
		
		mockNotificationSvc.AssertExpectations(t)
	})

	t.Run("UpdateTrip sends notification to all members except updater", func(t *testing.T) {
		// Setup mocks
		mockStore := new(storemocks.TripStore)
		mockUserStore := new(MockUserStore)
		mockEventPublisher := new(typesMocks.EventPublisher)
		mockWeatherSvc := new(MockWeatherService)
		mockNotificationSvc := &MockNotificationService{enabled: true}

		service := tripservice.NewTripManagementService(
			mockStore,
			mockUserStore,
			mockEventPublisher,
			mockWeatherSvc,
			nil, // supabaseService
		)

		// Setup test data
		tripID := "trip-123"
		updaterID := "user-123"
		existingTrip := &types.Trip{
			ID:          tripID,
			Name:        "Old Name",
			Description: "Old description",
			Status:      types.TripStatusPlanning,
		}

		updateData := types.TripUpdate{
			Name: ptr("New Name"),
			Description: ptr("New description"),
		}

		updatedTrip := &types.Trip{
			ID:          tripID,
			Name:        "New Name",
			Description: "New description",
			Status:      types.TripStatusPlanning,
		}

		members := []types.TripMembership{
			{TripID: tripID, UserID: updaterID, Role: types.MemberRoleOwner},
			{TripID: tripID, UserID: "user-456", Role: types.MemberRoleMember},
			{TripID: tripID, UserID: "user-789", Role: types.MemberRoleMember},
		}

		// Setup expectations
		mockStore.On("GetUserRole", mock.Anything, tripID, updaterID).Return(types.MemberRoleOwner, nil)
		mockStore.On("GetTrip", mock.Anything, tripID).Return(existingTrip, nil)
		mockStore.On("UpdateTrip", mock.Anything, tripID, updateData).Return(updatedTrip, nil)
		mockStore.On("GetTripMembers", mock.Anything, tripID).Return(members, nil)
		mockUserStore.On("GetUserByID", mock.Anything, updaterID).Return(&types.User{Username: "John Doe"}, nil)
		mockEventPublisher.On("PublishEvent", mock.Anything, mock.Anything).Return(nil)
		
		// Expect notification to members except updater
		mockNotificationSvc.On("SendTripUpdate",
			mock.Anything,
			[]string{"user-456", "user-789"},
			mock.MatchedBy(func(data notification.TripUpdateData) bool {
				return data.TripID == tripID &&
					data.TripName == "New Name" &&
					data.UpdateType == "trip_updated" &&
					data.UpdatedBy == "John Doe"
			}),
			notification.PriorityMedium,
		).Return(nil)

		// Execute
		result, err := service.UpdateTrip(context.Background(), tripID, updaterID, updateData)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "New Name", result.Name)
		
		// Wait for async notification
		time.Sleep(100 * time.Millisecond)
		
		mockNotificationSvc.AssertExpectations(t)
	})

	t.Run("DeleteTrip sends high priority notification", func(t *testing.T) {
		// Setup mocks
		mockStore := new(storemocks.TripStore)
		mockUserStore := new(MockUserStore)
		mockEventPublisher := new(typesMocks.EventPublisher)
		mockWeatherSvc := new(MockWeatherService)
		mockNotificationSvc := &MockNotificationService{enabled: true}

		service := tripservice.NewTripManagementService(
			mockStore,
			mockUserStore,
			mockEventPublisher,
			mockWeatherSvc,
			nil, // supabaseService
		)

		// Setup test data
		tripID := "trip-123"
		deleterID := "user-123"
		trip := &types.Trip{
			ID:   tripID,
			Name: "Trip to Delete",
		}

		members := []types.TripMembership{
			{TripID: tripID, UserID: deleterID, Role: types.MemberRoleOwner},
			{TripID: tripID, UserID: "user-456", Role: types.MemberRoleMember},
			{TripID: tripID, UserID: "user-789", Role: types.MemberRoleMember},
		}

		// Setup expectations
		mockStore.On("GetTrip", mock.Anything, tripID).Return(trip, nil)
		mockStore.On("GetTripMembers", mock.Anything, tripID).Return(members, nil)
		mockStore.On("SoftDeleteTrip", mock.Anything, tripID).Return(nil)
		mockUserStore.On("GetUserByID", mock.Anything, deleterID).Return(&types.User{Username: "John Doe"}, nil)
		mockEventPublisher.On("PublishEvent", mock.Anything, mock.Anything).Return(nil)
		
		// Expect high priority notification
		mockNotificationSvc.On("SendTripUpdate",
			mock.Anything,
			[]string{"user-456", "user-789"},
			mock.MatchedBy(func(data notification.TripUpdateData) bool {
				return data.TripID == tripID &&
					data.TripName == "Trip to Delete" &&
					data.UpdateType == "trip_deleted" &&
					data.UpdatedBy == "John Doe"
			}),
			notification.PriorityHigh, // High priority for deletion
		).Return(nil)

		// Execute with context that has user ID
		ctx := context.WithValue(context.Background(), middleware.UserIDKey, deleterID)
		err := service.DeleteTrip(ctx, tripID)

		// Assert
		assert.NoError(t, err)
		
		// Wait for async notification
		time.Sleep(100 * time.Millisecond)
		
		mockNotificationSvc.AssertExpectations(t)
	})
}

// Helper function
func ptr(s string) *string {
	return &s
}