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
)

// DBConfig holds database configuration
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

// GetDBConfig loads database configuration from environment variables
func GetDBConfig() *DBConfig {
	port, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))
	maxOpenConns, _ := strconv.Atoi(getEnv("DB_MAX_OPEN_CONNS", "5")) // Conservative for free tier
	maxIdleConns, _ := strconv.Atoi(getEnv("DB_MAX_IDLE_CONNS", "2")) // Conservative for free tier
	connMaxLife, _ := time.ParseDuration(getEnv("DB_CONN_MAX_LIFE", "1h"))

	return &DBConfig{
		Host:         getEnv("DB_HOST", "ep-blue-sun-a8kj1qdc-pooler.eastus2.azure.neon.tech"),
		Port:         port,
		User:         getEnv("DB_USER", "neondb_owner"),
		Password:     getEnv("DB_PASSWORD", ""),
		DatabaseName: getEnv("DB_NAME", "neondb"),
		SSLMode:      getEnv("DB_SSL_MODE", "require"),
		MaxOpenConns: maxOpenConns,
		MaxIdleConns: maxIdleConns,
		ConnMaxLife:  connMaxLife,
	}
}

// GetRedisConfig loads Redis configuration from environment variables
func GetRedisConfig() *RedisConfig {
	db, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	poolSize, _ := strconv.Atoi(getEnv("REDIS_POOL_SIZE", "3"))
	minIdleConns, _ := strconv.Atoi(getEnv("REDIS_MIN_IDLE_CONNS", "1"))
	useTLS := getEnv("REDIS_USE_TLS", "true") == "true"

	return &RedisConfig{
		Address:      getEnv("REDIS_ADDRESS", "actual-serval-57447.upstash.io:6379"),
		Password:     getEnv("REDIS_PASSWORD", ""),
		DB:           db,
		UseTLS:       useTLS,
		PoolSize:     poolSize,
		MinIdleConns: minIdleConns,
	}
}

// InitDB initializes a database connection
func InitDB(config *DBConfig) (*sql.DB, error) {
	// For Neon PostgreSQL, we can use either format
	// Standard connection string format
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.Password, config.DatabaseName, config.SSLMode,
	)

	// Alternative: use the direct Neon connection string format
	// connStr := fmt.Sprintf("postgresql://%s:%s@%s/%s?sslmode=%s",
	//    config.User, config.Password, config.Host, config.DatabaseName, config.SSLMode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Set connection pool parameters optimized for free tier
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLife)

	// Test connection
	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// InitRedis initializes a Redis client
func InitRedis(config *RedisConfig) (*redis.Client, error) {
	// For Upstash, we need to use TLS
	opts := &redis.Options{
		Addr:         config.Address,
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
		// Set reasonable connection lifetime for free tier
		ConnMaxLifetime: time.Hour,
	}

	// Enable TLS for Upstash
	if config.UseTLS {
		opts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	client := redis.NewClient(opts)

	// Test connection
	_, err := client.Ping(context.Background()).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	return client, nil
}

// Helper function to get environment variables with defaults
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
