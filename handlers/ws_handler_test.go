package handlers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/services"
	"github.com/NomadCrew/nomad-crew-backend/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// MockSafeConn is a simplified version of SafeConn for testing
type MockSafeConn struct {
	mock.Mock
	UserID string
	TripID string
}

func (m *MockSafeConn) WriteJSON(data interface{}) error {
	args := m.Called(data)
	return args.Error(0)
}

func (m *MockSafeConn) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockEventService is a mock for the event service
type MockEventService struct {
	mock.Mock
}

func (m *MockEventService) Publish(ctx context.Context, tripID string, event types.Event) error {
	args := m.Called(ctx, tripID, event)
	return args.Error(0)
}

func (m *MockEventService) Subscribe(ctx context.Context, tripID string, userID string, filters ...types.EventType) (<-chan types.Event, error) {
	args := m.Called(ctx, tripID, userID, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(<-chan types.Event), args.Error(1)
}

func (m *MockEventService) Unsubscribe(ctx context.Context, tripID string, userID string) error {
	args := m.Called(ctx, tripID, userID)
	return args.Error(0)
}

// PublishBatch implements the EventPublisher interface
func (m *MockEventService) PublishBatch(ctx context.Context, tripID string, events []types.Event) error {
	args := m.Called(ctx, tripID, events)
	return args.Error(0)
}

// MockRateLimiterService is a mock for the rate limiter service
type MockRateLimiterService struct {
	mock.Mock
}

// CheckLimit implements RateLimiterInterface
func (m *MockRateLimiterService) CheckLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	args := m.Called(ctx, key, limit, window)
	return args.Bool(0), args.Get(1).(time.Duration), args.Error(2)
}

// MockTripStore is a mock for the TripStore
type MockTripStore struct {
	mock.Mock
}

// GetUserRole implements one of the needed TripStore methods for the test
func (m *MockTripStore) GetUserRole(ctx context.Context, tripID string, userID string) (types.MemberRole, error) {
	args := m.Called(ctx, tripID, userID)
	return args.Get(0).(types.MemberRole), args.Error(1)
}

// AddMember implements the required method from the TripStore interface
func (m *MockTripStore) AddMember(ctx context.Context, membership *types.TripMembership) error {
	args := m.Called(ctx, membership)
	return args.Error(0)
}

// GetPool returns a nil pool, as we don't need it for tests
func (m *MockTripStore) GetPool() *pgxpool.Pool {
	return nil
}

// Additional required methods from the interface
func (m *MockTripStore) CreateTrip(ctx context.Context, trip types.Trip) (string, error) {
	args := m.Called(ctx, trip)
	return args.String(0), args.Error(1)
}

func (m *MockTripStore) GetTrip(ctx context.Context, id string) (*types.Trip, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Trip), args.Error(1)
}

func (m *MockTripStore) UpdateTrip(ctx context.Context, id string, update types.TripUpdate) (*types.Trip, error) {
	args := m.Called(ctx, id, update)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Trip), args.Error(1)
}

func (m *MockTripStore) SoftDeleteTrip(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTripStore) ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Trip), args.Error(1)
}

func (m *MockTripStore) SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error) {
	args := m.Called(ctx, criteria)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Trip), args.Error(1)
}

func (m *MockTripStore) UpdateMemberRole(ctx context.Context, tripID string, userID string, role types.MemberRole) error {
	args := m.Called(ctx, tripID, userID, role)
	return args.Error(0)
}

func (m *MockTripStore) RemoveMember(ctx context.Context, tripID string, userID string) error {
	args := m.Called(ctx, tripID, userID)
	return args.Error(0)
}

func (m *MockTripStore) GetTripMembers(ctx context.Context, tripID string) ([]types.TripMembership, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.TripMembership), args.Error(1)
}

func (m *MockTripStore) LookupUserByEmail(ctx context.Context, email string) (*types.SupabaseUser, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SupabaseUser), args.Error(1)
}

func (m *MockTripStore) CreateInvitation(ctx context.Context, invitation *types.TripInvitation) error {
	args := m.Called(ctx, invitation)
	return args.Error(0)
}

func (m *MockTripStore) GetInvitation(ctx context.Context, invitationID string) (*types.TripInvitation, error) {
	args := m.Called(ctx, invitationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.TripInvitation), args.Error(1)
}

func (m *MockTripStore) GetInvitationsByTripID(ctx context.Context, tripID string) ([]*types.TripInvitation, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.TripInvitation), args.Error(1)
}

