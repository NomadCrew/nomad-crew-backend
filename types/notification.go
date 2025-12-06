package types

import (
	"encoding/json"
	"time"
)

// NotificationType represents the type of notification
type NotificationType string

const (
	NotificationTypeTripInvitationReceived NotificationType = "TRIP_INVITATION_RECEIVED"
	NotificationTypeTripInvitationAccepted NotificationType = "TRIP_INVITATION_ACCEPTED"
	NotificationTypeTripInvitationDeclined NotificationType = "TRIP_INVITATION_DECLINED"
	NotificationTypeTripUpdate             NotificationType = "TRIP_UPDATE"
	NotificationTypeNewChatMessage         NotificationType = "NEW_CHAT_MESSAGE"
	NotificationTypeExpenseReportSubmitted NotificationType = "EXPENSE_REPORT_SUBMITTED"
	NotificationTypeTaskAssigned           NotificationType = "TASK_ASSIGNED"
	NotificationTypeTaskCompleted          NotificationType = "TASK_COMPLETED"
	NotificationTypeLocationShared         NotificationType = "LOCATION_SHARED"
	NotificationTypeMembershipChange       NotificationType = "MEMBERSHIP_CHANGE"
)

// Notification represents a user notification in the system
type Notification struct {
	ID        string           `json:"id"`
	UserID    string           `json:"userId"`
	Type      NotificationType `json:"type"`
	Metadata  json.RawMessage  `json:"metadata"`
	IsRead    bool             `json:"isRead"`
	CreatedAt time.Time        `json:"createdAt"`
	UpdatedAt time.Time        `json:"updatedAt"`
}

// NotificationCreate represents the data needed to create a new notification
type NotificationCreate struct {
	UserID   string           `json:"userId" binding:"required"`
	Type     NotificationType `json:"type" binding:"required"`
	Metadata json.RawMessage  `json:"metadata"`
}

// NotificationListParams represents parameters for listing notifications
type NotificationListParams struct {
	Limit  int32  `json:"limit"`
	Offset int32  `json:"offset"`
	Status *bool  `json:"status,omitempty"` // nil = all, true = read, false = unread
}

// Specific Metadata Structs (for type safety when constructing notifications)

// TripInvitationMetadata is the structure for TRIP_INVITATION notification metadata
type TripInvitationMetadata struct {
	Type        string `json:"type"` // Should always be "TRIP_INVITATION"
	InviterID   string `json:"inviterId"`
	InviterName string `json:"inviterName"`
	TripID      string `json:"tripId"`
	TripName    string `json:"tripName"`
}

// ChatMessageMetadata is the structure for CHAT_MESSAGE notification metadata
type ChatMessageMetadata struct {
	Type           string `json:"type"` // Should always be "CHAT_MESSAGE"
	SenderID       string `json:"senderId"`
	SenderName     string `json:"senderName"`
	TripID         string `json:"tripId"`
	TripName       string `json:"tripName"`
	MessagePreview string `json:"messagePreview"`
}

// TripMemberAddedMetadata is the structure for TRIP_MEMBER_ADDED notification metadata
type TripMemberAddedMetadata struct {
	Type          string `json:"type"` // Should always be "TRIP_MEMBER_ADDED"
	AdderID       string `json:"adderId"`
	AdderName     string `json:"adderName"`
	AddedUserID   string `json:"addedUserId"`
	AddedUserName string `json:"addedUserName"`
	TripID        string `json:"tripId"`
	TripName      string `json:"tripName"`
}
