package mocks

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/stretchr/testify/mock"
)

type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) Publish(ctx context.Context, tripID string, event types.Event) error {
	args := m.Called(ctx, tripID, event)
	return args.Error(0)
}

func (m *MockEventPublisher) PublishBatch(ctx context.Context, tripID string, events []types.Event) error {
	args := m.Called(ctx, tripID, events)
	return args.Error(0)
}

func (m *MockEventPublisher) Subscribe(ctx context.Context, tripID string, userID string, eventTypes ...types.EventType) (<-chan types.Event, error) {
	args := m.Called(ctx, tripID, userID, eventTypes)
	return args.Get(0).(<-chan types.Event), args.Error(1)
}

func (m *MockEventPublisher) Unsubscribe(ctx context.Context, tripID string, userID string) error {
	args := m.Called(ctx, tripID, userID)
	return args.Error(0)
}
