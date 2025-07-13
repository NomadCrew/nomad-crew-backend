// Package db provides implementations for data access interfaces defined in internal/store.
// It interacts with the PostgreSQL database using pgxpool and potentially other external services like Supabase.
package db

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/jackc/pgx/v4/pgxpool"
)

// DatabaseClient wraps a pgxpool.Pool with reconnection capability.
// It stores the configuration for recreating the pool when needed.
type DatabaseClient struct {
	pool       *pgxpool.Pool
	config     *pgxpool.Config
	mu         sync.RWMutex  // Protects access to the pool during refresh
	maxRetries int           // Maximum reconnection attempts
	retryDelay time.Duration // Initial delay between retries
}

// NewDatabaseClient creates a new DatabaseClient, wrapping the provided pool.
// This version now stores the pool's configuration for potential reconnection.
func NewDatabaseClient(pool *pgxpool.Pool) *DatabaseClient {
	dc := &DatabaseClient{
		pool:       pool,
		maxRetries: 5,               // Default to 5 retries
		retryDelay: time.Second * 1, // Start with 1 second delay
	}

	// Extract the config from the pool if possible
	if pool != nil {
		dc.config = pool.Config()
	}

	return dc
}

// NewDatabaseClientWithConfig creates a new DatabaseClient with the provided config and pool,
// useful when you need to ensure the config is properly stored for reconnection.
func NewDatabaseClientWithConfig(pool *pgxpool.Pool, config *pgxpool.Config) *DatabaseClient {
	return &DatabaseClient{
		pool:       pool,
		config:     config,
		maxRetries: 5,               // Default to 5 retries
		retryDelay: time.Second * 1, // Start with 1 second delay
	}
}

// GetPool returns the underlying pgxpool.Pool in a thread-safe manner.
// If the pool is nil or appears to be broken, it attempts to reconnect.
func (dc *DatabaseClient) GetPool() *pgxpool.Pool {
	dc.mu.RLock()
	pool := dc.pool
	dc.mu.RUnlock()

	// If pool is nil, try to reconnect
	if pool == nil {
		// Switch to write lock for potential reconnection
		dc.mu.Lock()
		defer dc.mu.Unlock()

		// Double-check condition after acquiring write lock
		if dc.pool == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Attempt to reconnect
			_ = dc.reconnect(ctx)
		}
		return dc.pool
	}

	return pool
}

// RefreshPool attempts to close the current pool and create a new one with retry logic.
// This is useful when the pool appears to be in a bad state.
func (dc *DatabaseClient) RefreshPool(ctx context.Context) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	return dc.reconnect(ctx)
}

// reconnect handles the logic of closing the existing pool and creating a new one.
// It includes retry logic and must be called with a write lock held.
func (dc *DatabaseClient) reconnect(ctx context.Context) error {
	log := logger.GetLogger()

	if dc.config == nil {
		return fmt.Errorf("cannot refresh pool: database configuration not available")
	}

	// Close the existing pool if present
	if dc.pool != nil {
		log.Info("Closing existing database connection pool")
		dc.pool.Close()
		dc.pool = nil
	}

	// Attempt to reconnect with retries
	var lastErr error
	delay := dc.retryDelay

	for attempt := 1; attempt <= dc.maxRetries; attempt++ {
		log.Infow("Attempting to establish database connection",
			"attempt", attempt,
			"max_attempts", dc.maxRetries)

		// Create a timeout context for this attempt
		attemptCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		newPool, err := pgxpool.ConnectConfig(attemptCtx, dc.config)
		cancel()

		if err == nil {
			// Successfully connected
			dc.pool = newPool
			log.Infow("Successfully established database connection",
				"attempt", attempt)
			return nil
		}

		lastErr = err
		log.Warnw("Failed to connect to database, will retry",
			"error", err,
			"attempt", attempt,
			"max_attempts", dc.maxRetries)

		// Check if context is done before waiting
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled during reconnection attempt: %w", ctx.Err())
		case <-time.After(delay):
			// Apply exponential backoff with jitter
			delay = time.Duration(float64(delay) * 1.5)
		}
	}

	return fmt.Errorf("failed to reconnect after %d attempts: %w", dc.maxRetries, lastErr)
}

// SetMaxRetries configures the maximum number of reconnection attempts.
func (dc *DatabaseClient) SetMaxRetries(maxRetries int) {
	dc.maxRetries = maxRetries
}

// SetRetryDelay configures the initial delay between reconnection attempts.
func (dc *DatabaseClient) SetRetryDelay(delay time.Duration) {
	dc.retryDelay = delay
}
