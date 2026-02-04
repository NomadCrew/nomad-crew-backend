package notification

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	client := NewClient("https://api.example.com", "test-key")
	
	assert.NotNil(t, client)
	assert.Equal(t, "https://api.example.com", client.apiURL)
	assert.Equal(t, "test-key", client.apiKey)
	assert.NotNil(t, client.httpClient)
	assert.Equal(t, 10*time.Second, client.httpClient.Timeout)
}

func TestNewClientWithCustomHTTPClient(t *testing.T) {
	customClient := &http.Client{
		Timeout: 5 * time.Second,
	}
	
	client := NewClient("https://api.example.com", "test-key", WithHTTPClient(customClient))
	
	assert.Equal(t, customClient, client.httpClient)
	assert.Equal(t, 5*time.Second, client.httpClient.Timeout)
}

func TestValidateRequest(t *testing.T) {
	client := NewClient("https://api.example.com", "test-key")
	
	tests := []struct {
		name    string
		request *Request
		wantErr string
	}{
		{
			name: "valid request",
			request: &Request{
				UserID:    "user-123",
				EventType: EventTypeTripUpdate,
				Priority:  PriorityHigh,
				Data:      map[string]interface{}{"test": "data"},
			},
			wantErr: "",
		},
		{
			name: "missing user ID",
			request: &Request{
				EventType: EventTypeTripUpdate,
			},
			wantErr: "userId is required",
		},
		{
			name: "missing event type",
			request: &Request{
				UserID: "user-123",
			},
			wantErr: "eventType is required",
		},
		{
			name: "invalid event type",
			request: &Request{
				UserID:    "user-123",
				EventType: "INVALID_TYPE",
			},
			wantErr: "invalid eventType: INVALID_TYPE",
		},
		{
			name: "invalid priority",
			request: &Request{
				UserID:    "user-123",
				EventType: EventTypeTripUpdate,
				Priority:  "INVALID_PRIORITY",
			},
			wantErr: "invalid priority: INVALID_PRIORITY",
		},
		{
			name: "nil data is okay",
			request: &Request{
				UserID:    "user-123",
				EventType: EventTypeTripUpdate,
				Data:      nil,
			},
			wantErr: "",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.validateRequest(tt.request)
			
			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
				// Ensure Data is initialized if it was nil
				assert.NotNil(t, tt.request.Data)
			}
		})
	}
}

func TestSend_Success(t *testing.T) {
	expectedResponse := &Response{
		NotificationID: "notif-123",
		MessageID:      "msg-456",
		Status:         "success",
		ChannelsUsed:   []string{"PUSH", "EMAIL"},
	}
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "test-key", r.Header.Get("x-api-key"))
		
		// Verify request body
		var req Request
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "user-123", req.UserID)
		assert.Equal(t, EventTypeTripUpdate, req.EventType)
		
		// Send response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(expectedResponse)
	}))
	defer server.Close()
	
	client := NewClient(server.URL, "test-key")
	
	req := &Request{
		UserID:    "user-123",
		EventType: EventTypeTripUpdate,
		Priority:  PriorityHigh,
		Data: map[string]interface{}{
			"tripId": "trip-456",
		},
	}
	
	resp, err := client.Send(context.Background(), req)
	
	assert.NoError(t, err)
	assert.Equal(t, expectedResponse.NotificationID, resp.NotificationID)
	assert.Equal(t, expectedResponse.MessageID, resp.MessageID)
	assert.Equal(t, expectedResponse.Status, resp.Status)
	assert.Equal(t, expectedResponse.ChannelsUsed, resp.ChannelsUsed)
}

func TestSend_ValidationError(t *testing.T) {
	client := NewClient("https://api.example.com", "test-key")
	
	req := &Request{
		// Missing UserID
		EventType: EventTypeTripUpdate,
	}
	
	resp, err := client.Send(context.Background(), req)
	
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "invalid request")
	assert.Contains(t, err.Error(), "userId is required")
}

func TestSend_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(&Response{
			Error: "Internal server error",
		})
	}))
	defer server.Close()
	
	client := NewClient(server.URL, "test-key")
	
	req := &Request{
		UserID:    "user-123",
		EventType: EventTypeTripUpdate,
	}
	
	resp, err := client.Send(context.Background(), req)
	
	assert.Error(t, err)
	assert.NotNil(t, resp)
	assert.Contains(t, err.Error(), "notification failed with status 500")
	assert.Contains(t, err.Error(), "Internal server error")
}

