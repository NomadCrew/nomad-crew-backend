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
