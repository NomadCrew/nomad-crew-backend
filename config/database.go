// Package config handles loading and validation of application configuration
// from environment variables and potentially configuration files.
package config

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// DBConfig holds database configuration.
// Deprecated: This struct is likely superseded by DatabaseConfig in config.go.
// Consider migrating usage to the main Config struct loaded by LoadConfig.
type DBConfig struct {
	Host         string
	Port         int
	User         string
	Password     string
	DatabaseName string
	SSLMode      string
	MaxOpenConns int
	MaxIdleConns int
	ConnMaxLife  time.Duration
}

// RedisConfig is now defined in config.go

// GetDBConfig loads database configuration from environment variables.
// Deprecated: This function is likely superseded by LoadConfig in config.go.
// Consider using the unified Config struct instead.
func GetDBConfig() *DBConfig {
	port, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))
	maxOpenConns, _ := strconv.Atoi(getEnv("DB_MAX_OPEN_CONNS", "5")) // Conservative default for free tier DBs
	maxIdleConns, _ := strconv.Atoi(getEnv("DB_MAX_IDLE_CONNS", "2")) // Conservative default for free tier DBs
	connMaxLife, _ := time.ParseDuration(getEnv("DB_CONN_MAX_LIFE", "1h"))

	return &DBConfig{
		Host:         getEnv("DB_HOST", "ep-blue-sun-a8kj1qdc-pooler.eastus2.azure.neon.tech"), // Default for Neon
		Port:         port,
		User:         getEnv("DB_USER", "neondb_owner"), // Default for Neon
		Password:     getEnv("DB_PASSWORD", ""),
		DatabaseName: getEnv("DB_NAME", "neondb"),      // Default for Neon
		SSLMode:      getEnv("DB_SSL_MODE", "require"), // Default required by Neon
		MaxOpenConns: maxOpenConns,
		MaxIdleConns: maxIdleConns,
		ConnMaxLife:  connMaxLife,
	}
}

// GetRedisConfig loads Redis configuration from environment variables.
// Deprecated: This function is likely superseded by LoadConfig in config.go.
// Consider using the unified Config struct instead.
func GetRedisConfig() *RedisConfig {
	db, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	poolSize, _ := strconv.Atoi(getEnv("REDIS_POOL_SIZE", "3"))          // Conservative default for free tier Redis
	minIdleConns, _ := strconv.Atoi(getEnv("REDIS_MIN_IDLE_CONNS", "1")) // Conservative default
	useTLS := getEnv("REDIS_USE_TLS", "true") == "true"

	return &RedisConfig{
		Address:      getEnv("REDIS_ADDRESS", "actual-serval-57447.upstash.io:6379"), // Default for Upstash
		Password:     getEnv("REDIS_PASSWORD", ""),
		DB:           db,
		UseTLS:       useTLS, // Required for Upstash
		PoolSize:     poolSize,
		MinIdleConns: minIdleConns,
	}
}

// InitDB initializes a standard sql.DB connection using the provided DBConfig.
// Deprecated: This function uses database/sql directly and is likely superseded
// by the pgxpool setup in main.go using ConfigureNeonPostgresPool.
func InitDB(config *DBConfig) (*sql.DB, error) {
	// Standard connection string format compatible with lib/pq
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.Password, config.DatabaseName, config.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Set connection pool parameters
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLife)

	// Test connection
	err = db.Ping()
	if err != nil {
		// Close the connection pool if ping fails to prevent resource leak
		if closeErr := db.Close(); closeErr != nil {
			zap.L().Warn("Error closing PostgreSQL pool after ping failure", zap.Error(closeErr))
		}
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// InitRedis initializes a Redis client connection using the provided RedisConfig.
// Deprecated: This function is likely superseded by the setup in main.go
// using ConfigureUpstashRedisOptions and TestRedisConnection.
func InitRedis(config *RedisConfig) (*redis.Client, error) {
	opts := &redis.Options{
		Addr:            config.Address,
		Password:        config.Password,
		DB:              config.DB,
		PoolSize:        config.PoolSize,
		MinIdleConns:    config.MinIdleConns,
		ConnMaxLifetime: time.Hour, // Set reasonable connection lifetime for free tier
	}

	// Enable TLS if specified, typically required for Upstash
	if config.UseTLS {
		opts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12, // Ensure modern TLS version
		}
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // Add timeout
	defer cancel()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		// Close the client if ping fails
		if closeErr := client.Close(); closeErr != nil {
			zap.L().Warn("Error closing Redis client after ping failure", zap.Error(closeErr))
		}
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	return client, nil
}

// getEnv retrieves an environment variable by key or returns a default value.
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
