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
	ID          string                `json:"id"`
	TripID      string                `json:"tripId"`  // Often the primary key for partitioning/lookup
	GroupID     string                `json:"groupId"` // Specific group within the trip, if applicable
	UserID      string                `json:"userId"`
	Content     string                `json:"content"`
	ContentType string                `json:"contentType"` // Added: e.g., "text", "image_url", "system"
	CreatedAt   time.Time             `json:"createdAt"`
	UpdatedAt   time.Time             `json:"updatedAt"`
	IsEdited    bool                  `json:"isEdited"`
	IsDeleted   bool                  `json:"isDeleted"`
	Reactions   []ChatMessageReaction `json:"reactions,omitempty"` // Added
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
	// Added missing message types
	WebSocketMessageTypeChatUpdate     WebSocketMessageType = "chat_update"
	WebSocketMessageTypeChatDelete     WebSocketMessageType = "chat_delete"
	WebSocketMessageTypeReactionUpdate WebSocketMessageType = "reaction_update"
	// Existing types
	WebSocketMessageTypeRead  WebSocketMessageType = "read"
	WebSocketMessageTypeInfo  WebSocketMessageType = "info"
	WebSocketMessageTypeError WebSocketMessageType = "error"
	// Renamed for clarity?
	// WebSocketMessageTypeEditMessage WebSocketMessageType = "edit_message" // Now ChatUpdate
	// WebSocketMessageTypeDeleteMessage WebSocketMessageType = "delete_message" // Now ChatDelete
	// WebSocketMessageTypeAddReaction WebSocketMessageType = "add_reaction"
	// WebSocketMessageTypeRemoveReaction WebSocketMessageType = "remove_reaction"
	WebSocketMessageTypeReadReceipt  WebSocketMessageType = "read_receipt"
	WebSocketMessageTypeTypingStatus WebSocketMessageType = "typing_status"
	// Types used in incoming messages (from service)
	WebSocketMessageTypeUpdateChat     WebSocketMessageType = "update_chat" // Keep separate if FE sends this?
	WebSocketMessageTypeDeleteChat     WebSocketMessageType = "delete_chat" // Keep separate if FE sends this?
	WebSocketMessageTypeAddReaction    WebSocketMessageType = "add_reaction"
	WebSocketMessageTypeRemoveReaction WebSocketMessageType = "remove_reaction"
	WebSocketMessageTypeUpdateLastRead WebSocketMessageType = "update_last_read"
)

// WebSocketChatMessage represents a message sent over WebSocket
// Updated fields for consistency with service usage
type WebSocketChatMessage struct {
	Type        WebSocketMessageType  `json:"type"`
	MessageID   string                `json:"messageId,omitempty"`
	TripID      string                `json:"tripId"`           // Keep TripID as primary context
	UserID      string                `json:"userId,omitempty"` // Renamed from SenderID
	User        *UserResponse         `json:"user,omitempty"`   // Changed to pointer type
	Content     string                `json:"content,omitempty"`
	ContentType string                `json:"contentType,omitempty"` // Added
	Timestamp   time.Time             `json:"timestamp,omitempty"`
	Reactions   []ChatMessageReaction `json:"reactions,omitempty"` // Added, use existing struct
	Reaction    string                `json:"reaction,omitempty"`  // Keep for specific reaction add/remove events
	IsTyping    bool                  `json:"isTyping,omitempty"`  // Keep for typing status
	Message     string                `json:"message,omitempty"`   // General message field for info/error
	Error       string                `json:"error,omitempty"`     // Specific error string
	// Removed GroupID as TripID is primary context here
	// Removed generic Message interface{}
}

// WebSocketIncomingMessage represents a message received from a client WebSocket
type WebSocketIncomingMessage struct {
	Type      WebSocketMessageType `json:"type" binding:"required"`
	TripID    string               `json:"tripId" binding:"required"`
	MessageID string               `json:"messageId,omitempty"`
	Content   string               `json:"content,omitempty"`  // Used for chat, update
	Reaction  string               `json:"reaction,omitempty"` // Used for add/remove reaction
	// GroupID might be needed if FE sends it for specific group contexts
	// GroupID   string               `json:"groupId,omitempty"`
}

// ContentTypeText represents plain text content.
const ContentTypeText = "text"

// PaginationInfo represents pagination metadata
type PaginationInfo struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// PaginatedResponse represents a generic paginated response
type PaginatedResponse struct {
	Data       interface{}    `json:"data"`
	Pagination PaginationInfo `json:"pagination"`
}
