// Package handlers contains the HTTP handlers for the application's API endpoints.
package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/service"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// TripServiceInterface defines the trip service methods needed by ChatHandler
type TripServiceInterface interface {
	IsTripMember(ctx context.Context, tripID, userID string) (bool, error)
}

// ChatHandler encapsulates dependencies and methods for handling chat-related HTTP requests.
type ChatHandler struct {
	chatService    service.ChatService
	tripService    TripServiceInterface
	eventPublisher types.EventPublisher
	logger         *zap.Logger
}

// NewChatHandler creates a new instance of ChatHandler with required dependencies.
func NewChatHandler(
	chatService service.ChatService,
	tripService TripServiceInterface,
	eventPublisher types.EventPublisher,
	logger *zap.Logger,
) *ChatHandler {
	return &ChatHandler{
		chatService:    chatService,
		tripService:    tripService,
		eventPublisher: eventPublisher,
		logger:         logger,
	}
}

// verifyTripMembership checks if the user is a member of the specified trip
func (h *ChatHandler) verifyTripMembership(ctx context.Context, tripID, userID string) error {
	// Check if the user is a member of the trip
	isMember, err := h.tripService.IsTripMember(ctx, tripID, userID)
	if err != nil {
		return err
	}

	if !isMember {
		return errors.Forbidden("not_trip_member", "User is not a member of this trip")
	}

	return nil
}

// ListMessages godoc
// @Summary List chat messages
// @Description Retrieves messages for a trip's chat with pagination
// @Tags chat
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Param limit query int false "Number of messages to return (default 50)"
// @Param offset query int false "Offset for pagination (default 0)"
// @Success 200 {object} types.ChatMessagePaginatedResponse "List of chat messages with pagination info"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid trip ID or query parameters"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User is not a member of this trip"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips/{id}/chat/messages [get]
// @Security BearerAuth
func (h *ChatHandler) ListMessages(c *gin.Context) {
	log := logger.GetLogger()

	// Get user ID from context
	userID := c.GetString("user_id")
	if userID == "" {
		log.Warn("ListMessages: User ID not found in context")
		_ = c.Error(errors.Unauthorized("unauthorized", "User ID missing from context"))
		return
	}

	// Get trip ID from path
	tripID := c.Param("id")
	if tripID == "" {
		log.Warn("ListMessages: Trip ID path parameter not provided")
		_ = c.Error(errors.ValidationFailed("missing_param", "Trip ID path parameter is required"))
		return
	}

	// Verify the user is a member of the trip
	if err := h.verifyTripMembership(c.Request.Context(), tripID, userID); err != nil {
		log.Warnw("ListMessages: User is not a member of the trip", "userID", userID, "tripID", tripID)
		_ = c.Error(err)
		return
	}

	// Parse pagination parameters
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil || limit <= 0 {
		log.Warnw("ListMessages: Invalid limit query parameter", "value", c.Query("limit"), "error", err)
		limit = 50 // Default limit
	}

	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		log.Warnw("ListMessages: Invalid offset query parameter", "value", c.Query("offset"), "error", err)
		offset = 0 // Default offset
	}

	// Create pagination parameters
	paginationParams := types.PaginationParams{
		Limit:  limit,
		Offset: offset,
	}

	// Get messages from the service - we need to get the first group for the trip
	// In a real implementation, we would have a more robust way to get the correct group
	groups, err := h.chatService.ListTripGroups(c.Request.Context(), tripID, userID, types.PaginationParams{Limit: 1, Offset: 0})
	if err != nil {
		log.Errorw("ListMessages: Failed to get trip groups", "error", err, "tripID", tripID)
		_ = c.Error(err)
		return
	}

	if len(groups.Groups) == 0 {
		log.Warn("ListMessages: No chat groups found for trip", "tripID", tripID)
		c.JSON(http.StatusOK, types.ChatMessagePaginatedResponse{
			Messages: []types.ChatMessageWithUser{},
			Total:    0,
			Limit:    limit,
			Offset:   offset,
		})
		return
	}

	groupID := groups.Groups[0].ID
	messagesResponse, err := h.chatService.ListMessages(c.Request.Context(), groupID, userID, paginationParams)
	if err != nil {
		log.Errorw("ListMessages: Failed to list chat messages", "error", err, "tripID", tripID, "groupID", groupID)
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, messagesResponse)
}

