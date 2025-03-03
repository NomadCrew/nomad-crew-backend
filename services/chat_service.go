package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gorilla/websocket"
)

// ChatService handles chat operations
type ChatService struct {
	chatStore      store.ChatStore
	tripStore      store.TripStore
	eventPublisher types.EventPublisher
	connections    map[string]map[string]*middleware.SafeConn // map[tripID]map[userID]*SafeConn
	connectionsMux sync.RWMutex
}

// NewChatService creates a new ChatService
func NewChatService(chatStore store.ChatStore, tripStore store.TripStore, eventPublisher types.EventPublisher) *ChatService {
	return &ChatService{
		chatStore:      chatStore,
		tripStore:      tripStore,
		eventPublisher: eventPublisher,
		connections:    make(map[string]map[string]*middleware.SafeConn),
	}
}

// RegisterConnection registers a WebSocket connection for a user in a trip
func (s *ChatService) RegisterConnection(tripID, userID string, conn *middleware.SafeConn) {
	log := logger.GetLogger()
	log.Infow("Registering WebSocket connection", "tripID", tripID, "userID", userID)

	s.connectionsMux.Lock()
	defer s.connectionsMux.Unlock()

	// Initialize the trip map if it doesn't exist
	if _, ok := s.connections[tripID]; !ok {
		s.connections[tripID] = make(map[string]*middleware.SafeConn)
	}

	// Store the connection
	s.connections[tripID][userID] = conn

	// Send a welcome message
	welcomeMsg := types.WebSocketChatMessage{
		Type:    types.WebSocketMessageTypeInfo,
		Message: "Connected to chat",
	}

	welcomeJSON, err := json.Marshal(welcomeMsg)
	if err != nil {
		log.Errorw("Failed to marshal welcome message", "error", err)
		return
	}

	if err := conn.WriteMessage(1, welcomeJSON); err != nil {
		log.Errorw("Failed to send welcome message", "error", err)
	}
}

// UnregisterConnection removes a WebSocket connection for a user in a trip
func (s *ChatService) UnregisterConnection(tripID, userID string) {
	log := logger.GetLogger()
	log.Infow("Unregistering WebSocket connection", "tripID", tripID, "userID", userID)

	s.connectionsMux.Lock()
	defer s.connectionsMux.Unlock()

	// Check if the trip exists
	if _, ok := s.connections[tripID]; !ok {
		return
	}

	// Remove the connection
	delete(s.connections[tripID], userID)

	// Remove the trip if it's empty
	if len(s.connections[tripID]) == 0 {
		delete(s.connections, tripID)
	}
}

// BroadcastMessage broadcasts a message to all connections in a trip
func (s *ChatService) BroadcastMessage(ctx context.Context, wsMessage types.WebSocketChatMessage, tripID string, excludeUserID string) {
	log := logger.GetLogger()

	// Marshal the message
	messageJSON, err := json.Marshal(wsMessage)
	if err != nil {
		log.Errorw("Failed to marshal WebSocket message", "error", err)
		return
	}

	s.connectionsMux.RLock()
	defer s.connectionsMux.RUnlock()

	// Check if the trip exists
	groupConns, ok := s.connections[tripID]
	if !ok {
		return
	}

	// Send the message to all connections in the trip
	for userID, conn := range groupConns {
		// Skip the excluded user
		if excludeUserID != "" && userID == excludeUserID {
			continue
		}

		if err := conn.WriteMessage(1, messageJSON); err != nil {
			log.Errorw("Failed to broadcast message", "error", err, "tripID", tripID, "userID", userID)
		}
	}
}

