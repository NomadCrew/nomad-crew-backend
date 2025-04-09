// Package handlers contains the HTTP handlers for the application's API endpoints.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/models"
	services "github.com/NomadCrew/nomad-crew-backend/models/chat/service"
	stor "github.com/NomadCrew/nomad-crew-backend/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// ChatHandler encapsulates dependencies and methods for handling chat-related HTTP requests.
type ChatHandler struct {
	// chatService contains business logic for chat operations (e.g., creating groups).
	chatService *services.ChatService
	// chatStore provides direct data access methods for chat entities.
	chatStore store.ChatStore
	// userStore provides access to user data.
	userStore stor.UserStore
}

// NewChatHandler creates a new instance of ChatHandler with required dependencies.
func NewChatHandler(chatService *services.ChatService, chatStore store.ChatStore, userStore stor.UserStore) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		chatStore:   chatStore,
		userStore:   userStore,
	}
}

// CreateChatGroup handles the HTTP POST request to create a new chat group for a trip.
// It expects trip details in the request body and the creator's user ID from the context.
// @Summary Create Chat Group
// @Description Creates a new chat group associated with a trip.
// @Tags chat
// @Accept json
// @Produce json
// @Param group body types.ChatGroupCreateRequest true "Chat Group Creation Payload"
// @Success 201 {object} types.ChatGroup "Successfully created chat group"
// @Failure 400 {object} errors.HTTPError "Invalid request body"
// @Failure 401 {object} errors.HTTPError "Unauthorized (user ID not found in context)"
// @Failure 500 {object} errors.HTTPError "Internal server error (failed to create or retrieve group)"
// @Router /chat/groups [post]
func (h *ChatHandler) CreateChatGroup(c *gin.Context) {
	log := logger.GetLogger()

	userIDAny, exists := c.Get(string(middleware.UserIDKey)) // Use string cast
	if !exists {
		log.Warn("CreateChatGroup: User ID not found in context")
		_ = c.Error(errors.Unauthorized("unauthorized", "User ID missing from context"))
		return
	}
	userID, ok := userIDAny.(string)
	if !ok || userID == "" {
		log.Error("CreateChatGroup: User ID in context is not a valid string")
		_ = c.Error(errors.InternalServerError("Invalid user ID in context"))
		return
	}

	var req types.ChatGroupCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnw("CreateChatGroup: Failed to bind JSON request", "error", err)
		_ = c.Error(errors.ValidationFailed("invalid_request", fmt.Sprintf("Invalid request body: %v", err)))
		return
	}

	// Prepare the group data for creation.
	group := types.ChatGroup{
		TripID:      req.TripID,
		Name:        req.Name,
		Description: req.Description,
		CreatedBy:   userID,
	}

	// Call the service to create the group.
	groupID, err := h.chatService.CreateChatGroup(c.Request.Context(), group)
	if err != nil {
		log.Errorw("CreateChatGroup: Failed to create chat group via service", "error", err)
		_ = c.Error(err) // Propagate the error from the service/store layer
		return
	}

	// Retrieve the newly created group to return it in the response.
	createdGroup, err := h.chatStore.GetChatGroup(c.Request.Context(), groupID)
	if err != nil {
		// Log the error, but might still return success (201) if creation itself was okay.
		// However, failing to retrieve is usually indicative of a problem.
		log.Errorw("CreateChatGroup: Failed to retrieve created chat group", "error", err, "groupID", groupID)
		_ = c.Error(errors.InternalServerError(fmt.Sprintf("Failed to retrieve created chat group %s", groupID)))
		return
	}

	c.JSON(http.StatusCreated, createdGroup)
}

