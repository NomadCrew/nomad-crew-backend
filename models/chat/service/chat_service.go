package service // Changed from services

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors" // Renamed import alias
	"github.com/NomadCrew/nomad-crew-backend/internal/events"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"

	// Removed trip_service import as TripStore interface is used
	"github.com/NomadCrew/nomad-crew-backend/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gorilla/websocket"
)

// WebSocketConnection defines the interface for WebSocket connections used by ChatService.
// This allows for decoupling and easier testing.
type WebSocketConnection interface {
	WriteMessage(messageType int, data []byte) error
	Close() error
	// Add other methods from middleware.SafeConn if needed by ChatService, e.g., ReadMessage()
}

// ChatService handles chat operations
type ChatService struct {
	chatStore      store.ChatStore
	tripStore      store.TripStore
	eventService   *events.Service
	connections    map[string]map[string]WebSocketConnection // map[tripID]map[userID]WebSocketConnection
	connectionsMux sync.RWMutex
}

// NewChatService creates a new ChatService
func NewChatService(chatStore store.ChatStore, tripStore store.TripStore, eventService *events.Service) *ChatService {
	service := &ChatService{
		chatStore:    chatStore,
		tripStore:    tripStore,
		eventService: eventService,
		connections:  make(map[string]map[string]WebSocketConnection),
	}

	// Register event handlers
	if err := eventService.RegisterHandler("chat-message-handler", service); err != nil {
		panic(fmt.Sprintf("failed to register chat message handler: %v", err))
	}

	return service
}

// RegisterConnection registers a WebSocket connection for a user in a trip
// Accept the interface type
func (s *ChatService) RegisterConnection(tripID, userID string, conn WebSocketConnection) {
	log := logger.GetLogger()
	log.Infow("Registering WebSocket connection", "tripID", tripID, "userID", userID)

	s.connectionsMux.Lock()
	defer s.connectionsMux.Unlock()

	// Initialize the trip map if it doesn't exist
	if _, ok := s.connections[tripID]; !ok {
		s.connections[tripID] = make(map[string]WebSocketConnection)
	}

	// Store the connection (which implements the interface)
	s.connections[tripID][userID] = conn

	// Send a welcome message
	welcomeMsg := types.WebSocketChatMessage{
		Type:    types.WebSocketMessageTypeInfo,
		Message: "Connected to chat",
	}

	welcomeJSON, err := json.Marshal(welcomeMsg)
	if err != nil {
		log.Errorw("Failed to marshal welcome message", "error", err)
		// Consider sending an error message back? For now, just log.
		return
	}

	// Use the interface method
	if err := conn.WriteMessage(websocket.TextMessage, welcomeJSON); err != nil {
		log.Errorw("Failed to send welcome message", "error", err)
		// Might indicate a problem with the connection, consider unregistering here?
	}
}

// UnregisterConnection removes a WebSocket connection for a user in a trip
func (s *ChatService) UnregisterConnection(tripID, userID string) {
	log := logger.GetLogger()
	log.Infow("Unregistering WebSocket connection", "tripID", tripID, "userID", userID)

	s.connectionsMux.Lock()

	// Check if the trip exists
	userConns, tripExists := s.connections[tripID]
	if !tripExists {
		s.connectionsMux.Unlock()
		return
	}

	// Check if user connection exists
	conn, userExists := userConns[userID]
	if !userExists {
		s.connectionsMux.Unlock()
		return
	}

	// Remove the connection from the map
	delete(s.connections[tripID], userID)

	// Remove the trip if it's empty
	if len(s.connections[tripID]) == 0 {
		delete(s.connections, tripID)
	}
	s.connectionsMux.Unlock() // Unlock before potentially long Close operation

	// Close the connection outside the main lock
	if conn != nil {
		if err := conn.Close(); err != nil {
			log.Warnw("Error closing WebSocket connection during unregister", "error", err, "tripID", tripID, "userID", userID)
		}
	}
}

// BroadcastMessage broadcasts a message to all connections in a trip
func (s *ChatService) BroadcastMessage(ctx context.Context, wsMessage types.WebSocketChatMessage, tripID string, excludeUserID string) {
	log := logger.GetLogger()

	// Marshal the message
	messageJSON, err := json.Marshal(wsMessage)
	if err != nil {
		log.Errorw("Failed to marshal WebSocket message for broadcast", "error", err, "tripID", tripID)
		// Cannot broadcast if marshalling fails
		return
	}

	s.connectionsMux.RLock()

	// Check if the trip exists
	groupConns, ok := s.connections[tripID]
	if !ok {
		s.connectionsMux.RUnlock()
		return // No connections for this trip
	}

	// Create a copy of connections to iterate over outside the lock
	connsToBroadcast := make(map[string]WebSocketConnection)
	for uid, conn := range groupConns {
		if excludeUserID == "" || uid != excludeUserID {
			connsToBroadcast[uid] = conn
		}
	}
	s.connectionsMux.RUnlock()

	// Send the message to all relevant connections
	var wg sync.WaitGroup
	for userID, conn := range connsToBroadcast {
		wg.Add(1)
		go func(uid string, c WebSocketConnection) {
			defer wg.Done()
			// Use interface method
			if err := c.WriteMessage(websocket.TextMessage, messageJSON); err != nil {
				// Log error and potentially unregister the problematic connection
				log.Errorw("Failed to broadcast message to user", "error", err, "tripID", tripID, "userID", uid)
				// Consider unregistering the connection if write fails consistently
				// This needs careful handling due to potential concurrent map writes
				// s.UnregisterConnection(tripID, uid)
			}
		}(userID, conn)
	}
	wg.Wait() // Wait for all broadcasts to attempt sending
}

