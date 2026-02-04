package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/internal/notification"
	"github.com/stretchr/testify/assert"
)

func TestNewNotificationFacadeService(t *testing.T) {
	tests := []struct {
		name      string
		config    *config.NotificationConfig
		expectNil bool
		enabled   bool
	}{
		{
			name: "enabled with valid config",
			config: &config.NotificationConfig{
				Enabled:        true,
				APIUrl:         "https://api.example.com",
				APIKey:         "test-key",
				TimeoutSeconds: 10,
			},
			enabled: true,
		},
		{
			name: "disabled config",
			config: &config.NotificationConfig{
				Enabled: false,
			},
			enabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewNotificationFacadeService(tt.config, nil)

			assert.NotNil(t, service)
			assert.Equal(t, tt.enabled, service.IsEnabled())
		})
	}
}

func TestSendTripUpdate(t *testing.T) {
	mockResponse := &notification.Response{
		NotificationID: "notif-123",
		MessageID:      "msg-456",
		Status:         "success",
		ChannelsUsed:   []string{"PUSH", "EMAIL"},
	}

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		
		// Verify request
		var req notification.Request
		json.NewDecoder(r.Body).Decode(&req)
		
		assert.Equal(t, notification.EventTypeTripUpdate, req.EventType)
		assert.Equal(t, notification.PriorityHigh, req.Priority)
		
		// Send response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	cfg := &config.NotificationConfig{
		Enabled:        true,
		APIUrl:         server.URL,
		APIKey:         "test-key",
		TimeoutSeconds: 10,
	}

	service := NewNotificationFacadeService(cfg, nil)

	data := notification.TripUpdateData{
		TripID:      "trip-123",
		TripName:    "Europe Trip",
		Message:     "Itinerary updated",
		UpdateType:  "itinerary_change",
		UpdatedBy:   "John Doe",
		ChangesMade: "Added Paris visit",
	}

	userIDs := []string{"user-1", "user-2", "user-3"}

	err := service.SendTripUpdate(context.Background(), userIDs, data, notification.PriorityHigh)

	assert.NoError(t, err)
	assert.Equal(t, 3, requestCount) // One request per user
}

func TestSendTripUpdate_Disabled(t *testing.T) {
	cfg := &config.NotificationConfig{
		Enabled: false,
	}

	service := NewNotificationFacadeService(cfg, nil)

	data := notification.TripUpdateData{
		TripID: "trip-123",
	}

	err := service.SendTripUpdate(context.Background(), []string{"user-1"}, data, notification.PriorityHigh)

	assert.NoError(t, err) // Should not error when disabled
}

func TestSendTripUpdate_PartialFailure(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		
		// Fail on second request
		if requestCount == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(&notification.Response{
				Error: "Internal server error",
			})
			return
		}
		
		// Success for others
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(&notification.Response{
			NotificationID: "notif-123",
			Status:         "success",
		})
	}))
	defer server.Close()

	cfg := &config.NotificationConfig{
		Enabled:        true,
		APIUrl:         server.URL,
		APIKey:         "test-key",
		TimeoutSeconds: 10,
	}

	service := NewNotificationFacadeService(cfg, nil)

	data := notification.TripUpdateData{
		TripID: "trip-123",
	}

	userIDs := []string{"user-1", "user-2", "user-3"}

	err := service.SendTripUpdate(context.Background(), userIDs, data, notification.PriorityHigh)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send notifications to 1 users")
	assert.Equal(t, 3, requestCount)
}

func TestSendChatMessage(t *testing.T) {
	mockResponse := &notification.Response{
		NotificationID: "notif-123",
		Status:         "success",
		ChannelsUsed:   []string{"PUSH", "WEBSOCKET"},
	}

	receivedUserIDs := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req notification.Request
		json.NewDecoder(r.Body).Decode(&req)
		
		receivedUserIDs = append(receivedUserIDs, req.UserID)
		
		assert.Equal(t, notification.EventTypeChatMessage, req.EventType)
		assert.Equal(t, notification.PriorityMedium, req.Priority)
		
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	cfg := &config.NotificationConfig{
		Enabled:        true,
		APIUrl:         server.URL,
		APIKey:         "test-key",
		TimeoutSeconds: 10,
	}

	service := NewNotificationFacadeService(cfg, nil)

	data := notification.ChatMessageData{
		TripID:         "trip-123",
		ChatID:         "chat-456",
		SenderID:       "sender-123",
		SenderName:     "John Doe",
		Message:        "Hello everyone!",
		MessagePreview: "Hello everyone!",
	}

	// Include sender in recipients to test filtering
	recipientIDs := []string{"user-1", "sender-123", "user-2"}

	err := service.SendChatMessage(context.Background(), recipientIDs, data)

	assert.NoError(t, err)
	assert.Len(t, receivedUserIDs, 2) // Should skip sender
	assert.NotContains(t, receivedUserIDs, "sender-123")
}

func TestSendWeatherAlert(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req notification.Request
		json.NewDecoder(r.Body).Decode(&req)
		
		assert.Equal(t, notification.EventTypeWeatherAlert, req.EventType)
		assert.Equal(t, notification.PriorityCritical, req.Priority)
		assert.Equal(t, "severe", req.Data["severity"])
		
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(&notification.Response{
			NotificationID: "notif-123",
			Status:         "success",
		})
	}))
	defer server.Close()

	cfg := &config.NotificationConfig{
		Enabled:        true,
		APIUrl:         server.URL,
		APIKey:         "test-key",
		TimeoutSeconds: 10,
	}

	service := NewNotificationFacadeService(cfg, nil)

	data := notification.WeatherAlertData{
		TripID:    "trip-123",
		Location:  "Paris, France",
		AlertType: "storm_warning",
		Message:   "Severe thunderstorm expected",
		Date:      "2025-08-15",
		Severity:  "severe",
	}

	err := service.SendWeatherAlert(context.Background(), []string{"user-1"}, data, notification.PriorityCritical)

	assert.NoError(t, err)
}