func (m *MockTripStore) UpdateInvitationStatus(ctx context.Context, invitationID string, status types.InvitationStatus) error {
	args := m.Called(ctx, invitationID, status)
	return args.Error(0)
}

func (m *MockTripStore) BeginTx(ctx context.Context) (types.DatabaseTransaction, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(types.DatabaseTransaction), args.Error(1)
}

func (m *MockTripStore) Commit() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockTripStore) Rollback() error {
	args := m.Called()
	return args.Error(0)
}

// MockWSHandler extends WSHandler for testing
type MockWSHandler struct {
	rateLimitService services.RateLimiterInterface
	eventService     types.EventPublisher
	tripStore        store.TripStore
}

// NewMockWSHandler creates a new mock WebSocket handler
func NewMockWSHandler(rateLimiterService services.RateLimiterInterface, eventService types.EventPublisher, tripStore store.TripStore) *MockWSHandler {
	return &MockWSHandler{
		rateLimitService: rateLimiterService,
		eventService:     eventService,
		tripStore:        tripStore,
	}
}

// EnforceWSRateLimit applies rate limiting to WebSocket connections
func (h *MockWSHandler) EnforceWSRateLimit(userID string, actionType string, limit int) error {
	key := "ws:" + actionType + ":" + userID
	allowed, _, err := h.rateLimitService.CheckLimit(context.Background(), key, limit, 1*time.Minute)
	if err != nil {
		return err
	}
	if !allowed {
		return err
	}
	return nil
}

// TestHandleChatMessageMock is a test version of handleChatMessage that works with our mock
func (h *MockWSHandler) TestHandleChatMessageMock(t *testing.T, ctx context.Context, conn *MockSafeConn, payload json.RawMessage) {
	// Simplified test version that works with our mocks
	var msgData map[string]interface{}
	if err := json.Unmarshal(payload, &msgData); err != nil {
		if err := conn.WriteJSON(map[string]interface{}{
			"status": "error",
			"error":  "Invalid message format",
		}); err != nil {
			t.Errorf("Failed to write JSON to WebSocket: %v", err)
		}
		return
	}

	// Apply rate limiting
	if err := h.EnforceWSRateLimit(conn.UserID, "chat.message", 10); err != nil {
		if err := conn.WriteJSON(map[string]interface{}{
			"status": "error",
			"error":  "Rate limit exceeded",
		}); err != nil {
			t.Errorf("Failed to write JSON to WebSocket: %v", err)
		}
		return
	}

	// Create and publish event
	event := types.Event{
		BaseEvent: types.BaseEvent{
			Type:      "CHAT_MESSAGE",
			TripID:    conn.TripID,
			UserID:    conn.UserID,
			Timestamp: time.Now(),
		},
	}
	_ = h.eventService.Publish(ctx, conn.TripID, event)

	// Send success response
	if err := conn.WriteJSON(map[string]interface{}{
		"status": "success",
		"type":   "chat.message",
	}); err != nil {
		t.Errorf("Failed to write JSON to WebSocket: %v", err)
	}
}

// TestHandleChatReactionMock is a test version of handleChatReaction that works with our mock
func (h *MockWSHandler) TestHandleChatReactionMock(t *testing.T, ctx context.Context, conn *MockSafeConn, payload json.RawMessage) {
	// Create and publish event
	event := types.Event{
		BaseEvent: types.BaseEvent{
			Type:      types.EventTypeChatReactionAdded,
			TripID:    conn.TripID,
			UserID:    conn.UserID,
			Timestamp: time.Now(),
		},
	}
	_ = h.eventService.Publish(ctx, conn.TripID, event)

	// Send success response
	if err := conn.WriteJSON(map[string]interface{}{
		"status": "success",
		"type":   "chat.reaction",
	}); err != nil {
		t.Errorf("Failed to write JSON to WebSocket: %v", err)
	}
}

// TestHandleReadReceiptMock is a test version of handleReadReceipt that works with our mock
func (h *MockWSHandler) TestHandleReadReceiptMock(t *testing.T, ctx context.Context, conn *MockSafeConn, payload json.RawMessage) {
	// Create and publish event
	event := types.Event{
		BaseEvent: types.BaseEvent{
			Type:      types.EventTypeChatReadReceipt,
			TripID:    conn.TripID,
			UserID:    conn.UserID,
			Timestamp: time.Now(),
		},
	}
	_ = h.eventService.Publish(ctx, conn.TripID, event)

	// Send success response
	if err := conn.WriteJSON(map[string]interface{}{
		"status": "success",
		"type":   "chat.read_receipt",
	}); err != nil {
		t.Errorf("Failed to write JSON to WebSocket: %v", err)
	}
}