// CreateChatGroup creates a new chat group
// Note: Currently assumes one main chat group per trip, created implicitly or explicitly.
// This implementation might need adjustment based on product requirements (e.g., multiple named groups per trip).
func (s *ChatService) CreateChatGroup(ctx context.Context, group types.ChatGroup) (string, error) {
	log := logger.GetLogger()

	// Validate input
	if group.TripID == "" || group.CreatedBy == "" || group.Name == "" {
		return "", apperrors.ValidationFailed("missing_fields", "TripID, CreatedBy, and Name are required to create a chat group")
	}

	// Check if the trip exists
	_, err := s.tripStore.GetTrip(ctx, group.TripID) // Use GetTrip for existence check
	if err != nil {
		log.Warnw("Attempted to create chat group for non-existent trip", "error", err, "tripID", group.TripID)
		if appErr, ok := err.(*apperrors.AppError); ok && appErr.Type == apperrors.NotFoundError {
			return "", apperrors.NotFound("Trip", group.TripID)
		}
		return "", apperrors.Wrap(err, apperrors.DatabaseError, "failed to verify trip existence")
	}

	// Check if the user is a member of the trip
	role, err := s.tripStore.GetUserRole(ctx, group.TripID, group.CreatedBy)
	if err != nil {
		log.Errorw("Failed to get user role for chat group creation", "error", err, "tripID", group.TripID, "userID", group.CreatedBy)
		if appErr, ok := err.(*apperrors.AppError); ok && appErr.Type == apperrors.NotFoundError {
			// If role check returns NotFound, user is not a member
			return "", apperrors.Forbidden("access_denied", "User is not a member of the trip")
		}
		return "", apperrors.Wrap(err, apperrors.DatabaseError, "Failed to verify user membership")
	}

	if role == types.MemberRoleNone { // Explicit check just in case GetUserRole doesn't return NotFoundError
		log.Warnw("User is not a member of the trip", "tripID", group.TripID, "userID", group.CreatedBy)
		return "", apperrors.Forbidden("access_denied", "User is not a member of the trip")
	}

	// Create the chat group in the store
	groupID, err := s.chatStore.CreateChatGroup(ctx, group)
	if err != nil {
		log.Errorw("Failed to create chat group in store", "error", err, "tripID", group.TripID, "groupName", group.Name)
		// Check for specific AppError types (e.g., Conflict) or wrap as DB error
		if appErr, ok := err.(*apperrors.AppError); ok {
			return "", appErr // Propagate specific errors like Conflict
		}
		return "", apperrors.NewDatabaseError(err)
	}
	log.Infow("Chat group created successfully", "groupID", groupID, "tripID", group.TripID, "name", group.Name)

	// Add all current trip members to the chat group automatically
	members, err := s.tripStore.GetTripMembers(ctx, group.TripID)
	if err != nil {
		log.Errorw("Failed to list trip members for auto-adding to chat group", "error", err, "tripID", group.TripID, "groupID", groupID)
		// Continue, group created but members might need manual add or retry
		// Return the groupID but log the error critically
		return groupID, apperrors.Wrap(err, apperrors.ServerError, "Group created, but failed to automatically add members")
	}

	memberAddErrors := false
	for _, member := range members {
		if member.UserID == "" {
			continue
		}
		err := s.chatStore.AddChatGroupMember(ctx, groupID, member.UserID)
		if err != nil {
			log.Warnw("Failed to auto-add member to chat group", "error", err, "groupID", groupID, "userID", member.UserID)
			memberAddErrors = true
			// Don't stop, try adding others
		}
	}

	if memberAddErrors {
		// Return success but maybe indicate partial failure if needed?
		log.Warnw("Some members failed to be auto-added to the chat group", "groupID", groupID)
	}

	// Publish ChatGroupCreated event
	err = events.PublishEventWithContext(
		s.eventService,
		ctx,
		string(types.EventTypeChatGroupCreated), // Assuming this event type exists
		group.TripID,                            // Use TripID as resource ID
		group.CreatedBy,                         // User who initiated
		map[string]interface{}{
			"groupID":   groupID,
			"groupName": group.Name,
		},
		"chat-service",
	)
	if err != nil {
		log.Warnw("Failed to publish chat group created event", "error", err, "groupID", groupID)
		// Non-fatal, group is created
	}

	return groupID, nil
}

