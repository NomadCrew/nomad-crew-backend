package types

import (
	"context"
	"encoding/json"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
)

type EventType string

const (
	CategoryTrip     = "TRIP"
	CategoryTodo     = "TODO"
	CategoryWeather  = "WEATHER"
	CategoryLocation = "LOCATION"
	CategoryMember   = "MEMBER"
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

	// Member events
	EventTypeMemberAdded       EventType = CategoryMember + "_ADDED"
	EventTypeMemberRoleUpdated EventType = CategoryMember + "_ROLE_UPDATED"
	EventTypeMemberRemoved     EventType = CategoryMember + "_REMOVED"

	// Invitation events
	EventTypeInvitationCreated       EventType = CategoryTrip + "_INVITATION_CREATED"
	EventTypeInvitationAccepted      EventType = CategoryTrip + "_INVITATION_ACCEPTED"
	EventTypeInvitationStatusUpdated EventType = "invitation_status_updated"
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
	InvitationID string    `json:"invitationId"`
	InviteeEmail string    `json:"inviteeEmail"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

type InvitationStatusUpdatedEvent struct {
	InvitationID string           `json:"invitationId"`
	NewStatus    InvitationStatus `json:"newStatus"`
}
