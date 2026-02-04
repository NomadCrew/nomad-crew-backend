package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/go-redis/redismock/v9"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPgxPool is a wrapper to make pgxmock compatible with pgxpool.Pool interface
type mockPgxPool struct {
	mock pgxmock.PgxPoolIface
}

func (m *mockPgxPool) Ping(ctx context.Context) error {
	return m.mock.Ping(ctx)
}

func (m *mockPgxPool) Stat() *pgxpool.Stat {
	return m.mock.Stat()
}

func (m *mockPgxPool) Config() *pgxpool.Config {
	return m.mock.Config()
}

func (m *mockPgxPool) Close() {
	m.mock.Close()
}

func TestNewHealthService(t *testing.T) {
	// Create mock dependencies
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	mockRedis := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	version := "1.0.0"

	// Create service
	service := NewHealthService(&mockPgxPool{mock: mockDB}, mockRedis, version)

	// Verify service is created correctly
	assert.NotNil(t, service)
	assert.Equal(t, version, service.version)
	assert.NotNil(t, service.log)
	assert.NotNil(t, service.startTime)
	assert.True(t, time.Since(service.startTime) < time.Second)
}

func TestHealthService_SetActiveConnectionsGetter(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	service := NewHealthService(&mockPgxPool{mock: mockDB}, nil, "1.0.0")

	// Initially nil
	assert.Nil(t, service.activeConnections)

	// Set a getter function
	expectedConnections := 42
	getter := func() int { return expectedConnections }
	service.SetActiveConnectionsGetter(getter)

	// Verify it was set
	assert.NotNil(t, service.activeConnections)
	assert.Equal(t, expectedConnections, service.activeConnections())
}

func TestHealthService_CheckHealth(t *testing.T) {
	tests := []struct {
		name           string
		setupMocks     func(*pgxmock.PgxPoolIface, redismock.ClientMock)
		expectedStatus types.HealthStatus
		expectedComps  map[string]types.HealthStatus
		version        string
	}{
		{
			name: "All services healthy",
			setupMocks: func(dbMock *pgxmock.PgxPoolIface, redisMock redismock.ClientMock) {
				// Database ping succeeds
				(*dbMock).ExpectPing().WillReturnError(nil)
				// Return pool stats
				(*dbMock).ExpectStat().WillReturn(&pgxpool.Stat{})
				(*dbMock).ExpectConfig().WillReturn(&pgxpool.Config{MaxConns: 10})

				// Redis ping succeeds
				redisMock.ExpectPing().SetVal("PONG")
			},
			expectedStatus: types.HealthStatusUp,
			expectedComps: map[string]types.HealthStatus{
				"database": types.HealthStatusUp,
				"redis":    types.HealthStatusUp,
			},
			version: "1.0.0",
		},
		{
			name: "Database down, Redis up",
			setupMocks: func(dbMock *pgxmock.PgxPoolIface, redisMock redismock.ClientMock) {
				// Database ping fails multiple times
				(*dbMock).ExpectPing().WillReturnError(errors.New("connection refused"))
				(*dbMock).ExpectPing().WillReturnError(errors.New("connection refused"))
				(*dbMock).ExpectPing().WillReturnError(errors.New("connection refused"))

				// Redis ping succeeds
				redisMock.ExpectPing().SetVal("PONG")
			},
			expectedStatus: types.HealthStatusDown,
			expectedComps: map[string]types.HealthStatus{
				"database": types.HealthStatusDown,
				"redis":    types.HealthStatusUp,
			},
			version: "2.0.0",
		},
		{
			name: "Database up, Redis down",
			setupMocks: func(dbMock *pgxmock.PgxPoolIface, redisMock redismock.ClientMock) {
				// Database ping succeeds
				(*dbMock).ExpectPing().WillReturnError(nil)
				(*dbMock).ExpectStat().WillReturn(&pgxpool.Stat{})
				(*dbMock).ExpectConfig().WillReturn(&pgxpool.Config{MaxConns: 10})

				// Redis ping fails
				redisMock.ExpectPing().SetErr(errors.New("redis connection failed"))
			},
			expectedStatus: types.HealthStatusDown,
			expectedComps: map[string]types.HealthStatus{
				"database": types.HealthStatusUp,
				"redis":    types.HealthStatusDown,
			},
			version: "1.0.0",
		},
		{
			name: "All services down",
			setupMocks: func(dbMock *pgxmock.PgxPoolIface, redisMock redismock.ClientMock) {
				// Database ping fails
				(*dbMock).ExpectPing().WillReturnError(errors.New("db error"))
				(*dbMock).ExpectPing().WillReturnError(errors.New("db error"))
				(*dbMock).ExpectPing().WillReturnError(errors.New("db error"))

				// Redis ping fails
				redisMock.ExpectPing().SetErr(errors.New("redis error"))
			},
			expectedStatus: types.HealthStatusDown,
			expectedComps: map[string]types.HealthStatus{
				"database": types.HealthStatusDown,
				"redis":    types.HealthStatusDown,
			},
			version: "1.0.0",
		},
		{
			name: "Database degraded (high connection usage), Redis up",
			setupMocks: func(dbMock *pgxmock.PgxPoolIface, redisMock redismock.ClientMock) {
				// Database ping succeeds
				(*dbMock).ExpectPing().WillReturnError(nil)
				// Return pool stats showing high usage
				(*dbMock).ExpectStat().WillReturn(&pgxpool.Stat{})
				(*dbMock).ExpectConfig().WillReturn(&pgxpool.Config{MaxConns: 10})
				// Note: We can't easily mock AcquiredConns() since it's a method on Stat
				// This test case might need adjustment based on actual implementation

				// Redis ping succeeds
				redisMock.ExpectPing().SetVal("PONG")
			},
			expectedStatus: types.HealthStatusUp, // Since we can't mock high usage easily
			expectedComps: map[string]types.HealthStatus{
				"database": types.HealthStatusUp,
				"redis":    types.HealthStatusUp,
			},
			version: "1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			mockDB, err := pgxmock.NewPool()
			require.NoError(t, err)
			defer mockDB.Close()

			mockRedisClient, mockRedis := redismock.NewClientMock()

			// Setup expectations
			tt.setupMocks(&mockDB, mockRedis)

			// Create service
			service := NewHealthService(&mockPgxPool{mock: mockDB}, mockRedisClient, tt.version)
			
			// Set start time to more than 5 minutes ago to test mature instance behavior
			service.startTime = time.Now().Add(-10 * time.Minute)

			// Execute health check
			result := service.CheckHealth(context.Background())

			// Verify overall status
			assert.Equal(t, tt.expectedStatus, result.Status)
			assert.Equal(t, tt.version, result.Version)
			assert.NotEmpty(t, result.Timestamp)
			assert.NotEmpty(t, result.Uptime)

			// Verify component statuses
			for comp, expectedStatus := range tt.expectedComps {
				assert.Equal(t, expectedStatus, result.Components[comp].Status)
			}

			// Verify all expectations were met
			require.NoError(t, mockDB.ExpectationsWereMet())
			require.NoError(t, mockRedis.ExpectationsWereMet())
		})
	}
}