// SendMessage godoc
// @Summary Send chat message
// @Description Sends a new message in a trip's chat
// @Tags chat
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Param request body types.ChatMessageCreateRequest true "Message content"
// @Success 201 {object} types.ChatMessage "Created message details"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid trip ID or message content"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User is not a member of this trip"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips/{id}/chat/messages [post]
// @Security BearerAuth
func (h *ChatHandler) SendMessage(c *gin.Context) {
	log := logger.GetLogger()

	// Get user ID from context
	userID := c.GetString("user_id")
	if userID == "" {
		log.Warn("SendMessage: User ID not found in context")
		_ = c.Error(errors.Unauthorized("unauthorized", "User ID missing from context"))
		return
	}

	// Get trip ID from path
	tripID := c.Param("id")
	if tripID == "" {
		log.Warn("SendMessage: Trip ID path parameter not provided")
		_ = c.Error(errors.ValidationFailed("missing_param", "Trip ID path parameter is required"))
		return
	}

	// Verify the user is a member of the trip
	if err := h.verifyTripMembership(c.Request.Context(), tripID, userID); err != nil {
		log.Warnw("SendMessage: User is not a member of the trip", "userID", userID, "tripID", tripID)
		_ = c.Error(err)
		return
	}

	// Parse request body
	var req types.ChatMessageCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnw("SendMessage: Failed to bind JSON request", "error", err)
		_ = c.Error(errors.ValidationFailed("invalid_request", fmt.Sprintf("Invalid request body: %v", err)))
		return
	}

	// Get the chat group for this trip
	groups, err := h.chatService.ListTripGroups(c.Request.Context(), tripID, userID, types.PaginationParams{Limit: 1, Offset: 0})
	if err != nil {
		log.Errorw("SendMessage: Failed to get trip groups", "error", err, "tripID", tripID)
		_ = c.Error(err)
		return
	}

	var groupID string
	if len(groups.Groups) == 0 {
		// Create a default group if none exists
		log.Info("SendMessage: No group found for trip, creating default group", "tripID", tripID)
		group, err := h.chatService.CreateGroup(c.Request.Context(), tripID, "Trip Chat", userID)
		if err != nil {
			log.Errorw("SendMessage: Failed to create default group", "error", err, "tripID", tripID)
			_ = c.Error(err)
			return
		}
		groupID = group.ID
	} else {
		groupID = groups.Groups[0].ID
	}

	// Send message via service
	message, err := h.chatService.PostMessage(c.Request.Context(), groupID, userID, req.Content)
	if err != nil {
		log.Errorw("SendMessage: Failed to send message", "error", err, "tripID", tripID, "groupID", groupID)
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, message)
}

// UpdateMessage handles the HTTP PUT request to update a message in a trip's chat.
func (h *ChatHandler) UpdateMessage(c *gin.Context) {
	log := logger.GetLogger()

	// Get user ID from context
	userID := c.GetString("user_id")
	if userID == "" {
		log.Warn("UpdateMessage: User ID not found in context")
		_ = c.Error(errors.Unauthorized("unauthorized", "User ID missing from context"))
		return
	}

	// Get trip ID and message ID from path
	tripID := c.Param("id")
	if tripID == "" {
		log.Warn("UpdateMessage: Trip ID path parameter not provided")
		_ = c.Error(errors.ValidationFailed("missing_param", "Trip ID path parameter is required"))
		return
	}

	messageID := c.Param("messageId")
	if messageID == "" {
		log.Warn("UpdateMessage: Message ID path parameter not provided")
		_ = c.Error(errors.ValidationFailed("missing_param", "Message ID path parameter is required"))
		return
	}

	// Verify the user is a member of the trip
	if err := h.verifyTripMembership(c.Request.Context(), tripID, userID); err != nil {
		log.Warnw("UpdateMessage: User is not a member of the trip", "userID", userID, "tripID", tripID)
		_ = c.Error(err)
		return
	}

	// Parse request body
	var req types.ChatMessageUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnw("UpdateMessage: Failed to bind JSON request", "error", err)
		_ = c.Error(errors.ValidationFailed("invalid_request", fmt.Sprintf("Invalid request body: %v", err)))
		return
	}

	// Update message via service
	message, err := h.chatService.UpdateMessage(c.Request.Context(), messageID, userID, req.Content)
	if err != nil {
		log.Errorw("UpdateMessage: Failed to update message", "error", err, "messageID", messageID)
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, message)
}