// CreateChatGroup creates a new chat group
func (s *ChatService) CreateChatGroup(ctx context.Context, group types.ChatGroup) (string, error) {
	log := logger.GetLogger()

	// Check if the trip exists
	trip, err := s.tripStore.GetTrip(ctx, group.TripID)
	if err != nil {
		log.Errorw("Failed to get trip", "error", err, "tripID", group.TripID)
		return "", fmt.Errorf("trip not found: %w", err)
	}

	// Check if the user is a member of the trip
	role, err := s.tripStore.GetUserRole(ctx, trip.ID, group.CreatedBy)
	if err != nil {
		log.Errorw("Failed to get user role", "error", err, "tripID", trip.ID, "userID", group.CreatedBy)
		return "", fmt.Errorf("failed to verify user membership: %w", err)
	}

	if role == types.MemberRoleNone {
		log.Warnw("User is not a member of the trip", "tripID", trip.ID, "userID", group.CreatedBy)
		return "", fmt.Errorf("user is not a member of the trip")
	}

	// Create the chat group
	groupID, err := s.chatStore.CreateChatGroup(ctx, group)
	if err != nil {
		log.Errorw("Failed to create chat group", "error", err)
		return "", err
	}

	// Add all trip members to the chat group
	members, err := s.tripStore.GetTripMembers(ctx, trip.ID)
	if err != nil {
		log.Errorw("Failed to list trip members", "error", err, "tripID", trip.ID)
		return "", fmt.Errorf("failed to add members to chat group: %w", err)
	}

	for _, member := range members {
		err := s.chatStore.AddChatGroupMember(ctx, groupID, member.UserID)
		if err != nil {
			log.Warnw("Failed to add member to chat group", "error", err, "groupID", groupID, "userID", member.UserID)
		}
	}

	return groupID, nil
}

// SendChatMessage sends a chat message to a trip
func (s *ChatService) SendChatMessage(ctx context.Context, message types.ChatMessage, user types.UserResponse) (string, error) {
	log := logger.GetLogger()

	// Check if the user is a member of the trip
	role, err := s.tripStore.GetUserRole(ctx, message.TripID, message.UserID)
	if err != nil {
		log.Errorw("Failed to get user role", "error", err, "tripID", message.TripID, "userID", message.UserID)
		return "", fmt.Errorf("failed to verify user membership: %w", err)
	}

	if role == types.MemberRoleNone {
		log.Warnw("User is not a member of the trip", "tripID", message.TripID, "userID", message.UserID)
		return "", fmt.Errorf("user is not a member of the trip")
	}

	// Create the chat message
	messageID, err := s.chatStore.CreateChatMessage(ctx, message)
	if err != nil {
		log.Errorw("Failed to create chat message", "error", err)
		return "", err
	}

	// If user info wasn't provided or is incomplete, fetch it
	if user.ID == "" || user.ID != message.UserID {
		// Fetch user info from the database
		fetchedUser, err := s.chatStore.GetUserInfo(ctx, message.UserID)
		if err != nil {
			log.Warnw("Failed to get user info, using partial data", "error", err, "userID", message.UserID)
			// Use a minimal user object if we can't fetch the full data
			user = types.UserResponse{
				ID:       message.UserID,
				Username: "Unknown User", // Provide a fallback username
			}
		} else {
			user = *fetchedUser
		}
	}

	// Create a chat message event
	chatEvent := types.ChatMessageEvent{
		MessageID: messageID,
		TripID:    message.TripID,
		Content:   message.Content,
		User:      user, // Use the user object here
		Timestamp: time.Now(),
	}

	// Marshal the event payload
	payload, err := json.Marshal(chatEvent)
	if err != nil {
		log.Errorw("Failed to marshal chat message event", "error", err)
		return messageID, nil // Still return the message ID even if event publishing fails
	}

	// Create and publish the event
	event := types.Event{
		BaseEvent: types.BaseEvent{
			Type:      types.EventTypeChatMessageSent,
			TripID:    message.TripID,
			UserID:    message.UserID,
			Timestamp: time.Now(),
			Version:   1,
		},
		Metadata: types.EventMetadata{
			Source: "chat_service",
		},
		Payload: payload,
	}

	// Publish the event asynchronously to not block the response
	go func() {
		if err := s.eventPublisher.Publish(context.Background(), message.TripID, event); err != nil {
			log.Errorw("Failed to publish chat message event", "error", err, "messageID", messageID)
		}
	}()

	// For backward compatibility, also broadcast to any existing websocket connections
	// This can be removed once the frontend is updated to use the event system
	wsMessage := types.WebSocketChatMessage{
		Type:      types.WebSocketMessageTypeChat,
		MessageID: messageID,
		TripID:    message.TripID,
		Content:   message.Content,
		Timestamp: time.Now(),
		User:      user,           // Use the user object here
		SenderID:  message.UserID, // Include SenderID for backward compatibility
	}

	go s.BroadcastMessage(ctx, wsMessage, message.TripID, "")

	return messageID, nil
}

