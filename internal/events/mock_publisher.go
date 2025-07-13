package events

import (
	"context"
	"fmt"
	"sync"

	"github.com/NomadCrew/nomad-crew-backend/types"
)

// MockPublisher implements types.EventPublisher for testing
type MockPublisher struct {
	mu            sync.RWMutex
	events        map[string][]types.Event // key: tripID
	subscriptions map[string]chan types.Event
	closed        bool
}

// NewMockPublisher creates a new mock publisher for testing
func NewMockPublisher() *MockPublisher {
	return &MockPublisher{
		events:        make(map[string][]types.Event),
		subscriptions: make(map[string]chan types.Event),
	}
}

// Publish records an event for testing
func (m *MockPublisher) Publish(ctx context.Context, tripID string, event types.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return fmt.Errorf("publisher is closed")
	}

	m.events[tripID] = append(m.events[tripID], event)

	// Notify subscribers
	for subKey, ch := range m.subscriptions {
		if subKey == tripID {
			select {
			case ch <- event:
			default:
				// Channel is full or closed, skip
			}
		}
	}

	return nil
}

// PublishBatch records multiple events for testing
func (m *MockPublisher) PublishBatch(ctx context.Context, tripID string, events []types.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return fmt.Errorf("publisher is closed")
	}

	m.events[tripID] = append(m.events[tripID], events...)

	// Notify subscribers
	for subKey, ch := range m.subscriptions {
		if subKey == tripID {
			for _, event := range events {
				select {
				case ch <- event:
				default:
					// Channel is full or closed, skip
				}
			}
		}
	}

	return nil
}

// Subscribe creates a subscription for testing
func (m *MockPublisher) Subscribe(ctx context.Context, tripID string, userID string, filters ...types.EventType) (<-chan types.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, fmt.Errorf("publisher is closed")
	}

	subKey := tripID
	if _, exists := m.subscriptions[subKey]; exists {
		return nil, fmt.Errorf("subscription already exists")
	}

	// Create buffered channel
	ch := make(chan types.Event, 100)
	m.subscriptions[subKey] = ch

	// Send existing events that match filters
	go func() {
		m.mu.RLock()
		events := m.events[tripID]
		m.mu.RUnlock()

		for _, event := range events {
			if len(filters) > 0 {
				matched := false
				for _, filter := range filters {
					if event.Type == filter {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}

			select {
			case ch <- event:
			case <-ctx.Done():
				return
			default:
				// Channel is full, skip
			}
		}
	}()

	return ch, nil
}

// Unsubscribe removes a subscription for testing
func (m *MockPublisher) Unsubscribe(ctx context.Context, tripID string, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	subKey := tripID
	ch, exists := m.subscriptions[subKey]
	if !exists {
		return fmt.Errorf("subscription not found")
	}

	close(ch)
	delete(m.subscriptions, subKey)
	return nil
}

// GetEvents returns all events for a trip (for testing assertions)
func (m *MockPublisher) GetEvents(tripID string) []types.Event {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.events[tripID]
}

// Reset clears all events and subscriptions (for test cleanup)
func (m *MockPublisher) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close all subscriptions
	for _, ch := range m.subscriptions {
		close(ch)
	}

	m.events = make(map[string][]types.Event)
	m.subscriptions = make(map[string]chan types.Event)
	m.closed = false
}

// Close marks the publisher as closed (for testing cleanup)
func (m *MockPublisher) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close all subscriptions
	for _, ch := range m.subscriptions {
		close(ch)
	}

	m.closed = true
}
