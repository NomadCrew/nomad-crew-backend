package db

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createMockPool creates a mock pool for testing
func createMockPool(t *testing.T) (pgxmock.PgxPoolIface, func()) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)

	cleanup := func() {
		mock.Close()
	}

	return mock, cleanup
}

func TestNewDatabaseClient(t *testing.T) {
	tests := []struct {
		name string
		pool *pgxpool.Pool
		want *DatabaseClient
	}{
		{
			name: "with nil pool",
			pool: nil,
			want: &DatabaseClient{
				pool:       nil,
				config:     nil,
				maxRetries: 5,
				retryDelay: time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewDatabaseClient(tt.pool)
			assert.NotNil(t, got)
			assert.Equal(t, tt.want.pool, got.pool)
			assert.Equal(t, tt.want.maxRetries, got.maxRetries)
			assert.Equal(t, tt.want.retryDelay, got.retryDelay)
		})
	}
}

func TestNewDatabaseClientWithConfig(t *testing.T) {
	config := &pgxpool.Config{}
	
	tests := []struct {
		name   string
		pool   *pgxpool.Pool
		config *pgxpool.Config
	}{
		{
			name:   "with config",
			pool:   nil,
			config: config,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewDatabaseClientWithConfig(tt.pool, tt.config)
			assert.NotNil(t, got)
			assert.Equal(t, tt.pool, got.pool)
			assert.Equal(t, tt.config, got.config)
			assert.Equal(t, 5, got.maxRetries)
			assert.Equal(t, time.Second, got.retryDelay)
		})
	}
}

func TestDatabaseClient_GetPool(t *testing.T) {
	t.Run("returns existing pool", func(t *testing.T) {
		// This test verifies the thread-safe access to the pool
		dc := &DatabaseClient{
			pool:       &pgxpool.Pool{}, // Mock pool
			maxRetries: 5,
			retryDelay: time.Second,
		}

		// Concurrent access test
		var wg sync.WaitGroup
		results := make([]*pgxpool.Pool, 10)

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				results[idx] = dc.GetPool()
			}(i)
		}

		wg.Wait()

		// All goroutines should get the same pool reference
		for i := 1; i < 10; i++ {
			assert.Equal(t, results[0], results[i])
		}
	})

	t.Run("attempts reconnect when pool is nil", func(t *testing.T) {
		dc := &DatabaseClient{
			pool:       nil,
			config:     nil, // Will cause reconnect to fail
			maxRetries: 1,
			retryDelay: time.Millisecond,
		}

		pool := dc.GetPool()
		assert.Nil(t, pool)
	})
}

func TestDatabaseClient_RefreshPool(t *testing.T) {
	t.Run("returns error when config is nil", func(t *testing.T) {
		dc := &DatabaseClient{
			pool:   nil,
			config: nil,
		}

		ctx := context.Background()
		err := dc.RefreshPool(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database configuration not available")
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		dc := &DatabaseClient{
			pool:       nil,
			config:     &pgxpool.Config{},
			maxRetries: 5,
			retryDelay: time.Second,
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := dc.RefreshPool(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})
}

func TestDatabaseClient_reconnect(t *testing.T) {
	t.Run("successful reconnection", func(t *testing.T) {
		// This would require a proper mock of pgxpool.ConnectConfig
		// For now, we'll skip the actual connection test
		t.Skip("Requires integration test setup")
	})

	t.Run("handles retry with exponential backoff", func(t *testing.T) {
		dc := &DatabaseClient{
			pool:       nil,
			config:     &pgxpool.Config{},
			maxRetries: 3,
			retryDelay: 10 * time.Millisecond,
		}

		start := time.Now()
		ctx := context.Background()
		
		// This will fail but test the retry logic
		err := dc.reconnect(ctx)
		
		elapsed := time.Since(start)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to reconnect after 3 attempts")
		
		// Should have taken at least the sum of delays
		// Initial: 10ms, then 15ms, then 22.5ms = ~47.5ms minimum
		assert.True(t, elapsed >= 40*time.Millisecond)
	})

	t.Run("respects context timeout", func(t *testing.T) {
		dc := &DatabaseClient{
			pool:       nil,
			config:     &pgxpool.Config{},
			maxRetries: 10,
			retryDelay: 100 * time.Millisecond,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		start := time.Now()
		err := dc.reconnect(ctx)
		elapsed := time.Since(start)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context")
		// Should timeout before completing all retries
		assert.True(t, elapsed < 200*time.Millisecond)
	})
}

func TestDatabaseClient_SetMaxRetries(t *testing.T) {
	dc := &DatabaseClient{}
	dc.SetMaxRetries(10)
	assert.Equal(t, 10, dc.maxRetries)
}

func TestDatabaseClient_SetRetryDelay(t *testing.T) {
	dc := &DatabaseClient{}
	delay := 5 * time.Second
	dc.SetRetryDelay(delay)
	assert.Equal(t, delay, dc.retryDelay)
}

func TestDatabaseClient_ConcurrentAccess(t *testing.T) {
	dc := &DatabaseClient{
		pool:       &pgxpool.Pool{},
		config:     &pgxpool.Config{},
		maxRetries: 5,
		retryDelay: time.Millisecond,
	}

	ctx := context.Background()
	var wg sync.WaitGroup

	// Test concurrent GetPool and RefreshPool calls
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = dc.GetPool()
		}()

		if i%10 == 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = dc.RefreshPool(ctx)
			}()
		}
	}

	wg.Wait()
	// If we get here without deadlock, the mutex is working correctly
}

// BenchmarkDatabaseClient_GetPool benchmarks the GetPool method
func BenchmarkDatabaseClient_GetPool(b *testing.B) {
	dc := &DatabaseClient{
		pool: &pgxpool.Pool{},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = dc.GetPool()
		}
	})
}

// BenchmarkDatabaseClient_GetPoolWithReconnect benchmarks GetPool when reconnection is needed
func BenchmarkDatabaseClient_GetPoolWithReconnect(b *testing.B) {
	dc := &DatabaseClient{
		pool:       nil,
		config:     nil, // Will fail but tests the reconnect path
		maxRetries: 1,
		retryDelay: time.Microsecond,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dc.GetPool()
		dc.pool = nil // Reset for next iteration
	}
}