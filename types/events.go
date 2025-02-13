package types

import (
    "context"
    "encoding/json"
    "time"
)

// EventType represents all possible event types in the system
type EventType string

const (
    // System Events
    EventTypeHeartbeat EventType = "HEARTBEAT"

    // Trip Lifecycle Events
    EventTypeTripUpdated      EventType = "TRIP_UPDATED"
    EventTypeTripStatusChange EventType = "TRIP_STATUS_CHANGED"
    EventTypeTripActivated    EventType = "TRIP_ACTIVATED"
    EventTypeTripCompleted    EventType = "TRIP_COMPLETED"
    EventTypeTripCancelled    EventType = "TRIP_CANCELLED"

    // Member Events
    EventTypeMemberJoined      EventType = "MEMBER_JOINED"
    EventTypeMemberLeft        EventType = "MEMBER_LEFT"
    EventTypeMemberRoleChange  EventType = "MEMBER_ROLE_CHANGED"

    // Todo Events
    EventTypeTodoCreated EventType = "TODO_CREATED"
    EventTypeTodoUpdated EventType = "TODO_UPDATED"
    EventTypeTodoDeleted EventType = "TODO_DELETED"

    // Weather Events
    EventTypeWeatherUpdated EventType = "WEATHER_UPDATED"
)

// Event represents a domain event in the system
type Event struct {
    ID        string          `json:"id"`
    Type      EventType       `json:"type"`
    Payload   json.RawMessage `json:"payload"`
    Timestamp time.Time       `json:"timestamp"`
}

// EventService defines the interface for event handling
type EventService interface {
    // Publish sends an event to all subscribers of a trip
    Publish(ctx context.Context, tripID string, event Event) error
    
    // Subscribe returns a channel that receives events for a specific trip
    Subscribe(ctx context.Context, tripID string, userID string) (<-chan Event, error)
    
    // Shutdown gracefully closes all subscriptions and stops the service
    Shutdown()
}

// EventServiceConfig holds configuration for the event service
type EventServiceConfig struct {
    MaxConnPerTrip    int           `json:"maxConnPerTrip"`
    MaxConnPerUser    int           `json:"maxConnPerUser"`
    HeartbeatInterval time.Duration `json:"heartbeatInterval"`
    EventBufferSize   int           `json:"eventBufferSize"`
    CleanupInterval   time.Duration `json:"cleanupInterval"`
}

// Event Payloads

// TripStatusChangeEvent represents the payload for trip status changes
type TripStatusChangeEvent struct {
    TripID         string     `json:"tripId"`
    PreviousStatus TripStatus `json:"previousStatus"`
    NewStatus      TripStatus `json:"newStatus"`
    ChangedBy      string     `json:"changedBy"`
    Timestamp      time.Time  `json:"timestamp"`
    Reason         string     `json:"reason,omitempty"`
    Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// MemberEvent represents the payload for member-related events
type MemberEvent struct {
    TripID    string     `json:"tripId"`
    UserID    string     `json:"userId"`
    Role      MemberRole `json:"role"`
    Timestamp time.Time  `json:"timestamp"`
    AddedBy   string    `json:"addedBy,omitempty"`  // For member joined
    Reason    string    `json:"reason,omitempty"`   // For member left
}