func TestSend_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(&Response{
			Error: "Rate limit exceeded",
		})
	}))
	defer server.Close()
	
	client := NewClient(server.URL, "test-key")
	
	req := &Request{
		UserID:    "user-123",
		EventType: EventTypeTripUpdate,
	}
	
	resp, err := client.Send(context.Background(), req)
	
	assert.Error(t, err)
	assert.NotNil(t, resp)
	assert.Contains(t, err.Error(), "notification failed with status 429")
	assert.Contains(t, err.Error(), "Rate limit exceeded")
}

func TestSend_ContextCancellation(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	client := NewClient(server.URL, "test-key")
	
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	
	req := &Request{
		UserID:    "user-123",
		EventType: EventTypeTripUpdate,
	}
	
	resp, err := client.Send(ctx, req)
	
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestSendAsync(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(&Response{
			NotificationID: "notif-123",
			Status:         "success",
		})
	}))
	defer server.Close()
	
	client := NewClient(server.URL, "test-key")
	
	req := &Request{
		UserID:    "user-123",
		EventType: EventTypeTripUpdate,
	}
	
	errChan := client.SendAsync(context.Background(), req)
	
	select {
	case err := <-errChan:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for async response")
	}
}

func TestSendBatch(t *testing.T) {
	successCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		successCount++
		if successCount == 2 {
			// Fail the second request
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(&Response{
				Error: "Bad request",
			})
		} else {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(&Response{
				NotificationID: "notif-123",
				Status:         "success",
			})
		}
	}))
	defer server.Close()
	
	client := NewClient(server.URL, "test-key")
	
	requests := []*Request{
		{
			UserID:    "user-1",
			EventType: EventTypeTripUpdate,
		},
		{
			UserID:    "user-2",
			EventType: EventTypeChatMessage,
		},
		{
			UserID:    "user-3",
			EventType: EventTypeWeatherAlert,
		},
	}
	
	errors, err := client.SendBatch(context.Background(), requests)
	
	assert.NoError(t, err) // SendBatch itself doesn't return error
	assert.Len(t, errors, 3)
	assert.NoError(t, errors[0])
	assert.Error(t, errors[1])
	assert.NoError(t, errors[2])
}

// Test helper methods

func TestSendTripUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		json.NewDecoder(r.Body).Decode(&req)
		
		assert.Equal(t, EventTypeTripUpdate, req.EventType)
		assert.Equal(t, PriorityHigh, req.Priority)
		assert.Equal(t, "trip-123", req.Data["tripId"])
		assert.Equal(t, "Europe Trip", req.Data["tripName"])
		
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(&Response{Status: "success"})
	}))
	defer server.Close()
	
	client := NewClient(server.URL, "test-key")
	
	data := TripUpdateData{
		TripID:      "trip-123",
		TripName:    "Europe Trip",
		Message:     "Itinerary updated",
		UpdateType:  "itinerary_change",
		UpdatedBy:   "John Doe",
		ChangesMade: "Added new activity",
	}
	
	resp, err := client.SendTripUpdate(context.Background(), "user-123", data, PriorityHigh)
	
	assert.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
}

func TestSendChatMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		json.NewDecoder(r.Body).Decode(&req)
		
		assert.Equal(t, EventTypeChatMessage, req.EventType)
		assert.Equal(t, PriorityMedium, req.Priority)
		assert.Equal(t, "John Doe", req.Data["senderName"])
		
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(&Response{Status: "success"})
	}))
	defer server.Close()
	
	client := NewClient(server.URL, "test-key")
	
	data := ChatMessageData{
		TripID:         "trip-123",
		ChatID:         "chat-456",
		SenderID:       "user-789",
		SenderName:     "John Doe",
		Message:        "Hey everyone!",
		MessagePreview: "Hey everyone!",
	}
	
	resp, err := client.SendChatMessage(context.Background(), "user-123", data)
	
	assert.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
}

func TestSendWeatherAlert(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		json.NewDecoder(r.Body).Decode(&req)
		
		assert.Equal(t, EventTypeWeatherAlert, req.EventType)
		assert.Equal(t, PriorityCritical, req.Priority)
		assert.Equal(t, "severe", req.Data["severity"])
		
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(&Response{Status: "success"})
	}))
	defer server.Close()
	
	client := NewClient(server.URL, "test-key")
	
	data := WeatherAlertData{
		TripID:    "trip-123",
		Location:  "Paris, France",
		AlertType: "storm_warning",
		Message:   "Severe thunderstorm expected",
		Date:      "2025-08-15",
		Severity:  "severe",
	}
	
	resp, err := client.SendWeatherAlert(context.Background(), "user-123", data, PriorityCritical)
	
	assert.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
}

func TestSendLocationUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		json.NewDecoder(r.Body).Decode(&req)
		
		assert.Equal(t, EventTypeLocationUpdate, req.EventType)
		assert.Equal(t, PriorityLow, req.Priority)
		
		location := req.Data["location"].(map[string]interface{})
		assert.Equal(t, 48.8566, location["lat"])
		assert.Equal(t, 2.3522, location["lng"])
		
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(&Response{Status: "success"})
	}))
	defer server.Close()
	
	client := NewClient(server.URL, "test-key")
	
	data := LocationUpdateData{
		TripID:       "trip-123",
		SharedByID:   "user-456",
		SharedByName: "Jane Smith",
		Location: LocationCoordinate{
			Lat:  48.8566,
			Lng:  2.3522,
			Name: "Eiffel Tower",
		},
	}
	
	resp, err := client.SendLocationUpdate(context.Background(), "user-123", data)
	
	assert.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
}

func TestSendSystemAlert(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		json.NewDecoder(r.Body).Decode(&req)
		
		assert.Equal(t, EventTypeSystemAlert, req.EventType)
		assert.Equal(t, PriorityCritical, req.Priority)
		assert.Equal(t, true, req.Data["actionRequired"])
		
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(&Response{Status: "success"})
	}))
	defer server.Close()
	
	client := NewClient(server.URL, "test-key")
	
	data := SystemAlertData{
		AlertType:      "payment_failed",
		Message:        "Your subscription payment failed",
		ActionRequired: true,
		ActionURL:      "https://app.nomadcrew.com/billing",
	}
	
	resp, err := client.SendSystemAlert(context.Background(), "user-123", data, PriorityCritical)
	
	assert.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
}

