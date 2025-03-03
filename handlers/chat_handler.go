package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/services"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ChatHandler handles chat-related requests
type ChatHandler struct {
	chatService *services.ChatService
	chatStore   store.ChatStore
}

// NewChatHandler creates a new ChatHandler
func NewChatHandler(chatService *services.ChatService, chatStore store.ChatStore) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		chatStore:   chatStore,
	}
}

// CreateChatGroup creates a new chat group
func (h *ChatHandler) CreateChatGroup(c *gin.Context) {
	log := logger.GetLogger()

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		log.Warnw("User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse request body
	var req types.ChatGroupCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnw("Failed to parse request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Create chat group
	group := types.ChatGroup{
		TripID:      req.TripID,
		Name:        req.Name,
		Description: req.Description,
		CreatedBy:   userID.(string),
	}

	groupID, err := h.chatService.CreateChatGroup(c.Request.Context(), group)
	if err != nil {
		log.Errorw("Failed to create chat group", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create chat group"})
		return
	}

	// Get the created group
	createdGroup, err := h.chatStore.GetChatGroup(c.Request.Context(), groupID)
	if err != nil {
		log.Errorw("Failed to get created chat group", "error", err, "groupID", groupID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get created chat group"})
		return
	}

	c.JSON(http.StatusCreated, createdGroup)
}

// GetChatGroup gets a chat group by ID
func (h *ChatHandler) GetChatGroup(c *gin.Context) {
	log := logger.GetLogger()

	// Get user ID from context
	_, exists := c.Get("user_id")
	if !exists {
		log.Warnw("User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get group ID from path
	groupID := c.Param("groupID")
	if groupID == "" {
		log.Warnw("Group ID not provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Group ID is required"})
		return
	}

	// Get the chat group
	group, err := h.chatStore.GetChatGroup(c.Request.Context(), groupID)
	if err != nil {
		log.Errorw("Failed to get chat group", "error", err, "groupID", groupID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get chat group"})
		return
	}

	c.JSON(http.StatusOK, group)
}

// UpdateChatGroup updates a chat group
func (h *ChatHandler) UpdateChatGroup(c *gin.Context) {
	log := logger.GetLogger()

	// Get user ID from context
	_, exists := c.Get("user_id")
	if !exists {
		log.Warnw("User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get group ID from path
	groupID := c.Param("groupID")
	if groupID == "" {
		log.Warnw("Group ID not provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Group ID is required"})
		return
	}

	// Parse request body
	var req types.ChatGroupUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnw("Failed to parse request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Get the chat group to verify it exists
	_, err := h.chatStore.GetChatGroup(c.Request.Context(), groupID)
	if err != nil {
		log.Errorw("Failed to get chat group", "error", err, "groupID", groupID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get chat group"})
		return
	}

	// Update the chat group
	err = h.chatStore.UpdateChatGroup(c.Request.Context(), groupID, req)
	if err != nil {
		log.Errorw("Failed to update chat group", "error", err, "groupID", groupID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update chat group"})
		return
	}

	// Get the updated group
	updatedGroup, err := h.chatStore.GetChatGroup(c.Request.Context(), groupID)
	if err != nil {
		log.Errorw("Failed to get updated chat group", "error", err, "groupID", groupID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get updated chat group"})
		return
	}

	c.JSON(http.StatusOK, updatedGroup)
}

// DeleteChatGroup deletes a chat group
func (h *ChatHandler) DeleteChatGroup(c *gin.Context) {
	log := logger.GetLogger()

	// Get user ID from context
	_, exists := c.Get("user_id")
	if !exists {
		log.Warnw("User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get group ID from path
	groupID := c.Param("groupID")
	if groupID == "" {
		log.Warnw("Group ID not provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Group ID is required"})
		return
	}

	// Get the chat group to verify it exists
	_, err := h.chatStore.GetChatGroup(c.Request.Context(), groupID)
	if err != nil {
		log.Errorw("Failed to get chat group", "error", err, "groupID", groupID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get chat group"})
		return
	}

	// Delete the chat group
	err = h.chatStore.DeleteChatGroup(c.Request.Context(), groupID)
	if err != nil {
		log.Errorw("Failed to delete chat group", "error", err, "groupID", groupID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete chat group"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Chat group deleted successfully"})
}

// ListChatGroups lists all chat groups for a trip
func (h *ChatHandler) ListChatGroups(c *gin.Context) {
	log := logger.GetLogger()

	// Get user ID from context
	_, exists := c.Get("user_id")
	if !exists {
		log.Warnw("User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get trip ID from query
	tripID := c.Query("tripID")
	if tripID == "" {
		log.Warnw("Trip ID not provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trip ID is required"})
		return
	}

	// Parse pagination parameters
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if err != nil {
		limit = 10
	}

	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil {
		offset = 0
	}

	// List chat groups
	groups, err := h.chatStore.ListChatGroupsByTrip(c.Request.Context(), tripID, limit, offset)
	if err != nil {
		log.Errorw("Failed to list chat groups", "error", err, "tripID", tripID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list chat groups"})
		return
	}

	c.JSON(http.StatusOK, groups)
}

// ListChatMessages lists all messages in a chat group
func (h *ChatHandler) ListChatMessages(c *gin.Context) {
	log := logger.GetLogger()

	// Get user ID from context
	_, exists := c.Get("user_id")
	if !exists {
		log.Warnw("User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get group ID from path
	groupID := c.Param("groupID")
	if groupID == "" {
		log.Warnw("Group ID not provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Group ID is required"})
		return
	}

	// Parse pagination parameters
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil {
		limit = 20
	}

	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil {
		offset = 0
	}

	// List chat messages
	messages, err := h.chatStore.ListChatMessages(c.Request.Context(), groupID, limit, offset)
	if err != nil {
		log.Errorw("Failed to list chat messages", "error", err, "groupID", groupID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list chat messages"})
		return
	}

	c.JSON(http.StatusOK, messages)
}

// ListChatGroupMembers lists all members of a chat group
func (h *ChatHandler) ListChatGroupMembers(c *gin.Context) {
	log := logger.GetLogger()

	// Get user ID from context
	_, exists := c.Get("user_id")
	if !exists {
		log.Warnw("User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get group ID from path
	groupID := c.Param("groupID")
	if groupID == "" {
		log.Warnw("Group ID not provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Group ID is required"})
		return
	}

	// List chat group members
	members, err := h.chatStore.ListChatGroupMembers(c.Request.Context(), groupID)
	if err != nil {
		log.Errorw("Failed to list chat group members", "error", err, "groupID", groupID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list chat group members"})
		return
	}

	c.JSON(http.StatusOK, members)
}

// HandleChatWebSocket handles WebSocket connections for chat
func (h *ChatHandler) HandleChatWebSocket(c *gin.Context) {
	log := logger.GetLogger()

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		log.Warnw("User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get group ID from path
	groupID := c.Param("groupID")
	if groupID == "" {
		log.Warnw("Group ID not provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Group ID is required"})
		return
	}

	// Get the chat group to verify it exists
	_, err := h.chatStore.GetChatGroup(c.Request.Context(), groupID)
	if err != nil {
		log.Errorw("Failed to get chat group", "error", err, "groupID", groupID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get chat group"})
		return
	}

	// Get WebSocket connection from middleware
	conn, ok := c.MustGet("wsConnection").(*middleware.SafeConn)
	if !ok {
		log.Error("WebSocket connection not found in context")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Register the connection with the chat service
	h.chatService.RegisterConnection(groupID, userID.(string), conn)

	// Create context with cancellation for cleanup
	wsCtx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Set up a goroutine to read messages from the WebSocket
	go func() {
		defer func() {
			// Unregister the connection when done
			h.chatService.UnregisterConnection(groupID, userID.(string))

			// Close the connection
			if err := conn.Close(); err != nil {
				log.Warnw("Error closing WebSocket connection", "error", err, "groupID", groupID, "userID", userID)
			}

			// Cancel the context
			cancel()
		}()

		for {
			// Check if the context is done
			select {
			case <-wsCtx.Done():
				return
			default:
				// Continue processing
			}

			// Read a message from the connection
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Infow("WebSocket connection closed normally", "groupID", groupID, "userID", userID)
				} else {
					log.Warnw("Error reading WebSocket message", "error", err, "groupID", groupID, "userID", userID)
				}
				return
			}

			// Handle the message
			if err := h.chatService.HandleWebSocketMessage(wsCtx, conn, message, userID.(string)); err != nil {
				log.Errorw("Failed to handle WebSocket message", "error", err, "groupID", groupID, "userID", userID)

				// Send an error message back to the client
				errorMsg := types.WebSocketChatMessage{
					Type:  types.WebSocketMessageTypeError,
					Error: err.Error(),
				}

				errorJSON, err := json.Marshal(errorMsg)
				if err != nil {
					log.Errorw("Failed to marshal error message", "error", err)
					continue
				}

				if err := conn.WriteMessage(websocket.TextMessage, errorJSON); err != nil {
					log.Errorw("Failed to send error message", "error", err)
				}
			}
		}
	}()

	// Keep the handler running until the request context is done
	<-c.Request.Context().Done()
}

// UpdateLastReadMessage updates the last read message for a user in a chat group
func (h *ChatHandler) UpdateLastReadMessage(c *gin.Context) {
	log := logger.GetLogger()

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		log.Warnw("User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get group ID from path
	groupID := c.Param("groupID")
	if groupID == "" {
		log.Warnw("Group ID not provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Group ID is required"})
		return
	}

	// Get message ID from request body
	var req struct {
		MessageID string `json:"messageId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnw("Failed to parse request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate message ID
	if req.MessageID == "" {
		log.Warnw("Empty message ID provided", "groupID", groupID, "userID", userID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Message ID cannot be empty"})
		return
	}

	// Update the last read message
	err := h.chatService.UpdateLastReadMessage(c.Request.Context(), groupID, userID.(string), req.MessageID)
	if err != nil {
		log.Errorw("Failed to update last read message", "error", err, "groupID", groupID, "userID", userID, "messageID", req.MessageID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update last read message"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Last read message updated successfully"})
}