// SendChatMessage sends a chat message to a trip
func (s *ChatService) SendChatMessage(ctx context.Context, message types.ChatMessage, user types.UserResponse) (string, error) {
	log := logger.GetLogger()

	// Store the message
	messageID, err := s.chatStore.CreateChatMessage(ctx, message)
	if err != nil {
		log.Errorw("Failed to store chat message", "error", err)
		return "", apperrors.Wrap(err, apperrors.DatabaseError, "failed to store chat message")
	}
	message.ID = messageID

	// Create and publish the event
	event := types.Event{
		BaseEvent: types.BaseEvent{
			ID:        messageID,
			Type:      types.EventTypeChatMessageSent,
			TripID:    message.TripID,
			UserID:    message.UserID,
			Timestamp: message.CreatedAt,
			Version:   1,
		},
		Metadata: types.EventMetadata{
			Source: "chat-service",
		},
		Payload: mustMarshal(types.ChatMessageEvent{
			MessageID: messageID,
			TripID:    message.TripID,
			Content:   message.Content,
			User:      user,
			Timestamp: message.CreatedAt,
		}),
	}

	if err := s.eventService.Publish(ctx, message.TripID, event); err != nil {
		log.Errorw("Failed to publish chat message event", "error", err)
		// Don't return error since message is stored
	}

	return messageID, nil
}

// UpdateChatMessage updates an existing chat message
func (s *ChatService) UpdateChatMessage(ctx context.Context, messageID, userID, newContent string) error {
	log := logger.GetLogger()

	// Get the original message
	message, err := s.chatStore.GetChatMessageByID(ctx, messageID)
	if err != nil {
		log.Errorw("Failed to get chat message for update", "error", err, "messageID", messageID)
		return apperrors.Wrap(err, apperrors.DatabaseError, "failed to get chat message")
	}

	// Update the message
	message.Content = newContent
	message.IsEdited = true
	message.UpdatedAt = time.Now()

	if err := s.chatStore.UpdateChatMessage(ctx, messageID, newContent); err != nil {
		log.Errorw("Failed to update chat message", "error", err)
		return apperrors.Wrap(err, apperrors.DatabaseError, "failed to update chat message")
	}

	// Get user info for the event
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		log.Errorw("Failed to get user info for chat message update", "error", err)
		// Continue since message is updated
	}

	// Create and publish the event
	event := types.Event{
		BaseEvent: types.BaseEvent{
			ID:        messageID,
			Type:      types.EventTypeChatMessageEdited,
			TripID:    message.TripID,
			UserID:    userID,
			Timestamp: message.UpdatedAt,
			Version:   1,
		},
		Metadata: types.EventMetadata{
			Source: "chat-service",
		},
		Payload: mustMarshal(types.ChatMessageEvent{
			MessageID: messageID,
			TripID:    message.TripID,
			Content:   newContent,
			User:      *ConvertSupabaseUserToUserResponse(user),
			Timestamp: message.UpdatedAt,
		}),
	}

	if err := s.eventService.Publish(ctx, message.TripID, event); err != nil {
		log.Errorw("Failed to publish chat message update event", "error", err)
		// Don't return error since message is updated
	}

	return nil
}

// DeleteChatMessage deletes a chat message
func (s *ChatService) DeleteChatMessage(ctx context.Context, messageID, userID string) error {
	log := logger.GetLogger()

	// Validate input
	if messageID == "" || userID == "" {
		return apperrors.ValidationFailed("missing_fields", "MessageID and UserID are required")
	}

	// Get the original message (using renamed store method)
	originalMessage, err := s.chatStore.GetChatMessageByID(ctx, messageID)
	if err != nil {
		log.Errorw("Failed to get original message for delete", "error", err, "messageID", messageID)
		if appErr, ok := err.(*apperrors.AppError); ok && appErr.Type == apperrors.NotFoundError {
			return apperrors.NotFound("ChatMessage", messageID)
		}
		return apperrors.Wrap(err, apperrors.DatabaseError, "failed to retrieve message for delete")
	}

	// Check ownership (or potentially allow trip owner/admin role)
	// Simple ownership check for now:
	if originalMessage.UserID != userID {
		// TODO: Implement role check (e.g., allow trip owner/admin to delete)
		// role, err := s.tripStore.GetUserRole(ctx, originalMessage.TripID, userID)
		// if err != nil || (role != types.MemberRoleOwner && role != types.MemberRoleAdmin) {
		log.Warnw("User attempted to delete another user's message without permission", "messageID", messageID, "requestingUserID", userID, "originalUserID", originalMessage.UserID)
		return apperrors.Forbidden("permission_denied", "Cannot delete another user's message")
		// }
	}

	// Delete the message from the store (using renamed interface method)
	if err := s.chatStore.DeleteChatMessage(ctx, messageID); err != nil {
		log.Errorw("Failed to delete chat message from store", "error", err, "messageID", messageID)
		if appErr, ok := err.(*apperrors.AppError); ok {
			return appErr
		}
		return apperrors.NewDatabaseError(err)
	}

	// Prepare WebSocket message for delete notification
	wsMessage := types.WebSocketChatMessage{
		Type:      types.WebSocketMessageTypeChatDelete,
		MessageID: messageID,
		TripID:    originalMessage.TripID,
		UserID:    userID, // User who performed the delete
	}

	// Broadcast the deletion
	s.BroadcastMessage(ctx, wsMessage, originalMessage.TripID, "")

	// Publish event (optional)
	err = events.PublishEventWithContext(
		s.eventService,
		ctx,
		string(types.EventTypeChatMessageDeleted),
		originalMessage.TripID,
		userID,
		map[string]interface{}{"messageID": messageID},
		"chat-service",
	)
	if err != nil {
		log.Warnw("Failed to publish chat message deleted event", "error", err, "messageID", messageID)
	}

	return nil
}