// UpdateChatMessage updates a chat message
func (s *ChatService) UpdateChatMessage(ctx context.Context, messageID, userID, content string) error {
	log := logger.GetLogger()

	// Get the chat message
	message, err := s.chatStore.GetChatMessage(ctx, messageID)
	if err != nil {
		log.Errorw("Failed to get chat message", "error", err, "messageID", messageID)
		return fmt.Errorf("chat message not found: %w", err)
	}

	// Check if the user is the author of the message
	if message.UserID != userID {
		log.Warnw("User is not the author of the message", "messageID", messageID, "userID", userID, "authorID", message.UserID)
		return fmt.Errorf("user is not authorized to update this message")
	}

	// Update the chat message
	err = s.chatStore.UpdateChatMessage(ctx, messageID, content)
	if err != nil {
		log.Errorw("Failed to update chat message", "error", err, "messageID", messageID)
		return err
	}

	// Get the updated message
	updatedMessage, err := s.chatStore.GetChatMessage(ctx, messageID)
	if err != nil {
		log.Errorw("Failed to get updated chat message", "error", err, "messageID", messageID)
		return err
	}

	// Get user information
	// In a real implementation, you would get this from a user service
	user := types.UserResponse{
		ID: userID,
		// Other user fields would be populated here
	}

	// Broadcast the update to all connected users
	wsMessage := types.WebSocketChatMessage{
		Type:    types.WebSocketMessageTypeEditMessage,
		Message: *updatedMessage,
		User:    user,
	}

	go s.BroadcastMessage(ctx, wsMessage, message.TripID, "")

	return nil
}

// DeleteChatMessage deletes a chat message
func (s *ChatService) DeleteChatMessage(ctx context.Context, messageID, userID string) error {
	log := logger.GetLogger()

	// Get the chat message
	message, err := s.chatStore.GetChatMessage(ctx, messageID)
	if err != nil {
		log.Errorw("Failed to get chat message", "error", err, "messageID", messageID)
		return fmt.Errorf("chat message not found: %w", err)
	}

	// Check if the user is the author of the message
	if message.UserID != userID {
		// Check if the user is a trip admin or owner
		group, err := s.chatStore.GetChatGroup(ctx, message.TripID)
		if err != nil {
			log.Errorw("Failed to get chat group", "error", err, "groupID", message.TripID)
			return fmt.Errorf("chat group not found: %w", err)
		}

		role, err := s.tripStore.GetUserRole(ctx, group.TripID, userID)
		if err != nil {
			log.Errorw("Failed to get user role", "error", err, "tripID", group.TripID, "userID", userID)
			return fmt.Errorf("failed to verify user role: %w", err)
		}

		// Only allow trip owners or admins to delete other users' messages
		if role != types.MemberRoleOwner && role != types.MemberRoleAdmin {
			log.Warnw("User is not authorized to delete this message", "messageID", messageID, "userID", userID)
			return fmt.Errorf("user is not authorized to delete this message")
		}
	}

	// Delete the chat message
	err = s.chatStore.DeleteChatMessage(ctx, messageID)
	if err != nil {
		log.Errorw("Failed to delete chat message", "error", err, "messageID", messageID)
		return err
	}

	// Broadcast the deletion to all connected users
	wsMessage := types.WebSocketChatMessage{
		Type:      types.WebSocketMessageTypeDeleteMessage,
		MessageID: messageID,
	}

	go s.BroadcastMessage(ctx, wsMessage, message.TripID, "")

	return nil
}

