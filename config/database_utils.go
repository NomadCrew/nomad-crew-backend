// Package config handles loading and validation of application configuration
// from environment variables and potentially configuration files.
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

// ConfigureNeonPostgresPool creates and configures a pgxpool.Config suitable for connecting
// to a Neon PostgreSQL database using the provided DatabaseConfig.
// It sets up the connection string, configures TLS (required for Neon), and sets
// connection pool parameters, logging non-sensitive details.
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

// ConfigureUpstashRedisOptions creates and configures a redis.Options suitable for connecting
// to an Upstash Redis instance using the provided RedisConfig.
// It sets up connection details, pool parameters, timeouts, retry logic, and enables
// TLS (required for Upstash), logging non-sensitive details.
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
		// Add retry strategy for better resilience
		MaxRetries:      3,
		MinRetryBackoff: time.Millisecond * 100,
		MaxRetryBackoff: time.Second * 2,
		// Add reasonable timeouts
		DialTimeout:  time.Second * 5,
		ReadTimeout:  time.Second * 3,
		WriteTimeout: time.Second * 3,
	}

	// Log only non-sensitive Redis connection information
	log.Infow("Configuring Redis connection",
		"address", cfg.Address,
		"db", cfg.DB,
		"pool_size", cfg.PoolSize,
		"min_idle_conns", cfg.MinIdleConns,
		"use_tls", cfg.UseTLS)

	// Enable TLS only for Upstash Redis
	if strings.Contains(cfg.Address, "upstash.io") {
		log.Info("Enabling TLS for Upstash Redis")
		redisOptions.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	return redisOptions
}

// TestRedisConnection attempts to ping the Redis server using the provided client.
// It retries the connection up to a maximum number of times with a delay between attempts.
// Returns nil if the connection is successful, otherwise returns an error.
func TestRedisConnection(client *redis.Client) error {
	log := logger.GetLogger()
	maxRetries := 5
	retryDelay := time.Second * 2

	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		_, err := client.Ping(ctx).Result()
		cancel()

		if err == nil {
			if i > 0 {
				log.Infow("Successfully connected to Redis after retries", "attempt", i+1)
			}
			return nil
		}

		if i < maxRetries-1 {
			log.Warnw("Failed to ping Redis, retrying...",
				"error", err,
				"attempt", i+1,
				"max_attempts", maxRetries)
			time.Sleep(retryDelay)
			continue
		}

		return fmt.Errorf("failed to ping Redis after %d attempts: %w", maxRetries, err)
	}

	// This return should theoretically not be reached due to the loop structure,
	// but included for completeness.
	return nil
}
