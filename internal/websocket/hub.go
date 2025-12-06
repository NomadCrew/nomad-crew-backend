package websocket

import (
	"context"
	"sync"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"go.uber.org/zap"
	"nhooyr.io/websocket"
)

// Hub manages WebSocket connections for all users.
// Each user has a single connection through which they receive
// all events for their trips.
type Hub struct {
	log           *zap.SugaredLogger
	eventService  EventSubscriber
	tripLister    TripLister
	connections   map[string]*Connection // userID -> connection
	mu            sync.RWMutex
	shutdownCh    chan struct{}
	shutdownOnce  sync.Once
	pingInterval  time.Duration
	writeTimeout  time.Duration
	readTimeout   time.Duration
}

// EventSubscriber is the interface for subscribing to events.
// This allows decoupling from the concrete events.Service.
type EventSubscriber interface {
	Subscribe(ctx context.Context, tripID string, userID string, filters ...types.EventType) (<-chan types.Event, error)
	Unsubscribe(ctx context.Context, tripID string, userID string) error
}

// TripLister is the interface for listing user trips.
// This is a subset of store.TripStore to allow easier testing.
type TripLister interface {
	ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error)
}

// MembershipChecker is the interface for checking trip membership.
// Used to authorize dynamic trip subscriptions.
type MembershipChecker interface {
	IsTripMember(ctx context.Context, tripID, userID string) (bool, error)
}

// Connection represents a single WebSocket connection for a user.
type Connection struct {
	UserID      string
	Conn        *websocket.Conn
	TripIDs     []string           // trips this user is subscribed to
	cancelFuncs map[string]context.CancelFunc // tripID -> cancel func for subscription
	sendCh      chan types.Event   // buffered channel for outbound events
	mu          sync.Mutex
	closed      bool
}

// HubConfig contains configuration options for the Hub.
type HubConfig struct {
	PingInterval time.Duration
	WriteTimeout time.Duration
	ReadTimeout  time.Duration
	SendBuffer   int
}

// DefaultHubConfig returns sensible defaults for Hub configuration.
func DefaultHubConfig() HubConfig {
	return HubConfig{
		PingInterval: 30 * time.Second,
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  60 * time.Second,
		SendBuffer:   256,
	}
}

// NewHub creates a new WebSocket hub.
func NewHub(eventService EventSubscriber, tripLister TripLister, cfg ...HubConfig) *Hub {
	config := DefaultHubConfig()
	if len(cfg) > 0 {
		config = cfg[0]
	}

	return &Hub{
		log:          logger.GetLogger().Named("websocket_hub"),
		eventService: eventService,
		tripLister:   tripLister,
		connections:  make(map[string]*Connection),
		shutdownCh:   make(chan struct{}),
		pingInterval: config.PingInterval,
		writeTimeout: config.WriteTimeout,
		readTimeout:  config.ReadTimeout,
	}
}

// Register adds a new WebSocket connection for a user.
// If the user already has a connection, it is closed and replaced.
func (h *Hub) Register(ctx context.Context, userID string, conn *websocket.Conn) (*Connection, error) {
	// Get user's trips first (outside of lock to avoid holding lock during DB call)
	trips, err := h.tripLister.ListUserTrips(ctx, userID)
	if err != nil {
		h.log.Errorw("Failed to get user trips for WebSocket registration",
			"userID", userID,
			"error", err)
		return nil, err
	}

	tripIDs := make([]string, len(trips))
	for i, trip := range trips {
		tripIDs[i] = trip.ID
	}

	// Now lock and handle connection management atomically
	h.mu.Lock()
	// Capture existing connection for cleanup after unlock
	existing := h.connections[userID]

	connection := &Connection{
		UserID:      userID,
		Conn:        conn,
		TripIDs:     tripIDs,
		cancelFuncs: make(map[string]context.CancelFunc),
		sendCh:      make(chan types.Event, 256),
		closed:      false,
	}

	h.connections[userID] = connection
	h.mu.Unlock()

	// Close existing connection outside of lock (if any)
	if existing != nil {
		h.closeConnection(existing, "replaced by new connection")
	}

	// Subscribe to events for each trip
	for _, tripID := range tripIDs {
		if err := h.subscribeToTrip(ctx, connection, tripID); err != nil {
			h.log.Warnw("Failed to subscribe to trip events",
				"userID", userID,
				"tripID", tripID,
				"error", err)
			// Continue with other trips even if one fails
		}
	}

	h.log.Infow("WebSocket connection registered",
		"userID", userID,
		"tripCount", len(tripIDs))

	return connection, nil
}

// subscribeToTrip subscribes a connection to events for a specific trip.
func (h *Hub) subscribeToTrip(ctx context.Context, conn *Connection, tripID string) error {
	subCtx, cancel := context.WithCancel(ctx)

	conn.mu.Lock()
	conn.cancelFuncs[tripID] = cancel
	conn.mu.Unlock()

	eventCh, err := h.eventService.Subscribe(subCtx, tripID, conn.UserID)
	if err != nil {
		cancel()
		return err
	}

	// Forward events from this trip to the connection's send channel
	go func() {
		defer cancel()
		for {
			select {
			case <-subCtx.Done():
				return
			case event, ok := <-eventCh:
				if !ok {
					return
				}
				select {
				case conn.sendCh <- event:
				default:
					h.log.Warnw("Connection send buffer full, dropping event",
						"userID", conn.UserID,
						"tripID", tripID,
						"eventType", event.Type)
				}
			}
		}
	}()

	return nil
}

