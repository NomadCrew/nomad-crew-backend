package service

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/types"
)

// ChatServiceInterface defines the interface for chat-related business logic.
// This interface is exported for use in other packages and dependency injection.
type ChatServiceInterface interface {
	// Group Operations
	CreateGroup(ctx context.Context, tripID, groupName, createdByUserID string) (*types.ChatGroup, error)
	GetGroup(ctx context.Context, groupID, requestingUserID string) (*types.ChatGroup, error)
	UpdateGroup(ctx context.Context, groupID, requestingUserID string, updateReq types.ChatGroupUpdateRequest) (*types.ChatGroup, error)
	DeleteGroup(ctx context.Context, groupID, requestingUserID string) error
	ListTripGroups(ctx context.Context, tripID, requestingUserID string, params types.PaginationParams) (*types.ChatGroupPaginatedResponse, error)

	// Member Operations
	AddMember(ctx context.Context, groupID, actorUserID, targetUserID string) error
	RemoveMember(ctx context.Context, groupID, actorUserID, targetUserID string) error
	ListMembers(ctx context.Context, groupID, requestingUserID string) ([]types.UserResponse, error)

	// Message Operations
	PostMessage(ctx context.Context, groupID, userID, content string) (*types.ChatMessageWithUser, error)
	GetMessage(ctx context.Context, messageID, requestingUserID string) (*types.ChatMessageWithUser, error)
	UpdateMessage(ctx context.Context, messageID, requestingUserID, newContent string) (*types.ChatMessageWithUser, error)
	DeleteMessage(ctx context.Context, messageID, requestingUserID string) error
	ListMessages(ctx context.Context, groupID, requestingUserID string, params types.PaginationParams) (*types.ChatMessagePaginatedResponse, error)

	// Reaction Operations
	AddReaction(ctx context.Context, messageID, userID, reaction string) error
	RemoveReaction(ctx context.Context, messageID, userID, reaction string) error

	// Read Status Operations
	UpdateLastRead(ctx context.Context, groupID, userID, messageID string) error

	// Group Member Operations
	AddGroupMember(ctx context.Context, tripID, userID string) error
	GetChatGroupMembers(ctx context.Context, groupID string) ([]types.UserResponse, error)
	CreateChatMessage(ctx context.Context, message types.ChatMessage) (string, error)
}
