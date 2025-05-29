package events

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

// Config holds configuration for RedisPublisher
type Config struct {
	PublishTimeout   time.Duration
	SubscribeTimeout time.Duration
	EventBufferSize  int
}

// DefaultConfig returns default configuration values
func DefaultConfig() Config {
	return Config{
		PublishTimeout:   5 * time.Second,
		SubscribeTimeout: 10 * time.Second,
		EventBufferSize:  100,
	}
}

// metrics holds Prometheus metrics for the publisher
type metrics struct {
	publishLatency    prometheus.Histogram
	subscribeLatency  prometheus.Histogram
	errorCount        *prometheus.CounterVec
	eventCount        *prometheus.CounterVec
	activeSubscribers prometheus.Gauge
}

// metricsInstance is a singleton instance of metrics
var (
	metricsInstance *metrics
	metricsOnce     sync.Once
	defaultRegistry = prometheus.DefaultRegisterer
)

// newMetrics initializes and registers Prometheus metrics - now using a singleton pattern
func newMetrics() *metrics {
	metricsOnce.Do(func() {
		// Use a custom registry for metrics to avoid conflicts in tests
		metricsInstance = &metrics{
			publishLatency: promauto.With(defaultRegistry).NewHistogram(prometheus.HistogramOpts{
				Name:    "event_publish_duration_seconds",
				Help:    "Time taken to publish events",
				Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
			}),
			subscribeLatency: promauto.With(defaultRegistry).NewHistogram(prometheus.HistogramOpts{
				Name:    "event_subscribe_duration_seconds",
				Help:    "Time taken to establish subscriptions",
				Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
			}),
			errorCount: promauto.With(defaultRegistry).NewCounterVec(prometheus.CounterOpts{
				Name: "event_errors_total",
				Help: "Total number of event-related errors",
			}, []string{"operation", "type"}),
			eventCount: promauto.With(defaultRegistry).NewCounterVec(prometheus.CounterOpts{
				Name: "events_total",
				Help: "Total number of events by operation and type",
			}, []string{"operation", "type"}),
			activeSubscribers: promauto.With(defaultRegistry).NewGauge(prometheus.GaugeOpts{
				Name: "event_active_subscribers",
				Help: "Current number of active subscribers",
			}),
		}
	})
	return metricsInstance
}

// For testing purposes - reset metrics
func resetMetricsForTesting() {
	// Create a new registry for tests
	reg := prometheus.NewRegistry()
	defaultRegistry = reg

	// Reset the singleton
	metricsInstance = nil
	metricsOnce = sync.Once{}
}

// RedisPublisher implements types.EventPublisher using Redis Pub/Sub
type RedisPublisher struct {
	rdb     *redis.Client
	log     *zap.SugaredLogger
	metrics *metrics
	config  Config
	mu      sync.RWMutex
	subs    map[string]*subscription
	wg      sync.WaitGroup
}

type subscription struct {
	pubsub    *redis.PubSub
	cancelCtx context.CancelFunc
}

// NewRedisPublisher creates a new RedisPublisher instance
func NewRedisPublisher(rdb *redis.Client, cfg ...Config) *RedisPublisher {
	config := DefaultConfig()
	if len(cfg) > 0 {
		config = cfg[0]
	}

	return &RedisPublisher{
		rdb:     rdb,
		log:     logger.GetLogger().Named("events"),
		metrics: newMetrics(),
		config:  config,
		subs:    make(map[string]*subscription),
	}
}

// Publish publishes an event to Redis
func (p *RedisPublisher) Publish(ctx context.Context, tripID string, event types.Event) error {
	start := time.Now()
	defer func() {
		p.metrics.publishLatency.Observe(time.Since(start).Seconds())
	}()

	if err := event.Validate(); err != nil {
		p.metrics.errorCount.WithLabelValues("publish", "validation").Inc()
		return fmt.Errorf("invalid event: %w", err)
	}

	// Set defaults if needed
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.Version == 0 {
		event.Version = 1
	}

	data, err := json.Marshal(event)
	if err != nil {
		p.metrics.errorCount.WithLabelValues("publish", "marshal").Inc()
		return fmt.Errorf("marshal event: %w", err)
	}

	channel := fmt.Sprintf("trip:%s", tripID)
	ctx, cancel := context.WithTimeout(ctx, p.config.PublishTimeout)
	defer cancel()

	if err := p.rdb.Publish(ctx, channel, data).Err(); err != nil {
		p.metrics.errorCount.WithLabelValues("publish", "redis").Inc()
		return fmt.Errorf("redis publish: %w", err)
	}

	p.metrics.eventCount.WithLabelValues("publish", string(event.Type)).Inc()
	return nil
}

