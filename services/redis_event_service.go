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

// Configuration for RedisEventService
type RedisEventServiceConfig struct {
	PublishTimeout   time.Duration
	SubscribeTimeout time.Duration // Timeout for initial subscription setup
	EventBufferSize  int           // Buffer size for the client channel
}

// Default configuration values
func DefaultRedisEventServiceConfig() RedisEventServiceConfig {
	return RedisEventServiceConfig{
		PublishTimeout:   5 * time.Second,
		SubscribeTimeout: 10 * time.Second,
		EventBufferSize:  100, // Sensible default, adjust based on load
	}
}

// RedisEventService implements the EventPublisher interface using Redis Pub/Sub.
type RedisEventService struct {
	redisClient *redis.Client
	log         *zap.SugaredLogger
	metrics     *EventMetrics
	config      RedisEventServiceConfig // New: Service configuration
	mu          sync.RWMutex
	// Key: subscriptionKey (e.g., "tripID:userID")
	// Value: Manages the underlying Redis subscription and cancellation
	subscriptions map[string]subscription
}

// Ensure RedisEventService adheres to the interfaces
var _ types.EventPublisher = (*RedisEventService)(nil)

// var _ types.HealthChecker = (*RedisEventService)(nil) // Removed check for undefined: types.HealthChecker

type EventMetrics struct {
	publishLatency           prometheus.Histogram
	subscribeLatency         prometheus.Histogram
	publishErrorCount        prometheus.Counter
	subscribeErrorCount      prometheus.Counter     // Errors during initial subscribe setup
	processMessageErrorCount prometheus.Counter     // Errors during message processing (unmarshal etc.)
	droppedEventCount        prometheus.Counter     // Events dropped due to full client buffer
	publishedEventCount      *prometheus.CounterVec // Count of successfully published events by type
	receivedEventCount       *prometheus.CounterVec // Count of successfully received events by type (in processSubscription)
	activeSubscriptions      prometheus.Gauge
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
			Help:    "Time taken to publish events to Redis",
			Buckets: prometheus.DefBuckets, // Using default buckets
		}),
		subscribeLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "nomadcrew_event_subscribe_duration_seconds",
			Help:    "Time taken to establish a client subscription via Redis",
			Buckets: prometheus.DefBuckets,
		}),
		publishErrorCount: promauto.NewCounter(prometheus.CounterOpts{
			Name: "nomadcrew_event_publish_errors_total",
			Help: "Total number of errors during event publishing to Redis",
		}),
		subscribeErrorCount: promauto.NewCounter(prometheus.CounterOpts{
			Name: "nomadcrew_event_subscribe_errors_total",
			Help: "Total number of errors during client subscription setup",
		}),
		processMessageErrorCount: promauto.NewCounter(prometheus.CounterOpts{
			Name: "nomadcrew_event_process_message_errors_total",
			Help: "Total number of errors while processing received messages (e.g., unmarshal)",
		}),
		droppedEventCount: promauto.NewCounter(prometheus.CounterOpts{
			Name: "nomadcrew_event_dropped_total",
			Help: "Total number of events dropped because the client channel was full",
		}),
		publishedEventCount: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "nomadcrew_events_published_total",
			Help: "Total number of events successfully published to Redis",
		}, []string{"event_type"}),
		receivedEventCount: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "nomadcrew_events_received_total",
			Help: "Total number of events successfully received from Redis and passed filters",
		}, []string{"event_type"}),
		activeSubscriptions: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "nomadcrew_event_active_subscriptions",
			Help: "Current number of active client subscriptions",
		}),
	}
}

// NewRedisEventService returns a new instance of RedisEventService.
func NewRedisEventService(redisClient *redis.Client, cfg ...RedisEventServiceConfig) *RedisEventService {
	config := DefaultRedisEventServiceConfig()
	if len(cfg) > 0 {
		config = cfg[0] // Allow overriding defaults
	}

	return &RedisEventService{
		redisClient:   redisClient,
		log:           logger.GetLogger().Named("RedisEventService"), // Add name for clarity
		metrics:       initEventMetrics(),
		config:        config,
		subscriptions: make(map[string]subscription),
	}
}

