package events

import (
	"context"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_RegisterHandler(t *testing.T) {
	rdb, cleanup := setupRedisContainer(t)
	defer cleanup()

	service := NewService(rdb)
	handler := newMockHandler(types.EventTypeTripCreated)

	t.Run("successful registration", func(t *testing.T) {
		err := service.RegisterHandler("test-handler", handler)
		require.NoError(t, err)
		assert.Contains(t, service.GetHandlerNames(), "test-handler")
	})

	t.Run("duplicate registration", func(t *testing.T) {
		err := service.RegisterHandler("test-handler", handler)
		require.Error(t, err)
		assert.Equal(t, "handler already registered with name test-handler", err.Error())
	})
}

func TestService_UnregisterHandler(t *testing.T) {
	rdb, cleanup := setupRedisContainer(t)
	defer cleanup()

	service := NewService(rdb)
	handler := newMockHandler(types.EventTypeTripCreated)

	t.Run("successful unregistration", func(t *testing.T) {
		err := service.RegisterHandler("test-handler", handler)
		require.NoError(t, err)

		err = service.UnregisterHandler("test-handler")
		require.NoError(t, err)
		assert.NotContains(t, service.GetHandlerNames(), "test-handler")
	})

	t.Run("unregister non-existent handler", func(t *testing.T) {
		err := service.UnregisterHandler("test-handler")
		require.Error(t, err)
		assert.Equal(t, "handler not found with name test-handler", err.Error())
	})
}

func TestService_PublishAndSubscribe(t *testing.T) {
	rdb, cleanup := setupRedisContainer(t)
	defer cleanup()

	service := NewService(rdb)
	handler := newMockHandler(types.EventTypeTripCreated)
	err := service.RegisterHandler("test-handler", handler)
	require.NoError(t, err)

	ctx := context.Background()
	tripID := "test-trip"
	event := types.Event{
		BaseEvent: types.BaseEvent{
			ID:        "test-event",
			Type:      types.EventTypeTripCreated,
			TripID:    tripID,
			UserID:    "test-user",
			Timestamp: time.Now(),
			Version:   1,
		},
		Metadata: types.EventMetadata{
			Source: "test",
		},
		Payload: []byte(`{"id":"123","name":"test"}`),
	}

	// Create a subscriber
	eventCh := make(chan types.Event, 1)
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		ch, err := service.Subscribe(subCtx, tripID, "test-user")
		if err != nil && err != context.Canceled {
			t.Errorf("Subscribe error: %v", err)
			return
		}
		for evt := range ch {
			eventCh <- evt
		}
	}()

	// Wait for subscription to be ready
	time.Sleep(100 * time.Millisecond)

	// Publish event
	err = service.Publish(ctx, tripID, event)
	require.NoError(t, err)

	// Wait for event to be processed
	time.Sleep(100 * time.Millisecond)

	// Verify handler received event
	events := handler.GetEvents()
	require.Len(t, events, 1)
	assert.Equal(t, event, events[0])

	// Verify subscriber received event
	select {
	case receivedEvent := <-eventCh:
		assert.Equal(t, event, receivedEvent)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestService_PublishBatch(t *testing.T) {
	rdb, cleanup := setupRedisContainer(t)
	defer cleanup()

	service := NewService(rdb)
	handler := newMockHandler(types.EventTypeTripCreated, types.EventTypeTripUpdated)
	err := service.RegisterHandler("test-handler", handler)
	require.NoError(t, err)

	ctx := context.Background()
	tripID := "test-trip"
	events := []types.Event{
		{
			BaseEvent: types.BaseEvent{
				ID:        "test-event-1",
				Type:      types.EventTypeTripCreated,
				TripID:    tripID,
				UserID:    "test-user",
				Timestamp: time.Now(),
				Version:   1,
			},
			Metadata: types.EventMetadata{
				Source: "test",
			},
			Payload: []byte(`{"id":"123","name":"test1"}`),
		},
		{
			BaseEvent: types.BaseEvent{
				ID:        "test-event-2",
				Type:      types.EventTypeTripUpdated,
				TripID:    tripID,
				UserID:    "test-user",
				Timestamp: time.Now(),
				Version:   1,
			},
			Metadata: types.EventMetadata{
				Source: "test",
			},
			Payload: []byte(`{"id":"123","name":"test2"}`),
		},
	}

	// Create a subscriber
	eventCh := make(chan types.Event, 2)
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		ch, err := service.Subscribe(subCtx, tripID, "test-user")
		if err != nil && err != context.Canceled {
			t.Errorf("Subscribe error: %v", err)
			return
		}
		for evt := range ch {
			eventCh <- evt
		}
	}()

	// Wait for subscription to be ready
	time.Sleep(100 * time.Millisecond)

	// Publish events
	err = service.PublishBatch(ctx, tripID, events)
	require.NoError(t, err)

	// Wait for events to be processed
	time.Sleep(100 * time.Millisecond)

	// Verify handler received events
	receivedEvents := handler.GetEvents()
	require.Len(t, receivedEvents, 2)
	assert.Equal(t, events, receivedEvents)

	// Verify subscriber received events
	receivedCount := 0
	timeout := time.After(time.Second)
	for receivedCount < 2 {
		select {
		case receivedEvent := <-eventCh:
			assert.Contains(t, events, receivedEvent)
			receivedCount++
		case <-timeout:
			t.Fatal("timeout waiting for events")
		}
	}
}

func TestService_Shutdown(t *testing.T) {
	rdb, cleanup := setupRedisContainer(t)
	defer cleanup()

	service := NewService(rdb)
	handler := newMockHandler(types.EventTypeTripCreated)
	err := service.RegisterHandler("test-handler", handler)
	require.NoError(t, err)

	// Start a subscription
	eventCh := make(chan types.Event, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		ch, err := service.Subscribe(ctx, "test-trip", "test-user")
		if err != nil && err != context.Canceled {
			t.Errorf("Subscribe error: %v", err)
			return
		}
		for evt := range ch {
			eventCh <- evt
		}
	}()

	// Wait for subscription to be ready
	time.Sleep(100 * time.Millisecond)

	// Shutdown the service
	err = service.Shutdown(context.Background())
	require.NoError(t, err)

	// Verify that new subscriptions can still be created
	newEventCh := make(chan types.Event, 1)
	go func() {
		ch, err := service.Subscribe(ctx, "test-trip", "test-user")
		if err != nil && err != context.Canceled {
			t.Errorf("Subscribe error after shutdown: %v", err)
			return
		}
		for evt := range ch {
			newEventCh <- evt
		}
	}()

	// Wait briefly to ensure subscription attempt completes
	time.Sleep(100 * time.Millisecond)
}

func TestService_HandlerError(t *testing.T) {
	rdb, cleanup := setupRedisContainer(t)
	defer cleanup()

	service := NewService(rdb)
	handler := newMockHandler(types.EventTypeTripCreated)
	handler.shouldError = true
	err := service.RegisterHandler("test-handler", handler)
	require.NoError(t, err)

	ctx := context.Background()
	tripID := "test-trip"
	event := types.Event{
		BaseEvent: types.BaseEvent{
			ID:        "test-event",
			Type:      types.EventTypeTripCreated,
			TripID:    tripID,
			UserID:    "test-user",
			Timestamp: time.Now(),
			Version:   1,
		},
		Metadata: types.EventMetadata{
			Source: "test",
		},
		Payload: []byte(`{"id":"123","name":"test"}`),
	}

	// Create a subscriber
	eventCh := make(chan types.Event, 1)
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		ch, err := service.Subscribe(subCtx, tripID, "test-user")
		if err != nil && err != context.Canceled {
			t.Errorf("Subscribe error: %v", err)
			return
		}
		for evt := range ch {
			eventCh <- evt
		}
	}()

	// Wait for subscription to be ready
	time.Sleep(100 * time.Millisecond)

	// Publish event
	err = service.Publish(ctx, tripID, event)
	require.NoError(t, err)

	// Verify subscriber still received event despite handler error
	select {
	case receivedEvent := <-eventCh:
		assert.Equal(t, event, receivedEvent)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}
