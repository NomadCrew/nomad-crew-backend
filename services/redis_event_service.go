package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisEventService implements the EventPublisher interface using Redis Pub/Sub.
type RedisEventService struct {
	redisClient   *redis.Client
	log           *zap.SugaredLogger
	metrics       *EventMetrics // New: for monitoring
	handlers      map[types.EventType][]types.EventHandler
	mu            sync.RWMutex
	subscriptions map[string]subscription // Key: tripID:userID
}

var _ types.EventPublisher = (*RedisEventService)(nil) // Add interface assertion

type EventMetrics struct {
	publishLatency   prometheus.Histogram
	subscribeLatency prometheus.Histogram
	errorCount       prometheus.Counter
	eventCount       *prometheus.CounterVec
}

type subscription struct {
	pubsub    *redis.PubSub
	cancelCtx context.CancelFunc
}

// Metrics initialization
func initEventMetrics() *EventMetrics {
	return &EventMetrics{
		publishLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "nomadcrew_event_publish_duration_seconds",
			Help:    "Time taken to publish events",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		}),
		subscribeLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "nomadcrew_event_subscribe_duration_seconds",
			Help:    "Time taken to establish subscriptions",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		}),
		errorCount: promauto.NewCounter(prometheus.CounterOpts{
			Name: "nomadcrew_event_errors_total",
			Help: "Total number of event processing errors",
		}),
		eventCount: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "nomadcrew_events_processed_total",
			Help: "Total number of events processed",
		}, []string{"event_type"}),
	}
}

// NewRedisEventService returns a new instance of RedisEventService.
func NewRedisEventService(redisClient *redis.Client) *RedisEventService {
	return &RedisEventService{
		redisClient:   redisClient,
		log:           logger.GetLogger(),
		metrics:       initEventMetrics(),
		handlers:      make(map[types.EventType][]types.EventHandler),
		subscriptions: make(map[string]subscription),
	}
}

// Publish serializes the event and publishes it on a Redis channel.
// The channel is named "trip:{tripID}".
func (r *RedisEventService) Publish(ctx context.Context, tripID string, event types.Event) error {
	startTime := time.Now()
	defer func() {
		r.metrics.publishLatency.Observe(time.Since(startTime).Seconds())
	}()

	// Validate event
	if err := event.Validate(); err != nil {
		r.metrics.errorCount.Inc()
		return fmt.Errorf("invalid event: %w", err)
	}

	// Set default values if missing
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.Version == 0 {
		event.Version = 1
	}

	// Format the Redis channel name
	channel := fmt.Sprintf("trip:%s", tripID)

	// Marshal the event (once for all clients)
	data, err := json.Marshal(event)
	if err != nil {
		r.metrics.errorCount.Inc()
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Update metrics
	r.metrics.eventCount.WithLabelValues(string(event.Type)).Inc()

	// Publish to Redis
	r.log.Debugw("Publishing event",
		"channel", channel,
		"eventType", event.Type,
		"eventID", event.ID,
		"correlationID", event.Metadata.CorrelationID,
		"payloadSize", len(data),
	)

	// Use context timeout to prevent blocking indefinitely
	publishCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := r.redisClient.Publish(publishCtx, channel, data).Err(); err != nil {
		r.metrics.errorCount.Inc()
		return fmt.Errorf("failed to publish event: %w", err)
	}

	// Process handlers
	r.mu.RLock()
	handlers := r.handlers[event.Type]
	r.mu.RUnlock()

	for _, handler := range handlers {
		handler := handler // Create a new variable for the goroutine
		go func() {
			// Create a separate context with timeout for handler execution
			handlerCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := handler.HandleEvent(handlerCtx, event); err != nil {
				r.log.Errorw("Event handler failed",
					"error", err,
					"eventType", event.Type,
					"eventID", event.ID,
					"handler", fmt.Sprintf("%T", handler),
				)
			}
		}()
	}

	return nil
}

// Subscribe method with improved error handling and cleanup
func (r *RedisEventService) Subscribe(ctx context.Context, tripID string, userID string, filters ...types.EventType) (<-chan types.Event, error) {
	startTime := time.Now()
	defer func() {
		r.metrics.subscribeLatency.Observe(time.Since(startTime).Seconds())
	}()

	// Check for existing subscription and clean it up
	subscriptionKey := fmt.Sprintf("%s:%s", tripID, userID)
	r.mu.Lock()
	if _, exists := r.subscriptions[subscriptionKey]; exists {
		r.mu.Unlock()
		// Close existing subscription first
		if err := r.Unsubscribe(ctx, tripID, userID); err != nil {
			r.log.Warnw("Failed to clean up existing subscription",
				"error", err,
				"tripID", tripID,
				"userID", userID)
		}
		r.mu.Lock()
	}
	r.mu.Unlock()

	// Create new subscription with Redis
	channelName := fmt.Sprintf("trip:%s", tripID)
	pubsub := r.redisClient.Subscribe(ctx, channelName)

	// Create buffered channel for events with appropriate size
	eventChan := make(chan types.Event, 100)

	// Create subscription context with cancelation
	subCtx, cancel := context.WithCancel(context.Background())

	// Store subscription for management
	r.mu.Lock()
	r.subscriptions[subscriptionKey] = subscription{
		pubsub:    pubsub,
		cancelCtx: cancel,
	}
	r.mu.Unlock()

	// Start goroutine to process messages
	go r.processSubscription(subCtx, pubsub, eventChan, tripID, userID, subscriptionKey, filters)

	return eventChan, nil
}

// Helper method to process subscription messages
func (r *RedisEventService) processSubscription(
	ctx context.Context,
	pubsub *redis.PubSub,
	eventChan chan types.Event,
	tripID string,
	userID string,
	subscriptionKey string,
	filters []types.EventType,
) {
	defer func() {
		// Clean up on exit
		close(eventChan)

		r.mu.Lock()
		delete(r.subscriptions, subscriptionKey)
		r.mu.Unlock()

		if err := pubsub.Close(); err != nil {
			r.log.Warnw("Error closing Redis pubsub", "error", err)
		}
	}()

	ch := pubsub.Channel()

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				r.log.Infow("Redis pubsub channel closed",
					"tripID", tripID,
					"userID", userID)
				return
			}

			// Parse and process message
			var event types.Event
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				r.log.Errorw("Failed to unmarshal event",
					"error", err,
					"payload", msg.Payload)
				r.metrics.errorCount.Inc()
				continue
			}

			// Apply filters if any
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

			// Send event to client with non-blocking operation
			select {
			case eventChan <- event:
				r.metrics.eventCount.WithLabelValues(string(event.Type)).Inc()
			default:
				r.log.Warnw("Event channel full, dropping event",
					"eventType", event.Type,
					"eventID", event.ID,
					"tripID", tripID,
					"userID", userID)
			}

		case <-ctx.Done():
			r.log.Infow("Subscription context canceled",
				"tripID", tripID,
				"userID", userID)
			return
		}
	}
}

