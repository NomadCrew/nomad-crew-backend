package events

import (
	"context"
	"fmt"
	"sync"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// RouterMetrics holds Prometheus metrics for the router
type RouterMetrics struct {
	handlerCount    prometheus.Gauge
	handlerLatency  prometheus.Histogram
	handlerErrors   *prometheus.CounterVec
	eventsRouted    *prometheus.CounterVec
	eventsDiscarded *prometheus.CounterVec
}

var (
	routerMetricsOnce   sync.Once
	globalRouterMetrics *RouterMetrics
)

// getRouterMetrics initializes router metrics if they haven't been, and returns them.
// This ensures metrics are registered only once.
func getRouterMetrics() *RouterMetrics {
	routerMetricsOnce.Do(func() {
		globalRouterMetrics = &RouterMetrics{
			handlerCount: promauto.NewGauge(prometheus.GaugeOpts{
				Name: "event_handlers_total",
				Help: "Total number of registered event handlers",
			}),
			handlerLatency: promauto.NewHistogram(prometheus.HistogramOpts{
				Name:    "event_handler_duration_seconds",
				Help:    "Time taken to handle events",
				Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
			}),
			handlerErrors: promauto.NewCounterVec(prometheus.CounterOpts{
				Name: "event_handler_errors_total",
				Help: "Total number of handler errors by event type",
			}, []string{"event_type"}),
			eventsRouted: promauto.NewCounterVec(prometheus.CounterOpts{
				Name: "events_routed_total",
				Help: "Total number of events routed by type",
			}, []string{"event_type"}),
			eventsDiscarded: promauto.NewCounterVec(prometheus.CounterOpts{
				Name: "events_discarded_total",
				Help: "Total number of events discarded by reason",
			}, []string{"reason"}),
		}
	})
	return globalRouterMetrics
}

// Router handles event routing and distribution to registered handlers
type Router struct {
	log     *zap.SugaredLogger
	metrics *RouterMetrics
	mu      sync.RWMutex
	// Map of event type to slice of handlers
	handlers map[types.EventType][]types.EventHandler
}

// NewRouter creates a new event router
func NewRouter() *Router {
	return &Router{
		log:      logger.GetLogger().Named("event_router"),
		metrics:  getRouterMetrics(),
		handlers: make(map[types.EventType][]types.EventHandler),
	}
}

// RegisterHandler registers an event handler for specific event types
func (r *Router) RegisterHandler(handler types.EventHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	supportedEvents := handler.SupportedEvents()
	if len(supportedEvents) == 0 {
		r.log.Warnw("Handler registered with no supported events", "handler", fmt.Sprintf("%T", handler))
		return
	}

	for _, eventType := range supportedEvents {
		r.handlers[eventType] = append(r.handlers[eventType], handler)
		r.log.Infow("Registered event handler",
			"eventType", eventType,
			"handler", fmt.Sprintf("%T", handler),
		)
	}

	r.metrics.handlerCount.Set(float64(r.countHandlers()))
}

// UnregisterHandler removes a handler for all its supported events
func (r *Router) UnregisterHandler(handler types.EventHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, eventType := range handler.SupportedEvents() {
		handlers := r.handlers[eventType]
		for i, h := range handlers {
			if h == handler {
				// Remove handler by replacing it with the last element and truncating
				handlers[i] = handlers[len(handlers)-1]
				r.handlers[eventType] = handlers[:len(handlers)-1]
				r.log.Infow("Unregistered event handler",
					"eventType", eventType,
					"handler", fmt.Sprintf("%T", handler),
				)
				break
			}
		}
		// Remove the event type if no handlers remain
		if len(r.handlers[eventType]) == 0 {
			delete(r.handlers, eventType)
		}
	}

	r.metrics.handlerCount.Set(float64(r.countHandlers()))
}

// HandleEvent routes an event to all registered handlers for its type
func (r *Router) HandleEvent(ctx context.Context, event types.Event) error {
	r.mu.RLock()
	handlers := r.handlers[event.Type]
	r.mu.RUnlock()

	if len(handlers) == 0 {
		r.metrics.eventsDiscarded.WithLabelValues("no_handlers").Inc()
		r.log.Debugw("No handlers registered for event type", "eventType", event.Type)
		return nil
	}

	r.metrics.eventsRouted.WithLabelValues(string(event.Type)).Inc()

	var wg sync.WaitGroup
	errCh := make(chan error, len(handlers))

	for _, handler := range handlers {
		wg.Add(1)
		go func(h types.EventHandler) {
			defer wg.Done()
			timer := prometheus.NewTimer(r.metrics.handlerLatency)
			defer timer.ObserveDuration()

			if err := h.HandleEvent(ctx, event); err != nil {
				r.metrics.handlerErrors.WithLabelValues(string(event.Type)).Inc()
				r.log.Errorw("Handler error",
					"error", err,
					"eventType", event.Type,
					"handler", fmt.Sprintf("%T", h),
				)
				errCh <- fmt.Errorf("handler %T: %w", h, err)
			}
		}(handler)
	}

	// Wait for all handlers to complete
	wg.Wait()
	close(errCh)

	// Collect any errors
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("handler errors: %v", errs)
	}

	return nil
}

// countHandlers returns the total number of unique handlers across all event types
func (r *Router) countHandlers() int {
	// Use a map to count unique handlers
	unique := make(map[types.EventHandler]struct{})
	for _, handlers := range r.handlers {
		for _, h := range handlers {
			unique[h] = struct{}{}
		}
	}
	return len(unique)
}
