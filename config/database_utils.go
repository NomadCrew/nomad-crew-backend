package config

import (
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/redis/go-redis/v9"
)

// ConfigureNeonPostgresPool configures a pgxpool.Config for Neon PostgreSQL
func ConfigureNeonPostgresPool(cfg *DatabaseConfig) (*pgxpool.Config, error) {
	log := logger.GetLogger()

	// Create connection string
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.Name,
		cfg.SSLMode,
	)

	// Log only non-sensitive connection information
	log.Infow("Connecting to database",
		"host", cfg.Host,
		"port", cfg.Port,
		"database", cfg.Name,
		"sslmode", cfg.SSLMode,
		"connection_string", logger.MaskConnectionString(connStr))

	// Parse connection string to config
	poolConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	// Always use TLS for Neon PostgreSQL
	if strings.Contains(cfg.Host, "neon.tech") || cfg.SSLMode == "require" {
		poolConfig.ConnConfig.TLSConfig = &tls.Config{
			ServerName: cfg.Host,
			MinVersion: tls.VersionTLS12,
		}
	}

	// Set connection pool parameters optimized for free tier
	connMaxLifeStr := cfg.ConnMaxLife
	connMaxLife, err := time.ParseDuration(connMaxLifeStr)
	if err != nil {
		log.Warnw("Invalid connection max lifetime, using default 1h", "value", connMaxLifeStr, "error", err)
		connMaxLife = time.Hour
	}

	poolConfig.MaxConns = int32(math.Min(float64(cfg.MaxOpenConns), float64(math.MaxInt32)))
	poolConfig.MinConns = int32(math.Min(float64(cfg.MaxIdleConns), float64(math.MaxInt32)))
	poolConfig.MaxConnLifetime = connMaxLife

	return poolConfig, nil
}

// ConfigureUpstashRedisOptions configures redis.Options for Upstash Redis
func ConfigureUpstashRedisOptions(cfg *RedisConfig) *redis.Options {
	log := logger.GetLogger()

	// Create Redis options
	redisOptions := &redis.Options{
		Addr:         cfg.Address,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		// Set reasonable connection lifetime for free tier
		ConnMaxLifetime: time.Hour,
	}

	// Log only non-sensitive Redis connection information
	log.Infow("Connecting to Redis",
		"address", cfg.Address,
		"db", cfg.DB,
		"pool_size", cfg.PoolSize,
		"min_idle_conns", cfg.MinIdleConns)

	// Enable TLS for Upstash Redis
	if cfg.UseTLS || strings.Contains(cfg.Address, "upstash.io") {
		redisOptions.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	return redisOptions
}

// TestRedisConnection tests the Redis connection
func TestRedisConnection(client *redis.Client) error {
	_, err := client.Ping(context.Background()).Result()
	if err != nil {
		return fmt.Errorf("failed to ping Redis: %w", err)
	}
	return nil
}
