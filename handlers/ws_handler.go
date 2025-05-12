package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/services"
	"github.com/NomadCrew/nomad-crew-backend/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// WSHandler handles WebSocket connections
type WSHandler struct {
	rateLimitService services.RateLimiterInterface
	eventService     types.EventPublisher
	tripStore        store.TripStore // Add TripStore for membership verification
}

// NewWSHandler creates a new WebSocket handler
func NewWSHandler(rateLimitService services.RateLimiterInterface, eventService types.EventPublisher, tripStore store.TripStore) *WSHandler {
	return &WSHandler{
		rateLimitService: rateLimitService,
		eventService:     eventService,
		tripStore:        tripStore,
	}
}

// EnforceWSRateLimit applies rate limiting to WebSocket connections
func (h *WSHandler) EnforceWSRateLimit(userID string, actionType string, limit int) error {
	key := fmt.Sprintf("ws:%s:%s", actionType, userID)
	allowed, retryAfter, err := h.rateLimitService.CheckLimit(context.Background(), key, limit, 1*time.Minute)
	if err != nil {
		return err
	}
	if !allowed {
		return fmt.Errorf("rate limit exceeded, retry after %v", retryAfter)
	}
	return nil
}

// HandleWebSocketConnection godoc
// @Summary Establish WebSocket connection
// @Description Upgrades HTTP connection to WebSocket for real-time communication
// @Tags websocket
// @Accept json
// @Produce json
// @Success 101 "Switching Protocols to WebSocket"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 429 {object} types.ErrorResponse "Too Many Requests - Rate limit exceeded"
// @Router /ws [get]
// @Security BearerAuth
func (h *WSHandler) HandleWebSocketConnection(c *gin.Context) {
	log := zap.L()

	// Get user ID from context (set by WSJwtAuth middleware)
	userID, exists := c.Get(string(middleware.UserIDKey))
	if !exists || userID == "" {
		log.Warn("WebSocket connection attempt without authenticated user",
			zap.String("path", c.Request.URL.Path),
			zap.String("ip", c.ClientIP()))
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	// Apply rate limiting
	if err := h.EnforceWSRateLimit(userID.(string), "connect", 5); err != nil {
		log.Warn("WebSocket rate limit exceeded",
			zap.String("userID", userID.(string)),
			zap.Error(err))
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"error":       "Too many connections",
			"retry_after": 60, // seconds
		})
		return
	}

	// Configure WebSocket upgrader with enhanced settings
	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096, // Increased to handle larger messages
		WriteBufferSize: 4096, // Increased for better performance
		CheckOrigin: func(r *http.Request) bool {
			// Allow connections from the same origin or trusted origins
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true // Some clients don't send Origin header
			}

			// Allow standard development and production origins
			if strings.Contains(origin, "localhost") ||
				strings.Contains(origin, "127.0.0.1") ||
				strings.Contains(origin, "nomadcrew.uk") {
				return true
			}

			// Log suspicious origins but still allow them for now
			// This helps with debugging without breaking functionality
			log.Warn("WebSocket connection from unexpected origin",
				zap.String("origin", origin),
				zap.String("userID", userID.(string)),
				zap.String("ip", c.ClientIP()))

			return true // Accept all origins for now, can be restricted later
		},
		// Explicitly set handshake timeout for stability
		HandshakeTimeout: 10 * time.Second,
	}

	// Upgrade HTTP connection to WebSocket with additional response headers
	responseHeader := http.Header{}
	responseHeader.Add("X-Connection-ID", uuid.NewString())

	conn, err := upgrader.Upgrade(c.Writer, c.Request, responseHeader)
	if err != nil {
		log.Error("WebSocket upgrade failed",
			zap.String("userID", userID.(string)),
			zap.String("ip", c.ClientIP()),
			zap.Error(err))
		return
	}

	// Set more robust connection parameters
	conn.SetReadLimit(1024 * 1024) // 1MB max message size to prevent abuse

	// Configure a more robust WebSocket SafeConn with improved defaults
	wsConfig := middleware.WSConfig{
		WriteBufferSize: 32,               // Smaller buffer since we don't need to queue many messages
		ReadBufferSize:  32,               // Reasonable buffer size for incoming messages
		MaxMessageSize:  1024 * 1024,      // 1MB max message size
		WriteWait:       10 * time.Second, // More generous timeout for writes
		PongWait:        60 * time.Second, // Longer timeout for pongs from mobile clients
		PingPeriod:      30 * time.Second, // More frequent pings for better connection stability
	}

	// Create a safe connection wrapper with the enhanced config
	safeConn := middleware.NewSafeConn(conn, nil, wsConfig)
	if safeConn == nil {
		log.Error("Failed to create safe WebSocket connection",
			zap.String("userID", userID.(string)),
			zap.String("ip", c.ClientIP()))
		conn.Close()
		return
	}

	// Set user metadata on connection
	safeConn.UserID = userID.(string)

	// Set connection in context for downstream handlers
	c.Set("wsConnection", safeConn)

	// Log successful connection
	log.Info("WebSocket connection established",
		zap.String("userID", userID.(string)),
		zap.String("remoteAddr", conn.RemoteAddr().String()),
		zap.String("path", c.Request.URL.Path))

	// Continue to the next handler which will use the WebSocket connection
	c.Next()

	// When c.Next() returns, ensure the connection is closed if not handled elsewhere
	wsConnInterface, exists := c.Get("wsConnection")
	if exists {
		if safeConn, ok := wsConnInterface.(*middleware.SafeConn); ok && safeConn != nil {
			// Optional: Check if the connection is still open before closing
			if middleware.ConnIsClosed(safeConn) {
				log.Debug("Connection already closed after handler",
					zap.String("userID", userID.(string)))
			} else {
				log.Debug("Closing connection after handler completed",
					zap.String("userID", userID.(string)))
				if err := safeConn.Close(); err != nil {
					log.Warn("Error closing WebSocket connection after handler", zap.Error(err), zap.String("userID", userID.(string)))
				}
			}
		}
	}
}

