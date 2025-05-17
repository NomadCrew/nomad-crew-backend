package services

import (
	"context"
	"fmt"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type HealthService struct {
	dbPool            *pgxpool.Pool
	redisClient       *redis.Client
	version           string
	log               *zap.SugaredLogger
	activeConnections func() int
	startTime         time.Time
}

func NewHealthService(dbPool *pgxpool.Pool, redisClient *redis.Client, version string) *HealthService {
	return &HealthService{
		dbPool:      dbPool,
		redisClient: redisClient,
		version:     version,
		log:         logger.GetLogger(),
		startTime:   time.Now(),
	}
}

func (h *HealthService) SetActiveConnectionsGetter(getter func() int) {
	h.activeConnections = getter
}

func (h *HealthService) checkWebSocketHealth(ctx context.Context) types.HealthComponent {
	// Check if Redis pubsub is working for WebSockets
	const testChannel = "health:ws:test"
	const testMessage = "ping"

	// Create a test pubsub instance
	pubsub := h.redisClient.Subscribe(ctx, testChannel)
	defer pubsub.Close()

	// Create a channel to receive the test message
	msgChan := make(chan struct{})
	go func() {
		defer close(msgChan)
		msg, err := pubsub.ReceiveMessage(ctx)
		if err != nil || msg.Payload != testMessage {
			return
		}
		msgChan <- struct{}{}
	}()

	// Publish test message
	if err := h.redisClient.Publish(ctx, testChannel, testMessage).Err(); err != nil {
		h.log.Errorw("WebSocket health check failed to publish", "error", err)
		return types.HealthComponent{
			Status:  types.HealthStatusDegraded,
			Details: "PubSub publish failed: " + err.Error(),
		}
	}

	// Wait for message or timeout
	select {
	case <-msgChan:
		// Success, message received
	case <-time.After(2 * time.Second):
		return types.HealthComponent{
			Status:  types.HealthStatusDegraded,
			Details: "PubSub message not received in time",
		}
	}

	// Get active connection count if available
	var activeConns int
	if h.activeConnections != nil {
		activeConns = h.activeConnections()
	}

	return types.HealthComponent{
		Status:  types.HealthStatusUp,
		Details: fmt.Sprintf("Active connections: %d", activeConns),
	}
}

func (h *HealthService) CheckHealth(ctx context.Context) types.HealthCheck {
	components := make(map[string]types.HealthComponent)
	overallStatus := types.HealthStatusUp

	// Check database
	dbStatus := h.checkDatabase(ctx)
	components["database"] = dbStatus
	if dbStatus.Status == types.HealthStatusDown {
		overallStatus = types.HealthStatusDown
	} else if dbStatus.Status == types.HealthStatusDegraded && overallStatus != types.HealthStatusDown {
		overallStatus = types.HealthStatusDegraded
	}

	// Check Redis
	redisStatus := h.checkRedis(ctx)
	components["redis"] = redisStatus
	if redisStatus.Status == types.HealthStatusDown {
		overallStatus = types.HealthStatusDown
	} else if redisStatus.Status == types.HealthStatusDegraded && overallStatus != types.HealthStatusDown {
		overallStatus = types.HealthStatusDegraded
	}

	// Add WebSocket health check
	wsStatus := h.checkWebSocketHealth(ctx)
	components["websocket"] = wsStatus
	if wsStatus.Status == types.HealthStatusDown {
		overallStatus = types.HealthStatusDown
	} else if wsStatus.Status == types.HealthStatusDegraded && overallStatus != types.HealthStatusDown {
		overallStatus = types.HealthStatusDegraded
	}

	return types.HealthCheck{
		Status:     overallStatus,
		Components: components,
		Version:    h.version,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Uptime:     time.Since(h.startTime).String(), // Add uptime information
	}
}

func (h *HealthService) checkDatabase(ctx context.Context) types.HealthComponent {
	// Add retry logic for database ping with a timeout
	maxRetries := 3
	retryDelay := 500 * time.Millisecond
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		// Create a timeout context for each attempt
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := h.dbPool.Ping(pingCtx)
		cancel()

		if err == nil {
			// Successfully pinged database
			if i > 0 {
				h.log.Infow("Database connection restored after retries", "attempts", i+1)
			}

			// Check connection pool metrics with adjusted thresholds for serverless
			stat := h.dbPool.Stat()
			totalConns := stat.TotalConns()

			// Only check pool capacity if we have connections
			if totalConns > 0 {
				acquireRatio := float64(stat.AcquireCount()) / float64(totalConns)

				// For recently started instances, use a higher threshold
				uptime := time.Since(h.startTime)
				threshold := 0.9 // More lenient threshold for serverless

				// If service has been up for a while, use stricter threshold
				if uptime > 5*time.Minute {
					threshold = 0.8
				}

				if acquireRatio > threshold {
					return types.HealthComponent{
						Status:  types.HealthStatusDegraded,
						Details: fmt.Sprintf("Connection pool near capacity (%.1f%%)", acquireRatio*100),
					}
				}
			}

			return types.HealthComponent{
				Status: types.HealthStatusUp,
			}
		}

		lastErr = err

		// If we have more retries left, wait and try again
		if i < maxRetries-1 {
			h.log.Warnw("Database ping failed, retrying",
				"error", err,
				"attempt", i+1,
				"max_attempts", maxRetries)
			time.Sleep(retryDelay)
			// Increase retry delay for next attempt (simple backoff)
			retryDelay *= 2
		}
	}

	h.log.Errorw("Database health check failed after retries",
		"error", lastErr,
		"attempts", maxRetries)
	return types.HealthComponent{
		Status:  types.HealthStatusDown,
		Details: "Database connection failed after multiple attempts",
	}
}

func (h *HealthService) checkRedis(ctx context.Context) types.HealthComponent {
	if err := h.redisClient.Ping(ctx).Err(); err != nil {
		h.log.Errorw("Redis health check failed", "error", err)
		return types.HealthComponent{
			Status:  types.HealthStatusDown,
			Details: "Redis connection failed",
		}
	}

	return types.HealthComponent{
		Status: types.HealthStatusUp,
	}
}