// Publish serializes the event and publishes it on a Redis channel specific to the trip.
func (r *RedisEventService) Publish(ctx context.Context, tripID string, event types.Event) error {
	startTime := time.Now()
	defer func() {
		// Observe latency regardless of success/failure
		r.metrics.publishLatency.Observe(time.Since(startTime).Seconds())
	}()

	// Validate event structure
	if err := event.Validate(); err != nil {
		r.metrics.publishErrorCount.Inc()
		// It's often better to log validation errors here than return them,
		// unless the caller needs to know specifically about validation failure.
		// Returning an error might halt processes unnecessarily if the event source doesn't handle it.
		r.log.Errorw("Invalid event structure during publish",
			"error", err,
			"tripID", tripID,
			"eventType", event.Type,
			"eventID", event.ID, // Log event ID if available
		)
		// Decide whether to return the error or just log it.
		// Returning it makes the publisher aware, logging makes it fire-and-forget.
		// Let's return it for now, assuming callers might want to know.
		return fmt.Errorf("invalid event: %w", err)
	}

	// Set default values if missing (ID should usually be set by originator)
	if event.ID == "" {
		event.ID = uuid.New().String()
		r.log.Warnw("Event published without ID, generated one", "eventType", event.Type, "tripID", tripID)
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.Version == 0 {
		event.Version = 1 // Default version
	}

	// Format the Redis channel name
	channel := r.tripChannelName(tripID)

	// Marshal the event
	data, err := json.Marshal(event)
	if err != nil {
		r.metrics.publishErrorCount.Inc()
		r.log.Errorw("Failed to marshal event for publish",
			"error", err,
			"tripID", tripID,
			"eventType", event.Type,
			"eventID", event.ID,
		)
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Use context with configured timeout for the publish operation
	publishCtx, cancel := context.WithTimeout(ctx, r.config.PublishTimeout)
	defer cancel()

	r.log.Debugw("Publishing event to Redis",
		"channel", channel,
		"eventType", event.Type,
		"eventID", event.ID,
		"correlationID", event.Metadata.CorrelationID,
		"payloadSize", len(data),
	)

	// Publish to Redis
	if err := r.redisClient.Publish(publishCtx, channel, data).Err(); err != nil {
		r.metrics.publishErrorCount.Inc()
		r.log.Errorw("Failed to publish event to Redis",
			"error", err,
			"channel", channel,
			"eventType", event.Type,
			"eventID", event.ID,
		)
		return fmt.Errorf("failed to publish event: %w", err)
	}

	// Increment success metric
	r.metrics.publishedEventCount.WithLabelValues(string(event.Type)).Inc()

	// Removed internal handler dispatch logic

	return nil
}

// Subscribe creates a Redis Pub/Sub subscription for a given user and trip,
// returning a channel that emits filtered events.
func (r *RedisEventService) Subscribe(ctx context.Context, tripID string, userID string, filters ...types.EventType) (<-chan types.Event, error) {
	startTime := time.Now()
	subscriptionKey := r.subscriptionKey(tripID, userID)

	r.mu.Lock() // Lock early to manage concurrent subscribes for the same user

	// Check if already subscribed
	if _, exists := r.subscriptions[subscriptionKey]; exists {
		r.mu.Unlock()
		r.log.Warnw("User already subscribed, closing previous subscription", "tripID", tripID, "userID", userID)
		// Attempt to close the old one cleanly before creating a new one.
		// Pass a short timeout context for the unsubscribe operation.
		unsubscribeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := r.Unsubscribe(unsubscribeCtx, tripID, userID); err != nil {
			// Log error but proceed with new subscription attempt
			r.log.Errorw("Error closing previous subscription", "error", err, "tripID", tripID, "userID", userID)
		}
		// Need to re-acquire lock for the new subscription attempt
		r.mu.Lock()
	}

	channelName := r.tripChannelName(tripID)
	eventChan := make(chan types.Event, r.config.EventBufferSize)

	// Use context with timeout for setting up the subscription
	subCtx, subCancel := context.WithTimeout(ctx, r.config.SubscribeTimeout)
	defer subCancel() // Cancel timeout context if setup fails early

	pubsub := r.redisClient.Subscribe(subCtx, channelName)

	// Wait for confirmation that subscription is active
	_, err := pubsub.Receive(subCtx)
	if err != nil {
		r.mu.Unlock()
		r.metrics.subscribeErrorCount.Inc()
		close(eventChan) // Close channel on failure
		// Ensure pubsub is closed if partially opened
		_ = pubsub.Close()
		r.log.Errorw("Failed to subscribe to Redis channel",
			"error", err,
			"channel", channelName,
			"tripID", tripID,
			"userID", userID)
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	// Subscription successful, store it and start processing goroutine
	// Create a separate context for the lifetime of the processing goroutine
	processCtx, processCancel := context.WithCancel(context.Background()) // Use background context

	sub := subscription{
		pubsub:    pubsub,
		cancelCtx: processCancel,
	}
	r.subscriptions[subscriptionKey] = sub
	r.metrics.activeSubscriptions.Inc() // Increment active subscription gauge
	r.mu.Unlock()                       // Unlock *after* modifying the map

	// Start the goroutine to process messages
	go r.processSubscription(processCtx, pubsub, eventChan, tripID, userID, subscriptionKey, filters)

	r.metrics.subscribeLatency.Observe(time.Since(startTime).Seconds())
	r.log.Infow("User subscribed to events", "tripID", tripID, "userID", userID, "filters", filters)

	return eventChan, nil
}

// Helper method to generate consistent channel names
func (r *RedisEventService) tripChannelName(tripID string) string {
	return fmt.Sprintf("trip-events:%s", tripID)
}

// Helper method to generate consistent subscription map keys
func (r *RedisEventService) subscriptionKey(tripID, userID string) string {
	return fmt.Sprintf("%s:%s", tripID, userID)
}

// processSubscription runs in a goroutine, receiving messages from Redis
// and forwarding them to the client's channel.
func (r *RedisEventService) processSubscription(
	ctx context.Context, // This context is controlled by Subscribe/Unsubscribe lifecycle
	pubsub *redis.PubSub,
	eventChan chan types.Event,
	tripID string,
	userID string,
	subscriptionKey string,
	filters []types.EventType,
) {
	defer func() {
		// Cleanup: Close channel, remove from map, decrement gauge
		close(eventChan)

		r.mu.Lock()
		// Check if the subscription still exists (might have been removed by Unsubscribe)
		if _, exists := r.subscriptions[subscriptionKey]; exists {
			delete(r.subscriptions, subscriptionKey)
			r.metrics.activeSubscriptions.Dec() // Decrement active subscription gauge
		}
		r.mu.Unlock()

		// Close the Redis PubSub connection
		if err := pubsub.Close(); err != nil {
			// Avoid logging error if Redis client itself is already closed
			if err.Error() != "redis: client is closed" {
				r.log.Warnw("Error closing Redis pubsub during cleanup", "error", err, "subscriptionKey", subscriptionKey)
			}
		}
		r.log.Debugw("Stopped processing subscription", "subscriptionKey", subscriptionKey)
	}()

	// Get the message channel from pubsub
	ch := pubsub.Channel()

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				r.log.Infow("Redis pubsub channel closed", "subscriptionKey", subscriptionKey)
				return // Exit goroutine if channel is closed
			}

			var event types.Event
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				r.metrics.processMessageErrorCount.Inc()
				r.log.Errorw("Failed to unmarshal event from Redis",
					"error", err,
					"payload", msg.Payload, // Be careful logging payloads in production (PII/size)
					"subscriptionKey", subscriptionKey)
				// Consider adding logic for poison messages here if needed
				continue // Skip this message
			}

			// Apply filters if any are provided
			if len(filters) > 0 {
				matched := false
				for _, filter := range filters {
					if event.Type == filter {
						matched = true
						break
					}
				}
				if !matched {
					continue // Skip event if it doesn't match filters
				}
			}

			// Send event to client channel (non-blocking)
			select {
			case eventChan <- event:
				r.metrics.receivedEventCount.WithLabelValues(string(event.Type)).Inc()
			default:
				// Client channel is full, drop the event
				r.metrics.droppedEventCount.Inc()
				r.log.Warnw("Event channel full, dropping event",
					"eventType", event.Type,
					"eventID", event.ID,
					"subscriptionKey", subscriptionKey)
			}

		case <-ctx.Done():
			// Context was canceled (likely by Unsubscribe or Shutdown)
			r.log.Infow("Subscription context canceled, stopping processor", "subscriptionKey", subscriptionKey)
			return // Exit goroutine
		}
	}
}

