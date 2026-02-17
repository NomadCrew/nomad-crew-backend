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

		// Create service (notification service not wired in this test)
		service := tripservice.NewTripManagementService(
			mockStore,
			mockUserStore,
			mockEventPublisher,
			mockWeatherSvc,
			nil, // supabaseService
			nil, // notificationSvc
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
		mockEventPublisher.On("Publish", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("types.Event")).Return(nil)

		// Execute
		createdTrip, err := service.CreateTrip(context.Background(), trip)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, createdTrip)
		assert.Equal(t, "trip-123", createdTrip.ID)

		mockStore.AssertExpectations(t)
		mockEventPublisher.AssertExpectations(t)
	})

	t.Run("UpdateTrip sends notification to all members except updater", func(t *testing.T) {
		// Setup mocks
		mockStore := new(storemocks.TripStore)
		mockUserStore := new(MockUserStore)
		mockEventPublisher := new(typesMocks.EventPublisher)
		mockWeatherSvc := new(MockWeatherService)

		service := tripservice.NewTripManagementService(
			mockStore,
			mockUserStore,
			mockEventPublisher,
			mockWeatherSvc,
			nil, // supabaseService
			nil, // notificationSvc
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

		// Setup expectations
		mockStore.On("GetUserRole", mock.Anything, tripID, updaterID).Return(types.MemberRoleOwner, nil)
		mockStore.On("GetTrip", mock.Anything, tripID).Return(existingTrip, nil)
		mockStore.On("UpdateTrip", mock.Anything, tripID, updateData).Return(updatedTrip, nil)
		mockEventPublisher.On("Publish", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("types.Event")).Return(nil)

		// Execute
		result, err := service.UpdateTrip(context.Background(), tripID, updaterID, updateData)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "New Name", result.Name)

		mockStore.AssertExpectations(t)
		mockEventPublisher.AssertExpectations(t)
	})

	t.Run("DeleteTrip sends high priority notification", func(t *testing.T) {
		// Setup mocks
		mockStore := new(storemocks.TripStore)
		mockUserStore := new(MockUserStore)
		mockEventPublisher := new(typesMocks.EventPublisher)
		mockWeatherSvc := new(MockWeatherService)
		service := tripservice.NewTripManagementService(
			mockStore,
			mockUserStore,
			mockEventPublisher,
			mockWeatherSvc,
			nil, // supabaseService
			nil, // notificationSvc
		)

		// Setup test data
		tripID := "trip-123"
		deleterID := "user-123"
		trip := &types.Trip{
			ID:   tripID,
			Name: "Trip to Delete",
		}

		// Setup expectations
		mockStore.On("GetTrip", mock.Anything, tripID).Return(trip, nil)
		mockStore.On("SoftDeleteTrip", mock.Anything, tripID).Return(nil)
		mockEventPublisher.On("Publish", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("types.Event")).Return(nil)

		// Execute with context that has user ID
		ctx := context.WithValue(context.Background(), middleware.UserIDKey, deleterID)
		err := service.DeleteTrip(ctx, tripID)

		// Assert
		assert.NoError(t, err)

		mockStore.AssertExpectations(t)
		mockEventPublisher.AssertExpectations(t)
	})
}

// Helper function
func ptr(s string) *string {
	return &s
}