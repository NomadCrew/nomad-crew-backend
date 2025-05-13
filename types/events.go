package types

import (
	"context"
	"encoding/json"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	EventSerializeDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "event_serialize_seconds",
		Help:    "Time spent serializing events",
		Buckets: []float64{.0001, .0005, .001, .005, .01, .05},
	})
	EventSizeBytes = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "event_size_bytes",
		Help:    "Serialized event sizes in bytes",
		Buckets: []float64{64, 128, 256, 512, 1024, 2048, 4096},
	})
)

func init() {
	prometheus.MustRegister(
		EventSerializeDuration,
		EventSizeBytes,
	)
}

type EventType string

const (
	CategoryTrip     = "TRIP"
	CategoryTodo     = "TODO"
	CategoryWeather  = "WEATHER"
	CategoryLocation = "LOCATION"
	CategoryMember   = "MEMBER"
	CategoryChat     = "CHAT"
)

const (
	// Trip events
	EventTypeTripCreated       EventType = CategoryTrip + "_CREATED"
	EventTypeTripUpdated       EventType = CategoryTrip + "_UPDATED"
	EventTypeTripDeleted       EventType = CategoryTrip + "_DELETED"
	EventTypeTripStarted       EventType = CategoryTrip + "_STARTED"
	EventTypeTripEnded         EventType = CategoryTrip + "_ENDED"
	EventTypeTripStatusUpdated EventType = CategoryTrip + "_STATUS_UPDATED"

	// Todo events
	EventTypeTodoCreated  EventType = CategoryTodo + "_CREATED"
	EventTypeTodoUpdated  EventType = CategoryTodo + "_UPDATED"
	EventTypeTodoDeleted  EventType = CategoryTodo + "_DELETED"
	EventTypeTodoComplete EventType = CategoryTodo + "_COMPLETED"

	// Weather events
	EventTypeWeatherUpdated EventType = CategoryWeather + "_UPDATED"
	EventTypeWeatherAlert   EventType = CategoryWeather + "_ALERT"

	// Location events
	EventTypeLocationUpdated EventType = CategoryLocation + "_UPDATED"

	// Member events
	EventTypeMemberAdded       EventType = CategoryMember + "_ADDED"
	EventTypeMemberRoleUpdated EventType = CategoryMember + "_ROLE_UPDATED"
	EventTypeMemberRemoved     EventType = CategoryMember + "_REMOVED"

	// Invitation events
	EventTypeInvitationCreated       EventType = CategoryTrip + "_INVITATION_CREATED"
	EventTypeInvitationAccepted      EventType = CategoryTrip + "_INVITATION_ACCEPTED"
	EventTypeInvitationStatusUpdated EventType = "invitation_status_updated"

	// Chat events
	EventTypeChatGroupCreated       EventType = CategoryChat + "_GROUP_CREATED"
	EventTypeChatMessageSent        EventType = CategoryChat + "_MESSAGE_SENT"
	EventTypeChatMessageEdited      EventType = CategoryChat + "_MESSAGE_EDITED"
	EventTypeChatMessageDeleted     EventType = CategoryChat + "_MESSAGE_DELETED"
	EventTypeChatReactionAdded      EventType = CategoryChat + "_REACTION_ADDED"
	EventTypeChatReactionRemoved    EventType = CategoryChat + "_REACTION_REMOVED"
	EventTypeChatReadReceipt        EventType = CategoryChat + "_READ_RECEIPT"
	EventTypeChatMemberAdded        EventType = CategoryChat + "_MEMBER_ADDED"
	EventTypeChatMemberRemoved      EventType = CategoryChat + "_MEMBER_REMOVED"
	EventTypeChatReadReceiptUpdated EventType = CategoryChat + "_READ_RECEIPT_UPDATED"
	EventTypeChatTypingStatus       EventType = CategoryChat + "_TYPING_STATUS"
	EventTypeChatGroupUpdated       EventType = CategoryChat + "_GROUP_UPDATED"
	EventTypeChatGroupDeleted       EventType = CategoryChat + "_GROUP_DELETED"

	// EventTypeLocationProcessed indicates that a user's location has been processed.
	EventTypeLocationProcessed = "location.processed"

	// EventTypeUserJoinedTrip indicates a user has joined a trip.
	EventTypeUserJoinedTrip = "user.joined_trip"

	// EventTypeUserLeftTrip indicates a user has left a trip.
	EventTypeUserLeftTrip = "user.left_trip"

	// EventTypeUserInvitedToTrip indicates a user has been invited to a trip.
	EventTypeUserInvitedToTrip = "user.invited_to_trip"

	// EventTypeTripInvitationAccepted indicates a trip invitation was accepted.
	EventTypeTripInvitationAccepted = "trip.invitation_accepted"

	// EventTypeTripInvitationDeclined indicates a trip invitation was declined.
	EventTypeTripInvitationDeclined = "trip.invitation_declined"

	// EventTypeNotificationCreated indicates a new notification has been created.
	EventTypeNotificationCreated = "notification.created"
)