// Unsubscribe removes a user's subscription.
func (r *RedisEventService) Unsubscribe(ctx context.Context, tripID string, userID string) error {
	subscriptionKey := r.subscriptionKey(tripID, userID)

	r.mu.Lock()
	sub, exists := r.subscriptions[subscriptionKey]
	if !exists {
		r.mu.Unlock()
		r.log.Debugw("Unsubscribe called for non-existent subscription", "subscriptionKey", subscriptionKey)
		return nil // Already unsubscribed or never existed
	}

	// Remove from map *before* canceling context and closing pubsub
	// This prevents new messages potentially being processed after unsubscribe starts
	delete(r.subscriptions, subscriptionKey)
	r.metrics.activeSubscriptions.Dec() // Decrement gauge immediately
	r.mu.Unlock()

	// Cancel the context for the processSubscription goroutine
	sub.cancelCtx()

	// Closing the pubsub object also signals the underlying Redis connection to stop listening.
	// The processSubscription goroutine's defer function will also attempt to close,
	// but closing twice is generally safe with go-redis.
	if err := sub.pubsub.Close(); err != nil {
		if err.Error() != "redis: client is closed" {
			r.log.Errorw("Failed to close Redis subscription during Unsubscribe",
				"error", err,
				"subscriptionKey", subscriptionKey)
			// Return error, but the subscription state is already cleaned up locally
			return fmt.Errorf("failed to close redis pubsub: %w", err)
		}
	}

	r.log.Infow("Successfully unsubscribed", "subscriptionKey", subscriptionKey)
	return nil
}