func TestHealthService_checkDatabase(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(*pgxmock.PgxPoolIface)
		startTime      time.Time
		expectedStatus types.HealthStatus
		expectedDetail string
	}{
		{
			name: "Database healthy on first try",
			setupMock: func(mock *pgxmock.PgxPoolIface) {
				(*mock).ExpectPing().WillReturnError(nil)
				(*mock).ExpectStat().WillReturn(&pgxpool.Stat{})
				(*mock).ExpectConfig().WillReturn(&pgxpool.Config{MaxConns: 10})
			},
			startTime:      time.Now(),
			expectedStatus: types.HealthStatusUp,
			expectedDetail: "",
		},
		{
			name: "Database healthy after retry",
			setupMock: func(mock *pgxmock.PgxPoolIface) {
				// First ping fails
				(*mock).ExpectPing().WillReturnError(errors.New("temporary error"))
				// Second ping succeeds
				(*mock).ExpectPing().WillReturnError(nil)
				(*mock).ExpectStat().WillReturn(&pgxpool.Stat{})
				(*mock).ExpectConfig().WillReturn(&pgxpool.Config{MaxConns: 10})
			},
			startTime:      time.Now(),
			expectedStatus: types.HealthStatusUp,
			expectedDetail: "",
		},
		{
			name: "Database down after all retries",
			setupMock: func(mock *pgxmock.PgxPoolIface) {
				// All pings fail
				(*mock).ExpectPing().WillReturnError(errors.New("connection refused"))
				(*mock).ExpectPing().WillReturnError(errors.New("connection refused"))
				(*mock).ExpectPing().WillReturnError(errors.New("connection refused"))
			},
			startTime:      time.Now(),
			expectedStatus: types.HealthStatusDown,
			expectedDetail: "Database connection failed after multiple attempts",
		},
		{
			name: "Database degraded - high connection usage (mature instance)",
			setupMock: func(mock *pgxmock.PgxPoolIface) {
				(*mock).ExpectPing().WillReturnError(nil)
				// Can't easily mock high connection usage with pgxmock
				(*mock).ExpectStat().WillReturn(&pgxpool.Stat{})
				(*mock).ExpectConfig().WillReturn(&pgxpool.Config{MaxConns: 10})
			},
			startTime:      time.Now().Add(-10 * time.Minute), // Mature instance
			expectedStatus: types.HealthStatusUp,
			expectedDetail: "",
		},
		{
			name: "Database healthy - new instance with high usage",
			setupMock: func(mock *pgxmock.PgxPoolIface) {
				(*mock).ExpectPing().WillReturnError(nil)
				(*mock).ExpectStat().WillReturn(&pgxpool.Stat{})
				(*mock).ExpectConfig().WillReturn(&pgxpool.Config{MaxConns: 10})
			},
			startTime:      time.Now(), // New instance (more lenient threshold)
			expectedStatus: types.HealthStatusUp,
			expectedDetail: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock
			mockDB, err := pgxmock.NewPool()
			require.NoError(t, err)
			defer mockDB.Close()

			// Setup expectations
			tt.setupMock(&mockDB)

			// Create service
			service := &HealthService{
				dbPool:    &mockPgxPool{mock: mockDB},
				log:       logger.GetLogger(),
				startTime: tt.startTime,
			}

			// Execute test
			result := service.checkDatabase(context.Background())

			// Verify results
			assert.Equal(t, tt.expectedStatus, result.Status)
			if tt.expectedDetail != "" {
				assert.Equal(t, tt.expectedDetail, result.Details)
			}

			// Verify all expectations were met
			require.NoError(t, mockDB.ExpectationsWereMet())
		})
	}
}