// HandleChatWebSocketConnection godoc
// @Summary Establish trip chat WebSocket connection
// @Description Upgrades HTTP connection to WebSocket for real-time trip chat
// @Tags websocket,chat
// @Accept json
// @Produce json
// @Param tripID path string true "Trip ID"
// @Success 101 "Switching Protocols to WebSocket"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User not a member of this trip"
// @Failure 429 {object} types.ErrorResponse "Too Many Requests - Rate limit exceeded"
// @Router /trips/{tripID}/ws [get]
// @Security BearerAuth
func (h *WSHandler) HandleChatWebSocketConnection(c *gin.Context) {
	log := zap.L()

	// Get trip ID from request before upgrading connection
	tripID := c.Param("tripID")
	if tripID == "" {
		log.Warn("HandleChatWebSocketConnection: Trip ID is missing")
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Missing trip ID"})
		return
	}

	// Store trip ID in the context for access after upgrading
	c.Set("tripID", tripID)

	// First use the base handler to establish a WebSocket connection
	h.HandleWebSocketConnection(c)

	// Get connection from context
	wsConnInterface, exists := c.Get("wsConnection")
	if !exists {
		log.Error("WebSocket connection not found in context")
		return
	}

	safeConn, ok := wsConnInterface.(*middleware.SafeConn)
	if !ok || safeConn == nil {
		log.Error("Invalid WebSocket connection type in context")
		return
	}

	// Get trip ID that we stored above
	tripIDInterface, exists := c.Get("tripID")
	if !exists {
		log.Error("Missing tripID in context after WebSocket upgrade")
		h.sendErrorResponse(safeConn, "Server error: missing trip ID")
		if err := safeConn.Close(); err != nil {
			log.Warn("Error closing WebSocket connection", zap.Error(err), zap.String("userID", safeConn.UserID), zap.String("reason", "missing tripID"))
		}
		return
	}

	tripID, ok = tripIDInterface.(string)
	if !ok {
		log.Error("Invalid tripID type in context")
		h.sendErrorResponse(safeConn, "Server error: invalid trip ID")
		if err := safeConn.Close(); err != nil {
			log.Warn("Error closing WebSocket connection", zap.Error(err), zap.String("userID", safeConn.UserID), zap.String("reason", "invalid tripID format"))
		}
		return
	}

	// Set trip ID on connection
	safeConn.TripID = tripID

	// Create a context with timeout for database queries
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Verify user is a member of the trip
	role, err := h.tripStore.GetUserRole(ctx, tripID, safeConn.UserID)
	if err != nil {
		log.Error("HandleChatWebSocketConnection: Error checking trip membership", zap.String("tripID", tripID), zap.String("userID", safeConn.UserID), zap.Error(err))
		h.sendErrorResponse(safeConn, "Server error checking trip membership")
		if err := safeConn.Close(); err != nil {
			log.Warn("Error closing WebSocket connection", zap.Error(err), zap.String("userID", safeConn.UserID), zap.String("reason", "error checking membership"))
		}
		return
	}

	// Check if the user has a valid role (not NONE)
	if role == types.MemberRoleNone {
		log.Warn("HandleChatWebSocketConnection: User not a member of the trip", zap.String("tripID", tripID), zap.String("userID", safeConn.UserID))
		h.sendErrorResponse(safeConn, "Not a member of this trip")
		if err := safeConn.Close(); err != nil {
			log.Warn("Error closing WebSocket connection", zap.Error(err), zap.String("userID", safeConn.UserID), zap.String("reason", "not a member"))
		}
		return
	}

	// Send a welcome message to confirm successful connection
	welcomeMsg, _ := json.Marshal(map[string]interface{}{
		"type": "welcome",
		"data": map[string]interface{}{
			"message": "Connected to trip chat",
			"tripID":  tripID,
			"role":    string(role),
		},
	})

	if err := safeConn.WriteMessage(websocket.TextMessage, welcomeMsg); err != nil {
		log.Warn("Failed to send welcome message",
			zap.String("userID", safeConn.UserID),
			zap.String("tripID", tripID),
			zap.Error(err))
	}

	log.Info("Chat WebSocket connection established",
		zap.String("userID", safeConn.UserID),
		zap.String("tripID", tripID),
		zap.String("role", string(role)))

	// Initialize chat message event handling
	client := middleware.GetWSClient(safeConn)
	if client == nil {
		log.Error("Failed to get WebSocket client",
			zap.String("userID", safeConn.UserID),
			zap.String("tripID", tripID))
		h.sendErrorResponse(safeConn, "Failed to initialize chat session")
		if err := safeConn.Close(); err != nil {
			log.Warn("Error closing WebSocket connection", zap.Error(err), zap.String("userID", safeConn.UserID), zap.String("reason", "failed to init session"))
		}
		return
	}

	// Handle WebSocket messages in a separate goroutine
	// This ensures the HTTP handler can return while the connection remains active
	go func() {
		// Create a new context for the goroutine
		goCtx := context.Background()

		// Handle chat events through the event service
		client.HandleChatMessages(goCtx, h.eventService, tripID)

		// Process incoming messages
		h.handleWebSocketMessages(goCtx, safeConn)

		// Ensure connection is closed when done
		if !middleware.ConnIsClosed(safeConn) {
			if err := safeConn.Close(); err != nil {
				log.Warn("Error closing chat WebSocket connection from main handler loop", zap.Error(err), zap.String("userID", safeConn.UserID))
			}
		}

		log.Info("Chat WebSocket handler completed",
			zap.String("userID", safeConn.UserID),
			zap.String("tripID", tripID))
	}()
}

