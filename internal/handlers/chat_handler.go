package handlers

import (
	"github.com/NomadCrew/nomad-crew-backend/internal/service"
	"go.uber.org/zap"
)

// ChatHandler handles HTTP requests related to chat groups and messages.
type ChatHandler struct {
	// Use the interface type
	chatService service.ChatService
	logger      *zap.SugaredLogger
}

// NewChatHandler creates a new ChatHandler instance.
// Accept the interface type
func NewChatHandler(cs service.ChatService, logger *zap.SugaredLogger) *ChatHandler {
	return &ChatHandler{
		chatService: cs,
		logger:      logger.Named("ChatHandler"),
	}
}

// Note: Chat handler methods will be implemented according to the
// service.ChatService. The current implementation contains
// methods not defined in the interface.

// Future endpoints:
// - CreateChatGroup - Creates a new chat group
// - GetChatGroup - Retrieves a specific chat group by ID
// - UpdateChatGroup - Updates an existing chat group
// - DeleteChatGroup - Deletes a chat group
// - ListChatGroupMembers - Lists all members of a chat group
// - AddChatGroupMember - Adds a user to a chat group
// - RemoveChatGroupMember - Removes a user from a chat group