// AddReaction adds a reaction to a chat message
func (s *ChatService) AddReaction(ctx context.Context, messageID, userID, reaction string) error {
	log := logger.GetLogger()

	// Get the chat message
	message, err := s.chatStore.GetChatMessage(ctx, messageID)
	if err != nil {
		log.Errorw("Failed to get chat message", "error", err, "messageID", messageID)
		return fmt.Errorf("chat message not found: %w", err)
	}

	// Get the chat group
	group, err := s.chatStore.GetChatGroup(ctx, message.TripID)
	if err != nil {
		log.Errorw("Failed to get chat group", "error", err, "groupID", message.TripID)
		return fmt.Errorf("chat group not found: %w", err)
	}

	// Check if the user is a member of the trip
	role, err := s.tripStore.GetUserRole(ctx, group.TripID, userID)
	if err != nil {
		log.Errorw("Failed to get user role", "error", err, "tripID", group.TripID, "userID", userID)
		return fmt.Errorf("failed to verify user membership: %w", err)
	}

	if role == types.MemberRoleNone {
		log.Warnw("User is not a member of the trip", "tripID", group.TripID, "userID", userID)
		return fmt.Errorf("user is not a member of the trip")
	}

	// Add the reaction
	err = s.chatStore.AddChatMessageReaction(ctx, messageID, userID, reaction)
	if err != nil {
		log.Errorw("Failed to add reaction", "error", err, "messageID", messageID, "userID", userID, "reaction", reaction)
		return err
	}

	// Broadcast the reaction to all connected users
	wsMessage := types.WebSocketChatMessage{
		Type:      types.WebSocketMessageTypeAddReaction,
		MessageID: messageID,
		Reaction:  reaction,
		User: types.UserResponse{
			ID: userID,
			// Other user fields would be populated here
		},
	}

	go s.BroadcastMessage(ctx, wsMessage, message.TripID, "")

	return nil
}

// RemoveReaction removes a reaction from a chat message
func (s *ChatService) RemoveReaction(ctx context.Context, messageID, userID, reaction string) error {
	log := logger.GetLogger()

	// Get the chat message
	message, err := s.chatStore.GetChatMessage(ctx, messageID)
	if err != nil {
		log.Errorw("Failed to get chat message", "error", err, "messageID", messageID)
		return fmt.Errorf("chat message not found: %w", err)
	}

	// Remove the reaction
	err = s.chatStore.RemoveChatMessageReaction(ctx, messageID, userID, reaction)
	if err != nil {
		log.Errorw("Failed to remove reaction", "error", err, "messageID", messageID, "userID", userID, "reaction", reaction)
		return err
	}

	// Broadcast the reaction removal to all connected users
	wsMessage := types.WebSocketChatMessage{
		Type:      types.WebSocketMessageTypeRemoveReaction,
		MessageID: messageID,
		Reaction:  reaction,
		User: types.UserResponse{
			ID: userID,
			// Other user fields would be populated here
		},
	}

	go s.BroadcastMessage(ctx, wsMessage, message.TripID, "")

	return nil
}

// UpdateLastReadMessage updates the last read message for a user
func (s *ChatService) UpdateLastReadMessage(ctx context.Context, groupID, userID, messageID string) error {
	log := logger.GetLogger()

	// Update the last read message in the database
	err := s.chatStore.UpdateLastReadMessage(ctx, groupID, userID, messageID)
	if err != nil {
		log.Errorw("Failed to update last read message", "error", err, "groupID", groupID, "userID", userID, "messageID", messageID)
		return err
	}

	// Create a read receipt event
	readReceiptEvent := types.ChatReadReceiptEvent{
		TripID:    groupID, // Using groupID as TripID for now
		MessageID: messageID,
		User: types.UserResponse{
			ID: userID,
			// Other user fields would be populated here
		},
	}

	// Marshal the event payload
	payload, err := json.Marshal(readReceiptEvent)
	if err != nil {
		log.Errorw("Failed to marshal read receipt event", "error", err)
		return nil // Still return success even if event publishing fails
	}

	// Create and publish the event
	event := types.Event{
		BaseEvent: types.BaseEvent{
			Type:      types.EventTypeChatReadReceipt,
			TripID:    groupID,
			UserID:    userID,
			Timestamp: time.Now(),
			Version:   1,
		},
		Metadata: types.EventMetadata{
			Source: "chat_service",
		},
		Payload: payload,
	}

	// Publish the event asynchronously to not block the response
	go func() {
		if err := s.eventPublisher.Publish(context.Background(), groupID, event); err != nil {
			log.Errorw("Failed to publish read receipt event", "error", err, "messageID", messageID)
		}
	}()

	// For backward compatibility, also broadcast to any existing websocket connections
	// This can be removed once the frontend is updated to use the event system
	wsMessage := types.WebSocketChatMessage{
		Type:      types.WebSocketMessageTypeReadReceipt,
		TripID:    groupID,
		MessageID: messageID,
		User: types.UserResponse{
			ID: userID,
			// Other user fields would be populated here
		},
	}

	go s.BroadcastMessage(ctx, wsMessage, groupID, userID)

	return nil
}

