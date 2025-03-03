package types

import (
	"time"
)

// ChatGroup represents a chat group within a trip
type ChatGroup struct {
	ID          string    `json:"id"`
	TripID      string    `json:"tripId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedBy   string    `json:"createdBy"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ChatMessage represents a message in a chat group
type ChatMessage struct {
	ID        string    `json:"id"`
	TripID    string    `json:"tripId"`
	UserID    string    `json:"userId"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	IsEdited  bool      `json:"isEdited"`
	IsDeleted bool      `json:"isDeleted"`
}

// ChatGroupMember represents a member of a chat group
type ChatGroupMember struct {
	ID                string    `json:"id"`
	GroupID           string    `json:"groupId"`
	UserID            string    `json:"userId"`
	JoinedAt          time.Time `json:"joinedAt"`
	LastReadMessageID string    `json:"lastReadMessageId,omitempty"`
}

// ChatMessageAttachment represents a file attachment to a message
type ChatMessageAttachment struct {
	ID        string    `json:"id"`
	MessageID string    `json:"messageId"`
	FileURL   string    `json:"fileUrl"`
	FileType  string    `json:"fileType"`
	FileName  string    `json:"fileName"`
	FileSize  int       `json:"fileSize"`
	CreatedAt time.Time `json:"createdAt"`
}

// ChatMessageReaction represents an emoji reaction to a message
type ChatMessageReaction struct {
	ID        string    `json:"id"`
	MessageID string    `json:"messageId"`
	UserID    string    `json:"userId"`
	Reaction  string    `json:"reaction"`
	CreatedAt time.Time `json:"createdAt"`
}

// ChatMessageWithUser represents a chat message with user information
type ChatMessageWithUser struct {
	Message ChatMessage  `json:"message"`
	User    UserResponse `json:"user"`
}

// ChatGroupWithMembers represents a chat group with its members
type ChatGroupWithMembers struct {
	Group   ChatGroup      `json:"group"`
	Members []UserResponse `json:"members"`
}

// ChatMessageCreateRequest represents a request to create a new chat message
type ChatMessageCreateRequest struct {
	TripID  string `json:"tripId"`
	Content string `json:"content"`
}

// ChatGroupCreateRequest represents a request to create a new chat group
type ChatGroupCreateRequest struct {
	TripID      string `json:"tripId"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ChatMessageUpdateRequest represents a request to update a chat message
type ChatMessageUpdateRequest struct {
	Content string `json:"content"`
}

// ChatGroupUpdateRequest represents a request to update a chat group
type ChatGroupUpdateRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// ChatMessageReactionRequest represents a request to add/remove a reaction
type ChatMessageReactionRequest struct {
	Reaction string `json:"reaction"`
}

// ChatMessagePaginatedResponse represents a paginated list of chat messages
type ChatMessagePaginatedResponse struct {
	Messages []ChatMessageWithUser `json:"messages"`
	Total    int                   `json:"total"`
	Limit    int                   `json:"limit"`
	Offset   int                   `json:"offset"`
}

// ChatGroupPaginatedResponse represents a paginated list of chat groups
type ChatGroupPaginatedResponse struct {
	Groups     []ChatGroup `json:"groups"`
	Pagination struct {
		Total  int `json:"total"`
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	} `json:"pagination"`
}

// WebSocketMessageType defines the type of WebSocket message
type WebSocketMessageType string

const (
	// WebSocketMessageTypeChat is a chat message
	WebSocketMessageTypeChat WebSocketMessageType = "chat"
	// WebSocketMessageTypeRead is a read receipt
	WebSocketMessageTypeRead WebSocketMessageType = "read"
	// WebSocketMessageTypeInfo is an informational message
	WebSocketMessageTypeInfo WebSocketMessageType = "info"
	// WebSocketMessageTypeError is an error message
	WebSocketMessageTypeError WebSocketMessageType = "error"
	// WebSocketMessageTypeEditMessage is an edited message
	WebSocketMessageTypeEditMessage WebSocketMessageType = "edit_message"
	// WebSocketMessageTypeDeleteMessage is a deleted message
	WebSocketMessageTypeDeleteMessage WebSocketMessageType = "delete_message"
	// WebSocketMessageTypeAddReaction is an added reaction
	WebSocketMessageTypeAddReaction WebSocketMessageType = "add_reaction"
	// WebSocketMessageTypeRemoveReaction is a removed reaction
	WebSocketMessageTypeRemoveReaction WebSocketMessageType = "remove_reaction"
	// WebSocketMessageTypeReadReceipt is a read receipt
	WebSocketMessageTypeReadReceipt WebSocketMessageType = "read_receipt"
	// WebSocketMessageTypeTypingStatus is a typing status
	WebSocketMessageTypeTypingStatus WebSocketMessageType = "typing_status"
)

// WebSocketChatMessage represents a message sent over WebSocket
type WebSocketChatMessage struct {
	Type      WebSocketMessageType `json:"type"`
	MessageID string               `json:"messageId,omitempty"`
	GroupID   string               `json:"groupId,omitempty"`
	TripID    string               `json:"tripId,omitempty"`
	SenderID  string               `json:"senderId,omitempty"`
	Content   string               `json:"content,omitempty"`
	Timestamp time.Time            `json:"timestamp,omitempty"`
	Message   interface{}          `json:"message,omitempty"`
	Error     string               `json:"error,omitempty"`
	User      UserResponse         `json:"user,omitempty"`
	Reaction  string               `json:"reaction,omitempty"`
	IsTyping  bool                 `json:"isTyping,omitempty"`
}