// AddReaction adds a reaction to a chat message
func (s *ChatService) AddReaction(ctx context.Context, messageID, userID, reaction string) error {
	log := logger.GetLogger()

	// Validate input
	if messageID == "" || userID == "" || reaction == "" {
		return apperrors.ValidationFailed("missing_fields", "MessageID, UserID, and Reaction are required")
	}
	// TODO: Validate reaction format/allowed characters/length if needed

	// Get the message (using renamed store method)
	msg, err := s.chatStore.GetChatMessageByID(ctx, messageID)
	if err != nil {
		log.Errorw("Failed to get message for adding reaction", "error", err, "messageID", messageID)
		if appErr, ok := err.(*apperrors.AppError); ok && appErr.Type == apperrors.NotFoundError {
			return apperrors.NotFound("ChatMessage", messageID)
		}
		return apperrors.Wrap(err, apperrors.DatabaseError, "failed to retrieve message for reaction")
	}

	// Check if the reacting user is a member of the trip
	role, err := s.tripStore.GetUserRole(ctx, msg.TripID, userID)
	if err != nil || role == types.MemberRoleNone {
		log.Warnw("User not member of trip attempted to react", "messageID", messageID, "userID", userID, "tripID", msg.TripID)
		return apperrors.Forbidden("permission_denied", "User must be a member of the trip to react")
	}

	// Add reaction in the store (using renamed store method)
	if err := s.chatStore.AddReaction(ctx, messageID, userID, reaction); err != nil {
		log.Errorw("Failed to add reaction in store", "error", err, "messageID", messageID, "userID", userID, "reaction", reaction)
		if appErr, ok := err.(*apperrors.AppError); ok {
			return appErr // e.g., Conflict if reaction already exists by user?
		}
		return apperrors.NewDatabaseError(err)
	}

	// Fetch updated reactions
	updatedMsg, err := s.chatStore.GetChatMessageByID(ctx, messageID)
	if err != nil {
		log.Errorw("Failed to get updated message after adding reaction", "error", err, "messageID", messageID)
		// Proceed with broadcast using potentially stale reaction list? Or return error?
		// Let's log and proceed, UI might handle inconsistency.
		updatedMsg = msg // Fallback to original if fetch fails
	}

	// Prepare WebSocket message for reaction update
	wsMessage := types.WebSocketChatMessage{
		Type:      types.WebSocketMessageTypeReactionUpdate,
		MessageID: messageID,
		TripID:    msg.TripID,
		Reactions: updatedMsg.Reactions, // Send the full updated list
	}

	// Broadcast the reaction update
	s.BroadcastMessage(ctx, wsMessage, msg.TripID, "")

	// Publish event (optional)
	err = events.PublishEventWithContext(
		s.eventService,
		ctx,
		string(types.EventTypeChatReactionAdded),
		msg.TripID,
		userID,
		map[string]interface{}{"messageID": messageID, "reaction": reaction},
		"chat-service",
	)
	if err != nil {
		log.Warnw("Failed to publish reaction added event", "error", err, "messageID", messageID)
	}

	return nil
}

// RemoveReaction removes a reaction from a chat message
func (s *ChatService) RemoveReaction(ctx context.Context, messageID, userID, reaction string) error {
	log := logger.GetLogger()

	// Validate input
	if messageID == "" || userID == "" || reaction == "" {
		return apperrors.ValidationFailed("missing_fields", "MessageID, UserID, and Reaction are required")
	}

	// Get the message (using renamed store method)
	msg, err := s.chatStore.GetChatMessageByID(ctx, messageID)
	if err != nil {
		log.Errorw("Failed to get message for removing reaction", "error", err, "messageID", messageID)
		if appErr, ok := err.(*apperrors.AppError); ok && appErr.Type == apperrors.NotFoundError {
			return apperrors.NotFound("ChatMessage", messageID)
		}
		return apperrors.Wrap(err, apperrors.DatabaseError, "failed to retrieve message for removing reaction")
	}

	// Remove reaction in the store (using renamed store method)
	if err := s.chatStore.RemoveReaction(ctx, messageID, userID, reaction); err != nil {
		log.Errorw("Failed to remove reaction in store", "error", err, "messageID", messageID, "userID", userID, "reaction", reaction)
		if appErr, ok := err.(*apperrors.AppError); ok {
			return appErr // e.g., NotFound if reaction didn't exist for user
		}
		return apperrors.NewDatabaseError(err)
	}

	// Fetch updated reactions
	updatedMsg, err := s.chatStore.GetChatMessageByID(ctx, messageID)
	if err != nil {
		log.Errorw("Failed to get updated message after removing reaction", "error", err, "messageID", messageID)
		updatedMsg = msg // Fallback
	}

	// Prepare WebSocket message for reaction update
	wsMessage := types.WebSocketChatMessage{
		Type:      types.WebSocketMessageTypeReactionUpdate,
		MessageID: messageID,
		TripID:    msg.TripID,
		Reactions: updatedMsg.Reactions, // Send the full updated list
	}

	// Broadcast the reaction update
	s.BroadcastMessage(ctx, wsMessage, msg.TripID, "")

	// Publish event (optional)
	err = events.PublishEventWithContext(
		s.eventService,
		ctx,
		string(types.EventTypeChatReactionRemoved),
		msg.TripID,
		userID,
		map[string]interface{}{"messageID": messageID, "reaction": reaction},
		"chat-service",
	)
	if err != nil {
		log.Warnw("Failed to publish reaction removed event", "error", err, "messageID", messageID)
	}

	return nil
}

