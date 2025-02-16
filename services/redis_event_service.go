package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
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

	// Always publish weather updates regardless of client version
	if event.Type == types.EventTypeWeatherUpdated {
		r.log.Infow("Handling weather event publication",
			"tripID", tripID,
			"eventID", event.ID,
			"source", event.Metadata.Source)

		data, err := json.Marshal(event)
		if err != nil {
			r.metrics.errorCount.Inc()
			return fmt.Errorf("failed to marshal event: %w", err)
		}

		channel := fmt.Sprintf("trip:%s", tripID)
		r.log.Debugw("Publishing weather update",
			"channel", channel,
			"eventType", event.Type,
			"eventID", event.ID,
		)

		if err := r.redisClient.Publish(ctx, channel, data).Err(); err != nil {
			r.metrics.errorCount.Inc()
			return fmt.Errorf("failed to publish event: %w", err)
		}

		r.log.Infow("Successfully published weather event to Redis",
			"channel", channel,
			"eventID", event.ID,
			"payloadSize", len(data))

		r.metrics.eventCount.WithLabelValues(string(event.Type)).Inc()

		// Process handlers
		r.mu.RLock()
		handlers := r.handlers[event.Type]
		r.mu.RUnlock()

		for _, handler := range handlers {
			go func(h types.EventHandler) {
				if err := h.HandleEvent(ctx, event); err != nil {
					r.log.Errorw("Event handler failed",
						"error", err,
						"eventType", event.Type,
						"eventID", event.ID,
						"handler", fmt.Sprintf("%T", h),
					)
				}
			}(handler)
		}

		return nil
	}

	// Existing version check logic for other event types
	if headers, ok := metadata.FromIncomingContext(ctx); ok {
		if version := headers.Get("X-Client-Version"); len(version) > 0 {
			if ver, err := strconv.ParseFloat(version[0], 64); err == nil && ver >= 1.2 {
				data, err := json.Marshal(event)
				if err != nil {
					r.metrics.errorCount.Inc()
					return fmt.Errorf("failed to marshal event: %w", err)
				}
				payload := data
				channel := fmt.Sprintf("trip:%s", tripID)
				r.log.Debugw("Publishing event",
					"channel", channel,
					"eventType", event.Type,
					"eventID", event.ID,
					"correlationID", event.Metadata.CorrelationID,
					"payloadSize", len(payload),
				)

				ctx = metadata.NewOutgoingContext(ctx, metadata.New(map[string]string{
					"X-Client-Version": strconv.Itoa(event.Version),
				}))

				if err := r.redisClient.Publish(ctx, channel, payload).Err(); err != nil {
					r.metrics.errorCount.Inc()
					return fmt.Errorf("failed to publish event: %w", err)
				}

				r.metrics.eventCount.WithLabelValues(string(event.Type)).Inc()

				// Process handlers
				r.mu.RLock()
				handlers := r.handlers[event.Type]
				r.mu.RUnlock()

				for _, handler := range handlers {
					go func(h types.EventHandler) {
						if err := h.HandleEvent(ctx, event); err != nil {
							r.log.Errorw("Event handler failed",
								"error", err,
								"eventType", event.Type,
								"eventID", event.ID,
								"handler", fmt.Sprintf("%T", h),
							)
						}
					}(handler)
				}

				return nil
			}
		}
	}

	data, err := json.Marshal(event)
	if err != nil {
		r.metrics.errorCount.Inc()
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	channel := fmt.Sprintf("trip:%s", tripID)
	r.log.Debugw("Publishing event",
		"channel", channel,
		"eventType", event.Type,
		"eventID", event.ID,
		"correlationID", event.Metadata.CorrelationID,
		"payloadSize", len(data),
	)

	ctx = metadata.NewOutgoingContext(ctx, metadata.New(map[string]string{
		"X-Client-Version": strconv.Itoa(event.Version),
	}))

	if err := r.redisClient.Publish(ctx, channel, data).Err(); err != nil {
		r.metrics.errorCount.Inc()
		return fmt.Errorf("failed to publish event: %w", err)
	}

	r.log.Infow("Successfully published weather event to Redis",
		"channel", channel,
		"eventID", event.ID,
		"payloadSize", len(data))

	r.metrics.eventCount.WithLabelValues(string(event.Type)).Inc()

	// Process handlers
	r.mu.RLock()
	handlers := r.handlers[event.Type]
	r.mu.RUnlock()

	for _, handler := range handlers {
		go func(h types.EventHandler) {
			if err := h.HandleEvent(ctx, event); err != nil {
				r.log.Errorw("Event handler failed",
					"error", err,
					"eventType", event.Type,
					"eventID", event.ID,
					"handler", fmt.Sprintf("%T", h),
				)
			}
		}(handler)
	}

	return nil
}

