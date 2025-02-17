package shared

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
    "github.com/NomadCrew/nomad-crew-backend/types"
    "github.com/google/uuid"
)

type EventEmitter struct {
    eventBus types.EventPublisher
}

func NewEventEmitter(eventBus types.EventPublisher) *EventEmitter {
    return &EventEmitter{eventBus: eventBus}
}

func (e *EventEmitter) EmitTripEvent(ctx context.Context, tripID string, eventType types.EventType, payload interface{}, userID string) error {
    data, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("failed to marshal event payload: %w", err)
    }

    return e.eventBus.Publish(ctx, tripID, types.Event{
        BaseEvent: types.BaseEvent{
            ID:        uuid.New().String(),
            Type:      eventType,
            TripID:    tripID,
            UserID:    userID,
            Timestamp: time.Now(),
            Version:   1,
        },
        Metadata: types.EventMetadata{
            Source: "trip_model",
        },
        Payload: data,
    })
}