package services

import (
	"context"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type HealthService struct {
	dbPool      *pgxpool.Pool
	redisClient *redis.Client
	version     string
	log         *zap.SugaredLogger
}

func NewHealthService(dbPool *pgxpool.Pool, redisClient *redis.Client, version string) *HealthService {
	return &HealthService{
		dbPool:      dbPool,
		redisClient: redisClient,
		version:     version,
		log:         logger.GetLogger(),
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
	} else if dbStatus.Status == types.HealthStatusDegraded {
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
	}
}

func (h *HealthService) checkDatabase(ctx context.Context) types.HealthComponent {
	if err := h.dbPool.Ping(ctx); err != nil {
		h.log.Errorw("Database health check failed", "error", err)
		return types.HealthComponent{
			Status:  types.HealthStatusDown,
			Details: "Database connection failed",
		}
	}

	// Check connection pool metrics
	stat := h.dbPool.Stat()
	if float64(stat.AcquireCount())/float64(stat.TotalConns()) > 0.8 {
		return types.HealthComponent{
			Status:  types.HealthStatusDegraded,
			Details: "Connection pool near capacity",
		}
	}

	return types.HealthComponent{
		Status: types.HealthStatusUp,
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
