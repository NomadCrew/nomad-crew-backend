package types

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Notification represents a user notification in the system.
type Notification struct {
	ID        uuid.UUID       `json:"id" db:"id"`
	UserID    uuid.UUID       `json:"user_id" db:"user_id"`
	Type      string          `json:"type" db:"type"`
	Metadata  json.RawMessage `json:"metadata" db:"metadata"` // Use json.RawMessage for flexible JSONB handling
	IsRead    bool            `json:"is_read" db:"is_read"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
}

// Specific Metadata Structs (Optional but recommended for type safety)

// TripInvitationMetadata is the structure for TRIP_INVITATION notification metadata.
type TripInvitationMetadata struct {
	Type        string    `json:"type"` // Should always be "TRIP_INVITATION"
	InviterID   uuid.UUID `json:"inviterId"`
	InviterName string    `json:"inviterName"`
	TripID      uuid.UUID `json:"tripId"`
	TripName    string    `json:"tripName"`
}

// ChatMessageMetadata is the structure for CHAT_MESSAGE notification metadata.
type ChatMessageMetadata struct {
	Type           string    `json:"type"` // Should always be "CHAT_MESSAGE"
	SenderID       uuid.UUID `json:"senderId"`
	SenderName     string    `json:"senderName"`
	TripID         uuid.UUID `json:"tripId"`
	TripName       string    `json:"tripName"`
	MessagePreview string    `json:"messagePreview"`
}

// TripMemberAddedMetadata is the structure for TRIP_MEMBER_ADDED notification metadata.
type TripMemberAddedMetadata struct {
	Type          string    `json:"type"` // Should always be "TRIP_MEMBER_ADDED"
	AdderID       uuid.UUID `json:"adderId"`
	AdderName     string    `json:"adderName"`
	AddedUserID   uuid.UUID `json:"addedUserId"`
	AddedUserName string    `json:"addedUserName"`
	TripID        uuid.UUID `json:"tripId"`
	TripName      string    `json:"tripName"`
}