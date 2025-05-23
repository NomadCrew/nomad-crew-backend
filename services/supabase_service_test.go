package services

import (
	"context"
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
	// Test server to mock Supabase API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/rest/v1/locations" {
			t.Errorf("Expected path /rest/v1/locations, got %s", r.URL.Path)
		}
		if r.Header.Get("apikey") != "test-key" {
			t.Errorf("Expected apikey header test-key, got %s", r.Header.Get("apikey"))
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
	err := service.UpdateLocation(context.Background(), "user123", LocationUpdate{
		TripID:    "trip123",
		Latitude:  40.7128,
		Longitude: -74.0060,
		Accuracy:  10.5,
		Privacy:   "approximate",
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestSupabaseService_SendChatMessage(t *testing.T) {
	// Test server to mock Supabase API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/rest/v1/supabase_chat_messages" {
			t.Errorf("Expected path /rest/v1/supabase_chat_messages, got %s", r.URL.Path)
		}
		if r.Header.Get("apikey") != "test-key" {
			t.Errorf("Expected apikey header test-key, got %s", r.Header.Get("apikey"))
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
	now := time.Now()
	err := service.SendChatMessage(context.Background(), ChatMessage{
		ID:        "msg123",
		TripID:    "trip123",
		UserID:    "user123",
		Message:   "Hello World",
		CreatedAt: now,
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}