// Batch publishing remains largely the same, simplified as no handlers to call
func (r *RedisEventService) PublishBatch(ctx context.Context, tripID string, events []types.Event) error {
	if len(events) == 0 {
		return nil
	}

	pipe := r.redisClient.Pipeline()
	channel := r.tripChannelName(tripID)
	processedCount := 0

	for i, event := range events {
		// Validate and set defaults
		if err := event.Validate(); err != nil {
			r.metrics.publishErrorCount.Inc()
			r.log.Errorw("Invalid event in batch", "error", err, "index", i, "tripID", tripID)
			// Decide whether to fail the whole batch or skip the event
			// Failing whole batch for now:
			return fmt.Errorf("invalid event at index %d: %w", i, err)
		}
		if event.ID == "" {
			event.ID = uuid.NewString()
		}
		if event.Timestamp.IsZero() {
			event.Timestamp = time.Now()
		}
		if event.Version == 0 {
			event.Version = 1
		}

		data, err := json.Marshal(event)
		if err != nil {
			r.metrics.publishErrorCount.Inc()
			r.log.Errorw("Failed to marshal event in batch", "error", err, "index", i, "tripID", tripID)
			return fmt.Errorf("failed to marshal event at index %d: %w", i, err)
		}

		pipe.Publish(ctx, channel, data) // Use the pipeline context
		processedCount++
		r.metrics.publishedEventCount.WithLabelValues(string(event.Type)).Inc()
	}

	// Execute the pipeline
	publishCtx, cancel := context.WithTimeout(ctx, r.config.PublishTimeout) // Apply timeout to batch exec
	defer cancel()
	_, err := pipe.Exec(publishCtx)
	if err != nil {
		r.metrics.publishErrorCount.Inc()
		r.log.Errorw("Failed to execute Redis pipeline for batch publish", "error", err, "tripID", tripID)
		// Decrement metrics for events that were added but failed execution? Or keep as published attempt?
		// Keeping as published attempt for now.
		return fmt.Errorf("failed to publish batch: %w", err)
	}

	r.log.Debugw("Successfully published batch", "tripID", tripID, "count", processedCount)
	// Note: Batch metrics are now handled per-event type above.
	// r.metrics.eventCount.WithLabelValues("batch").Add(float64(processedCount)) // Old batch metric removed
	return nil
}

