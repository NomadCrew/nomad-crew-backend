package tests

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/handlers"
	internalStoreMocks "github.com/NomadCrew/nomad-crew-backend/internal/store/mocks"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	serviceMocks "github.com/NomadCrew/nomad-crew-backend/services/mocks"
	"github.com/NomadCrew/nomad-crew-backend/types"
	typesMocks "github.com/NomadCrew/nomad-crew-backend/types/mocks"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestChatWebSocketHandling tests the WebSocket handlers for chat functionality
func TestChatWebSocketHandling(t *testing.T) {
	// Skip this test in normal builds
	t.Skip("WebSocket integration test - run manually")

	// Set up mocks
	mockTripStore := new(internalStoreMocks.TripStore)
	mockEventService := new(typesMocks.EventPublisher)
	mockRateLimitService := new(serviceMocks.MockRateLimiter)

	// Mock behavior for trip membership verification
	mockTripStore.On("GetUserRole", mock.Anything, "trip123", "user456").Return(types.MemberRoleOwner, nil)

	// Mock event subscription
	mockEventChan := make(chan types.Event, 5)
	mockEventService.On("Subscribe", mock.Anything, "trip123", "user456", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockEventChan, nil)
	mockEventService.On("Unsubscribe", mock.Anything, "trip123", "user456").Return(nil)

	// Create handler
	wsHandler := handlers.NewWSHandler(mockRateLimitService, mockEventService, mockTripStore)

	// Set up test server
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		// Mock auth middleware
		c.Set(string(middleware.UserIDKey), "user456")
		c.Next()
	})
	router.GET("/trips/:tripID/ws", wsHandler.HandleChatWebSocketConnection)

	server := httptest.NewServer(router)
	defer server.Close()

	// Convert HTTP server to WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/trips/trip123/ws"

	// Connect WebSocket client
	dialer := &websocket.Dialer{}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	// Test receiving event
	go func() {
		// Wait briefly for connection setup
		time.Sleep(100 * time.Millisecond)

		// Send a test event through the mock channel
		testEvent := types.Event{
			BaseEvent: types.BaseEvent{
				ID:        "test-event-123",
				Type:      types.EventTypeChatMessageSent,
				TripID:    "trip123",
				UserID:    "user789",
				Timestamp: time.Now(),
				Version:   1,
			},
			Metadata: types.EventMetadata{
				Source: "test",
			},
			Payload: json.RawMessage(`{"messageId":"msg123","content":"Test message"}`),
		}

		mockEventChan <- testEvent
	}()

	// Read response message
	_, message, err := conn.ReadMessage()
	assert.NoError(t, err)

	// Parse response
	var response map[string]interface{}
	err = json.Unmarshal(message, &response)
	assert.NoError(t, err)

	// Verify response fields
	assert.Equal(t, "CHAT_MESSAGE_SENT", response["type"])
	assert.Equal(t, "trip123", response["tripID"])

	// Test sending message
	testMsg := map[string]interface{}{
		"type": "typing",
		"payload": map[string]interface{}{
			"isTyping": true,
		},
	}
	testMsgJSON, _ := json.Marshal(testMsg)

	// Mock event publish
	mockEventService.On("Publish", mock.Anything, "trip123", mock.AnythingOfType("types.Event")).Return(nil)

	// Send message
	err = conn.WriteMessage(websocket.TextMessage, testMsgJSON)
	assert.NoError(t, err)

	// Allow time for processing
	time.Sleep(100 * time.Millisecond)

	// Verify event was published
	mockEventService.AssertCalled(t, "Publish", mock.Anything, "trip123", mock.AnythingOfType("types.Event"))
}