// UpdateLastReadMessage updates the last read message ID for a user in a group
// Note: This assumes a single main chat group per trip identified by TripID for simplicity.
// If multiple groups exist, this needs a groupID parameter.
func (s *ChatService) UpdateLastReadMessage(ctx context.Context, tripID, userID, messageID string) error {
	log := logger.GetLogger()

	// Validate input
	if tripID == "" || userID == "" || messageID == "" {
		return apperrors.ValidationFailed("missing_fields", "TripID, UserID, and MessageID are required")
	}

	// Check if the user is a member of the trip
	role, err := s.tripStore.GetUserRole(ctx, tripID, userID)
	if err != nil || role == types.MemberRoleNone {
		log.Warnw("User not member of trip attempted to update last read message", "tripID", tripID, "userID", userID)
		return apperrors.Forbidden("permission_denied", "User must be a member of the trip")
	}

	// Check if the message exists (using renamed store method)
	msg, err := s.chatStore.GetChatMessageByID(ctx, messageID)
	if err != nil {
		log.Errorw("Failed to get message for last read update", "error", err, "messageID", messageID)
		if appErr, ok := err.(*apperrors.AppError); ok && appErr.Type == apperrors.NotFoundError {
			return apperrors.NotFound("ChatMessage", messageID)
		}
		return apperrors.Wrap(err, apperrors.DatabaseError, "failed to retrieve message for last read update")
	}
	if msg.TripID != tripID {
		log.Errorw("Message ID does not belong to the specified trip", "messageID", messageID, "messageTripID", msg.TripID, "requestTripID", tripID)
		return apperrors.ValidationFailed("invalid_message_id", "Message does not belong to this trip")
	}

	// Determine GroupID (assuming TripID maps to the main group for now)
	groupID := msg.GroupID // Use GroupID from the fetched message
	if groupID == "" {
		// Fallback or error if groupID is unexpectedly empty
		log.Errorw("Message is missing GroupID for UpdateLastReadMessage", "messageID", messageID, "tripID", tripID)
		return apperrors.InternalServerError("Cannot update last read: message missing group context")
	}

	// Update last read message in the store (using interface method with groupID)
	if err := s.chatStore.UpdateLastReadMessage(ctx, groupID, userID, messageID); err != nil {
		log.Errorw("Failed to update last read message in store", "error", err, "tripID", tripID, "userID", userID, "messageID", messageID)
		if appErr, ok := err.(*apperrors.AppError); ok {
			return appErr
		}
		return apperrors.NewDatabaseError(err)
	}

	// Optionally, publish an event or send a specific WS message if needed
	// For now, just log success
	log.Infow("Updated last read message", "tripID", tripID, "userID", userID, "messageID", messageID)

	return nil
}

