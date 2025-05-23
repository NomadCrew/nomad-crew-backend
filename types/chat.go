package types

import (
	"time"
)

// ChatGroup represents a chat group within a trip
type ChatGroup struct {
	ID          string     `json:"id"`
	TripID      string     `json:"tripId"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	CreatedBy   string     `json:"createdBy"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	DeletedAt   *time.Time `json:"deletedAt,omitempty"`
}

// ChatMessage represents a message in a chat group
type ChatMessage struct {
	ID          string                `json:"id"`
	TripID      string                `json:"tripId"`  // Often the primary key for partitioning/lookup
	GroupID     string                `json:"groupId"` // Specific group within the trip, if applicable
	UserID      string                `json:"userId"`
	Content     string                `json:"content"`
	ContentType string                `json:"contentType"` // e.g., "text", "image_url", "system"
	CreatedAt   time.Time             `json:"createdAt"`
	UpdatedAt   time.Time             `json:"updatedAt"`
	DeletedAt   *time.Time            `json:"deletedAt,omitempty"`
	Reactions   []ChatMessageReaction `json:"reactions,omitempty"`
	Sender      *MessageSender        `json:"sender,omitempty"` // Added for sender details
}

// MessageSender defines the structure for sender information in a chat message
type MessageSender struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl,omitempty"`
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
	MessageID string    `json:"messageId"`
	UserID    string    `json:"userId"`
	Reaction  string    `json:"reaction"` // The emoji character(s)
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

// ChatLastReadRequest represents a request to update the last read message
type ChatLastReadRequest struct {
	LastReadMessageID *string `json:"lastReadMessageId" binding:"required"`
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

// ContentTypeText represents plain text content.
const ContentTypeText = "text"

// PaginationInfo represents pagination metadata
type PaginationInfo struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}
