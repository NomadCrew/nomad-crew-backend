package services

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/stretchr/testify/mock"
)

// MockEventService is a mock implementation of the EventService interface
type MockEventService struct {
	mock.Mock
}

// Subscribe mocks the Subscribe method
func (m *MockEventService) Subscribe(ctx context.Context, tripID string, userID string) (<-chan types.Event, error) {
	args := m.Called(ctx, tripID, userID)
	return args.Get(0).(<-chan types.Event), args.Error(1)
}

// Publish mocks the Publish method
func (m *MockEventService) Publish(ctx context.Context, tripID string, event types.Event) error {
	args := m.Called(ctx, tripID, event)
	return args.Error(0)
}

// Shutdown mocks the Shutdown method
func (m *MockEventService) Shutdown() {
	m.Called()
}
