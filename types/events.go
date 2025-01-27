package types

import (
	"context"
	"encoding/json"
	"time"
)

type EventType string

const (
	EventTypeTripUpdated  EventType = "TRIP_UPDATED"
	EventTypeTodoCreated  EventType = "TODO_CREATED"
	EventTypeTodoUpdated  EventType = "TODO_UPDATED"
	EventTypeTodoDeleted  EventType = "TODO_DELETED"
)

type Event struct {
	Type      EventType       `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
}

type EventPublisher interface {
	Publish(ctx context.Context, tripID string, event Event) error
	Subscribe(ctx context.Context, tripID string) (<-chan Event, error)
	Unsubscribe(ctx context.Context, tripID string, ch <-chan Event)
} 