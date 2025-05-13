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
	Sender      *MessageSender        `json:"sender,omitempty"`    // Added for sender details
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

// WebSocketMessageType defines the type of WebSocket message
type WebSocketMessageType string

const (
	// Message Flow (Client -> Server -> Broadcast)
	WebSocketMessageTypeChat WebSocketMessageType = "chat" // User sends a message

	// Message Flow (Server -> Client Broadcasts based on Events)
	WebSocketMessageTypeChatUpdate     WebSocketMessageType = "chat_update"     // Message content was edited
	WebSocketMessageTypeChatDelete     WebSocketMessageType = "chat_delete"     // Message was deleted
	WebSocketMessageTypeReactionAdd    WebSocketMessageType = "reaction_add"    // Reaction was added
	WebSocketMessageTypeReactionRemove WebSocketMessageType = "reaction_remove" // Reaction was removed
	WebSocketMessageTypeMemberAdd      WebSocketMessageType = "member_add"      // Member added to group
	WebSocketMessageTypeMemberRemove   WebSocketMessageType = "member_remove"   // Member removed from group
	WebSocketMessageTypeGroupUpdate    WebSocketMessageType = "group_update"    // Group details updated
	WebSocketMessageTypeGroupDelete    WebSocketMessageType = "group_delete"    // Group was deleted
	WebSocketMessageTypeReadReceipt    WebSocketMessageType = "read_receipt"    // Someone read messages (less common?)
	WebSocketMessageTypeTypingStatus   WebSocketMessageType = "typing_status"   // Someone is typing
	WebSocketMessageTypeInfo           WebSocketMessageType = "info"            // General info from server (e.g., connection success)
	WebSocketMessageTypeError          WebSocketMessageType = "error"           // Error message from server

	// Client Actions (Client -> Server)
	WebSocketMessageTypeUpdateLastRead WebSocketMessageType = "update_last_read" // Client telling server last read message
	// Client also sends "chat", "add_reaction", "remove_reaction", "delete_chat", "update_chat" as types
	// These are handled in HandleWebSocketMessage, map to appropriate service calls, and trigger events above.
	// Let's keep Client -> Server types separate if needed for clarity/validation
	// WebSocketIncomingTypeChat = "chat"
	// WebSocketIncomingTypeUpdateChat = "update_chat"
	// WebSocketIncomingTypeDeleteChat = "delete_chat"
	// WebSocketIncomingTypeAddReaction = "add_reaction"
	// WebSocketIncomingTypeRemoveReaction = "remove_reaction"
	// WebSocketIncomingTypeUpdateLastRead = "update_last_read"
)

// WebSocketChatMessage represents a message sent over WebSocket
// Updated fields for consistency with service usage
type WebSocketChatMessage struct {
	Type        WebSocketMessageType  `json:"type"`
	MessageID   string                `json:"messageId,omitempty"`
	TripID      string                `json:"tripId"`            // Keep TripID as primary context
	GroupID     string                `json:"groupId,omitempty"` // Added for group-specific events
	UserID      string                `json:"userId,omitempty"`  // Renamed from SenderID
	User        *UserResponse         `json:"user,omitempty"`    // Changed to pointer type
	Content     string                `json:"content,omitempty"`
	ContentType string                `json:"contentType,omitempty"` // Added
	Timestamp   time.Time             `json:"timestamp,omitempty"`
	Reactions   []ChatMessageReaction `json:"reactions,omitempty"` // Added, use existing struct
	Reaction    string                `json:"reaction,omitempty"`  // Keep for specific reaction add/remove events
	IsTyping    bool                  `json:"isTyping,omitempty"`  // Keep for typing status
	Message     string                `json:"message,omitempty"`   // General message field for info/error
	Error       string                `json:"error,omitempty"`     // Specific error string
	// Removed GroupID as TripID is primary context here -> Re-added GroupID
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

// WebSocketMessage defines the generic structure for incoming WebSocket messages
// Used in trip_handler.go for initial message parsing
type WebSocketMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}