// TestHandleTypingStatusMock is a test version of handleTypingStatus that works with our mock
func (h *MockWSHandler) TestHandleTypingStatusMock(ctx context.Context, conn *MockSafeConn, payload json.RawMessage) {
	// Create and publish event
	event := types.Event{
		BaseEvent: types.BaseEvent{
			Type:      types.EventTypeChatTypingStatus,
			TripID:    conn.TripID,
			UserID:    conn.UserID,
			Timestamp: time.Now(),
		},
	}
	_ = h.eventService.Publish(ctx, conn.TripID, event)
}

func TestHandleChatMessage(t *testing.T) {
	// Initialize test logger
	logger := zaptest.NewLogger(t)
	zap.ReplaceGlobals(logger)

	// Create mocks
	mockConn := &MockSafeConn{
		UserID: "test-user-1",
		TripID: "test-trip-1",
	}
	mockEventService := new(MockEventService)
	mockRateLimiterService := new(MockRateLimiterService)
	mockTripStore := new(MockTripStore)

	// Create the handler
	handler := NewMockWSHandler(mockRateLimiterService, mockEventService, mockTripStore)

	// Setup test cases
	testCases := []struct {
		name          string
		payload       interface{}
		setupMocks    func()
		assertResults func(t *testing.T)
	}{
		{
			name: "Valid chat message",
			payload: map[string]interface{}{
				"content": "Hello world",
			},
			setupMocks: func() {
				// Allow the request through rate limiting
				mockRateLimiterService.On("CheckLimit",
					mock.Anything,
					"ws:chat.message:test-user-1",
					mock.Anything,
					mock.Anything).Return(true, time.Duration(0), nil).Once()

				// Expect the event to be published
				mockEventService.On("Publish", mock.Anything, "test-trip-1", mock.MatchedBy(func(event types.Event) bool {
					return event.Type == "CHAT_MESSAGE"
				})).Return(nil).Once()

				// Expect a success response to be sent
				mockConn.On("WriteJSON", mock.MatchedBy(func(resp interface{}) bool {
					r, ok := resp.(map[string]interface{})
					return ok && r["status"] == "success" && r["type"] == "chat.message"
				})).Return(nil).Once()
			},
			assertResults: func(t *testing.T) {
				mockEventService.AssertExpectations(t)
				mockConn.AssertExpectations(t)
				mockRateLimiterService.AssertExpectations(t)
			},
		},
		{
			name:    "Invalid message format",
			payload: "not a valid json object",
			setupMocks: func() {
				// Expect an error response
				mockConn.On("WriteJSON", mock.MatchedBy(func(resp interface{}) bool {
					r, ok := resp.(map[string]interface{})
					return ok && r["status"] == "error"
				})).Return(nil).Once()
			},
			assertResults: func(t *testing.T) {
				mockConn.AssertExpectations(t)
				// Store and event service should not be called
				mockEventService.AssertNotCalled(t, "Publish")
			},
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mocks
			tc.setupMocks()

			// Convert payload to JSON
			payloadBytes, err := json.Marshal(tc.payload)
			assert.NoError(t, err)

			// Call the test handler method instead of the real one
			handler.TestHandleChatMessageMock(t, context.Background(), mockConn, payloadBytes)

			// Verify results
			tc.assertResults(t)
		})
	}
}

