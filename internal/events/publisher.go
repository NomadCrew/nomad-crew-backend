package events

import (
	"context"
	"encoding/json"
	"time"

	internal_errors "github.com/NomadCrew/nomad-crew-backend/internal/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/utils"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

// PublishEventWithContext is a helper function to publish events with consistent structure
// It constructs a standard types.Event and publishes it using the provided publisher.
func PublishEventWithContext(publisher types.EventPublisher, ctx context.Context, eventType string, tripID string, userID string, data map[string]interface{}, source string) error {
	// Convert data to JSON
	payload, err := json.Marshal(data)
	if err != nil {
		return internal_errors.NewOperationFailedError("Failed to marshal event data", err)
	}

	// Create event with standard metadata
	event := types.Event{
		BaseEvent: types.BaseEvent{
			ID:        utils.GenerateEventID(), // Use centralized ID generator
			Type:      types.EventType(eventType),
			TripID:    tripID,
			UserID:    userID,
			Timestamp: time.Now(), // Use standard time func
			Version:   1,          // Default version
		},
		Metadata: types.EventMetadata{
			Source: source, // Identify the origin of the event
		},
		Payload: payload,
	}

	// Publish to appropriate topic based on event type
	if err := publisher.Publish(ctx, tripID, event); err != nil {
		// Wrap the error for better context
		return internal_errors.NewOperationFailedError("Failed to publish event", err)
	}

	return nil
}
