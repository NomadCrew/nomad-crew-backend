package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSupabaseService_IsEnabled(t *testing.T) {
	tests := []struct {
		name      string
		isEnabled bool
		want      bool
	}{
		{
			name:      "Service enabled",
			isEnabled: true,
			want:      true,
		},
		{
			name:      "Service disabled",
			isEnabled: false,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SupabaseService{
				isEnabled: tt.isEnabled,
			}
			if got := s.IsEnabled(); got != tt.want {
				t.Errorf("SupabaseService.IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSupabaseService_UpdateLocation_Disabled(t *testing.T) {
	// When service is disabled, no HTTP requests should be made
	service := &SupabaseService{
		isEnabled: false,
	}

	err := service.UpdateLocation(context.Background(), "user123", LocationUpdate{
		TripID:    "trip123",
		Latitude:  40.7128,
		Longitude: -74.0060,
		Accuracy:  10.5,
	})

	if err != nil {
		t.Errorf("Expected no error when service is disabled, got %v", err)
	}
}

func TestSupabaseService_SendChatMessage_Disabled(t *testing.T) {
	// When service is disabled, no HTTP requests should be made
	service := &SupabaseService{
		isEnabled: false,
	}

	now := time.Now()
	err := service.SendChatMessage(context.Background(), ChatMessage{
		ID:        "msg123",
		TripID:    "trip123",
		UserID:    "user123",
		Message:   "Hello World",
		CreatedAt: now,
	})

	if err != nil {
		t.Errorf("Expected no error when service is disabled, got %v", err)
	}
}

func TestSupabaseService_UpdateLocation(t *testing.T) {
	// Channel to capture errors from the handler goroutine
	errCh := make(chan string, 10)

	// Expected location data
	expectedUserID := "user123"
	expectedLocation := LocationUpdate{
		TripID:    "trip123",
		Latitude:  40.7128,
		Longitude: -74.0060,
		Accuracy:  10.5,
		Privacy:   "approximate",
	}

	// Test server to mock Supabase API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request method and path
		if r.Method != "POST" {
			errCh <- "Expected POST request, got " + r.Method
		}
		if r.URL.Path != "/rest/v1/locations" {
			errCh <- "Expected path /rest/v1/locations, got " + r.URL.Path
		}
		if r.Header.Get("apikey") != "test-key" {
			errCh <- "Expected apikey header test-key, got " + r.Header.Get("apikey")
		}

		// Verify request body
		var payload map[string]interface{}
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&payload); err != nil {
			errCh <- "Failed to parse request body: " + err.Error()
		} else {
			// Check user_id
			if user, ok := payload["user_id"].(string); !ok || user != expectedUserID {
				errCh <- "Expected user_id " + expectedUserID + ", got " + user
			}

			// Check trip_id
			if trip, ok := payload["trip_id"].(string); !ok || trip != expectedLocation.TripID {
				errCh <- "Expected trip_id " + expectedLocation.TripID + ", got " + trip
			}

			// Check latitude
			if lat, ok := payload["latitude"].(float64); !ok || lat != expectedLocation.Latitude {
				errCh <- "Expected latitude " + fmt.Sprintf("%f", expectedLocation.Latitude) + ", got " + fmt.Sprintf("%f", lat)
			}

			// Check longitude
			if lng, ok := payload["longitude"].(float64); !ok || lng != expectedLocation.Longitude {
				errCh <- "Expected longitude " + fmt.Sprintf("%f", expectedLocation.Longitude) + ", got " + fmt.Sprintf("%f", lng)
			}

			// Check accuracy (need to handle float32 vs float64 conversion)
			if acc, ok := payload["accuracy"].(float64); !ok || float32(acc) != expectedLocation.Accuracy {
				errCh <- "Expected accuracy " + fmt.Sprintf("%f", expectedLocation.Accuracy) + ", got " + fmt.Sprintf("%f", acc)
			}

			// Check privacy
			if privacy, ok := payload["privacy"].(string); !ok || privacy != expectedLocation.Privacy {
				errCh <- "Expected privacy " + expectedLocation.Privacy + ", got " + privacy
			}
		}

		// Return success
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create service with test server URL
	service := &SupabaseService{
		supabaseURL: server.URL,
		supabaseKey: "test-key",
		httpClient:  http.DefaultClient,
		isEnabled:   true,
	}

	// Call the method
	err := service.UpdateLocation(context.Background(), expectedUserID, expectedLocation)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check for any errors captured from the handler
	close(errCh)
	for msg := range errCh {
		t.Errorf("Request validation error: %s", msg)
	}
}

func TestSupabaseService_SendChatMessage(t *testing.T) {
	// Channel to capture errors from the handler goroutine
	errCh := make(chan string, 10)

	// Create a fixed test time to avoid time comparison issues
	now := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

	// Expected message data
	expectedMsg := ChatMessage{
		ID:        "msg123",
		TripID:    "trip123",
		UserID:    "user123",
		Message:   "Hello World",
		CreatedAt: now,
	}

	// Test server to mock Supabase API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request method and path
		if r.Method != "POST" {
			errCh <- "Expected POST request, got " + r.Method
		}
		if r.URL.Path != "/rest/v1/supabase_chat_messages" {
			errCh <- "Expected path /rest/v1/supabase_chat_messages, got " + r.URL.Path
		}
		if r.Header.Get("apikey") != "test-key" {
			errCh <- "Expected apikey header test-key, got " + r.Header.Get("apikey")
		}

		// Verify request body
		var payload map[string]interface{}
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&payload); err != nil {
			errCh <- "Failed to parse request body: " + err.Error()
		} else {
			// Check id
			if id, ok := payload["id"].(string); !ok || id != expectedMsg.ID {
				errCh <- "Expected id " + expectedMsg.ID + ", got " + id
			}

			// Check trip_id
			if tripID, ok := payload["trip_id"].(string); !ok || tripID != expectedMsg.TripID {
				errCh <- "Expected trip_id " + expectedMsg.TripID + ", got " + tripID
			}

			// Check user_id
			if userID, ok := payload["user_id"].(string); !ok || userID != expectedMsg.UserID {
				errCh <- "Expected user_id " + expectedMsg.UserID + ", got " + userID
			}

			// Check message
			if message, ok := payload["message"].(string); !ok || message != expectedMsg.Message {
				errCh <- "Expected message " + expectedMsg.Message + ", got " + message
			}

			// Note: We don't check created_at timestamp exactly since JSON serialization
			// and parsing might change its format. A more thorough test would need to parse
			// the timestamp back and compare it's within an acceptable range.
		}

		// Return success
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create service with test server URL
	service := &SupabaseService{
		supabaseURL: server.URL,
		supabaseKey: "test-key",
		httpClient:  http.DefaultClient,
		isEnabled:   true,
	}

	// Call the method
	err := service.SendChatMessage(context.Background(), expectedMsg)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check for any errors captured from the handler
	close(errCh)
	for msg := range errCh {
		t.Errorf("Request validation error: %s", msg)
	}
}
