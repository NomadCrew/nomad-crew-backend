package events

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// mockHandler is a test implementation of the EventHandler interface
type mockHandler struct {
	mu              sync.Mutex
	events          []types.Event
	supportedTypes  []types.EventType
	shouldError     bool
	handlerLatency  time.Duration
	handlerBlocking bool
}

func newMockHandler(supportedTypes ...types.EventType) *mockHandler {
	return &mockHandler{
		events:         make([]types.Event, 0),
		supportedTypes: supportedTypes,
	}
}

func (h *mockHandler) HandleEvent(ctx context.Context, event types.Event) error {
	if h.handlerLatency > 0 {
		time.Sleep(h.handlerLatency)
	}

	if h.handlerBlocking {
		<-ctx.Done()
		return ctx.Err()
	}

	if h.shouldError {
		return fmt.Errorf("mock handler error")
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, event)
	return nil
}

func (h *mockHandler) SupportedEvents() []types.EventType {
	return h.supportedTypes
}

func (h *mockHandler) GetEvents() []types.Event {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.events
}

// setupRedisContainer creates a Redis container for testing
func setupRedisContainer(t *testing.T) (*redis.Client, func()) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}

	redisC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start redis container: %v", err)
	}

	mappedPort, err := redisC.MappedPort(ctx, "6379")
	if err != nil {
		t.Fatalf("failed to get container external port: %v", err)
	}

	hostIP, err := redisC.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", hostIP, mappedPort.Port()),
	})

	cleanup := func() {
		if err := redisC.Terminate(ctx); err != nil {
			t.Errorf("failed to terminate container: %v", err)
		}
	}

	return rdb, cleanup
}
