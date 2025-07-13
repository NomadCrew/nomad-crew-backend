package events

import (
	"context"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Create a test router with non-registering metrics to avoid duplicate registration
func testRouter() *Router {
	// Create test metrics that don't use promauto (which always registers to the global registry)
	metrics := &RouterMetrics{
		handlerCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "test_event_handlers_total",
			Help: "Test metric",
		}),
		handlerLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "test_event_handler_duration_seconds",
			Help:    "Test metric",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		}),
		handlerErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "test_event_handler_errors_total",
			Help: "Test metric",
		}, []string{"event_type"}),
		eventsRouted: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "test_events_routed_total",
			Help: "Test metric",
		}, []string{"event_type"}),
		eventsDiscarded: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "test_events_discarded_total",
			Help: "Test metric",
		}, []string{"reason"}),
	}

	return &Router{
		log:      logger.GetLogger().Named("test_event_router"),
		metrics:  metrics,
		handlers: make(map[types.EventType][]types.EventHandler),
	}
}

// mockHandler is defined in test_helpers.go

func TestRouter_RegisterHandler(t *testing.T) {
	router := testRouter()
	handler := newMockHandler(types.EventTypeTripCreated, types.EventTypeTripUpdated)

	router.RegisterHandler(handler)

	// Verify handler count
	assert.Equal(t, 1, router.countHandlers())

	// Verify handler registration for each event type
	router.mu.RLock()
	for _, eventType := range handler.supportedTypes {
		handlers := router.handlers[eventType]
		assert.Len(t, handlers, 1)
		assert.Equal(t, handler, handlers[0])
	}
	router.mu.RUnlock()
}

func TestRouter_UnregisterHandler(t *testing.T) {
	router := testRouter()
	handler := newMockHandler(types.EventTypeTripCreated, types.EventTypeTripUpdated)

	router.RegisterHandler(handler)
	router.UnregisterHandler(handler)

	// Verify handler count is 0
	assert.Equal(t, 0, router.countHandlers())

	// Verify no handlers remain for event types
	router.mu.RLock()
	for _, eventType := range handler.supportedTypes {
		handlers := router.handlers[eventType]
		assert.Empty(t, handlers)
	}
	router.mu.RUnlock()
}

func TestRouter_HandleEvent(t *testing.T) {
	router := testRouter()
	handler1 := newMockHandler(types.EventTypeTripCreated)
	handler2 := newMockHandler(types.EventTypeTripCreated)

	router.RegisterHandler(handler1)
	router.RegisterHandler(handler2)

	event := types.Event{
		BaseEvent: types.BaseEvent{
			Type:   types.EventTypeTripCreated,
			TripID: "test-trip",
		},
	}

	err := router.HandleEvent(context.Background(), event)
	require.NoError(t, err)

	// Verify both handlers received the event
	assert.Len(t, handler1.GetEvents(), 1)
	assert.Len(t, handler2.GetEvents(), 1)
	assert.Equal(t, event, handler1.GetEvents()[0])
	assert.Equal(t, event, handler2.GetEvents()[0])
}

func TestRouter_HandleEvent_NoHandlers(t *testing.T) {
	router := testRouter()
	event := types.Event{
		BaseEvent: types.BaseEvent{
			Type:   types.EventTypeTripCreated,
			TripID: "test-trip",
		},
	}

	// Should not error when no handlers are registered
	err := router.HandleEvent(context.Background(), event)
	assert.NoError(t, err)
}

func TestRouter_HandleEvent_HandlerError(t *testing.T) {
	router := testRouter()
	handler := newMockHandler(types.EventTypeTripCreated)
	handler.shouldError = true

	router.RegisterHandler(handler)

	event := types.Event{
		BaseEvent: types.BaseEvent{
			Type:   types.EventTypeTripCreated,
			TripID: "test-trip",
		},
	}

	err := router.HandleEvent(context.Background(), event)
	assert.Error(t, err)
}

func TestRouter_HandleEvent_Concurrent(t *testing.T) {
	router := testRouter()
	handler1 := newMockHandler(types.EventTypeTripCreated)
	handler2 := newMockHandler(types.EventTypeTripCreated)
	handler1.handlerLatency = 100 * time.Millisecond
	handler2.handlerLatency = 100 * time.Millisecond

	router.RegisterHandler(handler1)
	router.RegisterHandler(handler2)

	event := types.Event{
		BaseEvent: types.BaseEvent{
			Type:   types.EventTypeTripCreated,
			TripID: "test-trip",
		},
	}

	start := time.Now()
	err := router.HandleEvent(context.Background(), event)
	duration := time.Since(start)

	require.NoError(t, err)
	// Both handlers should run concurrently, so total time should be ~100ms, not ~200ms
	assert.Less(t, duration, 150*time.Millisecond)
	assert.Len(t, handler1.GetEvents(), 1)
	assert.Len(t, handler2.GetEvents(), 1)
}

func TestRouter_HandleEvent_Context(t *testing.T) {
	router := testRouter()
	handler := newMockHandler(types.EventTypeTripCreated)
	handler.handlerBlocking = true

	router.RegisterHandler(handler)

	event := types.Event{
		BaseEvent: types.BaseEvent{
			Type:   types.EventTypeTripCreated,
			TripID: "test-trip",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := router.HandleEvent(ctx, event)
	assert.Error(t, err)
	assert.Empty(t, handler.GetEvents())
}

func TestRouter_MultipleEventTypes(t *testing.T) {
	router := testRouter()
	handler := newMockHandler(types.EventTypeTripCreated, types.EventTypeTripUpdated)

	router.RegisterHandler(handler)

	events := []types.Event{
		{
			BaseEvent: types.BaseEvent{
				Type:   types.EventTypeTripCreated,
				TripID: "test-trip",
			},
		},
		{
			BaseEvent: types.BaseEvent{
				Type:   types.EventTypeTripUpdated,
				TripID: "test-trip",
			},
		},
		{
			BaseEvent: types.BaseEvent{
				Type:   types.EventTypeTripDeleted,
				TripID: "test-trip",
			},
		},
	}

	for _, event := range events {
		err := router.HandleEvent(context.Background(), event)
		require.NoError(t, err)
	}

	// Handler should only receive events it's registered for
	receivedEvents := handler.GetEvents()
	assert.Len(t, receivedEvents, 2)
	assert.Equal(t, types.EventTypeTripCreated, receivedEvents[0].Type)
	assert.Equal(t, types.EventTypeTripUpdated, receivedEvents[1].Type)
}