// HandleWebSocketMessage processes an incoming message from a WebSocket connection
func (s *ChatService) HandleWebSocketMessage(ctx context.Context, conn *middleware.SafeConn, message []byte, userID string) error {
	log := logger.GetLogger()

	var wsMsg types.WebSocketIncomingMessage
	if err := json.Unmarshal(message, &wsMsg); err != nil {
		log.Errorw("Failed to unmarshal WebSocket message", "error", err, "userID", userID, "message", string(message))
		// Send error back to client?
		errMsg := types.WebSocketChatMessage{Type: types.WebSocketMessageTypeError, Message: "Invalid message format"}
		errJSON, _ := json.Marshal(errMsg)
		conn.WriteMessage(websocket.TextMessage, errJSON)
		return apperrors.ValidationFailed("invalid_ws_message_format", err.Error())
	}

	// Basic validation
	if wsMsg.TripID == "" {
		log.Warnw("WebSocket message missing TripID", "userID", userID)
		errMsg := types.WebSocketChatMessage{Type: types.WebSocketMessageTypeError, Message: "Missing tripId"}
		errJSON, _ := json.Marshal(errMsg)
		conn.WriteMessage(websocket.TextMessage, errJSON)
		return apperrors.ValidationFailed("missing_trip_id", "TripID is required in WebSocket message")
	}

	// Get user info (using renamed store method)
	userInfo, err := s.GetUserByID(ctx, userID)
	if err != nil {
		log.Errorw("Failed to get user info for handling WebSocket message", "error", err, "userID", userID)
		errMsg := types.WebSocketChatMessage{Type: types.WebSocketMessageTypeError, Message: "Internal server error"}
		errJSON, _ := json.Marshal(errMsg)
		conn.WriteMessage(websocket.TextMessage, errJSON)
		// Don't return the raw DB error to client
		return apperrors.Wrap(err, apperrors.ServerError, "failed to retrieve user info")
	}
	userResponse := ConvertSupabaseUserToUserResponse(userInfo)

	switch wsMsg.Type {
	case types.WebSocketMessageTypeChat: // Client sends a new chat message
		if wsMsg.Content == "" {
			errMsg := types.WebSocketChatMessage{Type: types.WebSocketMessageTypeError, Message: "Missing message content"}
			errJSON, _ := json.Marshal(errMsg)
			conn.WriteMessage(websocket.TextMessage, errJSON)
			return apperrors.ValidationFailed("missing_content", "Content is required for chat messages")
		}
		chatMsg := types.ChatMessage{
			TripID:      wsMsg.TripID,
			UserID:      userID,
			Content:     wsMsg.Content,
			ContentType: types.ContentTypeText, // Assume text for now, extend if needed
			GroupID:     wsMsg.TripID,          // Use TripID as GroupID proxy
		}
		_, err := s.SendChatMessage(ctx, chatMsg, *userResponse)
		if err != nil {
			log.Errorw("Failed to send chat message initiated via WebSocket", "error", err, "tripID", wsMsg.TripID, "userID", userID)
			// Send specific error back if possible
			errMsg := types.WebSocketChatMessage{Type: types.WebSocketMessageTypeError, Message: "Failed to send message"}
			if appErr, ok := err.(*apperrors.AppError); ok {
				errMsg.Message = appErr.Message // Provide a more specific error message
			}
			errJSON, _ := json.Marshal(errMsg)
			conn.WriteMessage(websocket.TextMessage, errJSON)
			return err // Return the original AppError
		}

	case types.WebSocketMessageTypeUpdateChat: // Client sends an update to a message
		if wsMsg.MessageID == "" || wsMsg.Content == "" {
			errMsg := types.WebSocketChatMessage{Type: types.WebSocketMessageTypeError, Message: "Missing messageId or content for update"}
			errJSON, _ := json.Marshal(errMsg)
			conn.WriteMessage(websocket.TextMessage, errJSON)
			return apperrors.ValidationFailed("missing_fields_for_update", "MessageID and Content required for update")
		}
		err := s.UpdateChatMessage(ctx, wsMsg.MessageID, userID, wsMsg.Content)
		if err != nil {
			log.Errorw("Failed to update chat message initiated via WebSocket", "error", err, "messageID", wsMsg.MessageID, "userID", userID)
			errMsg := types.WebSocketChatMessage{Type: types.WebSocketMessageTypeError, Message: "Failed to update message"}
			if appErr, ok := err.(*apperrors.AppError); ok {
				errMsg.Message = appErr.Message
			}
			errJSON, _ := json.Marshal(errMsg)
			conn.WriteMessage(websocket.TextMessage, errJSON)
			return err
		}

	case types.WebSocketMessageTypeDeleteChat: // Client requests to delete a message
		if wsMsg.MessageID == "" {
			errMsg := types.WebSocketChatMessage{Type: types.WebSocketMessageTypeError, Message: "Missing messageId for delete"}
			errJSON, _ := json.Marshal(errMsg)
			conn.WriteMessage(websocket.TextMessage, errJSON)
			return apperrors.ValidationFailed("missing_message_id_for_delete", "MessageID required for delete")
		}
		err := s.DeleteChatMessage(ctx, wsMsg.MessageID, userID)
		if err != nil {
			log.Errorw("Failed to delete chat message initiated via WebSocket", "error", err, "messageID", wsMsg.MessageID, "userID", userID)
			errMsg := types.WebSocketChatMessage{Type: types.WebSocketMessageTypeError, Message: "Failed to delete message"}
			if appErr, ok := err.(*apperrors.AppError); ok {
				errMsg.Message = appErr.Message
			}
			errJSON, _ := json.Marshal(errMsg)
			conn.WriteMessage(websocket.TextMessage, errJSON)
			return err
		}

	case types.WebSocketMessageTypeAddReaction: // Client adds a reaction
		if wsMsg.MessageID == "" || wsMsg.Reaction == "" {
			errMsg := types.WebSocketChatMessage{Type: types.WebSocketMessageTypeError, Message: "Missing messageId or reaction for adding reaction"}
			errJSON, _ := json.Marshal(errMsg)
			conn.WriteMessage(websocket.TextMessage, errJSON)
			return apperrors.ValidationFailed("missing_fields_for_add_reaction", "MessageID and Reaction required")
		}
		err := s.AddReaction(ctx, wsMsg.MessageID, userID, wsMsg.Reaction)
		if err != nil {
			log.Errorw("Failed to add reaction initiated via WebSocket", "error", err, "messageID", wsMsg.MessageID, "userID", userID, "reaction", wsMsg.Reaction)
			errMsg := types.WebSocketChatMessage{Type: types.WebSocketMessageTypeError, Message: "Failed to add reaction"}
			if appErr, ok := err.(*apperrors.AppError); ok {
				errMsg.Message = appErr.Message
			}
			errJSON, _ := json.Marshal(errMsg)
			conn.WriteMessage(websocket.TextMessage, errJSON)
			return err
		}

	case types.WebSocketMessageTypeRemoveReaction: // Client removes a reaction
		if wsMsg.MessageID == "" || wsMsg.Reaction == "" {
			errMsg := types.WebSocketChatMessage{Type: types.WebSocketMessageTypeError, Message: "Missing messageId or reaction for removing reaction"}
			errJSON, _ := json.Marshal(errMsg)
			conn.WriteMessage(websocket.TextMessage, errJSON)
			return apperrors.ValidationFailed("missing_fields_for_remove_reaction", "MessageID and Reaction required")
		}
		err := s.RemoveReaction(ctx, wsMsg.MessageID, userID, wsMsg.Reaction)
		if err != nil {
			log.Errorw("Failed to remove reaction initiated via WebSocket", "error", err, "messageID", wsMsg.MessageID, "userID", userID, "reaction", wsMsg.Reaction)
			errMsg := types.WebSocketChatMessage{Type: types.WebSocketMessageTypeError, Message: "Failed to remove reaction"}
			if appErr, ok := err.(*apperrors.AppError); ok {
				errMsg.Message = appErr.Message
			}
			errJSON, _ := json.Marshal(errMsg)
			conn.WriteMessage(websocket.TextMessage, errJSON)
			return err
		}

	case types.WebSocketMessageTypeUpdateLastRead: // Client updates their last read message
		if wsMsg.MessageID == "" {
			errMsg := types.WebSocketChatMessage{Type: types.WebSocketMessageTypeError, Message: "Missing messageId for updating last read"}
			errJSON, _ := json.Marshal(errMsg)
			conn.WriteMessage(websocket.TextMessage, errJSON)
			return apperrors.ValidationFailed("missing_message_id_for_last_read", "MessageID required")
		}
		// Assuming TripID from the wsMsg context is the correct 'group' context for now
		err := s.UpdateLastReadMessage(ctx, wsMsg.TripID, userID, wsMsg.MessageID)
		if err != nil {
			log.Errorw("Failed to update last read message initiated via WebSocket", "error", err, "tripID", wsMsg.TripID, "userID", userID, "messageID", wsMsg.MessageID)
			// Send error back?
			errMsg := types.WebSocketChatMessage{Type: types.WebSocketMessageTypeError, Message: "Failed to update last read message"}
			if appErr, ok := err.(*apperrors.AppError); ok {
				errMsg.Message = appErr.Message
			}
			errJSON, _ := json.Marshal(errMsg)
			conn.WriteMessage(websocket.TextMessage, errJSON)
			return err
		}

	default:
		log.Warnw("Received unhandled WebSocket message type", "type", wsMsg.Type, "userID", userID)
		errMsg := types.WebSocketChatMessage{Type: types.WebSocketMessageTypeError, Message: fmt.Sprintf("Unhandled message type: %s", wsMsg.Type)}
		errJSON, _ := json.Marshal(errMsg)
		conn.WriteMessage(websocket.TextMessage, errJSON)
		return apperrors.ValidationFailed("unhandled_ws_message_type", fmt.Sprintf("Type %s not supported", wsMsg.Type))
	}

	return nil
}