// Unregister removes a user's WebSocket connection.
func (h *Hub) Unregister(userID string) {
	h.mu.Lock()
	conn, ok := h.connections[userID]
	if !ok {
		h.mu.Unlock()
		return
	}
	delete(h.connections, userID)
	h.mu.Unlock()

	h.closeConnection(conn, "unregistered")
}

// closeConnection closes a connection and cleans up resources.
func (h *Hub) closeConnection(conn *Connection, reason string) {
	conn.mu.Lock()
	if conn.closed {
		conn.mu.Unlock()
		return
	}
	conn.closed = true

	// Cancel all trip subscriptions
	for tripID, cancel := range conn.cancelFuncs {
		cancel()
		_ = h.eventService.Unsubscribe(context.Background(), tripID, conn.UserID)
	}
	conn.cancelFuncs = nil
	conn.mu.Unlock()

	// Close the WebSocket connection
	_ = conn.Conn.Close(websocket.StatusNormalClosure, reason)

	// Close the send channel
	close(conn.sendCh)

	h.log.Infow("WebSocket connection closed",
		"userID", conn.UserID,
		"reason", reason)
}

// AddTripSubscription adds a new trip subscription for a connected user.
// This is used when a user joins a new trip while connected.
func (h *Hub) AddTripSubscription(ctx context.Context, userID string, tripID string) error {
	h.mu.RLock()
	conn, ok := h.connections[userID]
	h.mu.RUnlock()

	if !ok {
		// User not connected, nothing to do
		return nil
	}

	conn.mu.Lock()
	// Check if already subscribed
	for _, id := range conn.TripIDs {
		if id == tripID {
			conn.mu.Unlock()
			return nil
		}
	}
	conn.TripIDs = append(conn.TripIDs, tripID)
	conn.mu.Unlock()

	return h.subscribeToTrip(ctx, conn, tripID)
}

// RemoveTripSubscription removes a trip subscription for a connected user.
// This is used when a user leaves a trip while connected.
func (h *Hub) RemoveTripSubscription(ctx context.Context, userID string, tripID string) error {
	h.mu.RLock()
	conn, ok := h.connections[userID]
	h.mu.RUnlock()

	if !ok {
		return nil
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	// Remove from trip list
	for i, id := range conn.TripIDs {
		if id == tripID {
			conn.TripIDs = append(conn.TripIDs[:i], conn.TripIDs[i+1:]...)
			break
		}
	}

	// Cancel subscription
	if cancel, ok := conn.cancelFuncs[tripID]; ok {
		cancel()
		delete(conn.cancelFuncs, tripID)
	}

	return h.eventService.Unsubscribe(ctx, tripID, userID)
}

// BroadcastToUser sends an event directly to a user's connection.
// This is used for user-specific events that aren't tied to a trip.
func (h *Hub) BroadcastToUser(userID string, event types.Event) error {
	h.mu.RLock()
	conn, ok := h.connections[userID]
	h.mu.RUnlock()

	if !ok {
		return nil // User not connected
	}

	conn.mu.Lock()
	if conn.closed {
		conn.mu.Unlock()
		return nil
	}
	conn.mu.Unlock()

	select {
	case conn.sendCh <- event:
		return nil
	default:
		h.log.Warnw("Failed to broadcast to user, buffer full",
			"userID", userID,
			"eventType", event.Type)
		return nil
	}
}

// GetConnection returns the connection for a user, if connected.
func (h *Hub) GetConnection(userID string) (*Connection, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conn, ok := h.connections[userID]
	return conn, ok
}

// GetConnectedUsers returns a list of connected user IDs.
func (h *Hub) GetConnectedUsers() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	users := make([]string, 0, len(h.connections))
	for userID := range h.connections {
		users = append(users, userID)
	}
	return users
}

// GetConnectionCount returns the number of active connections.
func (h *Hub) GetConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}

// Shutdown gracefully shuts down the hub.
func (h *Hub) Shutdown(ctx context.Context) error {
	h.shutdownOnce.Do(func() {
		close(h.shutdownCh)

		h.mu.Lock()
		connections := make([]*Connection, 0, len(h.connections))
		for _, conn := range h.connections {
			connections = append(connections, conn)
		}
		h.connections = make(map[string]*Connection)
		h.mu.Unlock()

		for _, conn := range connections {
			h.closeConnection(conn, "server shutdown")
		}
	})

	h.log.Info("WebSocket hub shutdown complete")
	return nil
}

// SendChannel returns the send channel for a connection.
// This is used by the handler to write events to the WebSocket.
func (c *Connection) SendChannel() <-chan types.Event {
	return c.sendCh
}

// IsClosed returns whether the connection is closed.
func (c *Connection) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}