// GetChatGroup handles the HTTP GET request to retrieve a specific chat group by its ID.
// @Summary Get Chat Group
// @Description Retrieves details of a specific chat group by ID.
// @Tags chat
// @Produce json
// @Param groupID path string true "Chat Group ID"
// @Success 200 {object} types.ChatGroup "Successfully retrieved chat group"
// @Failure 400 {object} errors.HTTPError "Group ID parameter is missing"
// @Failure 401 {object} errors.HTTPError "Unauthorized (user ID not found in context)"
// @Failure 404 {object} errors.HTTPError "Chat group not found"
// @Failure 500 {object} errors.HTTPError "Internal server error (failed to retrieve group)"
// @Router /chat/groups/{groupID} [get]
func (h *ChatHandler) GetChatGroup(c *gin.Context) {
	log := logger.GetLogger()

	_, exists := c.Get(string(middleware.UserIDKey)) // Use string cast
	if !exists {
		log.Warn("GetChatGroup: User ID not found in context")
		_ = c.Error(errors.Unauthorized("unauthorized", "User ID missing from context"))
		return
	}

	groupID := c.Param("groupID")
	if groupID == "" {
		log.Warn("GetChatGroup: Group ID not provided in path")
		_ = c.Error(errors.ValidationFailed("missing_param", "Group ID path parameter is required"))
		return
	}

	// Retrieve the group directly from the store.
	group, err := h.chatStore.GetChatGroup(c.Request.Context(), groupID)
	if err != nil {
		log.Errorw("GetChatGroup: Failed to get chat group from store", "error", err, "groupID", groupID)
		_ = c.Error(err) // Propagate store/db error (e.g., NotFound, DatabaseError)
		return
	}

	c.JSON(http.StatusOK, group)
}

// UpdateChatGroup handles the HTTP PUT request to update an existing chat group.
// @Summary Update Chat Group
// @Description Updates the name and/or description of a specific chat group.
// @Tags chat
// @Accept json
// @Produce json
// @Param groupID path string true "Chat Group ID"
// @Param group body types.ChatGroupUpdateRequest true "Chat Group Update Payload"
// @Success 200 {object} types.ChatGroup "Successfully updated chat group"
// @Failure 400 {object} errors.HTTPError "Group ID parameter missing or Invalid request body"
// @Failure 401 {object} errors.HTTPError "Unauthorized (user ID not found in context)"
// @Failure 404 {object} errors.HTTPError "Chat group not found"
// @Failure 500 {object} errors.HTTPError "Internal server error (failed to update or retrieve group)"
// @Router /chat/groups/{groupID} [put]
func (h *ChatHandler) UpdateChatGroup(c *gin.Context) {
	log := logger.GetLogger()

	_, exists := c.Get(string(middleware.UserIDKey)) // Use string cast
	if !exists {
		log.Warn("UpdateChatGroup: User ID not found in context")
		_ = c.Error(errors.Unauthorized("unauthorized", "User ID missing from context"))
		return
	}

	groupID := c.Param("groupID")
	if groupID == "" {
		log.Warn("UpdateChatGroup: Group ID not provided in path")
		_ = c.Error(errors.ValidationFailed("missing_param", "Group ID path parameter is required"))
		return
	}

	var req types.ChatGroupUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnw("UpdateChatGroup: Failed to bind JSON request", "error", err, "groupID", groupID)
		_ = c.Error(errors.ValidationFailed("invalid_request", fmt.Sprintf("Invalid request body: %v", err)))
		return
	}

	// Update the group in the store.
	err := h.chatStore.UpdateChatGroup(c.Request.Context(), groupID, req)
	if err != nil {
		log.Errorw("UpdateChatGroup: Failed to update chat group in store", "error", err, "groupID", groupID)
		_ = c.Error(err) // Propagate store/db error (e.g., NotFound, DatabaseError)
		return
	}

	// Retrieve the updated group to return it.
	updatedGroup, err := h.chatStore.GetChatGroup(c.Request.Context(), groupID)
	if err != nil {
		log.Errorw("UpdateChatGroup: Failed to retrieve updated chat group", "error", err, "groupID", groupID)
		_ = c.Error(errors.InternalServerError(fmt.Sprintf("Failed to retrieve updated chat group %s", groupID)))
		return
	}

	c.JSON(http.StatusOK, updatedGroup)
}