// handleWebSocketMessages processes incoming WebSocket messages
func (h *WSHandler) handleWebSocketMessages(ctx context.Context, conn *middleware.SafeConn) {
	// Create a done channel for signaling when message handling is complete
	done := make(chan struct{})
	defer close(done)

	readCh := conn.ReadChannel()
	if readCh == nil {
		zap.L().Error("Read channel is nil in handleWebSocketMessages",
			zap.String("userID", conn.UserID),
			zap.String("tripID", conn.TripID))
		return
	}

	// Set up message processing
	for {
		select {
		case <-ctx.Done():
			// Context canceled, exit the loop
			return
		case <-conn.DoneChannel():
			// Connection closed, exit the loop
			return
		case message, ok := <-readCh:
			if !ok {
				// Channel closed, exit the loop
				return
			}

			// Parse the message
			var request struct {
				Type    string          `json:"type"`
				Payload json.RawMessage `json:"payload"`
			}

			if err := json.Unmarshal(message, &request); err != nil {
				zap.L().Error("Failed to parse WebSocket message",
					zap.String("userID", conn.UserID),
					zap.String("tripID", conn.TripID),
					zap.Error(err))

				// Send error response
				errorResponse := map[string]string{"error": "Invalid message format"}
				responseBytes, _ := json.Marshal(errorResponse)
				if err := conn.WriteMessage(websocket.TextMessage, responseBytes); err != nil {
					zap.L().Error("Failed to send error response",
						zap.String("userID", conn.UserID),
						zap.Error(err))
				}
				continue
			}

			// Process the request based on its type
			switch request.Type {
			case "chat_message":
				h.handleChatMessage(ctx, conn, request.Payload)
			case "chat_reaction":
				h.handleChatReaction(ctx, conn, request.Payload)
			case "read_receipt":
				h.handleReadReceipt(ctx, conn, request.Payload)
			case "typing":
				h.handleTypingStatus(ctx, conn, request.Payload)
			case "ping":
				// Handle ping message (keep-alive)
				h.handlePing(ctx, conn)
			default:
				zap.L().Warn("Unknown WebSocket message type",
					zap.String("type", request.Type),
					zap.String("userID", conn.UserID))

				// Send error response
				errorResponse := map[string]string{"error": "Unknown message type"}
				responseBytes, _ := json.Marshal(errorResponse)
				if err := conn.WriteMessage(websocket.TextMessage, responseBytes); err != nil {
					zap.L().Error("Failed to send error response",
						zap.String("userID", conn.UserID),
						zap.Error(err))
				}
			}
		}
	}
}

