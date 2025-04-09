package events

import (
	"context"
	"fmt"
	"sync"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Service coordinates event publishing and handling
type Service struct {
	log       *zap.SugaredLogger
	publisher *RedisPublisher
	router    *Router
	mu        sync.RWMutex
	handlers  map[string]types.EventHandler // key: handler name
}

// NewService creates a new event service
func NewService(rdb *redis.Client, cfg ...Config) *Service {
	return &Service{
		log:       logger.GetLogger().Named("event_service"),
		publisher: NewRedisPublisher(rdb, cfg...),
		router:    NewRouter(),
		handlers:  make(map[string]types.EventHandler),
	}
}

// RegisterHandler registers an event handler with both the router and service
func (s *Service) RegisterHandler(name string, handler types.EventHandler) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.handlers[name]; exists {
		return fmt.Errorf("handler with name %s already registered", name)
	}

	s.handlers[name] = handler
	s.router.RegisterHandler(handler)

	s.log.Infow("Registered event handler",
		"name", name,
		"type", fmt.Sprintf("%T", handler),
		"supportedEvents", handler.SupportedEvents(),
	)

	return nil
}

// UnregisterHandler removes a handler by name
func (s *Service) UnregisterHandler(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	handler, exists := s.handlers[name]
	if !exists {
		return fmt.Errorf("handler %s not found", name)
	}

	s.router.UnregisterHandler(handler)
	delete(s.handlers, name)

	s.log.Infow("Unregistered event handler", "name", name)
	return nil
}

// Publish publishes an event and routes it to handlers
func (s *Service) Publish(ctx context.Context, tripID string, event types.Event) error {
	// First route to local handlers
	if err := s.router.HandleEvent(ctx, event); err != nil {
		s.log.Errorw("Error handling event locally",
			"error", err,
			"tripID", tripID,
			"eventType", event.Type,
		)
		// Continue with publishing even if local handling fails
	}

	// Then publish to Redis for distributed handling
	return s.publisher.Publish(ctx, tripID, event)
}

// PublishBatch publishes multiple events atomically
func (s *Service) PublishBatch(ctx context.Context, tripID string, events []types.Event) error {
	// First route all events to local handlers
	for _, event := range events {
		if err := s.router.HandleEvent(ctx, event); err != nil {
			s.log.Errorw("Error handling event locally in batch",
				"error", err,
				"tripID", tripID,
				"eventType", event.Type,
			)
			// Continue with other events even if one fails
		}
	}

	// Then publish batch to Redis
	return s.publisher.PublishBatch(ctx, tripID, events)
}

// Subscribe subscribes to events for a specific trip
func (s *Service) Subscribe(ctx context.Context, tripID string, userID string, filters ...types.EventType) (<-chan types.Event, error) {
	return s.publisher.Subscribe(ctx, tripID, userID, filters...)
}

// Unsubscribe removes a subscription
func (s *Service) Unsubscribe(ctx context.Context, tripID string, userID string) error {
	return s.publisher.Unsubscribe(ctx, tripID, userID)
}

// Shutdown gracefully shuts down the service
func (s *Service) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear all handlers
	for name := range s.handlers {
		if err := s.UnregisterHandler(name); err != nil {
			s.log.Errorw("Error unregistering handler during shutdown",
				"error", err,
				"handler", name,
			)
		}
	}

	// Shutdown publisher
	return s.publisher.Shutdown(ctx)
}

// GetHandlerNames returns a list of registered handler names
func (s *Service) GetHandlerNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.handlers))
	for name := range s.handlers {
		names = append(names, name)
	}
	return names
}

// GetHandler returns a handler by name
func (s *Service) GetHandler(name string) (types.EventHandler, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	handler, exists := s.handlers[name]
	return handler, exists
}