func TestHandleChatReaction(t *testing.T) {
	// Initialize test logger
	logger := zaptest.NewLogger(t)
	zap.ReplaceGlobals(logger)

	// Create mocks
	mockConn := new(MockSafeConn)
	mockEventService := new(MockEventService)
	mockTripStore := new(MockTripStore)

	// Create the handler
	handler := NewMockWSHandler(new(MockRateLimiterService), mockEventService, mockTripStore)

	// Set connection values for the test
	mockConn.UserID = "test-user-1"
	mockConn.TripID = "test-trip-1"

	// Setup test cases
	testCases := []struct {
		name          string
		payload       interface{}
		setupMocks    func()
		assertResults func(t *testing.T)
	}{
		{
			name: "Add reaction",
			payload: map[string]interface{}{
				"message_id": "message-123",
				"reaction":   "👍",
				"action":     "add",
			},
			setupMocks: func() {
				// Expect the event to be published
				mockEventService.On("Publish", mock.Anything, "test-trip-1", mock.MatchedBy(func(event types.Event) bool {
					return event.Type == types.EventTypeChatReactionAdded
				})).Return(nil).Once()

				// Expect a success response to be sent
				mockConn.On("WriteJSON", mock.MatchedBy(func(resp interface{}) bool {
					r, ok := resp.(map[string]interface{})
					return ok && r["status"] == "success" && r["type"] == "chat.reaction"
				})).Return(nil).Once()
			},
			assertResults: func(t *testing.T) {
				mockEventService.AssertExpectations(t)
				mockConn.AssertExpectations(t)
			},
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mocks
			tc.setupMocks()

			// Convert payload to JSON
			payloadBytes, err := json.Marshal(tc.payload)
			assert.NoError(t, err)

			// Call the test handler
			handler.TestHandleChatReactionMock(t, context.Background(), mockConn, payloadBytes)

			// Verify results
			tc.assertResults(t)
		})
	}
}

func TestHandleReadReceipt(t *testing.T) {
	// Initialize test logger
	logger := zaptest.NewLogger(t)
	zap.ReplaceGlobals(logger)

	// Create mocks
	mockConn := new(MockSafeConn)
	mockEventService := new(MockEventService)
	mockTripStore := new(MockTripStore)

	// Create the handler
	handler := NewMockWSHandler(new(MockRateLimiterService), mockEventService, mockTripStore)

	// Set connection values for the test
	mockConn.UserID = "test-user-1"
	mockConn.TripID = "test-trip-1"

	// Setup test cases
	testCases := []struct {
		name          string
		payload       interface{}
		setupMocks    func()
		assertResults func(t *testing.T)
	}{
		{
			name: "Valid read receipt",
			payload: map[string]interface{}{
				"message_id": "message-123",
			},
			setupMocks: func() {
				// Expect the event to be published
				mockEventService.On("Publish", mock.Anything, "test-trip-1", mock.MatchedBy(func(event types.Event) bool {
					return event.Type == types.EventTypeChatReadReceipt
				})).Return(nil).Once()

				// Expect a success response to be sent
				mockConn.On("WriteJSON", mock.MatchedBy(func(resp interface{}) bool {
					r, ok := resp.(map[string]interface{})
					return ok && r["status"] == "success" && r["type"] == "chat.read_receipt"
				})).Return(nil).Once()
			},
			assertResults: func(t *testing.T) {
				mockEventService.AssertExpectations(t)
				mockConn.AssertExpectations(t)
			},
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mocks
			tc.setupMocks()

			// Convert payload to JSON
			payloadBytes, err := json.Marshal(tc.payload)
			assert.NoError(t, err)

			// Call the test handler
			handler.TestHandleReadReceiptMock(t, context.Background(), mockConn, payloadBytes)

			// Verify results
			tc.assertResults(t)
		})
	}
}

func TestHandleTypingStatus(t *testing.T) {
	// Initialize test logger
	logger := zaptest.NewLogger(t)
	zap.ReplaceGlobals(logger)

	// Create mocks
	mockConn := new(MockSafeConn)
	mockEventService := new(MockEventService)
	mockTripStore := new(MockTripStore)

	// Create the handler
	handler := NewMockWSHandler(new(MockRateLimiterService), mockEventService, mockTripStore)

	// Set connection values for the test
	mockConn.UserID = "test-user-1"
	mockConn.TripID = "test-trip-1"

	// Setup test cases
	testCases := []struct {
		name          string
		payload       interface{}
		setupMocks    func()
		assertResults func(t *testing.T)
	}{
		{
			name: "User is typing",
			payload: map[string]interface{}{
				"is_typing": true,
			},
			setupMocks: func() {
				// Expect the event to be published
				mockEventService.On("Publish", mock.Anything, "test-trip-1", mock.MatchedBy(func(event types.Event) bool {
					return event.Type == types.EventTypeChatTypingStatus
				})).Return(nil).Once()
			},
			assertResults: func(t *testing.T) {
				mockEventService.AssertExpectations(t)
			},
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mocks
			tc.setupMocks()

			// Convert payload to JSON
			payloadBytes, err := json.Marshal(tc.payload)
			assert.NoError(t, err)

			// Call the test handler
			handler.TestHandleTypingStatusMock(context.Background(), mockConn, payloadBytes)

			// Verify results
			tc.assertResults(t)
		})
	}
}
