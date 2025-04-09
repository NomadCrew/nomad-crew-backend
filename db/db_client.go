// Package db provides implementations for data access interfaces defined in internal/store.
// It interacts with the PostgreSQL database using pgxpool and potentially other external services like Supabase.
package db

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v4/pgxpool"
)

// DatabaseClient wraps a pgxpool.Pool, potentially for managing its lifecycle.
// Note: The current implementation has limitations, especially regarding RefreshPool.
type DatabaseClient struct {
	pool *pgxpool.Pool
	// config is intended to hold the configuration for recreating the pool,
	// but it is currently uninitialized and unused by NewDatabaseClient.
	config *pgxpool.Config
	mu     sync.RWMutex // Protects access to the pool during refresh
}

// NewDatabaseClient creates a new DatabaseClient, wrapping the provided pool.
// Note: This does not initialize the internal config field, which is needed by RefreshPool.
func NewDatabaseClient(pool *pgxpool.Pool) *DatabaseClient {
	return &DatabaseClient{pool: pool}
}

// GetPool returns the underlying pgxpool.Pool in a thread-safe manner.
func (dc *DatabaseClient) GetPool() *pgxpool.Pool {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	return dc.pool
}

// RefreshPool attempts to close the current pool and create a new one using
// the stored configuration. WARNING: This method is likely non-functional
// as the `config` field is not initialized by `NewDatabaseClient`. Re-creating
// connection pools during runtime is generally not recommended; prefer creating
// the pool once at application startup.
func (dc *DatabaseClient) RefreshPool(ctx context.Context) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if dc.config == nil {
		// Cannot refresh without the original config.
		return fmt.Errorf("cannot refresh pool: database configuration not available")
	}

	if dc.pool != nil {
		// Close the existing pool before creating a new one.
		dc.pool.Close()
	}

	// Attempt to create a new pool using the stored config.
	newPool, err := pgxpool.ConnectConfig(ctx, dc.config)
	if err != nil {
		// Failed to create a new pool; the client is left without a pool.
		dc.pool = nil // Ensure pool is nil on error
		return fmt.Errorf("failed to connect with new config during pool refresh: %w", err)
	}

	dc.pool = newPool
	return nil
}