func (r *RedisEventService) Subscribe(ctx context.Context, tripID string, userID string, filters ...types.EventType) (<-chan types.Event, error) {
	startTime := time.Now()
	defer func() {
		r.metrics.subscribeLatency.Observe(time.Since(startTime).Seconds())
	}()

	channelName := fmt.Sprintf("trip:%s", tripID)
	r.log.Debugw("Subscribing to channel",
		"channel", channelName,
		"userID", userID,
		"filters", filters,
	)

	pubsub := r.redisClient.Subscribe(ctx, channelName)
	eventChan := make(chan types.Event, 100) // Buffered channel

	// Create cancellable context
	subCtx, cancel := context.WithCancel(context.Background())

	// Store subscription
	key := fmt.Sprintf("%s:%s", tripID, userID)
	r.mu.Lock()
	r.subscriptions[key] = subscription{
		pubsub:    pubsub,
		cancelCtx: cancel,
	}
	r.mu.Unlock()

	go func() {
		defer func() {
			close(eventChan)
			r.mu.Lock()
			delete(r.subscriptions, key)
			r.mu.Unlock()
		}()

		ch := pubsub.Channel()
		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var event types.Event
				if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
					r.log.Errorw("Failed to unmarshal event",
						"error", err,
						"payload", msg.Payload,
					)
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

				// Non-blocking send
				select {
				case eventChan <- event:
				default:
					r.log.Warnw("Event channel full, dropping event",
						"eventType", event.Type,
						"eventID", event.ID,
					)
				}

			case <-subCtx.Done(): // Use subCtx instead of ctx
				return
			}
		}
	}()

	return eventChan, nil
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

	// Clean up any active subscriptions
	if err := r.redisClient.Close(); err != nil {
		return fmt.Errorf("failed to close redis client: %w", err)
	}

	return nil
}

// Add health check method
func (r *RedisEventService) HealthCheck(ctx context.Context) error {
	if err := r.redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("event service unhealthy: %w", err)
	}
	return nil
}

// Implement Unsubscribe
func (r *RedisEventService) Unsubscribe(ctx context.Context, tripID string, userID string) error {
	startTime := time.Now()
	defer func() {
		r.metrics.subscribeLatency.Observe(time.Since(startTime).Seconds())
	}()

	key := fmt.Sprintf("%s:%s", tripID, userID)
	r.mu.Lock()
	defer r.mu.Unlock()

	if sub, exists := r.subscriptions[key]; exists {
		// Cancel the subscription context
		sub.cancelCtx()

		// Close the pubsub connection
		if err := sub.pubsub.Close(); err != nil {
			r.log.Errorw("Failed to close pubsub connection",
				"tripID", tripID,
				"userID", userID,
				"error", err)
			return fmt.Errorf("failed to unsubscribe: %w", err)
		}

		delete(r.subscriptions, key)
		r.log.Debugw("Successfully unsubscribed user",
			"tripID", tripID,
			"userID", userID)
	}

	return nil
}