// Base event interface
type BaseEvent struct {
	ID        string    `json:"id"`
	Type      EventType `json:"type"`
	TripID    string    `json:"tripId"`
	UserID    string    `json:"userId,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Version   int       `json:"version"`
}

// EventMetadata for tracking and debugging
type EventMetadata struct {
	CorrelationID string            `json:"correlationId,omitempty"`
	CausationID   string            `json:"causationId,omitempty"`
	Source        string            `json:"source"`
	Tags          map[string]string `json:"tags,omitempty"`
}

// Enhanced Event structure
type Event struct {
	BaseEvent
	Metadata EventMetadata   `json:"metadata"`
	Payload  json.RawMessage `json:"payload"`
}

// Validation method for events
func (e Event) Validate() error {
	if e.ID == "" {
		return errors.ValidationFailed("invalid event", "event ID is required")
	}
	if e.Type == "" {
		return errors.ValidationFailed("invalid event", "event type is required")
	}
	if e.TripID == "" {
		return errors.ValidationFailed("invalid event", "trip ID is required")
	}
	if e.Timestamp.IsZero() {
		return errors.ValidationFailed("invalid event", "timestamp is required")
	}
	return nil
}

// EventPublisher with enhanced capabilities
type EventPublisher interface {
	Publish(ctx context.Context, tripID string, event Event) error
	PublishBatch(ctx context.Context, tripID string, events []Event) error
	Subscribe(ctx context.Context, tripID string, userID string, filters ...EventType) (<-chan Event, error)
	Unsubscribe(ctx context.Context, tripID string, userID string) error
}

// EventHandler for processing events
type EventHandler interface {
	HandleEvent(ctx context.Context, event Event) error
	SupportedEvents() []EventType
}

type MemberRoleUpdatedEvent struct {
	MemberID  string     `json:"memberId"`
	OldRole   MemberRole `json:"oldRole"`
	NewRole   MemberRole `json:"newRole"`
	UpdatedBy string     `json:"updatedBy"`
}

type MemberRemovedEvent struct {
	RemovedUserID string `json:"removedUserId"`
	RemovedBy     string `json:"removedBy"`
}

type InvitationCreatedEvent struct {
	EventID       string    `json:"event_id"`
	InvitationID  string    `json:"invitation_id"`
	InviteeEmail  string    `json:"invitee_email"`
	ExpiresAt     time.Time `json:"expires_at"`
	AcceptanceURL string    `json:"acceptance_url"`
}

type InvitationStatusUpdatedEvent struct {
	InvitationID string           `json:"invitationId"`
	NewStatus    InvitationStatus `json:"newStatus"`
}

// Chat event payloads
type ChatMessageEvent struct {
	MessageID string       `json:"messageId"`
	TripID    string       `json:"tripId"`
	Content   string       `json:"content,omitempty"`
	User      UserResponse `json:"user,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
}

type ChatReactionEvent struct {
	MessageID string       `json:"messageId"`
	TripID    string       `json:"tripId"`
	Reaction  string       `json:"reaction"`
	User      UserResponse `json:"user,omitempty"`
}

type ChatReadReceiptEvent struct {
	TripID    string       `json:"tripId"`
	MessageID string       `json:"messageId"`
	User      UserResponse `json:"user,omitempty"`
}

// ChatTypingStatusEvent represents a typing status event
type ChatTypingStatusEvent struct {
	TripID   string       `json:"tripId"`
	IsTyping bool         `json:"isTyping"`
	User     UserResponse `json:"user,omitempty"`
}

// ChatMemberEvent represents an event related to a chat group member being added or removed.
// Used for both ChatMemberAdded and ChatMemberRemoved event types.
type ChatMemberEvent struct {
	GroupID string       `json:"groupId"`
	User    UserResponse `json:"user"` // The user who was added or removed
}

// ChatGroupEvent represents an event related to chat group creation, update, or deletion.
type ChatGroupEvent struct {
	GroupID   string `json:"groupId"`
	TripID    string `json:"tripId"`
	UserID    string `json:"userId"`              // User performing the action
	GroupName string `json:"groupName,omitempty"` // Included for create/update
}

// NotificationCreatedEvent signifies that a new notification has been created.
// It contains identifiers for the consumer (e.g., WebSocket service) to fetch
// the full notification details before broadcasting.
type NotificationCreatedEvent struct {
	Timestamp      time.Time `json:"timestamp"`
	NotificationID uuid.UUID `json:"notificationId"`
	UserID         uuid.UUID `json:"userId"` // The ID of the user the notification is for
}