// Subscribe subscribes to events for a specific trip
func (p *RedisPublisher) Subscribe(ctx context.Context, tripID string, userID string, filters ...types.EventType) (<-chan types.Event, error) {
	start := time.Now()
	defer func() {
		p.metrics.subscribeLatency.Observe(time.Since(start).Seconds())
	}()

	subKey := fmt.Sprintf("%s:%s", tripID, userID)
	channel := fmt.Sprintf("trip:%s", tripID)

	p.mu.Lock()
	if _, exists := p.subs[subKey]; exists {
		p.mu.Unlock()
		p.metrics.errorCount.WithLabelValues("subscribe", "duplicate").Inc()
		return nil, fmt.Errorf("subscription already exists for trip %s and user %s", tripID, userID)
	}

	pubsub := p.rdb.Subscribe(ctx, channel)
	subCtx, cancel := context.WithCancel(context.Background())
	p.subs[subKey] = &subscription{pubsub: pubsub, cancelCtx: cancel}
	p.mu.Unlock()

	p.metrics.activeSubscribers.Inc()

	// Create buffered channel for events
	events := make(chan types.Event, p.config.EventBufferSize)

	// Start processing messages in a goroutine
	p.wg.Add(1)
	go p.processMessages(subCtx, pubsub, events, filters, subKey)

	// Small delay to ensure subscription is established
	time.Sleep(10 * time.Millisecond)

	return events, nil
}

// processMessages handles incoming Redis messages
func (p *RedisPublisher) processMessages(ctx context.Context, pubsub *redis.PubSub, events chan<- types.Event, filters []types.EventType, subKey string) {
	defer p.wg.Done()
	defer func() {
		close(events)
		p.metrics.activeSubscribers.Dec()
		p.log.Infow("Subscription closed", "subKey", subKey)
	}()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}

			var event types.Event
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				p.metrics.errorCount.WithLabelValues("process", "unmarshal").Inc()
				p.log.Errorw("Failed to unmarshal event", "error", err, "subKey", subKey)
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

			// Try to send event, drop if channel is full
			select {
			case events <- event:
				p.metrics.eventCount.WithLabelValues("receive", string(event.Type)).Inc()
			default:
				p.metrics.errorCount.WithLabelValues("process", "channel_full").Inc()
				p.log.Warnw("Dropped event due to full channel", "subKey", subKey, "eventType", event.Type)
			}
		}
	}
}

// Unsubscribe removes a subscription
func (p *RedisPublisher) Unsubscribe(ctx context.Context, tripID string, userID string) error {
	subKey := fmt.Sprintf("%s:%s", tripID, userID)

	p.mu.Lock()
	sub, exists := p.subs[subKey]
	if !exists {
		p.mu.Unlock()
		return fmt.Errorf("no subscription found for trip %s and user %s", tripID, userID)
	}

	// Cancel the context to signal processMessages to stop
	sub.cancelCtx()

	// Note: processMessages will call pubsub.Close() upon context cancellation and exit
	// We might not need to call it explicitly here if processMessages handles it robustly
	// However, closing it here too ensures it's closed if processMessages hasn't reached that point yet.
	if err := sub.pubsub.Close(); err != nil {
		p.log.Errorw("Error closing pubsub during unsubscribe", "error", err, "subKey", subKey)
	}

	delete(p.subs, subKey)
	p.mu.Unlock()

	// It's better to wait for the goroutine to actually finish if we rely on it.
	// However, Unsubscribe is usually fire-and-forget for the caller.
	// For Shutdown, waiting is critical.

	return nil
}

// PublishBatch publishes multiple events atomically using Redis pipeline
func (p *RedisPublisher) PublishBatch(ctx context.Context, tripID string, events []types.Event) error {
	if len(events) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, p.config.PublishTimeout)
	defer cancel()

	channel := fmt.Sprintf("trip:%s", tripID)
	pipe := p.rdb.Pipeline()

	for _, event := range events {
		if err := event.Validate(); err != nil {
			p.metrics.errorCount.WithLabelValues("publish_batch", "validation").Inc()
			return fmt.Errorf("invalid event in batch: %w", err)
		}

		// Set defaults
		if event.ID == "" {
			event.ID = uuid.New().String()
		}
		if event.Timestamp.IsZero() {
			event.Timestamp = time.Now()
		}
		if event.Version == 0 {
			event.Version = 1
		}

		data, err := json.Marshal(event)
		if err != nil {
			p.metrics.errorCount.WithLabelValues("publish_batch", "marshal").Inc()
			return fmt.Errorf("marshal event in batch: %w", err)
		}

		pipe.Publish(ctx, channel, data)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		p.metrics.errorCount.WithLabelValues("publish_batch", "redis").Inc()
		return fmt.Errorf("execute batch publish: %w", err)
	}

	for _, event := range events {
		p.metrics.eventCount.WithLabelValues("publish", string(event.Type)).Inc()
	}

	return nil
}

// Shutdown gracefully shuts down the publisher
func (p *RedisPublisher) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	localSubs := make(map[string]*subscription, len(p.subs))
	for k, v := range p.subs {
		localSubs[k] = v
	}
	p.subs = make(map[string]*subscription)
	p.mu.Unlock()

	p.log.Infow("Shutting down RedisPublisher, cancelling subscriptions...", "count", len(localSubs))

	for subKey, sub := range localSubs {
		p.log.Debugw("Cancelling context for subscription", "subKey", subKey)
		sub.cancelCtx()
	}

	p.log.Infow("Waiting for subscription goroutines to finish...")
	p.wg.Wait()
	p.log.Infow("All subscription goroutines finished. RedisPublisher shutdown complete.")

	return nil
}
