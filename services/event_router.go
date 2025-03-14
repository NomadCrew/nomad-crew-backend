// services/event_router.go
package services

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

// EventHandler defines a function that processes an event.
type EventHandler func(ctx context.Context, event types.Event)

// EventRouter aggregates handlers and routes events to them.
type EventRouter struct {
	handlers map[types.EventType][]EventHandler
}

// NewEventRouter creates and returns a new EventRouter.
func NewEventRouter() *EventRouter {
	return &EventRouter{
		handlers: make(map[types.EventType][]EventHandler),
	}
}

// Register adds a new handler for a given event type.
func (er *EventRouter) Register(eventType types.EventType, handler EventHandler) {
	er.handlers[eventType] = append(er.handlers[eventType], handler)
}

// Route dispatches the event to all registered handlers for its type.
// Handlers are executed concurrently.
func (er *EventRouter) Route(ctx context.Context, event types.Event) {
	if hs, ok := er.handlers[event.Type]; ok {
		for _, handler := range hs {
			go handler(ctx, event)
		}
	} else {
		logger.GetLogger().Warnf("No handlers registered for event type: %s", event.Type)
	}
}