// handleChatMessage processes incoming chat messages from WebSocket clients
func (h *WSHandler) handleChatMessage(ctx context.Context, conn *middleware.SafeConn, payload json.RawMessage) {
	if conn == nil {
		zap.L().Error("Received nil connection in handleChatMessage")
		return
	}

	// Parse the message
	var msgRequest struct {
		Text      string `json:"text"`
		GroupID   string `json:"group_id,omitempty"`
		ReplyToID string `json:"reply_to_id,omitempty"`
	}

	if err := json.Unmarshal(payload, &msgRequest); err != nil {
		zap.L().Error("Failed to parse chat message",
			zap.String("userID", conn.UserID),
			zap.String("tripID", conn.TripID),
			zap.Error(err))
		h.sendErrorResponse(conn, "Invalid message format")
		return
	}

	// Validate message
	if strings.TrimSpace(msgRequest.Text) == "" {
		zap.L().Warn("Received empty chat message",
			zap.String("userID", conn.UserID),
			zap.String("tripID", conn.TripID))
		h.sendErrorResponse(conn, "Message text cannot be empty")
		return
	}

	// Rate limit message sending
	if err := h.EnforceWSRateLimit(conn.UserID, "chat_message", 10); err != nil {
		zap.L().Warn("Chat message rate limit exceeded",
			zap.String("userID", conn.UserID),
			zap.Error(err))
		h.sendErrorResponse(conn, "Message rate limit exceeded")
		return
	}

	// Verify user is a member of the trip
	role, err := h.tripStore.GetUserRole(ctx, conn.TripID, conn.UserID)
	if err != nil || role == "" {
		zap.L().Error("Failed to verify trip membership for message",
			zap.String("userID", conn.UserID),
			zap.String("tripID", conn.TripID),
			zap.Error(err))
		h.sendErrorResponse(conn, "Not authorized to send messages in this trip")
		return
	}

	// Generate a unique message ID
	messageID := uuid.New().String()

	// Create message payload
	messagePayload := map[string]interface{}{
		"id":         messageID,
		"text":       msgRequest.Text,
		"trip_id":    conn.TripID,
		"group_id":   msgRequest.GroupID,
		"sender_id":  conn.UserID,
		"created_at": time.Now().Format(time.RFC3339),
		"updated_at": time.Now().Format(time.RFC3339),
	}

	// Add reply information if present
	if msgRequest.ReplyToID != "" {
		messagePayload["reply_to_id"] = msgRequest.ReplyToID
	}

	// Publish the message as an event
	err = h.eventService.Publish(ctx, conn.TripID, types.Event{
		BaseEvent: types.BaseEvent{
			Type:      types.EventTypeChatMessageSent,
			ID:        messageID,
			TripID:    conn.TripID,
			UserID:    conn.UserID,
			Timestamp: time.Now(),
			Version:   1,
		},
		Payload: func() []byte {
			data, _ := json.Marshal(messagePayload)
			return data
		}(),
		Metadata: types.EventMetadata{
			Source: "ws_handler",
		},
	})

	if err != nil {
		zap.L().Error("Failed to publish chat message event",
			zap.String("userID", conn.UserID),
			zap.String("tripID", conn.TripID),
			zap.Error(err))
		h.sendErrorResponse(conn, "Failed to send message")
		return
	}

	// Send acknowledgment to client
	ack := map[string]interface{}{
		"type":       "message_ack",
		"message_id": messageID,
		"status":     "sent",
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	ackData, _ := json.Marshal(ack)
	if err := conn.WriteMessage(websocket.TextMessage, ackData); err != nil {
		zap.L().Error("Failed to send message acknowledgment",
			zap.String("userID", conn.UserID),
			zap.String("tripID", conn.TripID),
			zap.Error(err))
	}

	zap.L().Debug("Chat message processed successfully",
		zap.String("userID", conn.UserID),
		zap.String("tripID", conn.TripID),
		zap.String("messageID", messageID))
}

// handleChatReaction processes chat message reaction requests
func (h *WSHandler) handleChatReaction(ctx context.Context, conn *middleware.SafeConn, payload json.RawMessage) {
	if conn == nil {
		zap.L().Error("Received nil connection in handleChatReaction")
		return
	}

	// Parse the reaction request
	var reactionRequest struct {
		MessageID string `json:"message_id"`
		Reaction  string `json:"reaction"`
		Action    string `json:"action"` // "add" or "remove"
	}

	if err := json.Unmarshal(payload, &reactionRequest); err != nil {
		zap.L().Error("Failed to parse chat reaction",
			zap.String("userID", conn.UserID),
			zap.String("tripID", conn.TripID),
			zap.Error(err))
		h.sendErrorResponse(conn, "Invalid reaction format")
		return
	}

	// Validate the request
	if reactionRequest.MessageID == "" {
		zap.L().Warn("Missing message ID in reaction request",
			zap.String("userID", conn.UserID),
			zap.String("tripID", conn.TripID))
		h.sendErrorResponse(conn, "Message ID is required")
		return
	}

	if reactionRequest.Reaction == "" {
		zap.L().Warn("Missing reaction emoji in request",
			zap.String("userID", conn.UserID),
			zap.String("tripID", conn.TripID))
		h.sendErrorResponse(conn, "Reaction emoji is required")
		return
	}

	// Validate action
	if reactionRequest.Action != "add" && reactionRequest.Action != "remove" {
		zap.L().Warn("Invalid action in reaction request",
			zap.String("userID", conn.UserID),
			zap.String("tripID", conn.TripID),
			zap.String("action", reactionRequest.Action))
		h.sendErrorResponse(conn, "Action must be 'add' or 'remove'")
		return
	}

	// Apply rate limiting
	if err := h.EnforceWSRateLimit(conn.UserID, "chat_reaction", 20); err != nil {
		zap.L().Warn("Chat reaction rate limit exceeded",
			zap.String("userID", conn.UserID),
			zap.Error(err))
		h.sendErrorResponse(conn, "Reaction rate limit exceeded")
		return
	}

	// Verify user is a member of the trip
	role, err := h.tripStore.GetUserRole(ctx, conn.TripID, conn.UserID)
	if err != nil || role == "" {
		zap.L().Error("Failed to verify trip membership for reaction",
			zap.String("userID", conn.UserID),
			zap.String("tripID", conn.TripID),
			zap.Error(err))
		h.sendErrorResponse(conn, "Not authorized to react in this trip")
		return
	}

	// Generate a reaction ID
	reactionID := uuid.New().String()

	// Create reaction payload
	reactionPayload := map[string]interface{}{
		"id":         reactionID,
		"message_id": reactionRequest.MessageID,
		"user_id":    conn.UserID,
		"reaction":   reactionRequest.Reaction,
		"trip_id":    conn.TripID,
		"created_at": time.Now().Format(time.RFC3339),
	}

	// Determine event type based on action
	var eventType types.EventType
	if reactionRequest.Action == "add" {
		eventType = types.EventTypeChatReactionAdded
	} else {
		eventType = types.EventTypeChatReactionRemoved
	}

	// Publish the reaction event
	err = h.eventService.Publish(ctx, conn.TripID, types.Event{
		BaseEvent: types.BaseEvent{
			Type:      eventType,
			ID:        reactionID,
			TripID:    conn.TripID,
			UserID:    conn.UserID,
			Timestamp: time.Now(),
			Version:   1,
		},
		Payload: func() []byte {
			data, _ := json.Marshal(reactionPayload)
			return data
		}(),
		Metadata: types.EventMetadata{
			Source: "ws_handler",
		},
	})

	if err != nil {
		zap.L().Error("Failed to publish reaction event",
			zap.String("userID", conn.UserID),
			zap.String("tripID", conn.TripID),
			zap.String("action", reactionRequest.Action),
			zap.Error(err))
		h.sendErrorResponse(conn, "Failed to process reaction")
		return
	}

	// Send acknowledgment
	ack := map[string]interface{}{
		"type":        "reaction_ack",
		"reaction_id": reactionID,
		"message_id":  reactionRequest.MessageID,
		"status":      "processed",
		"action":      reactionRequest.Action,
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	ackData, _ := json.Marshal(ack)
	if err := conn.WriteMessage(websocket.TextMessage, ackData); err != nil {
		zap.L().Error("Failed to send reaction acknowledgment",
			zap.String("userID", conn.UserID),
			zap.String("tripID", conn.TripID),
			zap.Error(err))
	}

	zap.L().Debug("Chat reaction processed successfully",
		zap.String("userID", conn.UserID),
		zap.String("tripID", conn.TripID),
		zap.String("action", reactionRequest.Action),
		zap.String("reactionID", reactionID),
		zap.String("messageID", reactionRequest.MessageID))
}

// handleReadReceipt processes chat message read receipt notifications
func (h *WSHandler) handleReadReceipt(ctx context.Context, conn *middleware.SafeConn, payload json.RawMessage) {
	if conn == nil {
		zap.L().Error("Received nil connection in handleReadReceipt")
		return
	}

	// Parse the read receipt request
	var readReceiptRequest struct {
		LastReadMessageID string `json:"last_read_message_id"`
		GroupID           string `json:"group_id,omitempty"`
	}

	if err := json.Unmarshal(payload, &readReceiptRequest); err != nil {
		zap.L().Error("Failed to parse read receipt",
			zap.String("userID", conn.UserID),
			zap.String("tripID", conn.TripID),
			zap.Error(err))
		h.sendErrorResponse(conn, "Invalid read receipt format")
		return
	}

	// Validate the request
	if readReceiptRequest.LastReadMessageID == "" {
		zap.L().Warn("Missing message ID in read receipt request",
			zap.String("userID", conn.UserID),
			zap.String("tripID", conn.TripID))
		h.sendErrorResponse(conn, "Last read message ID is required")
		return
	}

	// Apply rate limiting
	if err := h.EnforceWSRateLimit(conn.UserID, "chat_read_receipt", 30); err != nil {
		zap.L().Warn("Read receipt rate limit exceeded",
			zap.String("userID", conn.UserID),
			zap.Error(err))
		h.sendErrorResponse(conn, "Read receipt rate limit exceeded")
		return
	}

	// Verify user is a member of the trip
	role, err := h.tripStore.GetUserRole(ctx, conn.TripID, conn.UserID)
	if err != nil || role == "" {
		zap.L().Error("Failed to verify trip membership for read receipt",
			zap.String("userID", conn.UserID),
			zap.String("tripID", conn.TripID),
			zap.Error(err))
		h.sendErrorResponse(conn, "Not authorized to send read receipts in this trip")
		return
	}

	// Generate a receipt ID
	receiptID := uuid.New().String()

	// Create read receipt payload
	receiptPayload := map[string]interface{}{
		"id":           receiptID,
		"last_read_id": readReceiptRequest.LastReadMessageID,
		"user_id":      conn.UserID,
		"trip_id":      conn.TripID,
		"group_id":     readReceiptRequest.GroupID,
		"timestamp":    time.Now().Format(time.RFC3339),
	}

	// Publish the read receipt event
	err = h.eventService.Publish(ctx, conn.TripID, types.Event{
		BaseEvent: types.BaseEvent{
			Type:      types.EventTypeChatReadReceiptUpdated,
			ID:        receiptID,
			TripID:    conn.TripID,
			UserID:    conn.UserID,
			Timestamp: time.Now(),
			Version:   1,
		},
		Payload: func() []byte {
			data, _ := json.Marshal(receiptPayload)
			return data
		}(),
		Metadata: types.EventMetadata{
			Source: "ws_handler",
		},
	})

	if err != nil {
		zap.L().Error("Failed to publish read receipt event",
			zap.String("userID", conn.UserID),
			zap.String("tripID", conn.TripID),
			zap.Error(err))
		h.sendErrorResponse(conn, "Failed to process read receipt")
		return
	}

	// Send acknowledgment
	ack := map[string]interface{}{
		"type":       "read_receipt_ack",
		"receipt_id": receiptID,
		"status":     "processed",
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	ackData, _ := json.Marshal(ack)
	if err := conn.WriteMessage(websocket.TextMessage, ackData); err != nil {
		zap.L().Error("Failed to send read receipt acknowledgment",
			zap.String("userID", conn.UserID),
			zap.String("tripID", conn.TripID),
			zap.Error(err))
	}

	zap.L().Debug("Read receipt processed successfully",
		zap.String("userID", conn.UserID),
		zap.String("tripID", conn.TripID),
		zap.String("receiptID", receiptID),
		zap.String("lastReadID", readReceiptRequest.LastReadMessageID))
}

// handlePing responds to ping messages from clients
func (h *WSHandler) handlePing(ctx context.Context, conn *middleware.SafeConn) {
	// Send pong response
	pongResponse := map[string]string{
		"type": "pong",
		"time": time.Now().Format(time.RFC3339),
	}

	responseBytes, _ := json.Marshal(pongResponse)
	if err := conn.WriteMessage(websocket.TextMessage, responseBytes); err != nil {
		zap.L().Error("Failed to send pong response",
			zap.String("userID", conn.UserID),
			zap.Error(err))
	}
}

// handleTypingStatus processes chat typing status notifications
func (h *WSHandler) handleTypingStatus(ctx context.Context, conn *middleware.SafeConn, payload json.RawMessage) {
	if conn == nil {
		zap.L().Error("Received nil connection in handleTypingStatus")
		return
	}

	// Parse the typing status request
	var typingRequest struct {
		IsTyping bool   `json:"is_typing"`
		GroupID  string `json:"group_id,omitempty"`
	}

	if err := json.Unmarshal(payload, &typingRequest); err != nil {
		zap.L().Error("Failed to parse typing status",
			zap.String("userID", conn.UserID),
			zap.String("tripID", conn.TripID),
			zap.Error(err))
		h.sendErrorResponse(conn, "Invalid typing status format")
		return
	}

	// Apply rate limiting - allow more frequent updates for typing status
	if err := h.EnforceWSRateLimit(conn.UserID, "chat_typing", 50); err != nil {
		zap.L().Warn("Typing status rate limit exceeded",
			zap.String("userID", conn.UserID),
			zap.Error(err))
		// Don't send error response for typing, just ignore silently
		return
	}

	// Verify user is a member of the trip
	role, err := h.tripStore.GetUserRole(ctx, conn.TripID, conn.UserID)
	if err != nil || role == "" {
		zap.L().Debug("Non-member tried to send typing status",
			zap.String("userID", conn.UserID),
			zap.String("tripID", conn.TripID))
		// Don't send error response for typing, just ignore silently
		return
	}

	// Generate an event ID
	eventID := uuid.New().String()

	// Create typing status payload
	typingPayload := map[string]interface{}{
		"user_id":   conn.UserID,
		"trip_id":   conn.TripID,
		"group_id":  typingRequest.GroupID,
		"is_typing": typingRequest.IsTyping,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	// Publish the typing status event
	err = h.eventService.Publish(ctx, conn.TripID, types.Event{
		BaseEvent: types.BaseEvent{
			Type:      types.EventTypeChatTypingStatus,
			ID:        eventID,
			TripID:    conn.TripID,
			UserID:    conn.UserID,
			Timestamp: time.Now(),
			Version:   1,
		},
		Payload: func() []byte {
			data, _ := json.Marshal(typingPayload)
			return data
		}(),
		Metadata: types.EventMetadata{
			Source: "ws_handler",
		},
	})

	if err != nil {
		zap.L().Debug("Failed to publish typing status event",
			zap.String("userID", conn.UserID),
			zap.String("tripID", conn.TripID),
			zap.Error(err))
		// Don't send error response for typing failures, just ignore silently
	}

	// No acknowledgment for typing status to reduce traffic
}

// sendErrorResponse sends an error response to the client
func (h *WSHandler) sendErrorResponse(conn *middleware.SafeConn, message string) {
	errorResponse := map[string]string{"error": message}
	responseBytes, _ := json.Marshal(errorResponse)
	if err := conn.WriteMessage(websocket.TextMessage, responseBytes); err != nil {
		zap.L().Error("Failed to send error response",
			zap.String("userID", conn.UserID),
			zap.Error(err))
	}
}
