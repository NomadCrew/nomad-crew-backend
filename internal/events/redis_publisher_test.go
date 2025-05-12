package events

import (
	"context"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupRedisContainer is defined in test_helpers.go

func TestRedisPublisher_PublishAndSubscribe(t *testing.T) {
	resetMetricsForTesting()
	rdb, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	publisher := NewRedisPublisher(rdb)
	defer func() {
		if err := publisher.Shutdown(context.Background()); err != nil {
			t.Logf("Error during publisher shutdown: %v", err)
		}
	}()

	tripID := "test-trip"
	userID := "test-user"
	event := types.Event{
		BaseEvent: types.BaseEvent{
			ID:        "test-event",
			Type:      types.EventTypeTripCreated,
			TripID:    tripID,
			UserID:    userID,
			Timestamp: time.Now(),
			Version:   1,
		},
		Metadata: types.EventMetadata{
			Source: "test",
		},
		Payload: []byte(`{"id":"123","name":"test"}`),
	}

	// Subscribe first
	events, err := publisher.Subscribe(ctx, tripID, userID)
	require.NoError(t, err)

	// Publish event
	err = publisher.Publish(ctx, tripID, event)
	require.NoError(t, err)

	// Wait for event
	select {
	case received := <-events:
		assert.Equal(t, event.Type, received.Type)
		assert.Equal(t, event.TripID, received.TripID)
		assert.Equal(t, event.UserID, received.UserID)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Unsubscribe
	err = publisher.Unsubscribe(ctx, tripID, userID)
	require.NoError(t, err)
}

func TestRedisPublisher_PublishBatch(t *testing.T) {
	resetMetricsForTesting()
	rdb, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	publisher := NewRedisPublisher(rdb)
	defer func() {
		if err := publisher.Shutdown(context.Background()); err != nil {
			t.Logf("Error during publisher shutdown: %v", err)
		}
	}()

	tripID := "test-trip"
	userID := "test-user"
	events := []types.Event{
		{
			BaseEvent: types.BaseEvent{
				ID:        "test-event-1",
				Type:      types.EventTypeTripCreated,
				TripID:    tripID,
				UserID:    userID,
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
				UserID:    userID,
				Timestamp: time.Now(),
				Version:   1,
			},
			Metadata: types.EventMetadata{
				Source: "test",
			},
			Payload: []byte(`{"id":"123","name":"test2"}`),
		},
	}

	// Subscribe first
	ch, err := publisher.Subscribe(ctx, tripID, userID)
	require.NoError(t, err)

	// Publish batch
	err = publisher.PublishBatch(ctx, tripID, events)
	require.NoError(t, err)

	// Wait for events
	received := make([]types.Event, 0, len(events))
	timeout := time.After(time.Second)

	for i := 0; i < len(events); i++ {
		select {
		case event := <-ch:
			received = append(received, event)
		case <-timeout:
			t.Fatal("timeout waiting for events")
		}
	}

	assert.Len(t, received, len(events))
	for i, event := range events {
		assert.Equal(t, event.Type, received[i].Type)
		assert.Equal(t, event.TripID, received[i].TripID)
		assert.Equal(t, event.UserID, received[i].UserID)
	}
}

func TestRedisPublisher_FilteredSubscription(t *testing.T) {
	resetMetricsForTesting()
	rdb, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	publisher := NewRedisPublisher(rdb)
	defer func() {
		if err := publisher.Shutdown(context.Background()); err != nil {
			t.Logf("Error during publisher shutdown: %v", err)
		}
	}()

	tripID := "test-trip"
	userID := "test-user"
	events := []types.Event{
		{
			BaseEvent: types.BaseEvent{
				ID:        "test-event-1",
				Type:      types.EventTypeTripCreated,
				TripID:    tripID,
				UserID:    userID,
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
				UserID:    userID,
				Timestamp: time.Now(),
				Version:   1,
			},
			Metadata: types.EventMetadata{
				Source: "test",
			},
			Payload: []byte(`{"id":"123","name":"test2"}`),
		},
	}

	// Subscribe with filter
	ch, err := publisher.Subscribe(ctx, tripID, userID, types.EventTypeTripCreated)
	require.NoError(t, err)

	// Publish events
	for _, event := range events {
		err = publisher.Publish(ctx, tripID, event)
		require.NoError(t, err)
	}

	// Should only receive EventTypeTripCreated
	select {
	case event := <-ch:
		assert.Equal(t, types.EventTypeTripCreated, event.Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Should not receive EventTypeTripUpdated
	select {
	case event := <-ch:
		t.Fatalf("received unexpected event: %v", event)
	case <-time.After(100 * time.Millisecond):
		// Expected timeout
	}
}

func TestRedisPublisher_DuplicateSubscription(t *testing.T) {
	resetMetricsForTesting()
	rdb, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	publisher := NewRedisPublisher(rdb)
	defer func() {
		if err := publisher.Shutdown(context.Background()); err != nil {
			t.Logf("Error during publisher shutdown: %v", err)
		}
	}()

	tripID := "test-trip"
	userID := "test-user"

	// First subscription should succeed
	_, err := publisher.Subscribe(ctx, tripID, userID)
	require.NoError(t, err)

	// Second subscription should fail
	_, err = publisher.Subscribe(ctx, tripID, userID)
	assert.Error(t, err)
}

func TestRedisPublisher_UnsubscribeNonexistent(t *testing.T) {
	resetMetricsForTesting()
	rdb, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	publisher := NewRedisPublisher(rdb)
	defer func() {
		if err := publisher.Shutdown(context.Background()); err != nil {
			t.Logf("Error during publisher shutdown: %v", err)
		}
	}()

	err := publisher.Unsubscribe(ctx, "nonexistent-trip", "nonexistent-user")
	assert.Error(t, err)
}

func TestRedisPublisher_Shutdown(t *testing.T) {
	resetMetricsForTesting()
	rdb, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	publisher := NewRedisPublisher(rdb)

	tripID := "test-trip"
	userID := "test-user"

	// Create some subscriptions
	_, err := publisher.Subscribe(ctx, tripID, userID)
	require.NoError(t, err)

	// Shutdown should close all subscriptions
	err = publisher.Shutdown(ctx)
	require.NoError(t, err)

	// New subscriptions should still work after shutdown
	_, err = publisher.Subscribe(ctx, tripID+"2", userID)
	require.NoError(t, err)
}