// HandleWebSocketMessage handles a WebSocket message
func (s *ChatService) HandleWebSocketMessage(ctx context.Context, conn *middleware.SafeConn, message []byte, userID string) error {
	log := logger.GetLogger()

	// Parse the message
	var wsMessage types.WebSocketChatMessage
	err := json.Unmarshal(message, &wsMessage)
	if err != nil {
		log.Errorw("Failed to unmarshal WebSocket message", "error", err)
		return fmt.Errorf("invalid message format: %w", err)
	}

	// Handle different message types
	switch wsMessage.Type {
	case types.WebSocketMessageTypeChat:
		// Create a new chat message
		chatMessage := types.ChatMessage{
			TripID:  wsMessage.TripID,
			UserID:  userID,
			Content: wsMessage.Content,
		}

		// Send the message
		messageID, err := s.SendChatMessage(ctx, chatMessage, types.UserResponse{})
		if err != nil {
			log.Errorw("Failed to send chat message", "error", err)
			// Send error message back to client
			errorMsg := types.WebSocketChatMessage{
				Type:  types.WebSocketMessageTypeError,
				Error: "Failed to send message: " + err.Error(),
			}
			errorJSON, _ := json.Marshal(errorMsg)
			if conn != nil {
				if err := conn.WriteMessage(websocket.TextMessage, errorJSON); err != nil {
					log.Errorw("Failed to write error message to websocket", "error", err)
				}
			}
			return err
		}

		// Update the message ID
		wsMessage.MessageID = messageID

		// Broadcast the message to all connections in the trip
		s.BroadcastMessage(ctx, wsMessage, wsMessage.TripID, "")

		return nil

	case types.WebSocketMessageTypeRead:
		// Validate messageID
		if wsMessage.MessageID == "" {
			log.Warnw("Empty message ID in read receipt", "userID", userID, "tripID", wsMessage.TripID)
			// Send error message back to client
			errorMsg := types.WebSocketChatMessage{
				Type:  types.WebSocketMessageTypeError,
				Error: "Message ID cannot be empty",
			}
			errorJSON, _ := json.Marshal(errorMsg)
			if conn != nil {
				if err := conn.WriteMessage(websocket.TextMessage, errorJSON); err != nil {
					log.Errorw("Failed to write error message to websocket", "error", err)
				}
			}
			return fmt.Errorf("empty message ID in read receipt")
		}

		// Update the last read message
		err := s.UpdateLastReadMessage(ctx, wsMessage.TripID, userID, wsMessage.MessageID)
		if err != nil {
			log.Errorw("Failed to update last read message", "error", err)
			// Send error message back to client
			errorMsg := types.WebSocketChatMessage{
				Type:  types.WebSocketMessageTypeError,
				Error: "Failed to update read status: " + err.Error(),
			}
			errorJSON, _ := json.Marshal(errorMsg)
			if conn != nil {
				if err := conn.WriteMessage(websocket.TextMessage, errorJSON); err != nil {
					log.Errorw("Failed to write error message to websocket", "error", err)
				}
			}
			return err
		}

		return nil

	case types.WebSocketMessageTypeTypingStatus:
		// Broadcast the typing status to all connections in the trip
		s.BroadcastMessage(ctx, wsMessage, wsMessage.TripID, userID)
		return nil

	default:
		errMsg := fmt.Sprintf("unsupported message type: %s", wsMessage.Type)
		log.Warnw(errMsg, "userID", userID)
		// Send error message back to client
		errorMsg := types.WebSocketChatMessage{
			Type:  types.WebSocketMessageTypeError,
			Error: errMsg,
		}
		errorJSON, _ := json.Marshal(errorMsg)
		if conn != nil {
			if err := conn.WriteMessage(websocket.TextMessage, errorJSON); err != nil {
				log.Errorw("Failed to write error message to websocket", "error", err)
			}
		}
		return fmt.Errorf("%s", errMsg)
	}
}
