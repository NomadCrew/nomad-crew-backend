package shared

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/NomadCrew/nomad-crew-backend/internal/events"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

type EventEmitter struct {
	eventBus types.EventPublisher
}

func NewEventEmitter(eventBus types.EventPublisher) *EventEmitter {
	return &EventEmitter{eventBus: eventBus}
}

func (e *EventEmitter) EmitTripEvent(ctx context.Context, tripID string, eventType types.EventType, payload interface{}, userID string) error {
	var payloadMap map[string]interface{}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal event payload: %w", err)
	}
	if err := json.Unmarshal(payloadJSON, &payloadMap); err != nil {
		return fmt.Errorf("failed to unmarshal event payload to map: %w", err)
	}

	return events.PublishEventWithContext(
		e.eventBus,
		ctx,
		string(eventType),
		tripID,
		userID,
		payloadMap,
		"trip_model (via EventEmitter)",
	)
}
