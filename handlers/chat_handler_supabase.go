package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/notification"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/services"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ChatHandlerSupabase handles chat-related HTTP requests with Supabase Realtime integration
type ChatHandlerSupabase struct {
	tripService         TripServiceInterface
	supabaseService     *services.SupabaseService
	notificationService *services.NotificationFacadeService
	logger              *zap.Logger
}

// NewChatHandlerSupabase creates a new instance of ChatHandlerSupabase
func NewChatHandlerSupabase(
	tripService TripServiceInterface,
	supabaseService *services.SupabaseService,
	notificationService *services.NotificationFacadeService,
) *ChatHandlerSupabase {
	return &ChatHandlerSupabase{
		tripService:         tripService,
		supabaseService:     supabaseService,
		notificationService: notificationService,
		logger:              logger.GetLogger().Desugar(),
	}
}

// SendMessage handles sending a new chat message
// @Summary Send a chat message
// @Description Sends a new message to a trip chat
// @Tags chat
// @Accept json
// @Produce json
// @Param tripId path string true "Trip ID"
// @Param message body types.ChatSendMessageRequest true "Message data"
// @Success 201 {object} types.ChatSendMessageResponse
// @Failure 400 {object} types.ErrorResponse
// @Failure 401 {object} types.ErrorResponse
// @Failure 403 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/trips/{tripId}/chat/messages [post]
func (h *ChatHandlerSupabase) SendMessage(c *gin.Context) {
	tripID := c.Param("id")
	if tripID == "" {
		_ = c.Error(errors.ValidationFailed("missing_trip_id", "trip ID is required"))
		return
	}

	userID := getUserIDFromContext(c)
	if userID == "" {
		_ = c.Error(errors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	// Verify user is a member of the trip
	member, err := h.tripService.GetTripMember(c.Request.Context(), tripID, userID)
	if err != nil || member == nil || member.DeletedAt != nil {
		_ = c.Error(errors.Forbidden("not_trip_member", "you are not an active member of this trip"))
		return
	}

	var req types.ChatSendMessageRequest
	if !bindJSONOrError(c, &req) {
		return
	}

	// Trim whitespace and validate message is not empty
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		_ = c.Error(errors.ValidationFailed("empty_message", "message cannot be empty"))
		return
	}

	// Generate a new UUID for the message
	messageID := uuid.New().String()

	// Send to Supabase
	now := time.Now()
	err = h.supabaseService.SendChatMessage(c.Request.Context(), services.ChatMessage{
		ID:        messageID,
		TripID:    tripID,
		UserID:    userID,
		Message:   req.Message,
		ReplyToID: req.ReplyToID,
		CreatedAt: now,
	})

	if err != nil {
		_ = c.Error(errors.InternalServerError("failed to send message"))
		return
	}

	// Send notifications to trip members asynchronously
	if h.notificationService != nil && h.notificationService.IsEnabled() {
		go h.sendChatNotifications(tripID, userID, messageID, req.Message)
	}

	// Return success
	c.JSON(http.StatusCreated, types.ChatSendMessageResponse{
		ID:        messageID,
		TripID:    tripID,
		UserID:    userID,
		Message:   req.Message,
		ReplyToID: req.ReplyToID,
		CreatedAt: now,
	})
}

// sendChatNotifications sends push notifications to trip members for a new chat message
func (h *ChatHandlerSupabase) sendChatNotifications(tripID, senderID, messageID, message string) {
	// Use background context for async notification
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get trip members for notification
	members, err := h.tripService.GetTripMembers(ctx, tripID)
	if err != nil {
		h.logger.Error("Failed to get trip members for chat notification", zap.Error(err))
		return
	}

	// Use a generic sender name for notifications
	// User info is not embedded in TripMembership, so we use a fallback
	senderName := "A trip member"

	// Prepare message preview (truncate if too long)
	messagePreview := message
	if len(messagePreview) > 100 {
		messagePreview = messagePreview[:97] + "..."
	}

	data := notification.ChatMessageData{
		TripID:         tripID,
		ChatID:         tripID, // In this system, chat ID is same as trip ID
		SenderID:       senderID,
		SenderName:     senderName,
		Message:        message,
		MessagePreview: messagePreview,
	}

	// Collect recipient IDs (all members except sender)
	var recipientIDs []string
	for _, member := range members {
		if member.UserID != senderID && member.DeletedAt == nil {
			recipientIDs = append(recipientIDs, member.UserID)
		}
	}

	// Send notifications
	if len(recipientIDs) > 0 {
		if err := h.notificationService.SendChatMessage(ctx, recipientIDs, data); err != nil {
			h.logger.Error("Failed to send chat message notifications",
				zap.Error(err),
				zap.String("tripId", tripID),
				zap.String("messageId", messageID),
			)
		}
	}
}

// GetMessages handles retrieving chat messages
// @Summary Get chat messages
// @Description Retrieves messages from a trip chat with pagination
// @Tags chat
// @Produce json
// @Param tripId path string true "Trip ID"
// @Param limit query int false "Maximum number of messages to return" default(50)
// @Param before query string false "Return messages before this message ID (for pagination)"
// @Success 200 {object} types.ChatGetMessagesResponse
// @Failure 400 {object} types.ErrorResponse
// @Failure 401 {object} types.ErrorResponse
// @Failure 403 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/trips/{tripId}/chat/messages [get]
func (h *ChatHandlerSupabase) GetMessages(c *gin.Context) {
	tripID := c.Param("id")
	if tripID == "" {
		_ = c.Error(errors.ValidationFailed("missing_trip_id", "trip ID is required"))
		return
	}

	userID := getUserIDFromContext(c)
	if userID == "" {
		_ = c.Error(errors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	// Verify user is a member of the trip
	member, err := h.tripService.GetTripMember(c.Request.Context(), tripID, userID)
	if err != nil || member == nil || member.DeletedAt != nil {
		_ = c.Error(errors.Forbidden("not_trip_member", "you are not an active member of this trip"))
		return
	}

	// Parse pagination parameters
	limit := 50 // default
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	before := c.Query("before") // cursor for pagination

	// For the Supabase implementation, return proper structure with pagination
	// The actual chat messages will be retrieved by the client directly from Supabase
	// But we need to return the expected structure to prevent frontend crashes
	response := types.ChatGetMessagesResponse{
		Messages: []interface{}{}, // Empty array - client fetches from Supabase directly
		Pagination: types.ChatPaginationInfo{
			HasMore:    false,
			NextCursor: nil,
			Limit:      limit,
			Before:     before,
		},
	}

	c.JSON(http.StatusOK, response)
}

// AddReaction handles adding a reaction to a message
// @Summary Add a reaction
// @Description Adds an emoji reaction to a chat message
// @Tags chat
// @Accept json
// @Produce json
// @Param tripId path string true "Trip ID"
// @Param messageId path string true "Message ID"
// @Param reaction body types.ChatAddReactionRequest true "Reaction data"
// @Success 201 {object} types.ChatReactionResponse
// @Failure 400 {object} types.ErrorResponse
// @Failure 401 {object} types.ErrorResponse
// @Failure 403 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/trips/{tripId}/chat/messages/{messageId}/reactions [post]
func (h *ChatHandlerSupabase) AddReaction(c *gin.Context) {
	tripID := c.Param("id")
	messageID := c.Param("messageId")

	if tripID == "" || messageID == "" {
		_ = c.Error(errors.ValidationFailed("missing_ids", "trip ID and message ID are required"))
		return
	}

	userID := getUserIDFromContext(c)
	if userID == "" {
		_ = c.Error(errors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	// Verify user is a member of the trip
	member, err := h.tripService.GetTripMember(c.Request.Context(), tripID, userID)
	if err != nil || member == nil || member.DeletedAt != nil {
		_ = c.Error(errors.Forbidden("not_trip_member", "you are not an active member of this trip"))
		return
	}

	var req types.ChatAddReactionRequest
	if !bindJSONOrError(c, &req) {
		return
	}

	// Send to Supabase
	err = h.supabaseService.AddChatReaction(c.Request.Context(), services.ChatReaction{
		MessageID: messageID,
		UserID:    userID,
		Emoji:     req.Emoji,
	})

	if err != nil {
		_ = c.Error(errors.InternalServerError("failed to add reaction"))
		return
	}

	// Return success
	c.JSON(http.StatusCreated, types.ChatReactionResponse{
		MessageID: messageID,
		UserID:    userID,
		Emoji:     req.Emoji,
	})
}

// RemoveReaction handles removing a reaction from a message
// @Summary Remove a reaction
// @Description Removes an emoji reaction from a chat message
// @Tags chat
// @Produce json
// @Param tripId path string true "Trip ID"
// @Param messageId path string true "Message ID"
// @Param emoji path string true "Emoji to remove"
// @Success 200 {object} types.StatusResponse
// @Failure 400 {object} types.ErrorResponse
// @Failure 401 {object} types.ErrorResponse
// @Failure 403 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/trips/{tripId}/chat/messages/{messageId}/reactions/{emoji} [delete]
func (h *ChatHandlerSupabase) RemoveReaction(c *gin.Context) {
	tripID := c.Param("id")
	messageID := c.Param("messageId")
	emoji := c.Param("emoji")

	if tripID == "" || messageID == "" || emoji == "" {
		_ = c.Error(errors.ValidationFailed("missing_params", "trip ID, message ID, and emoji are required"))
		return
	}

	userID := getUserIDFromContext(c)
	if userID == "" {
		_ = c.Error(errors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	// Verify user is a member of the trip
	member, err := h.tripService.GetTripMember(c.Request.Context(), tripID, userID)
	if err != nil || member == nil || member.DeletedAt != nil {
		_ = c.Error(errors.Forbidden("not_trip_member", "you are not an active member of this trip"))
		return
	}

	// Remove from Supabase
	err = h.supabaseService.RemoveChatReaction(c.Request.Context(), services.ChatReaction{
		MessageID: messageID,
		UserID:    userID,
		Emoji:     emoji,
	})

	if err != nil {
		_ = c.Error(errors.InternalServerError("failed to remove reaction"))
		return
	}

	// Return success
	c.JSON(http.StatusOK, gin.H{
		"status": "Reaction removed",
	})
}

// UpdateReadStatus handles updating the user's read status
// @Summary Update read status
// @Description Updates the user's last read message for a trip
// @Tags chat
// @Accept json
// @Produce json
// @Param tripId path string true "Trip ID"
// @Param status body types.ChatUpdateReadStatusRequest true "Read status data"
// @Success 200 {object} types.StatusResponse
// @Failure 400 {object} types.ErrorResponse
// @Failure 401 {object} types.ErrorResponse
// @Failure 403 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/trips/{tripId}/chat/read-status [put]
func (h *ChatHandlerSupabase) UpdateReadStatus(c *gin.Context) {
	tripID := c.Param("id")
	if tripID == "" {
		_ = c.Error(errors.ValidationFailed("missing_trip_id", "trip ID is required"))
		return
	}

	userID := getUserIDFromContext(c)
	if userID == "" {
		_ = c.Error(errors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	// Verify user is a member of the trip
	member, err := h.tripService.GetTripMember(c.Request.Context(), tripID, userID)
	if err != nil || member == nil || member.DeletedAt != nil {
		_ = c.Error(errors.Forbidden("not_trip_member", "you are not an active member of this trip"))
		return
	}

	var req types.ChatUpdateReadStatusRequest
	if !bindJSONOrError(c, &req) {
		return
	}

	// For the Supabase implementation, we don't need to implement this on the backend
	// The client will update read status directly in Supabase
	c.JSON(http.StatusOK, gin.H{
		"status": "Read status updated",
	})
}