// DeleteChatGroup handles the HTTP DELETE request to remove a chat group.
// @Summary Delete Chat Group
// @Description Deletes a specific chat group by ID.
// @Tags chat
// @Produce json
// @Param groupID path string true "Chat Group ID"
// @Success 200 {object} gin.H{"message"=\"Chat group deleted successfully\"} "Successfully deleted chat group"
// @Failure 400 {object} errors.HTTPError "Group ID parameter is missing"
// @Failure 401 {object} errors.HTTPError "Unauthorized (user ID not found in context)"
// @Failure 404 {object} errors.HTTPError "Chat group not found"
// @Failure 500 {object} errors.HTTPError "Internal server error (failed to delete group)"
// @Router /chat/groups/{groupID} [delete]
func (h *ChatHandler) DeleteChatGroup(c *gin.Context) {
	log := logger.GetLogger()

	_, exists := c.Get(string(middleware.UserIDKey)) // Use string cast
	if !exists {
		log.Warn("DeleteChatGroup: User ID not found in context")
		_ = c.Error(errors.Unauthorized("unauthorized", "User ID missing from context"))
		return
	}

	groupID := c.Param("groupID")
	if groupID == "" {
		log.Warn("DeleteChatGroup: Group ID not provided in path")
		_ = c.Error(errors.ValidationFailed("missing_param", "Group ID path parameter is required"))
		return
	}

	// Delete the group via the store.
	err := h.chatStore.DeleteChatGroup(c.Request.Context(), groupID)
	if err != nil {
		log.Errorw("DeleteChatGroup: Failed to delete chat group from store", "error", err, "groupID", groupID)
		_ = c.Error(err) // Propagate store/db error (e.g., NotFound, DatabaseError)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Chat group deleted successfully"})
}

// ListChatGroups handles the HTTP GET request to list chat groups for a specific trip.
// Supports pagination via query parameters (limit, offset).
// @Summary List Chat Groups
// @Description Retrieves a paginated list of chat groups for a given trip ID.
// @Tags chat
// @Produce json
// @Param tripID query string true "Trip ID to filter groups by"
// @Param limit query int false "Pagination limit (default 10)"
// @Param offset query int false "Pagination offset (default 0)"
// @Success 200 {object} types.ChatGroupPaginatedResponse "Successfully retrieved chat groups"
// @Failure 400 {object} errors.HTTPError "Trip ID query parameter is missing or invalid pagination parameters"
// @Failure 401 {object} errors.HTTPError "Unauthorized (user ID not found in context)"
// @Failure 500 {object} errors.HTTPError "Internal server error (failed to list groups)"
// @Router /chat/groups [get]
func (h *ChatHandler) ListChatGroups(c *gin.Context) {
	log := logger.GetLogger()

	_, exists := c.Get(string(middleware.UserIDKey)) // Use string cast
	if !exists {
		log.Warn("ListChatGroups: User ID not found in context")
		_ = c.Error(errors.Unauthorized("unauthorized", "User ID missing from context"))
		return
	}

	tripID := c.Query("tripID")
	if tripID == "" {
		log.Warn("ListChatGroups: Trip ID query parameter not provided")
		_ = c.Error(errors.ValidationFailed("missing_query_param", "Trip ID query parameter is required"))
		return
	}

	// Parse pagination parameters with defaults and basic validation.
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if err != nil || limit <= 0 {
		log.Warnw("ListChatGroups: Invalid limit query parameter", "value", c.Query("limit"), "error", err)
		limit = 10 // Default limit
	}

	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		log.Warnw("ListChatGroups: Invalid offset query parameter", "value", c.Query("offset"), "error", err)
		offset = 0 // Default offset
	}

	// Retrieve the list from the store.
	groupsResponse, err := h.chatStore.ListChatGroupsByTrip(c.Request.Context(), tripID, limit, offset)
	if err != nil {
		log.Errorw("ListChatGroups: Failed to list chat groups from store", "error", err, "tripID", tripID)
		_ = c.Error(err) // Propagate store/db error
		return
	}

	c.JSON(http.StatusOK, groupsResponse)
}