// Shutdown gracefully closes all active subscriptions.
func (r *RedisEventService) Shutdown(ctx context.Context) error {
	r.log.Info("Shutting down Redis Event Service...")

	r.mu.Lock()
	// Create a slice of keys to avoid iteration issues while modifying the map
	keys := make([]string, 0, len(r.subscriptions))
	for k := range r.subscriptions {
		keys = append(keys, k)
	}
	r.mu.Unlock() // Unlock before calling Unsubscribe which locks again

	var wg sync.WaitGroup
	for _, key := range keys {
		wg.Add(1)
		go func(k string) {
			defer wg.Done()
			// Extract tripID and userID if needed for Unsubscribe (or modify Unsubscribe)
			// Simplified: Assuming key format is "tripID:userID"
			parts := splitSubscriptionKey(k)
			if len(parts) != 2 {
				r.log.Errorw("Invalid subscription key format during shutdown", "key", k)
				return
			}
			tripID, userID := parts[0], parts[1]

			// Create a context with timeout for each unsubscribe during shutdown
			shutdownUnsubCtx, cancel := context.WithTimeout(ctx, 5*time.Second) // Use shutdown context with timeout
			defer cancel()

			if err := r.Unsubscribe(shutdownUnsubCtx, tripID, userID); err != nil {
				r.log.Warnw("Error unsubscribing during shutdown", "error", err, "key", k)
			}
		}(key)
	}

	// Wait for all unsubscribe operations to complete
	wg.Wait()

	// Final check on subscription map (should be empty)
	r.mu.RLock()
	remaining := len(r.subscriptions)
	r.mu.RUnlock()
	if remaining > 0 {
		r.log.Warnf("%d subscriptions remained after shutdown attempt", remaining)
	}

	r.log.Info("Redis Event Service shutdown complete.")
	return nil
}

// Helper to split key, basic implementation
func splitSubscriptionKey(key string) []string {
	// A more robust implementation might be needed if keys can contain ':'
	parts := make([]string, 0, 2)
	lastColon := -1
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == ':' {
			lastColon = i
			break
		}
	}
	if lastColon != -1 && lastColon < len(key)-1 {
		parts = append(parts, key[:lastColon])
		parts = append(parts, key[lastColon+1:])
	} else {
		// Handle cases where format might be wrong or key doesn't contain ':'
		// For now, returning empty or single part might indicate an issue upstream
		parts = append(parts, key)
	}
	if len(parts) == 2 {
		return parts
	}
	return []string{} // Indicate failure clearly
}

// HealthCheck pings the Redis server.
func (r *RedisEventService) HealthCheck(ctx context.Context) error {
	if err := r.redisClient.Ping(ctx).Err(); err != nil {
		r.metrics.publishErrorCount.Inc() // Or a dedicated health check error metric
		return fmt.Errorf("Redis Event Service unhealthy: Redis ping failed: %w", err)
	}
	return nil
}