func TestMarkNotificationRead(t *testing.T) {
	tests := []struct {
		name           string
		notificationID string
		userID         string
		statusCode     int
		responseBody   string
		wantErr        bool
		errMsg         string
	}{
		{
			name:           "successful mark as read",
			notificationID: "notif-123",
			userID:         "user-456",
			statusCode:     http.StatusOK,
			responseBody:   `{"notificationId":"notif-123","readStatus":"READ","readAt":"2024-01-01T12:00:00Z"}`,
			wantErr:        false,
		},
		{
			name:           "notification not found",
			notificationID: "notif-999",
			userID:         "user-456",
			statusCode:     http.StatusNotFound,
			responseBody:   `{"error":"Notification not found"}`,
			wantErr:        true,
			errMsg:         "failed to mark notification as read: Notification not found",
		},
		{
			name:           "access denied",
			notificationID: "notif-123",
			userID:         "user-999",
			statusCode:     http.StatusForbidden,
			responseBody:   `{"error":"Access denied"}`,
			wantErr:        true,
			errMsg:         "failed to mark notification as read: Access denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				assert.Equal(t, "PUT", r.Method)
				assert.Equal(t, "/notifications/"+tt.notificationID+"/read", r.URL.Path)
				assert.Equal(t, "test-api-key", r.Header.Get("x-api-key"))

				// Check request body
				var body map[string]string
				json.NewDecoder(r.Body).Decode(&body)
				assert.Equal(t, tt.userID, body["userId"])

				// Send response
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			// Create client
			client := NewClient(server.URL, "test-api-key")

			// Make request
			err := client.MarkNotificationRead(context.Background(), tt.notificationID, tt.userID)

			// Check result
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetUserNotifications(t *testing.T) {
	tests := []struct {
		name         string
		userID       string
		opts         *NotificationQueryOptions
		statusCode   int
		responseBody string
		wantErr      bool
		checkResult  func(t *testing.T, result *NotificationList)
	}{
		{
			name:   "successful get notifications",
			userID: "user-123",
			opts: &NotificationQueryOptions{
				Limit:      10,
				ReadStatus: "UNREAD",
			},
			statusCode: http.StatusOK,
			responseBody: `{
				"notifications": [
					{
						"notificationId": "notif-1",
						"eventType": "TRIP_UPDATE",
						"priority": "MEDIUM",
						"readStatus": "UNREAD",
						"timestamp": "2024-01-01T12:00:00Z"
					}
				],
				"count": 1,
				"unreadCount": 1,
				"hasMore": false
			}`,
			wantErr: false,
			checkResult: func(t *testing.T, result *NotificationList) {
				assert.Equal(t, 1, result.Count)
				assert.Equal(t, 1, result.UnreadCount)
				assert.False(t, result.HasMore)
				assert.Len(t, result.Notifications, 1)
				assert.Equal(t, "notif-1", result.Notifications[0].NotificationID)
			},
		},
		{
			name:   "with pagination",
			userID: "user-123",
			opts: &NotificationQueryOptions{
				Limit:   20,
				LastKey: "some-key",
			},
			statusCode: http.StatusOK,
			responseBody: `{
				"notifications": [],
				"count": 0,
				"hasMore": true,
				"lastKey": "next-key"
			}`,
			wantErr: false,
			checkResult: func(t *testing.T, result *NotificationList) {
				assert.Equal(t, 0, result.Count)
				assert.True(t, result.HasMore)
				assert.Equal(t, "next-key", result.LastKey)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/users/"+tt.userID+"/notifications", r.URL.Path)
				assert.Equal(t, "test-api-key", r.Header.Get("x-api-key"))

				// Check query parameters
				query := r.URL.Query()
				if tt.opts != nil {
					if tt.opts.Limit > 0 {
						assert.Equal(t, "20", query.Get("limit"))
					}
					if tt.opts.ReadStatus != "" {
						assert.Equal(t, tt.opts.ReadStatus, query.Get("readStatus"))
					}
					if tt.opts.LastKey != "" {
						assert.Equal(t, tt.opts.LastKey, query.Get("lastKey"))
					}
				}

				// Send response
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			// Create client
			client := NewClient(server.URL, "test-api-key")

			// Make request
			result, err := client.GetUserNotifications(context.Background(), tt.userID, tt.opts)

			// Check result
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.checkResult != nil {
					tt.checkResult(t, result)
				}
			}
		})
	}
}

func TestGetUserPreferences(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/users/user-123/preferences", r.URL.Path)
		assert.Equal(t, "test-api-key", r.Header.Get("x-api-key"))

		// Send response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"userId": "user-123",
			"preferences": {
				"channels": {
					"PUSH": {"enabled": true},
					"EMAIL": {"enabled": false}
				},
				"quietHours": {
					"enabled": true,
					"start": "22:00",
					"end": "08:00"
				},
				"notificationTypes": {
					"TRIP_UPDATE": {"enabled": true},
					"CHAT_MESSAGE": {"enabled": false}
				}
			}
		}`))
	}))
	defer server.Close()

	// Create client
	client := NewClient(server.URL, "test-api-key")

	// Make request
	prefs, err := client.GetUserPreferences(context.Background(), "user-123")

	// Check result
	assert.NoError(t, err)
	assert.NotNil(t, prefs)
	assert.True(t, prefs.Channels["PUSH"].Enabled)
	assert.False(t, prefs.Channels["EMAIL"].Enabled)
	assert.True(t, prefs.QuietHours.Enabled)
	assert.Equal(t, "22:00", prefs.QuietHours.Start)
	assert.True(t, prefs.NotificationTypes[EventTypeTripUpdate].Enabled)
	assert.False(t, prefs.NotificationTypes[EventTypeChatMessage].Enabled)
}

func TestUpdateUserPreferences(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/users/user-123/preferences", r.URL.Path)
		assert.Equal(t, "test-api-key", r.Header.Get("x-api-key"))

		// Check request body
		var prefs UserPreferences
		json.NewDecoder(r.Body).Decode(&prefs)
		assert.False(t, prefs.Channels["EMAIL"].Enabled)
		assert.True(t, prefs.NotificationTypes[EventTypeTripUpdate].Enabled)

		// Send response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Preferences updated successfully"}`))
	}))
	defer server.Close()

	// Create client
	client := NewClient(server.URL, "test-api-key")

	// Create preferences
	prefs := &UserPreferences{
		Channels: map[string]ChannelPreference{
			"EMAIL": {Enabled: false},
		},
		NotificationTypes: map[EventType]NotificationTypeConfig{
			EventTypeTripUpdate: {Enabled: true},
		},
	}

	// Make request
	err := client.UpdateUserPreferences(context.Background(), "user-123", prefs)

	// Check result
	assert.NoError(t, err)
}