// GetChatMessages retrieves chat messages for a trip with pagination
func (s *ChatService) GetChatMessages(ctx context.Context, tripID string, userID string, params types.PaginationParams) (*types.PaginatedResponse, error) {
	log := logger.GetLogger()

	// Validate input
	if tripID == "" || userID == "" {
		return nil, apperrors.ValidationFailed("missing_fields", "TripID and UserID are required")
	}
	if params.Limit <= 0 {
		params.Limit = 50 // Default limit
	}
	if params.Offset < 0 {
		params.Offset = 0
	}

	// Check if the user is a member of the trip
	role, err := s.tripStore.GetUserRole(ctx, tripID, userID)
	if err != nil || role == types.MemberRoleNone {
		log.Warnw("User not member of trip attempted to get chat messages", "tripID", tripID, "userID", userID)
		return nil, apperrors.Forbidden("permission_denied", "User must be a member of the trip to view messages")
	}

	// Determine GroupID (assuming TripID maps to the main group for now)
	// TODO: Need a way to get the primary GroupID for a TripID if not directly passed
	groupID := tripID // Use TripID as GroupID proxy

	// Get messages from the store (using interface method with groupID)
	messages, totalCount, err := s.chatStore.ListChatMessages(ctx, groupID, params)
	if err != nil {
		log.Errorw("Failed to list chat messages from store", "error", err, "tripID", tripID)
		if appErr, ok := err.(*apperrors.AppError); ok {
			return nil, appErr
		}
		return nil, apperrors.NewDatabaseError(err)
	}

	// Get user info for all unique senders
	userInfoMap := make(map[string]*types.UserResponse)
	for _, msg := range messages {
		if _, exists := userInfoMap[msg.UserID]; !exists {
			userInfoMap[msg.UserID] = nil
		}
	}
	for uid := range userInfoMap {
		userInfo, err := s.GetUserByID(ctx, uid) // Use renamed method
		if err != nil {
			log.Warnw("Failed to get user info for chat message sender", "error", err, "userID", uid)
			userInfoMap[uid] = &types.UserResponse{ID: uid, DisplayName: "Unknown User"}
		} else {
			userInfoMap[uid] = ConvertSupabaseUserToUserResponse(userInfo) // Convert
		}
	}

	// Convert ChatMessage to WebSocketChatMessage
	wsMessages := make([]types.WebSocketChatMessage, len(messages))
	for i, msg := range messages {
		wsMessages[i] = types.WebSocketChatMessage{
			Type:        types.WebSocketMessageTypeChat,
			MessageID:   msg.ID,
			TripID:      msg.TripID,
			UserID:      msg.UserID,
			User:        userInfoMap[msg.UserID],
			Content:     msg.Content,
			ContentType: msg.ContentType,
			Timestamp:   msg.CreatedAt,
			Reactions:   msg.Reactions,
		}
	}

	// Create paginated response
	response := &types.PaginatedResponse{
		Data: wsMessages,
	}
	response.Pagination.Total = totalCount
	response.Pagination.Limit = params.Limit
	response.Pagination.Offset = params.Offset

	return response, nil
}