// DeleteMessage godoc
// @Summary Delete chat message
// @Description Deletes a message from a trip's chat
// @Tags chat
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Param messageId path string true "Message ID"
// @Success 200 {object} map[string]string "Success response with message"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid trip ID or message ID"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User is not a member of this trip"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips/{id}/chat/messages/{messageId} [delete]
// @Security BearerAuth
// DeleteMessage handles the HTTP DELETE request to delete a message in a trip's chat.
func (h *ChatHandler) DeleteMessage(c *gin.Context) {
	log := logger.GetLogger()

	// Get user ID from context
	userID := c.GetString("user_id")
	if userID == "" {
		log.Warn("DeleteMessage: User ID not found in context")
		_ = c.Error(errors.Unauthorized("unauthorized", "User ID missing from context"))
		return
	}

	// Get trip ID and message ID from path
	tripID := c.Param("id")
	if tripID == "" {
		log.Warn("DeleteMessage: Trip ID path parameter not provided")
		_ = c.Error(errors.ValidationFailed("missing_param", "Trip ID path parameter is required"))
		return
	}

	messageID := c.Param("messageId")
	if messageID == "" {
		log.Warn("DeleteMessage: Message ID path parameter not provided")
		_ = c.Error(errors.ValidationFailed("missing_param", "Message ID path parameter is required"))
		return
	}

	// Verify the user is a member of the trip
	if err := h.verifyTripMembership(c.Request.Context(), tripID, userID); err != nil {
		log.Warnw("DeleteMessage: User is not a member of the trip", "userID", userID, "tripID", tripID)
		_ = c.Error(err)
		return
	}

	// Delete message via service
	err := h.chatService.DeleteMessage(c.Request.Context(), messageID, userID)
	if err != nil {
		log.Errorw("DeleteMessage: Failed to delete message", "error", err, "messageID", messageID)
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message deleted successfully"})
}

// AddReaction godoc
// @Summary Add reaction to message
// @Description Adds a reaction to a message in a trip's chat
// @Tags chat
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Param messageId path string true "Message ID"
// @Param request body types.ChatMessageReactionRequest true "Reaction details"
// @Success 200 {object} map[string]string "Success response with message"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid trip ID, message ID, or reaction"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User is not a member of this trip"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips/{id}/chat/messages/{messageId}/reactions [post]
// @Security BearerAuth
// AddReaction handles the HTTP POST request to add a reaction to a message.
func (h *ChatHandler) AddReaction(c *gin.Context) {
	log := logger.GetLogger()

	// Get user ID from context
	userID := c.GetString("user_id")
	if userID == "" {
		log.Warn("AddReaction: User ID not found in context")
		_ = c.Error(errors.Unauthorized("unauthorized", "User ID missing from context"))
		return
	}

	// Get trip ID and message ID from path
	tripID := c.Param("id")
	if tripID == "" {
		log.Warn("AddReaction: Trip ID path parameter not provided")
		_ = c.Error(errors.ValidationFailed("missing_param", "Trip ID path parameter is required"))
		return
	}

	messageID := c.Param("messageId")
	if messageID == "" {
		log.Warn("AddReaction: Message ID path parameter not provided")
		_ = c.Error(errors.ValidationFailed("missing_param", "Message ID path parameter is required"))
		return
	}

	// Verify the user is a member of the trip
	if err := h.verifyTripMembership(c.Request.Context(), tripID, userID); err != nil {
		log.Warnw("AddReaction: User is not a member of the trip", "userID", userID, "tripID", tripID)
		_ = c.Error(err)
		return
	}

	// Parse request body
	var req types.ChatMessageReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnw("AddReaction: Failed to bind JSON request", "error", err)
		_ = c.Error(errors.ValidationFailed("invalid_request", fmt.Sprintf("Invalid request body: %v", err)))
		return
	}

	// Add reaction via service
	err := h.chatService.AddReaction(c.Request.Context(), messageID, userID, req.Reaction)
	if err != nil {
		log.Errorw("AddReaction: Failed to add reaction", "error", err, "messageID", messageID)
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Reaction added successfully"})
}

// RemoveReaction godoc
// @Summary Remove reaction from message
// @Description Removes a reaction from a message in a trip's chat
// @Tags chat
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Param messageId path string true "Message ID"
// @Param reactionType path string true "Reaction type"
// @Success 200 {object} map[string]string "Success response with message"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid trip ID, message ID, or reaction type"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User is not a member of this trip"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips/{id}/chat/messages/{messageId}/reactions/{reactionType} [delete]
// @Security BearerAuth
// RemoveReaction handles the HTTP DELETE request to remove a reaction from a message.
func (h *ChatHandler) RemoveReaction(c *gin.Context) {
	log := logger.GetLogger()

	// Get user ID from context
	userID := c.GetString("user_id")
	if userID == "" {
		log.Warn("RemoveReaction: User ID not found in context")
		_ = c.Error(errors.Unauthorized("unauthorized", "User ID missing from context"))
		return
	}

	// Get trip ID, message ID, and reaction type from path
	tripID := c.Param("id")
	if tripID == "" {
		log.Warn("RemoveReaction: Trip ID path parameter not provided")
		_ = c.Error(errors.ValidationFailed("missing_param", "Trip ID path parameter is required"))
		return
	}

	messageID := c.Param("messageId")
	if messageID == "" {
		log.Warn("RemoveReaction: Message ID path parameter not provided")
		_ = c.Error(errors.ValidationFailed("missing_param", "Message ID path parameter is required"))
		return
	}

	reactionType := c.Param("reactionType")
	if reactionType == "" {
		log.Warn("RemoveReaction: Reaction type path parameter not provided")
		_ = c.Error(errors.ValidationFailed("missing_param", "Reaction type path parameter is required"))
		return
	}

	// Verify the user is a member of the trip
	if err := h.verifyTripMembership(c.Request.Context(), tripID, userID); err != nil {
		log.Warnw("RemoveReaction: User is not a member of the trip", "userID", userID, "tripID", tripID)
		_ = c.Error(err)
		return
	}

	// Remove reaction via service
	err := h.chatService.RemoveReaction(c.Request.Context(), messageID, userID, reactionType)
	if err != nil {
		log.Errorw("RemoveReaction: Failed to remove reaction", "error", err, "messageID", messageID, "reactionType", reactionType)
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Reaction removed successfully"})
}

// ListReactions godoc
// @Summary List message reactions
// @Description Lists all reactions for a message in a trip's chat (currently not implemented)
// @Tags chat
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Param messageId path string true "Message ID"
// @Success 200 {object} map[string][]string "List of reactions by type"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid trip ID or message ID"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User is not a member of this trip"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Failure 501 {object} map[string]string "Not implemented"
// @Router /trips/{id}/chat/messages/{messageId}/reactions [get]
// @Security BearerAuth
// ListReactions handles the HTTP GET request to list all reactions for a message.
// NOTE: This functionality is not directly supported by the current ChatService interface
// and should be implemented in the future.
func (h *ChatHandler) ListReactions(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Listing reactions is not implemented"})
}

// UpdateLastRead godoc
// @Summary Update last read message
// @Description Updates the user's last read message for a trip's chat
// @Tags chat
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Param request body types.ChatLastReadRequest true "Last read details"
// @Success 200 {object} map[string]string "Success response with message"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid trip ID or missing last read message ID"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User is not a member of this trip"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips/{id}/chat/last-read [put]
// @Security BearerAuth
// UpdateLastRead handles the HTTP PUT request to update the user's last read message.
func (h *ChatHandler) UpdateLastRead(c *gin.Context) {
	log := logger.GetLogger()

	// Get user ID from context
	userID := c.GetString("user_id")
	if userID == "" {
		log.Warn("UpdateLastRead: User ID not found in context")
		_ = c.Error(errors.Unauthorized("unauthorized", "User ID missing from context"))
		return
	}

	// Get trip ID from path
	tripID := c.Param("id")
	if tripID == "" {
		log.Warn("UpdateLastRead: Trip ID path parameter not provided")
		_ = c.Error(errors.ValidationFailed("missing_param", "Trip ID path parameter is required"))
		return
	}

	// Verify the user is a member of the trip
	if err := h.verifyTripMembership(c.Request.Context(), tripID, userID); err != nil {
		log.Warnw("UpdateLastRead: User is not a member of the trip", "userID", userID, "tripID", tripID)
		_ = c.Error(err)
		return
	}

	// Parse request body
	var req types.ChatLastReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnw("UpdateLastRead: Failed to bind JSON request", "error", err)
		_ = c.Error(errors.ValidationFailed("invalid_request", fmt.Sprintf("Invalid request body: %v", err)))
		return
	}

	if req.LastReadMessageID == nil || *req.LastReadMessageID == "" {
		log.Warn("UpdateLastRead: Last read message ID not provided")
		_ = c.Error(errors.ValidationFailed("missing_param", "Last read message ID is required"))
		return
	}

	// Get the chat group for this trip
	groups, err := h.chatService.ListTripGroups(c.Request.Context(), tripID, userID, types.PaginationParams{Limit: 1, Offset: 0})
	if err != nil {
		log.Errorw("UpdateLastRead: Failed to get trip groups", "error", err, "tripID", tripID)
		_ = c.Error(err)
		return
	}

	if len(groups.Groups) == 0 {
		log.Warn("UpdateLastRead: No chat group found for trip", "tripID", tripID)
		_ = c.Error(errors.NotFound("group_not_found", "No chat group found for this trip"))
		return
	}

	groupID := groups.Groups[0].ID

	// Update last read message via service
	err = h.chatService.UpdateLastRead(c.Request.Context(), groupID, userID, *req.LastReadMessageID)
	if err != nil {
		log.Errorw("UpdateLastRead: Failed to update last read message", "error", err, "tripID", tripID, "groupID", groupID)
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Last read message updated successfully"})
}

// ListMembers handles the HTTP GET request to list all members of a trip's chat.
func (h *ChatHandler) ListMembers(c *gin.Context) {
	log := logger.GetLogger()

	// Get user ID from context
	userID := c.GetString("user_id")
	if userID == "" {
		log.Warn("ListMembers: User ID not found in context")
		_ = c.Error(errors.Unauthorized("unauthorized", "User ID missing from context"))
		return
	}

	// Get trip ID from path
	tripID := c.Param("id")
	if tripID == "" {
		log.Warn("ListMembers: Trip ID path parameter not provided")
		_ = c.Error(errors.ValidationFailed("missing_param", "Trip ID path parameter is required"))
		return
	}

	// Verify the user is a member of the trip
	if err := h.verifyTripMembership(c.Request.Context(), tripID, userID); err != nil {
		log.Warnw("ListMembers: User is not a member of the trip", "userID", userID, "tripID", tripID)
		_ = c.Error(err)
		return
	}

	// Get the chat group for this trip
	groups, err := h.chatService.ListTripGroups(c.Request.Context(), tripID, userID, types.PaginationParams{Limit: 1, Offset: 0})
	if err != nil {
		log.Errorw("ListMembers: Failed to get trip groups", "error", err, "tripID", tripID)
		_ = c.Error(err)
		return
	}

	if len(groups.Groups) == 0 {
		log.Warn("ListMembers: No chat group found for trip", "tripID", tripID)
		c.JSON(http.StatusOK, []types.UserResponse{})
		return
	}

	groupID := groups.Groups[0].ID

	// List group members via service
	members, err := h.chatService.ListMembers(c.Request.Context(), groupID, userID)
	if err != nil {
		log.Errorw("ListMembers: Failed to list group members", "error", err, "tripID", tripID, "groupID", groupID)
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, members)
}
