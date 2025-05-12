// Code generated mockery. DO NOT EDIT.

package mocks

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/stretchr/testify/mock"
)

// ChatStore is a mock of the ChatStore interface
type ChatStore struct {
	mock.Mock
}

// CreateChatGroup mocks the CreateChatGroup method
func (m *ChatStore) CreateChatGroup(ctx context.Context, group types.ChatGroup) (string, error) {
	args := m.Called(ctx, group)
	return args.String(0), args.Error(1)
}

// GetChatGroup mocks the GetChatGroup method
func (m *ChatStore) GetChatGroup(ctx context.Context, groupID string) (*types.ChatGroup, error) {
	args := m.Called(ctx, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.ChatGroup), args.Error(1)
}

// UpdateChatGroup mocks the UpdateChatGroup method
func (m *ChatStore) UpdateChatGroup(ctx context.Context, groupID string, update types.ChatGroupUpdateRequest) error {
	args := m.Called(ctx, groupID, update)
	return args.Error(0)
}

// DeleteChatGroup mocks the DeleteChatGroup method
func (m *ChatStore) DeleteChatGroup(ctx context.Context, groupID string) error {
	args := m.Called(ctx, groupID)
	return args.Error(0)
}

// ListChatGroupsByTrip mocks the ListChatGroupsByTrip method
func (m *ChatStore) ListChatGroupsByTrip(ctx context.Context, tripID string, limit, offset int) (*types.ChatGroupPaginatedResponse, error) {
	args := m.Called(ctx, tripID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.ChatGroupPaginatedResponse), args.Error(1)
}

// CreateChatMessage mocks the CreateChatMessage method
func (m *ChatStore) CreateChatMessage(ctx context.Context, message types.ChatMessage) (string, error) {
	args := m.Called(ctx, message)
	return args.String(0), args.Error(1)
}

// GetChatMessageByID mocks the GetChatMessageByID method
func (m *ChatStore) GetChatMessageByID(ctx context.Context, messageID string) (*types.ChatMessage, error) {
	args := m.Called(ctx, messageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.ChatMessage), args.Error(1)
}

// UpdateChatMessage mocks the UpdateChatMessage method
func (m *ChatStore) UpdateChatMessage(ctx context.Context, messageID string, content string) error {
	args := m.Called(ctx, messageID, content)
	return args.Error(0)
}

// DeleteChatMessage mocks the DeleteChatMessage method
func (m *ChatStore) DeleteChatMessage(ctx context.Context, messageID string) error {
	args := m.Called(ctx, messageID)
	return args.Error(0)
}

// ListChatMessages mocks the ListChatMessages method
func (m *ChatStore) ListChatMessages(ctx context.Context, groupID string, params types.PaginationParams) ([]types.ChatMessage, int, error) {
	args := m.Called(ctx, groupID, params)
	return args.Get(0).([]types.ChatMessage), args.Int(1), args.Error(2)
}

// AddChatGroupMember mocks the AddChatGroupMember method
func (m *ChatStore) AddChatGroupMember(ctx context.Context, groupID, userID string) error {
	args := m.Called(ctx, groupID, userID)
	return args.Error(0)
}

// RemoveChatGroupMember mocks the RemoveChatGroupMember method
func (m *ChatStore) RemoveChatGroupMember(ctx context.Context, groupID, userID string) error {
	args := m.Called(ctx, groupID, userID)
	return args.Error(0)
}

// ListChatGroupMembers mocks the ListChatGroupMembers method
func (m *ChatStore) ListChatGroupMembers(ctx context.Context, groupID string) ([]types.UserResponse, error) {
	args := m.Called(ctx, groupID)
	return args.Get(0).([]types.UserResponse), args.Error(1)
}

// UpdateLastReadMessage mocks the UpdateLastReadMessage method
func (m *ChatStore) UpdateLastReadMessage(ctx context.Context, groupID, userID, messageID string) error {
	args := m.Called(ctx, groupID, userID, messageID)
	return args.Error(0)
}

// AddReaction mocks the AddReaction method
func (m *ChatStore) AddReaction(ctx context.Context, messageID, userID, reaction string) error {
	args := m.Called(ctx, messageID, userID, reaction)
	return args.Error(0)
}

// RemoveReaction mocks the RemoveReaction method
func (m *ChatStore) RemoveReaction(ctx context.Context, messageID, userID, reaction string) error {
	args := m.Called(ctx, messageID, userID, reaction)
	return args.Error(0)
}

// ListChatMessageReactions mocks the ListChatMessageReactions method
func (m *ChatStore) ListChatMessageReactions(ctx context.Context, messageID string) ([]types.ChatMessageReaction, error) {
	args := m.Called(ctx, messageID)
	return args.Get(0).([]types.ChatMessageReaction), args.Error(1)
}

// GetUserByID mocks the GetUserByID method
func (m *ChatStore) GetUserByID(ctx context.Context, userID string) (*types.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}

// Additional methods can be added as needed to satisfy the interface 