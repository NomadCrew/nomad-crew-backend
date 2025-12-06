package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// Handler handles WebSocket connections.
type Handler struct {
	log               *zap.SugaredLogger
	hub               *Hub
	membershipChecker MembershipChecker
	pingInterval      time.Duration
	writeTimeout      time.Duration
	allowedOrigins    []string
	isDevelopment     bool
}

// NewHandler creates a new WebSocket handler.
func NewHandler(hub *Hub, serverCfg *config.ServerConfig, membershipChecker MembershipChecker) *Handler {
	hubCfg := DefaultHubConfig()
	return &Handler{
		log:               logger.GetLogger().Named("websocket_handler"),
		hub:               hub,
		membershipChecker: membershipChecker,
		pingInterval:      hubCfg.PingInterval,
		writeTimeout:      hubCfg.WriteTimeout,
		allowedOrigins:    serverCfg.AllowedOrigins,
		isDevelopment:     serverCfg.Environment == config.EnvDevelopment,
	}
}

// getAcceptOptions returns WebSocket accept options based on configuration.
// In development, all origins are allowed. In production, only configured origins are allowed.
func (h *Handler) getAcceptOptions() *websocket.AcceptOptions {
	opts := &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionContextTakeover,
	}

	if h.isDevelopment {
		// Allow all origins in development
		opts.InsecureSkipVerify = true
	} else {
		// In production, validate origins
		opts.OriginPatterns = h.allowedOrigins
	}

	return opts
}

// ClientMessage represents a message from the client.
type ClientMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// ServerMessage represents a message to the client.
type ServerMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// HandleWebSocket handles WebSocket upgrade and connection lifecycle.
func (h *Handler) HandleWebSocket(c *gin.Context) {
	// Get user ID from context (set by WSJwtAuth middleware)
	userID, exists := c.Get(string(middleware.UserIDKey))
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userIDStr := userID.(string)

	// Accept WebSocket connection with origin validation
	conn, err := websocket.Accept(c.Writer, c.Request, h.getAcceptOptions())
	if err != nil {
		h.log.Errorw("Failed to accept WebSocket connection",
			"userID", userIDStr,
			"error", err)
		return
	}

	// Create a context that cancels when the connection closes
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Register connection with the hub
	connection, err := h.hub.Register(ctx, userIDStr, conn)
	if err != nil {
		h.log.Errorw("Failed to register WebSocket connection",
			"userID", userIDStr,
			"error", err)
		_ = conn.Close(websocket.StatusInternalError, "registration failed")
		return
	}

	// Ensure cleanup on exit
	defer h.hub.Unregister(userIDStr)

	// Send initial connected message
	if err := h.sendMessage(ctx, conn, ServerMessage{
		Type: "connected",
		Payload: map[string]interface{}{
			"userId":    userIDStr,
			"tripCount": len(connection.TripIDs),
			"trips":     connection.TripIDs,
		},
	}); err != nil {
		h.log.Errorw("Failed to send connected message",
			"userID", userIDStr,
			"error", err)
		return
	}

	h.log.Infow("WebSocket connection established",
		"userID", userIDStr,
		"tripCount", len(connection.TripIDs))

	// Start goroutines for reading, writing, and pinging
	errCh := make(chan error, 3)

	// Read goroutine - handles incoming messages from client
	go func() {
		errCh <- h.readLoop(ctx, conn, userIDStr)
	}()

	// Write goroutine - sends events to client
	go func() {
		errCh <- h.writeLoop(ctx, conn, connection)
	}()

	// Ping goroutine - keeps connection alive
	go func() {
		errCh <- h.pingLoop(ctx, conn)
	}()

	// Wait for any goroutine to finish (usually due to error or close)
	err = <-errCh
	if err != nil && websocket.CloseStatus(err) != websocket.StatusNormalClosure {
		h.log.Warnw("WebSocket connection error",
			"userID", userIDStr,
			"error", err)
	}
}

// readLoop handles incoming messages from the client.
func (h *Handler) readLoop(ctx context.Context, conn *websocket.Conn, userID string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var msg ClientMessage
		err := wsjson.Read(ctx, conn, &msg)
		if err != nil {
			return err
		}

		h.handleClientMessage(ctx, conn, userID, msg)
	}
}

// writeLoop sends events from the hub to the client.
func (h *Handler) writeLoop(ctx context.Context, conn *websocket.Conn, connection *Connection) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-connection.SendChannel():
			if !ok {
				return nil // Channel closed
			}

			msg := ServerMessage{
				Type:    "event",
				Payload: event,
			}

			writeCtx, cancel := context.WithTimeout(ctx, h.writeTimeout)
			err := wsjson.Write(writeCtx, conn, msg)
			cancel()

			if err != nil {
				return err
			}
		}
	}
}

// pingLoop sends periodic pings to keep the connection alive.
func (h *Handler) pingLoop(ctx context.Context, conn *websocket.Conn) error {
	ticker := time.NewTicker(h.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, h.writeTimeout)
			err := conn.Ping(pingCtx)
			cancel()
			if err != nil {
				return err
			}
		}
	}
}

