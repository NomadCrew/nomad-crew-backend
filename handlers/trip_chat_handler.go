package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	istore "github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	tripTopicPrefix = "trip:"
	messageTypeText = "text"
	eventSourceUser = "USER"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// TripChatHandler handles WebSocket and HTTP requests related to trip chat.
type TripChatHandler struct {
	tripModel    interfaces.TripModelInterface
	eventService types.EventPublisher
	chatStore    istore.ChatStore
	userStore    istore.UserStore
	serverConfig *config.ServerConfig
}

// NewTripChatHandler creates a new TripChatHandler.
func NewTripChatHandler(
	tripModel interfaces.TripModelInterface,
	eventService types.EventPublisher,
	chatStore istore.ChatStore,
	userStore istore.UserStore,
	serverConfig *config.ServerConfig,
) *TripChatHandler {
	return &TripChatHandler{
		tripModel:    tripModel,
		eventService: eventService,
		chatStore:    chatStore,
		userStore:    userStore,
		serverConfig: serverConfig,
	}
}

// Local payload definitions - to be moved to types package later if appropriate
type ChatMessageSentPayload struct {
	Message string `json:"message"`
}
type ChatTypingStatusPayload struct {
	IsTyping bool `json:"isTyping"`
}

// Helper function to get integer query param (moved from original trip_handler or to be placed in a common util)
func getIntQueryParam(c *gin.Context, key string, defaultValue int) (int, error) {
	valStr := c.Query(key)
	if valStr == "" {
		return defaultValue, nil
	}
	valInt, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultValue, err
	}
	return valInt, nil
}

// WSStreamEvents godoc
// @Summary WebSocket stream for trip events
// @Description Establishes a WebSocket connection to stream real-time events for a trip (chat, updates, etc.).
// @Tags trips-chat websocket
// @Param tripId path string true "Trip ID"
// @Success 101 "Switching Protocols"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid trip ID or user not authorized"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User not a member of this trip"
// @Failure 500 {object} types.ErrorResponse "Internal server error - Failed to upgrade connection"
// @Router /trips/{tripId}/ws/events [get]
// @Security BearerAuth
func (h *TripChatHandler) WSStreamEvents(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("tripId")
	userID := c.GetString(string(middleware.UserIDKey))

	_, err := h.tripModel.GetUserRole(c.Request.Context(), tripID, userID)
	if err != nil {
		log.Warnw("User not authorized for trip WebSocket", "tripID", tripID, "userID", userID, "error", err)
		handleModelError(c, apperrors.Forbidden("not_member", "User is not a member of this trip or trip does not exist."))
		return
	}

	unsafeConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Errorw("Failed to upgrade WebSocket connection", "error", err, "tripID", tripID, "userID", userID)
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{Message: "Failed to upgrade to WebSocket"})
		return
	}
	conn := middleware.NewSafeConn(unsafeConn, nil, middleware.DefaultWSConfig())
	if conn == nil {
		log.Errorw("Failed to create SafeConn", "tripID", tripID, "userID", userID)
		if err := unsafeConn.Close(); err != nil {
			log.Warnw("Error closing underlying WebSocket connection after SafeConn creation failure", "error", err, "tripID", tripID, "userID", userID)
		}
		return
	}
	defer conn.Close()

	log.Infow("WebSocket connection established", "tripID", tripID, "userID", userID, "remoteAddr", conn.RemoteAddr().String())

	eventChan, err := h.eventService.Subscribe(c.Request.Context(), tripID, userID)
	if err != nil {
		log.Errorw("Failed to subscribe to trip events", "error", err, "tripID", tripID, "userID", userID)
		if writeErr := conn.WriteJSON(types.WebSocketMessage{Type: "error", Payload: "Failed to subscribe to events"}); writeErr != nil {
			log.Errorw("Failed to send subscription error to WebSocket", "writeError", writeErr, "tripID", tripID, "userID", userID)
		}
		return
	}
	defer func() {
		if unsubErr := h.eventService.Unsubscribe(c.Request.Context(), tripID, userID); unsubErr != nil {
			log.Warnw("Failed to unsubscribe from trip events", "error", unsubErr, "tripID", tripID, "userID", userID)
		}
	}()

	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Warnw("WebSocket read error", "error", err, "tripID", tripID, "userID", userID)
				} else {
					log.Infow("WebSocket connection closed by client or expected closure", "tripID", tripID, "userID", userID, "error", err)
				}
				return
			}

			if err := h.HandleChatMessage(c.Request.Context(), conn, message, userID, tripID); err != nil {
				log.Errorw("Failed to handle incoming chat message", "error", err, "tripID", tripID, "userID", userID)
				if writeErr := conn.WriteJSON(types.WebSocketMessage{Type: "error", Payload: gin.H{"message": "Failed to process message", "detail": err.Error()}}); writeErr != nil {
					log.Errorw("Failed to send chat message processing error to WebSocket", "writeError", writeErr, "tripID", tripID, "userID", userID)
				}
			}
		}
	}()

	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				log.Infow("Event channel closed for WebSocket", "tripID", tripID, "userID", userID)
				if writeErr := conn.WriteJSON(types.WebSocketMessage{Type: "system", Payload: "Event stream closed"}); writeErr != nil {
					log.Errorw("Failed to send event stream closed message to WebSocket", "writeError", writeErr, "tripID", tripID, "userID", userID)
				}
				return
			}
			if err := conn.WriteJSON(event); err != nil {
				log.Errorw("Failed to write event to WebSocket", "error", err, "tripID", tripID, "userID", userID)
				return
			}
		case <-c.Request.Context().Done():
			log.Infow("WebSocket context done, closing connection", "tripID", tripID, "userID", userID)
			return
		}
	}
}