func TestSendLocationUpdate(t *testing.T) {
	receivedUserIDs := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req notification.Request
		json.NewDecoder(r.Body).Decode(&req)
		
		receivedUserIDs = append(receivedUserIDs, req.UserID)
		
		assert.Equal(t, notification.EventTypeLocationUpdate, req.EventType)
		assert.Equal(t, notification.PriorityLow, req.Priority)
		
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(&notification.Response{
			NotificationID: "notif-123",
			Status:         "success",
		})
	}))
	defer server.Close()

	cfg := &config.NotificationConfig{
		Enabled:        true,
		APIUrl:         server.URL,
		APIKey:         "test-key",
		TimeoutSeconds: 10,
	}

	service := NewNotificationFacadeService(cfg, nil)

	data := notification.LocationUpdateData{
		TripID:       "trip-123",
		SharedByID:   "sharer-123",
		SharedByName: "Jane Smith",
		Location: notification.LocationCoordinate{
			Lat:  48.8566,
			Lng:  2.3522,
			Name: "Eiffel Tower",
		},
	}

	// Include sharer in recipients to test filtering
	recipientIDs := []string{"user-1", "sharer-123", "user-2"}

	err := service.SendLocationUpdate(context.Background(), recipientIDs, data)

	assert.NoError(t, err)
	assert.Len(t, receivedUserIDs, 2) // Should skip sharer
	assert.NotContains(t, receivedUserIDs, "sharer-123")
}

func TestSendSystemAlert(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req notification.Request
		json.NewDecoder(r.Body).Decode(&req)
		
		assert.Equal(t, notification.EventTypeSystemAlert, req.EventType)
		assert.Equal(t, notification.PriorityCritical, req.Priority)
		assert.Equal(t, true, req.Data["actionRequired"])
		
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(&notification.Response{
			NotificationID: "notif-123",
			Status:         "success",
		})
	}))
	defer server.Close()

	cfg := &config.NotificationConfig{
		Enabled:        true,
		APIUrl:         server.URL,
		APIKey:         "test-key",
		TimeoutSeconds: 10,
	}

	service := NewNotificationFacadeService(cfg, nil)

	data := notification.SystemAlertData{
		AlertType:      "payment_failed",
		Message:        "Your subscription payment failed",
		ActionRequired: true,
		ActionURL:      "https://app.nomadcrew.com/billing",
	}

	err := service.SendSystemAlert(context.Background(), "user-123", data, notification.PriorityCritical)

	assert.NoError(t, err)
}

func TestSendCustomNotification(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req notification.Request
		json.NewDecoder(r.Body).Decode(&req)
		
		assert.Equal(t, "user-123", req.UserID)
		assert.Equal(t, notification.EventTypeTripUpdate, req.EventType)
		assert.Equal(t, notification.PriorityMedium, req.Priority)
		assert.Equal(t, "custom-value", req.Data["customField"])
		
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(&notification.Response{
			NotificationID: "notif-123",
			Status:         "success",
		})
	}))
	defer server.Close()

	cfg := &config.NotificationConfig{
		Enabled:        true,
		APIUrl:         server.URL,
		APIKey:         "test-key",
		TimeoutSeconds: 10,
	}

	service := NewNotificationFacadeService(cfg, nil)

	customData := map[string]interface{}{
		"customField":  "custom-value",
		"anotherField": 123,
	}

	err := service.SendCustomNotification(
		context.Background(),
		"user-123",
		notification.EventTypeTripUpdate,
		notification.PriorityMedium,
		customData,
	)

	assert.NoError(t, err)
}

func TestAsyncNotifications(t *testing.T) {
	notificationReceived := make(chan bool, 1)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		notificationReceived <- true
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(&notification.Response{
			NotificationID: "notif-123",
			Status:         "success",
		})
	}))
	defer server.Close()

	cfg := &config.NotificationConfig{
		Enabled:        true,
		APIUrl:         server.URL,
		APIKey:         "test-key",
		TimeoutSeconds: 10,
	}

	service := NewNotificationFacadeService(cfg, nil)

	data := notification.TripUpdateData{
		TripID: "trip-123",
	}

	// Send async notification (uses fallback goroutine since no worker pool)
	service.SendTripUpdateAsync(context.Background(), []string{"user-1"}, data, notification.PriorityHigh)

	// Wait for notification to be received
	select {
	case <-notificationReceived:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Async notification not received within timeout")
	}
}

func TestChatMessageAsync(t *testing.T) {
	notificationReceived := make(chan bool, 1)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		notificationReceived <- true
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(&notification.Response{
			NotificationID: "notif-123",
			Status:         "success",
		})
	}))
	defer server.Close()

	cfg := &config.NotificationConfig{
		Enabled:        true,
		APIUrl:         server.URL,
		APIKey:         "test-key",
		TimeoutSeconds: 10,
	}

	service := NewNotificationFacadeService(cfg, nil)

	data := notification.ChatMessageData{
		TripID:   "trip-123",
		ChatID:   "chat-456",
		SenderID: "sender-123",
	}

	// Send async notification (uses fallback goroutine since no worker pool)
	service.SendChatMessageAsync(context.Background(), []string{"user-1"}, data)

	// Wait for notification to be received
	select {
	case <-notificationReceived:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Async chat notification not received within timeout")
	}
}