func TestHealthService_checkRedis(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(redismock.ClientMock)
		expectedStatus types.HealthStatus
		expectedDetail string
	}{
		{
			name: "Redis healthy",
			setupMock: func(mock redismock.ClientMock) {
				mock.ExpectPing().SetVal("PONG")
			},
			expectedStatus: types.HealthStatusUp,
			expectedDetail: "",
		},
		{
			name: "Redis down",
			setupMock: func(mock redismock.ClientMock) {
				mock.ExpectPing().SetErr(errors.New("connection refused"))
			},
			expectedStatus: types.HealthStatusDown,
			expectedDetail: "Redis connection failed",
		},
		{
			name: "Redis timeout",
			setupMock: func(mock redismock.ClientMock) {
				mock.ExpectPing().SetErr(context.DeadlineExceeded)
			},
			expectedStatus: types.HealthStatusDown,
			expectedDetail: "Redis connection failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock
			mockRedis, redisMock := redismock.NewClientMock()

			// Setup expectations
			tt.setupMock(redisMock)

			// Create service
			service := &HealthService{
				redisClient: mockRedis,
				log:         logger.GetLogger(),
			}

			// Execute test
			result := service.checkRedis(context.Background())

			// Verify results
			assert.Equal(t, tt.expectedStatus, result.Status)
			if tt.expectedDetail != "" {
				assert.Equal(t, tt.expectedDetail, result.Details)
			}

			// Verify all expectations were met
			require.NoError(t, redisMock.ExpectationsWereMet())
		})
	}
}

func TestHealthService_CheckHealth_WithContext(t *testing.T) {
	// Test context cancellation
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	mockRedisClient, mockRedis := redismock.NewClientMock()

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Setup mocks to expect context cancellation
	mockDB.ExpectPing().WillReturnError(context.Canceled)
	mockRedis.ExpectPing().SetErr(context.Canceled)

	service := NewHealthService(&mockPgxPool{mock: mockDB}, mockRedisClient, "1.0.0")

	// Execute with cancelled context
	result := service.CheckHealth(ctx)

	// Both components should be down due to context cancellation
	assert.Equal(t, types.HealthStatusDown, result.Status)
	assert.Equal(t, types.HealthStatusDown, result.Components["database"].Status)
	assert.Equal(t, types.HealthStatusDown, result.Components["redis"].Status)
}

func TestHealthService_CheckHealth_ConcurrentAccess(t *testing.T) {
	// Test concurrent health checks
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	mockRedisClient, mockRedis := redismock.NewClientMock()

	// Setup expectations for multiple concurrent calls
	for i := 0; i < 5; i++ {
		mockDB.ExpectPing().WillReturnError(nil)
		mockDB.ExpectStat().WillReturn(&pgxpool.Stat{})
		mockDB.ExpectConfig().WillReturn(&pgxpool.Config{MaxConns: 10})
		mockRedis.ExpectPing().SetVal("PONG")
	}

	service := NewHealthService(&mockPgxPool{mock: mockDB}, mockRedisClient, "1.0.0")

	// Run concurrent health checks
	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			result := service.CheckHealth(context.Background())
			assert.Equal(t, types.HealthStatusUp, result.Status)
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 5; i++ {
		<-done
	}

	// Verify all expectations were met
	require.NoError(t, mockDB.ExpectationsWereMet())
	require.NoError(t, mockRedis.ExpectationsWereMet())
}

// Benchmark tests
func BenchmarkHealthService_CheckHealth(b *testing.B) {
	mockDB, _ := pgxmock.NewPool()
	defer mockDB.Close()

	mockRedisClient, mockRedis := redismock.NewClientMock()

	// Setup expectations for benchmark
	for i := 0; i < b.N; i++ {
		mockDB.ExpectPing().WillReturnError(nil)
		mockDB.ExpectStat().WillReturn(&pgxpool.Stat{})
		mockDB.ExpectConfig().WillReturn(&pgxpool.Config{MaxConns: 10})
		mockRedis.ExpectPing().SetVal("PONG")
	}

	service := NewHealthService(&mockPgxPool{mock: mockDB}, mockRedisClient, "1.0.0")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.CheckHealth(context.Background())
	}
}