// GetUserByID retrieves user info from the store.
// Renamed from GetUserInfo to clarify it uses the store method.
func (s *ChatService) GetUserByID(ctx context.Context, userID string) (*types.SupabaseUser, error) {
	// Call the renamed store method
	return s.chatStore.GetUserByID(ctx, userID)
}

// ConvertSupabaseUserToUserResponse converts a SupabaseUser to UserResponse
func ConvertSupabaseUserToUserResponse(su *types.SupabaseUser) *types.UserResponse {
	if su == nil {
		return nil
	}

	displayName := su.UserMetadata.Username
	if su.UserMetadata.FirstName != "" {
		if su.UserMetadata.LastName != "" {
			displayName = fmt.Sprintf("%s %s", su.UserMetadata.FirstName, su.UserMetadata.LastName)
		} else {
			displayName = su.UserMetadata.FirstName
		}
	}

	return &types.UserResponse{
		ID:          su.ID,
		Email:       su.Email,
		Username:    su.UserMetadata.Username,
		FirstName:   su.UserMetadata.FirstName,
		LastName:    su.UserMetadata.LastName,
		AvatarURL:   su.UserMetadata.ProfilePicture,
		DisplayName: displayName,
	}
}

// Helper function to marshal JSON or panic
func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal JSON: %v", err))
	}
	return data
}

// HandleEvent implements the EventHandler interface
func (s *ChatService) HandleEvent(ctx context.Context, event types.Event) error {
	log := logger.GetLogger()
	log.Infow("Handling event", "type", event.Type, "tripID", event.TripID)

	// Handle different event types
	switch event.Type {
	case types.EventTypeChatMessageSent:
		var payload types.ChatMessageEvent
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return fmt.Errorf("failed to unmarshal chat message event: %w", err)
		}
		// Broadcast the message to all connected clients
		wsMessage := types.WebSocketChatMessage{
			Type:      types.WebSocketMessageTypeChat,
			MessageID: payload.MessageID,
			TripID:    payload.TripID,
			Content:   payload.Content,
			User:      &payload.User,
			Timestamp: payload.Timestamp,
		}
		s.BroadcastMessage(ctx, wsMessage, payload.TripID, event.UserID)

	case types.EventTypeChatMessageEdited:
		var payload types.ChatMessageEvent
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return fmt.Errorf("failed to unmarshal chat message edit event: %w", err)
		}
		wsMessage := types.WebSocketChatMessage{
			Type:      types.WebSocketMessageTypeChatUpdate,
			MessageID: payload.MessageID,
			TripID:    payload.TripID,
			Content:   payload.Content,
			User:      &payload.User,
			Timestamp: payload.Timestamp,
		}
		s.BroadcastMessage(ctx, wsMessage, payload.TripID, "")
	}

	return nil
}

// SupportedEvents implements the EventHandler interface
func (s *ChatService) SupportedEvents() []types.EventType {
	return []types.EventType{
		types.EventTypeChatMessageSent,
		types.EventTypeChatMessageEdited,
		types.EventTypeChatMessageDeleted,
		types.EventTypeChatReactionAdded,
		types.EventTypeChatReactionRemoved,
	}
}