// Add cleanup method for stale subscriptions
func (r *RedisEventService) cleanupStaleSubscriptions() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for key, sub := range r.subscriptions {
		select {
		case <-sub.pubsub.Channel():
			// Channel closed, remove subscription
			sub.cancelCtx()
			delete(r.subscriptions, key)
		default:
			// Check if subscription is still healthy
			if err := sub.pubsub.Ping(context.Background()); err != nil {
				r.log.Warnw("Removing unhealthy subscription",
					"key", key,
					"error", err)
				sub.cancelCtx()
				delete(r.subscriptions, key)
			}
		}
	}
}

func (r *RedisEventService) RegisterHandler(eventType types.EventType, handler types.EventHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.handlers[eventType] = append(r.handlers[eventType], handler)
	r.log.Infow("Registered event handler",
		"eventType", eventType,
		"handler", fmt.Sprintf("%T", handler),
	)
}

func (r *RedisEventService) RegisterHandlers(handler types.EventHandler) {
	for _, eventType := range handler.SupportedEvents() {
		r.RegisterHandler(eventType, handler)
	}
}

// Batch publish implementation
func (r *RedisEventService) PublishBatch(ctx context.Context, tripID string, events []types.Event) error {
	if len(events) == 0 {
		return nil
	}

	pipe := r.redisClient.Pipeline()
	channel := fmt.Sprintf("trip:%s", tripID)

	for _, event := range events {
		if err := event.Validate(); err != nil {
			r.metrics.errorCount.Inc()
			return fmt.Errorf("invalid event in batch: %w", err)
		}

		data, err := json.Marshal(event)
		if err != nil {
			r.metrics.errorCount.Inc()
			return fmt.Errorf("failed to marshal event in batch: %w", err)
		}

		pipe.Publish(ctx, channel, data)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		r.metrics.errorCount.Inc()
		return fmt.Errorf("failed to publish batch: %w", err)
	}

	r.metrics.eventCount.WithLabelValues("batch").Add(float64(len(events)))
	return nil
}

// Add helper method for graceful shutdown
func (r *RedisEventService) Shutdown(ctx context.Context) error {
	r.log.Info("Shutting down event service")

	// Close all active subscriptions
	r.mu.Lock()
	for key, sub := range r.subscriptions {
		r.log.Debugw("Closing subscription during shutdown", "key", key)
		sub.cancelCtx()
		if err := sub.pubsub.Close(); err != nil {
			r.log.Warnw("Error closing subscription", "key", key, "error", err)
		}
	}
	r.subscriptions = make(map[string]subscription)
	r.mu.Unlock()

	return nil
}

// Add health check method
func (r *RedisEventService) HealthCheck(ctx context.Context) error {
	if err := r.redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("event service unhealthy: %w", err)
	}
	return nil
}

func (r *RedisEventService) Unsubscribe(ctx context.Context, tripID string, userID string) error {
	key := fmt.Sprintf("%s:%s", tripID, userID)

	r.mu.Lock()
	sub, exists := r.subscriptions[key]
	if !exists {
		r.mu.Unlock()
		return nil // Already unsubscribed
	}

	// Remove from map immediately to prevent concurrent access
	delete(r.subscriptions, key)
	r.mu.Unlock()

	// Cancel the context to stop the processing goroutine
	sub.cancelCtx()

	// Close the Redis pubsub subscription
	if err := sub.pubsub.Close(); err != nil {
		r.log.Errorw("Failed to close Redis subscription",
			"error", err,
			"tripID", tripID,
			"userID", userID)
		return fmt.Errorf("failed to unsubscribe: %w", err)
	}

	r.log.Debugw("Successfully unsubscribed",
		"tripID", tripID,
		"userID", userID)

	return nil
}