func (h *TripChatHandler) HandleChatMessage(ctx context.Context, conn *middleware.SafeConn, rawMessage []byte, userIDStr string, tripID string) error {
	log := logger.GetLogger()

	var wsMsg types.WebSocketMessage
	if err := json.Unmarshal(rawMessage, &wsMsg); err != nil {
		log.Warnw("Failed to unmarshal WebSocket message", "error", err, "rawMessage", string(rawMessage))
		return apperrors.ValidationFailed("invalid_message_format", err.Error())
	}

	senderUUID, err := uuid.Parse(userIDStr)
	if err != nil {
		log.Errorw("Invalid userID in HandleChatMessage", "userID", userIDStr, "error", err)
		return apperrors.New("INTERNAL", "Invalid user ID format", err.Error())
	}

	senderProfile, err := h.userStore.GetUserByID(ctx, senderUUID.String())
	if err != nil {
		log.Errorw("Failed to get sender profile", "userID", userIDStr, "error", err)
		return err
	}

	senderInfo := &types.MessageSender{
		ID:        senderProfile.ID,
		Name:      senderProfile.Username,
		AvatarURL: senderProfile.ProfilePictureURL,
	}

	// Assuming groupID is the same as tripID for trip-wide chat
	groupID := tripID

	switch types.EventType(wsMsg.Type) {
	case types.EventTypeChatMessageSent:
		var chatPayload ChatMessageSentPayload
		payloadBytes, _ := json.Marshal(wsMsg.Payload)
		if err := json.Unmarshal(payloadBytes, &chatPayload); err != nil {
			log.Warnw("Failed to unmarshal ChatMessageSentPayload", "error", err, "payload", wsMsg.Payload)
			return apperrors.ValidationFailed("invalid_chat_payload", err.Error())
		}

		chatMsg := &types.ChatMessage{
			TripID:      tripID,
			GroupID:     groupID, // Set groupID
			UserID:      userIDStr,
			Content:     chatPayload.Message,
			ContentType: messageTypeText,
			CreatedAt:   time.Now().UTC(),
			Sender:      senderInfo,
		}

		messageID, err := h.chatStore.CreateChatMessage(ctx, *chatMsg)
		if err != nil {
			log.Errorw("Failed to save chat message", "error", err)
			return err
		}
		chatMsg.ID = messageID

		eventPayloadBytes, _ := json.Marshal(chatMsg)
		event := types.Event{
			BaseEvent: types.BaseEvent{
				ID:        uuid.NewString(),
				Type:      types.EventTypeChatMessageSent,
				TripID:    tripID,
				UserID:    userIDStr,
				Timestamp: time.Now().UTC(),
				Version:   1,
			},
			Metadata: types.EventMetadata{
				Source: eventSourceUser,
			},
			Payload: json.RawMessage(eventPayloadBytes),
		}
		if pubErr := h.eventService.Publish(ctx, tripTopicPrefix+tripID, event); pubErr != nil {
			log.Errorw("Failed to publish chat message event", "error", pubErr)
			return pubErr
		}
		log.Infow("Chat message processed and published", "tripID", tripID, "userID", userIDStr, "messageID", messageID)

	case types.EventTypeChatTypingStatus:
		var typingPayload ChatTypingStatusPayload
		payloadBytes, _ := json.Marshal(wsMsg.Payload)
		if err := json.Unmarshal(payloadBytes, &typingPayload); err != nil {
			log.Warnw("Failed to unmarshal ChatTypingStatusPayload", "error", err)
			return apperrors.ValidationFailed("invalid_typing_payload", err.Error())
		}

		augmentedTypingPayload := types.ChatTypingStatusEvent{
			TripID:   tripID,
			IsTyping: typingPayload.IsTyping,
			User: types.UserResponse{
				ID:          senderProfile.ID,
				Username:    senderProfile.Username,
				FirstName:   senderProfile.FirstName,
				LastName:    senderProfile.LastName,
				AvatarURL:   senderProfile.ProfilePictureURL,
				DisplayName: senderProfile.GetFullName(),
			},
		}
		eventPayloadBytes, _ := json.Marshal(augmentedTypingPayload)

		event := types.Event{
			BaseEvent: types.BaseEvent{
				ID:        uuid.NewString(),
				Type:      types.EventTypeChatTypingStatus,
				TripID:    tripID,
				UserID:    userIDStr,
				Timestamp: time.Now().UTC(),
				Version:   1,
			},
			Metadata: types.EventMetadata{
				Source: eventSourceUser,
			},
			Payload: json.RawMessage(eventPayloadBytes),
		}
		if pubErr := h.eventService.Publish(ctx, tripTopicPrefix+tripID, event); pubErr != nil {
			log.Errorw("Failed to publish typing status event", "error", pubErr)
			return pubErr
		}
		log.Infow("Typing status processed and published", "tripID", tripID, "userID", userIDStr, "isTyping", typingPayload.IsTyping)

	default:
		log.Warnw("Received unhandled WebSocket message type", "type", wsMsg.Type, "tripID", tripID, "userID", userIDStr)
		return apperrors.ValidationFailed("unhandled_message_type", "Unhandled message type: "+string(wsMsg.Type))
	}

	return nil
}

