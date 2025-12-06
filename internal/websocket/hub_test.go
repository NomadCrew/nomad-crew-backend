package websocket

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"nhooyr.io/websocket"
)

// MockEventSubscriber implements EventSubscriber for testing
type MockEventSubscriber struct {
	mock.Mock
	subscriptions map[string]chan types.Event
	mu            sync.Mutex
}

func NewMockEventSubscriber() *MockEventSubscriber {
	return &MockEventSubscriber{
		subscriptions: make(map[string]chan types.Event),
	}
}

func (m *MockEventSubscriber) Subscribe(ctx context.Context, tripID string, userID string, filters ...types.EventType) (<-chan types.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	args := m.Called(ctx, tripID, userID, filters)
	if args.Error(1) != nil {
		return nil, args.Error(1)
	}

	ch := make(chan types.Event, 10)
	key := tripID + ":" + userID
	m.subscriptions[key] = ch
	return ch, nil
}

func (m *MockEventSubscriber) Unsubscribe(ctx context.Context, tripID string, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	args := m.Called(ctx, tripID, userID)

	key := tripID + ":" + userID
	if ch, ok := m.subscriptions[key]; ok {
		close(ch)
		delete(m.subscriptions, key)
	}

	return args.Error(0)
}

func (m *MockEventSubscriber) SendEvent(tripID, userID string, event types.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := tripID + ":" + userID
	if ch, ok := m.subscriptions[key]; ok {
		select {
		case ch <- event:
		default:
		}
	}
}

// MockTripLister implements TripLister for testing
type MockTripLister struct {
	mock.Mock
}

func (m *MockTripLister) ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Trip), args.Error(1)
}

func TestHub_NewHub(t *testing.T) {
	eventSub := NewMockEventSubscriber()
	tripLister := &MockTripLister{}

	hub := NewHub(eventSub, tripLister)

	assert.NotNil(t, hub)
	assert.NotNil(t, hub.connections)
	assert.Equal(t, 0, hub.GetConnectionCount())
}

func TestHub_GetConnectedUsers_Empty(t *testing.T) {
	eventSub := NewMockEventSubscriber()
	tripLister := &MockTripLister{}

	hub := NewHub(eventSub, tripLister)

	users := hub.GetConnectedUsers()
	assert.Empty(t, users)
}

func TestHub_GetConnectionCount(t *testing.T) {
	eventSub := NewMockEventSubscriber()
	tripLister := &MockTripLister{}

	hub := NewHub(eventSub, tripLister)

	assert.Equal(t, 0, hub.GetConnectionCount())
}

func TestHub_GetConnection_NotFound(t *testing.T) {
	eventSub := NewMockEventSubscriber()
	tripLister := &MockTripLister{}

	hub := NewHub(eventSub, tripLister)

	conn, ok := hub.GetConnection("non-existent-user")
	assert.False(t, ok)
	assert.Nil(t, conn)
}

func TestHub_Shutdown(t *testing.T) {
	eventSub := NewMockEventSubscriber()
	tripLister := &MockTripLister{}

	hub := NewHub(eventSub, tripLister)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := hub.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestHub_BroadcastToUser_NotConnected(t *testing.T) {
	eventSub := NewMockEventSubscriber()
	tripLister := &MockTripLister{}

	hub := NewHub(eventSub, tripLister)

	event := types.Event{
		BaseEvent: types.BaseEvent{
			ID:        "test-event-1",
			Type:      types.EventTypeTripCreated,
			TripID:    "trip-1",
			Timestamp: time.Now(),
		},
	}

	// Should not error when user is not connected
	err := hub.BroadcastToUser("non-existent-user", event)
	assert.NoError(t, err)
}

func TestHub_AddTripSubscription_NotConnected(t *testing.T) {
	eventSub := NewMockEventSubscriber()
	tripLister := &MockTripLister{}

	hub := NewHub(eventSub, tripLister)

	ctx := context.Background()

	// Should return nil error when user is not connected (no-op)
	err := hub.AddTripSubscription(ctx, "non-existent-user", "trip-1")
	assert.NoError(t, err)
}

func TestHub_RemoveTripSubscription_NotConnected(t *testing.T) {
	eventSub := NewMockEventSubscriber()
	tripLister := &MockTripLister{}

	hub := NewHub(eventSub, tripLister)

	ctx := context.Background()

	// Should return nil error when user is not connected (no-op)
	err := hub.RemoveTripSubscription(ctx, "non-existent-user", "trip-1")
	assert.NoError(t, err)
}

func TestConnection_SendChannel(t *testing.T) {
	conn := &Connection{
		UserID:      "user-1",
		TripIDs:     []string{"trip-1"},
		cancelFuncs: make(map[string]context.CancelFunc),
		sendCh:      make(chan types.Event, 256),
		closed:      false,
	}

	ch := conn.SendChannel()
	assert.NotNil(t, ch)

	// Test that we can receive from the channel
	event := types.Event{
		BaseEvent: types.BaseEvent{
			ID:     "test-1",
			Type:   types.EventTypeTripCreated,
			TripID: "trip-1",
		},
	}

	// Send an event
	conn.sendCh <- event

	// Should receive it
	select {
	case received := <-ch:
		assert.Equal(t, event.ID, received.ID)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestConnection_IsClosed(t *testing.T) {
	conn := &Connection{
		UserID: "user-1",
		closed: false,
	}

	assert.False(t, conn.IsClosed())

	conn.closed = true
	assert.True(t, conn.IsClosed())
}

func TestDefaultHubConfig(t *testing.T) {
	config := DefaultHubConfig()

	assert.Equal(t, 30*time.Second, config.PingInterval)
	assert.Equal(t, 10*time.Second, config.WriteTimeout)
	assert.Equal(t, 60*time.Second, config.ReadTimeout)
	assert.Equal(t, 256, config.SendBuffer)
}

// TestHub_Unregister_NotExists tests unregistering non-existent user
func TestHub_Unregister_NotExists(t *testing.T) {
	eventSub := NewMockEventSubscriber()
	tripLister := &MockTripLister{}

	hub := NewHub(eventSub, tripLister)

	// Should not panic or error
	hub.Unregister("non-existent-user")
}

// MockWebSocketConn is a mock for testing purposes
type MockWebSocketConn struct {
	closed bool
	mu     sync.Mutex
}

func (m *MockWebSocketConn) Close(code websocket.StatusCode, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *MockWebSocketConn) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

// MockMembershipChecker implements MembershipChecker for testing
type MockMembershipChecker struct {
	mock.Mock
}

func (m *MockMembershipChecker) IsTripMember(ctx context.Context, tripID, userID string) (bool, error) {
	args := m.Called(ctx, tripID, userID)
	return args.Bool(0), args.Error(1)
}

func TestMembershipChecker_Interface(t *testing.T) {
	// Verify MockMembershipChecker implements MembershipChecker
	var _ MembershipChecker = (*MockMembershipChecker)(nil)
}

func TestMembershipChecker_IsTripMember(t *testing.T) {
	checker := &MockMembershipChecker{}
	ctx := context.Background()

	// Test case: user is a member
	checker.On("IsTripMember", ctx, "trip-1", "user-1").Return(true, nil)
	isMember, err := checker.IsTripMember(ctx, "trip-1", "user-1")
	assert.NoError(t, err)
	assert.True(t, isMember)

	// Test case: user is not a member
	checker.On("IsTripMember", ctx, "trip-2", "user-1").Return(false, nil)
	isMember, err = checker.IsTripMember(ctx, "trip-2", "user-1")
	assert.NoError(t, err)
	assert.False(t, isMember)

	checker.AssertExpectations(t)
}