// handleClientMessage processes messages from the client.
func (h *Handler) handleClientMessage(ctx context.Context, conn *websocket.Conn, userID string, msg ClientMessage) {
	switch msg.Type {
	case "ping":
		// Client ping - respond with pong
		_ = h.sendMessage(ctx, conn, ServerMessage{Type: "pong"})

	case "subscribe":
		// Request to subscribe to a specific trip
		var payload struct {
			TripID string `json:"tripId"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil || payload.TripID == "" {
			_ = h.sendMessage(ctx, conn, ServerMessage{
				Type:  "error",
				Error: "Invalid subscribe request: tripId required",
			})
			return
		}

		// Verify user is a member of the trip before allowing subscription
		isMember, err := h.membershipChecker.IsTripMember(ctx, payload.TripID, userID)
		if err != nil {
			h.log.Errorw("Failed to check trip membership",
				"userID", userID,
				"tripID", payload.TripID,
				"error", err)
			_ = h.sendMessage(ctx, conn, ServerMessage{
				Type:  "error",
				Error: "Failed to verify trip membership",
			})
			return
		}
		if !isMember {
			h.log.Warnw("Unauthorized subscribe attempt",
				"userID", userID,
				"tripID", payload.TripID)
			_ = h.sendMessage(ctx, conn, ServerMessage{
				Type:  "error",
				Error: "Not authorized to subscribe to this trip",
			})
			return
		}

		if err := h.hub.AddTripSubscription(ctx, userID, payload.TripID); err != nil {
			_ = h.sendMessage(ctx, conn, ServerMessage{
				Type:  "error",
				Error: "Failed to subscribe to trip",
			})
			return
		}

		_ = h.sendMessage(ctx, conn, ServerMessage{
			Type:    "subscribed",
			Payload: map[string]string{"tripId": payload.TripID},
		})

	case "unsubscribe":
		// Request to unsubscribe from a specific trip
		var payload struct {
			TripID string `json:"tripId"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil || payload.TripID == "" {
			_ = h.sendMessage(ctx, conn, ServerMessage{
				Type:  "error",
				Error: "Invalid unsubscribe request: tripId required",
			})
			return
		}

		if err := h.hub.RemoveTripSubscription(ctx, userID, payload.TripID); err != nil {
			_ = h.sendMessage(ctx, conn, ServerMessage{
				Type:  "error",
				Error: "Failed to unsubscribe from trip",
			})
			return
		}

		_ = h.sendMessage(ctx, conn, ServerMessage{
			Type:    "unsubscribed",
			Payload: map[string]string{"tripId": payload.TripID},
		})

	default:
		h.log.Debugw("Unknown message type from client",
			"userID", userID,
			"type", msg.Type)
	}
}

// sendMessage sends a message to the client.
func (h *Handler) sendMessage(ctx context.Context, conn *websocket.Conn, msg ServerMessage) error {
	writeCtx, cancel := context.WithTimeout(ctx, h.writeTimeout)
	defer cancel()
	return wsjson.Write(writeCtx, conn, msg)
}

// ServeHTTP allows the handler to be used directly with http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey)
	if userID == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userIDStr := userID.(string)

	conn, err := websocket.Accept(w, r, h.getAcceptOptions())
	if err != nil {
		h.log.Errorw("Failed to accept WebSocket connection",
			"userID", userIDStr,
			"error", err)
		return
	}

	ctx := r.Context()
	connection, err := h.hub.Register(ctx, userIDStr, conn)
	if err != nil {
		h.log.Errorw("Failed to register WebSocket connection",
			"userID", userIDStr,
			"error", err)
		_ = conn.Close(websocket.StatusInternalError, "registration failed")
		return
	}

	defer h.hub.Unregister(userIDStr)

	// Send connected message
	_ = h.sendMessage(ctx, conn, ServerMessage{
		Type: "connected",
		Payload: map[string]interface{}{
			"userId":    userIDStr,
			"tripCount": len(connection.TripIDs),
			"trips":     connection.TripIDs,
		},
	})

	// Run the connection loops
	errCh := make(chan error, 3)
	go func() { errCh <- h.readLoop(ctx, conn, userIDStr) }()
	go func() { errCh <- h.writeLoop(ctx, conn, connection) }()
	go func() { errCh <- h.pingLoop(ctx, conn) }()

	<-errCh
}

// GetHub returns the hub for testing or advanced usage.
func (h *Handler) GetHub() *Hub {
	return h.hub
}

// Event type constants for client messages
const (
	MessageTypePing        = "ping"
	MessageTypePong        = "pong"
	MessageTypeSubscribe   = "subscribe"
	MessageTypeUnsubscribe = "unsubscribe"
	MessageTypeEvent       = "event"
	MessageTypeConnected   = "connected"
	MessageTypeError       = "error"
)
