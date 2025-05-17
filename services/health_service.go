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

	// In serverless environments, give more time for websocket connections
	// Only use stricter timing after the service has been up for a while
	wsTimeout := 3 * time.Second // Reduced from 5s to 3s but adding retries
	uptime := time.Since(h.startTime)
	if uptime > 5*time.Minute {
		wsTimeout = 2 * time.Second // Use original value for established services
	}

	// Add retry logic for PubSub checks
	maxRetries := 2
	retryDelay := 500 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Create a test pubsub instance for this attempt
		pubsub := h.redisClient.Subscribe(ctx, testChannel)

		// Create a channel to receive the test message
		msgChan := make(chan struct{})
		go func() {
			defer close(msgChan)
			receiveCtx, cancel := context.WithTimeout(ctx, wsTimeout)
			defer cancel()

			msg, err := pubsub.ReceiveMessage(receiveCtx)
			if err != nil || msg.Payload != testMessage {
				h.log.Debugw("Failed to receive PubSub message",
					"attempt", attempt+1,
					"error", err,
					"payload", msg.Payload)
				return
			}
			msgChan <- struct{}{}
		}()

		// Publish test message
		publishErr := h.redisClient.Publish(ctx, testChannel, testMessage).Err()
		if publishErr != nil {
			pubsub.Close()
			if attempt == maxRetries-1 {
				h.log.Errorw("WebSocket health check failed to publish after retries",
					"error", publishErr,
					"attempts", attempt+1)
				return types.HealthComponent{
					Status:  types.HealthStatusDegraded,
					Details: "PubSub publish failed: " + publishErr.Error(),
				}
			}

			// Retry after delay
			time.Sleep(retryDelay)
			retryDelay *= 2
			continue
		}

		// Wait for message or timeout
		select {
		case <-msgChan:
			// Success, message received
			pubsub.Close()

			// If this wasn't the first attempt, log success after retry
			if attempt > 0 {
				h.log.Infow("PubSub message received successfully after retry",
					"attempt", attempt+1)
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

		case <-time.After(wsTimeout):
			// Timeout on this attempt
			pubsub.Close()

			// If we have more retries, try again
			if attempt < maxRetries-1 {
				h.log.Warnw("PubSub message not received in time, retrying",
					"attempt", attempt+1,
					"timeout", wsTimeout)
				time.Sleep(retryDelay)
				retryDelay *= 2
				continue
			}

			// Final attempt failed
			h.log.Warnw("PubSub message not received after all retries",
				"attempts", maxRetries,
				"timeout", wsTimeout)

			// Use degraded status instead of down for WebSocket issues
			// This is less critical than database connection issues
			return types.HealthComponent{
				Status: types.HealthStatusDegraded,
				Details: fmt.Sprintf("PubSub message not received after %d attempts (timeout: %s)",
					maxRetries, wsTimeout),
			}
		}
	}

	// This code should not be reached due to the loop structure
	return types.HealthComponent{
		Status:  types.HealthStatusDegraded,
		Details: "Unexpected flow in WebSocket health check",
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

	// For WebSocket, only set overall status to DOWN if websocket is DOWN
	// Don't mark the whole service as DEGRADED just because WebSocket is DEGRADED
	// This makes the readiness probe more reliable in Kubernetes
	if wsStatus.Status == types.HealthStatusDown {
		overallStatus = types.HealthStatusDown
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
			acquireCount := stat.AcquireCount()

			// Only check pool capacity if we have connections
			if totalConns > 0 {
				// Calculate pool capacity properly
				// The original formula "AcquireCount/TotalConns" is incorrect
				// as AcquireCount is cumulative over time and can exceed TotalConns
				// Instead, look at current vs max connections

				// Check current connections against max connections
				maxConns := h.dbPool.Config().MaxConns
				currentConns := stat.AcquiredConns()

				h.log.Debugw("Database pool stats",
					"total_conns", totalConns,
					"current_conns", currentConns,
					"max_conns", maxConns,
					"acquire_count", acquireCount)

				// Calculate usage percentage
				usageRatio := float64(currentConns) / float64(maxConns)
				usagePercent := usageRatio * 100

				// For recently started instances, use a higher threshold
				uptime := time.Since(h.startTime)
				threshold := 0.9 // More lenient threshold for serverless

				// If service has been up for a while, use stricter threshold
				if uptime > 5*time.Minute {
					threshold = 0.8
				}

				if usageRatio > threshold {
					return types.HealthComponent{
						Status:  types.HealthStatusDegraded,
						Details: fmt.Sprintf("Connection pool near capacity (%.1f%%)", usagePercent),
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