// ListChatMessages handles the HTTP GET request to list messages in a chat group.
// Supports pagination via query parameters (limit, offset).
// @Summary List Chat Messages
// @Description Retrieves a paginated list of messages for a given chat group ID.
// @Tags chat
// @Produce json
// @Param groupID path string true "Chat Group ID to list messages for"
// @Param limit query int false "Pagination limit (default 50)"
// @Param offset query int false "Pagination offset (default 0)"
// @Success 200 {object} types.ChatMessagePaginatedResponse "Successfully retrieved chat messages"
// @Failure 400 {object} errors.HTTPError "Group ID parameter missing or invalid pagination parameters"
// @Failure 401 {object} errors.HTTPError "Unauthorized"
// @Failure 500 {object} errors.HTTPError "Internal server error"
// @Router /chat/groups/{groupID}/messages [get]
func (h *ChatHandler) ListChatMessages(c *gin.Context) {
	log := logger.GetLogger()

	_, exists := c.Get(string(middleware.UserIDKey)) // Use string cast
	if !exists {
		log.Warn("ListChatMessages: User ID not found in context")
		_ = c.Error(errors.Unauthorized("unauthorized", "User ID missing from context"))
		return
	}

	groupID := c.Param("groupID")
	if groupID == "" {
		log.Warn("ListChatMessages: Group ID path parameter not provided")
		_ = c.Error(errors.ValidationFailed("missing_param", "Group ID path parameter is required"))
		return
	}

	// Parse pagination parameters.
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil || limit <= 0 {
		log.Warnw("ListChatMessages: Invalid limit query parameter", "value", c.Query("limit"), "error", err)
		limit = 50 // Default limit
	}

	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		log.Warnw("ListChatMessages: Invalid offset query parameter", "value", c.Query("offset"), "error", err)
		offset = 0 // Default offset
	}

	// Create pagination parameters struct.
	paginationParams := types.PaginationParams{
		Limit:  limit,
		Offset: offset,
	}

	// Call the store method with the correct signature.
	messages, total, err := h.chatStore.ListChatMessages(c.Request.Context(), groupID, paginationParams)
	if err != nil {
		log.Errorw("ListChatMessages: Failed to list chat messages from store", "error", err, "groupID", groupID)
		_ = c.Error(err) // Propagate store/db error
		return
	}

	// Fetch user details for each message and construct ChatMessageWithUser slice.
	messagesWithUser := make([]types.ChatMessageWithUser, 0, len(messages))
	for _, msg := range messages {
		userIDUUID, err := uuid.Parse(msg.UserID)
		if err != nil {
			// Log the error but potentially continue, or return an error depending on requirements
			log.Errorw("ListChatMessages: Failed to parse user ID for message", "error", err, "userID", msg.UserID, "messageID", msg.ID)
			// Decide how to handle this - skip message, return error? Skipping for now.
			continue
		}

		// Fetch user details using UserStore
		// Assuming UserStore.GetUserByID returns *models.User
		userModel, err := h.userStore.GetUserByID(c.Request.Context(), userIDUUID)
		if err != nil {
			// Log the error but potentially continue, or return an error.
			log.Errorw("ListChatMessages: Failed to get user details from store", "error", err, "userID", msg.UserID)
			// Skipping message if user not found or error occurs
			continue
		}

		// Convert models.User to types.UserResponse
		userResponse := types.UserResponse{
			ID:          userModel.ID.String(),
			Email:       userModel.Email,
			Username:    userModel.Username,
			FirstName:   userModel.FirstName,
			LastName:    userModel.LastName,
			AvatarURL:   userModel.ProfilePictureURL,
			DisplayName: getUserDisplayName(userModel),
		}

		messagesWithUser = append(messagesWithUser, types.ChatMessageWithUser{
			Message: msg,          // The original ChatMessage
			User:    userResponse, // The fetched user details
		})
	}

	// Construct the paginated response according to types.ChatMessagePaginatedResponse definition.
	response := types.ChatMessagePaginatedResponse{
		Messages: messagesWithUser,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
	}

	c.JSON(http.StatusOK, response)
}

// getUserDisplayName generates a display name from user data
func getUserDisplayName(user *models.User) string {
	if user.FirstName != "" {
		if user.LastName != "" {
			return fmt.Sprintf("%s %s", user.FirstName, user.LastName)
		}
		return user.FirstName
	}
	return user.Username
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