// ListTripMessages godoc
// @Summary List messages for a trip
// @Description Retrieves a paginated list of chat messages for a specific trip.
// @Tags trips-chat
// @Accept json
// @Produce json
// @Param tripId path string true "Trip ID"
// @Param before query string false "Timestamp to fetch messages before (ISO 8601)"
// @Param limit query int false "Number of messages to fetch (default 50)"
// @Success 200 {array} types.ChatMessage "List of chat messages"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid parameters"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User not a member of this trip"
// @Failure 404 {object} types.ErrorResponse "Not found - Trip not found"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips/{tripId}/chat/messages [get]
// @Security BearerAuth
func (h *TripChatHandler) ListTripMessages(c *gin.Context) {
	tripID := c.Param("tripId")
	userID := c.GetString(string(middleware.UserIDKey))
	groupID := tripID

	_, err := h.tripModel.GetUserRole(c.Request.Context(), tripID, userID)
	if err != nil {
		handleModelError(c, apperrors.Forbidden("not_member", "User is not a member of this trip or trip does not exist."))
		return
	}

	limit, err := getIntQueryParam(c, "limit", 50)
	if err != nil {
		handleModelError(c, apperrors.ValidationFailed("invalid_limit_param", "Invalid 'limit' parameter format."))
		return
	}
	offset, err := getIntQueryParam(c, "offset", 0)
	if err != nil {
		handleModelError(c, apperrors.ValidationFailed("invalid_offset_param", "Invalid 'offset' parameter format."))
		return
	}

	pagingParams := types.PaginationParams{
		Limit:  limit,
		Offset: offset,
	}

	messages, _, err := h.chatStore.ListChatMessages(c.Request.Context(), groupID, pagingParams)
	if err != nil {
		handleModelError(c, err)
		return
	}

	c.JSON(http.StatusOK, messages)
}

// ChatLastReadRequest defines the request body for updating the last read message.
// This will be removed from trip_handler.go later.
type ChatLastReadRequest types.ChatLastReadRequest

// UpdateLastReadMessage godoc
// @Summary Update user's last read message in a trip
// @Description Updates the timestamp of the last message read by the user in a specific trip's chat.
// @Tags trips-chat
// @Accept json
// @Produce json
// @Param tripId path string true "Trip ID"
// @Param request body ChatLastReadRequest true "Last read message ID"
// @Success 200 {object} docs.StatusResponse "Successfully updated last read timestamp"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid input"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User not a member of this trip"
// @Failure 404 {object} types.ErrorResponse "Not found - Trip or Message not found"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips/{tripId}/chat/read [post]
// @Security BearerAuth
func (h *TripChatHandler) UpdateLastReadMessage(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("tripId")
	userID := c.GetString(string(middleware.UserIDKey))
	groupID := tripID // Assuming trip-wide chat where tripID is the groupID

	var req types.ChatLastReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw("Invalid update last read message request", "error", err, "tripID", tripID, "userID", userID)
		handleModelError(c, apperrors.ValidationFailed("invalid_request_payload", err.Error()))
		return
	}

	_, err := h.tripModel.GetUserRole(c.Request.Context(), tripID, userID)
	if err != nil {
		handleModelError(c, apperrors.Forbidden("not_member", "User is not a member of this trip or trip does not exist."))
		return
	}

	if req.LastReadMessageID == nil || *req.LastReadMessageID == "" {
		handleModelError(c, apperrors.ValidationFailed("invalid_message_id", "Message ID is required."))
		return
	}

	if err := h.chatStore.UpdateLastReadMessage(c.Request.Context(), groupID, userID, *req.LastReadMessageID); err != nil {
		handleModelError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Last read message updated successfully"})
}
