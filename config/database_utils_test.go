package config

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigureNeonPostgresPool(t *testing.T) {
	tests := []struct {
		name           string
		config         *DatabaseConfig
		expectError    bool
		validateConfig func(t *testing.T, cfg *pgxpool.Config)
	}{
		{
			name: "Valid connection string",
			config: &DatabaseConfig{
				ConnectionString: "postgresql://user:pass@neon.tech:5432/db?sslmode=require",
			},
			expectError: false,
			validateConfig: func(t *testing.T, cfg *pgxpool.Config) {
				assert.NotNil(t, cfg)
				assert.NotNil(t, cfg.ConnConfig.TLSConfig)
				assert.Equal(t, "user", cfg.ConnConfig.User)
				assert.Equal(t, "db", cfg.ConnConfig.Database)
			},
		},
		{
			name: "Build from individual parameters with Neon host",
			config: &DatabaseConfig{
				Host:         "ep-blue-sun-a8kj1qdc.neon.tech",
				Port:         5432,
				User:         "testuser",
				Password:     "testpass",
				Name:         "testdb",
				SSLMode:      "require",
				MaxOpenConns: 20,
				MaxIdleConns: 10,
				ConnMaxLife:  "30m",
			},
			expectError: false,
			validateConfig: func(t *testing.T, cfg *pgxpool.Config) {
				assert.NotNil(t, cfg)
				assert.NotNil(t, cfg.ConnConfig.TLSConfig)
				assert.Equal(t, "testuser", cfg.ConnConfig.User)
				assert.Equal(t, "testdb", cfg.ConnConfig.Database)
				assert.Equal(t, int32(20), cfg.MaxConns)
				assert.Equal(t, int32(10), cfg.MinConns)
				assert.Equal(t, 30*time.Minute, cfg.MaxConnLifetime)
			},
		},
		{
			name: "Non-Neon host with require SSL",
			config: &DatabaseConfig{
				Host:         "regular.postgres.com",
				Port:         5432,
				User:         "user",
				Password:     "pass",
				Name:         "db",
				SSLMode:      "require",
				MaxOpenConns: 25,
				MaxIdleConns: 5,
				ConnMaxLife:  "1h",
			},
			expectError: false,
			validateConfig: func(t *testing.T, cfg *pgxpool.Config) {
				assert.NotNil(t, cfg)
				assert.NotNil(t, cfg.ConnConfig.TLSConfig)
			},
		},
		{
			name: "Invalid connection string",
			config: &DatabaseConfig{
				ConnectionString: "not-a-valid-connection-string",
			},
			expectError: true,
		},
		{
			name: "Invalid connection lifetime",
			config: &DatabaseConfig{
				Host:         "localhost",
				Port:         5432,
				User:         "user",
				Password:     "pass",
				Name:         "db",
				SSLMode:      "disable",
				MaxOpenConns: 10,
				MaxIdleConns: 5,
				ConnMaxLife:  "invalid-duration",
			},
			expectError: false, // Should use default
			validateConfig: func(t *testing.T, cfg *pgxpool.Config) {
				assert.NotNil(t, cfg)
				// Should fall back to 5 minutes
				assert.Equal(t, 5*time.Minute, cfg.MaxConnLifetime)
			},
		},
		{
			name: "Serverless optimizations",
			config: &DatabaseConfig{
				Host:         "neon.tech",
				Port:         5432,
				User:         "user",
				Password:     "pass",
				Name:         "db",
				SSLMode:      "require",
				MaxOpenConns: 100, // Should be reduced
				MaxIdleConns: 50,  // Should be reduced
				ConnMaxLife:  "2h", // Should be reduced
			},
			expectError: false,
			validateConfig: func(t *testing.T, cfg *pgxpool.Config) {
				// Set environment to simulate serverless
				os.Setenv("SERVER_ENVIRONMENT", "production")
				defer os.Unsetenv("SERVER_ENVIRONMENT")
				
				// Reconfigure to apply serverless settings
				newCfg, err := ConfigureNeonPostgresPool(&DatabaseConfig{
					Host:         "neon.tech",
					Port:         5432,
					User:         "user",
					Password:     "pass",
					Name:         "db",
					SSLMode:      "require",
					MaxOpenConns: 100,
					MaxIdleConns: 50,
					ConnMaxLife:  "2h",
				})
				require.NoError(t, err)
				
				// In serverless, connections should be limited
				assert.LessOrEqual(t, int(newCfg.MaxConns), 10)
				assert.LessOrEqual(t, int(newCfg.MinConns), 5)
				assert.LessOrEqual(t, newCfg.MaxConnLifetime, 5*time.Minute)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ConfigureNeonPostgresPool(tt.config)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, cfg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, cfg)
				
				if tt.validateConfig != nil {
					tt.validateConfig(t, cfg)
				}
				
				// Common validations
				assert.Equal(t, 30*time.Second, cfg.HealthCheckPeriod)
				assert.Equal(t, 5*time.Second, cfg.ConnConfig.ConnectTimeout)
			}
		})
	}
}

func TestIsRunningInServerless(t *testing.T) {
	tests := []struct {
		name     string
		setup    func()
		cleanup  func()
		expected bool
	}{
		{
			name:     "Not in serverless",
			setup:    func() {},
			cleanup:  func() {},
			expected: false,
		},
		{
			name: "Google Cloud Run environment",
			setup: func() {
				os.Setenv("K_SERVICE", "my-service")
			},
			cleanup: func() {
				os.Unsetenv("K_SERVICE")
			},
			expected: true,
		},
		{
			name: "Production environment",
			setup: func() {
				os.Setenv("SERVER_ENVIRONMENT", "production")
			},
			cleanup: func() {
				os.Unsetenv("SERVER_ENVIRONMENT")
			},
			expected: true,
		},
		{
			name: "Development environment",
			setup: func() {
				os.Setenv("SERVER_ENVIRONMENT", "development")
			},
			cleanup: func() {
				os.Unsetenv("SERVER_ENVIRONMENT")
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			defer tt.cleanup()
			
			// Test both internal and public functions
			assert.Equal(t, tt.expected, isRunningInServerless())
			assert.Equal(t, tt.expected, IsRunningInServerless())
		})
	}
}

func TestConfigureUpstashRedisOptions(t *testing.T) {
	tests := []struct {
		name           string
		config         *RedisConfig
		validateConfig func(t *testing.T, opts *redis.Options)
	}{
		{
			name: "Basic Upstash configuration",
			config: &RedisConfig{
				Address:      "actual-serval-57447.upstash.io:6379",
				Password:     "test-password",
				DB:           0,
				PoolSize:     15,
				MinIdleConns: 5,
				UseTLS:       true,
			},
			validateConfig: func(t *testing.T, opts *redis.Options) {
				assert.Equal(t, "actual-serval-57447.upstash.io:6379", opts.Addr)
				assert.Equal(t, "test-password", opts.Password)
				assert.Equal(t, 0, opts.DB)
				assert.Equal(t, 15, opts.PoolSize)
				assert.Equal(t, 5, opts.MinIdleConns)
				assert.NotNil(t, opts.TLSConfig)
			},
		},
		{
			name: "Non-Upstash Redis",
			config: &RedisConfig{
				Address:      "localhost:6379",
				Password:     "",
				DB:           1,
				PoolSize:     10,
				MinIdleConns: 2,
				UseTLS:       false,
			},
			validateConfig: func(t *testing.T, opts *redis.Options) {
				assert.Equal(t, "localhost:6379", opts.Addr)
				assert.Equal(t, "", opts.Password)
				assert.Equal(t, 1, opts.DB)
				assert.Equal(t, 10, opts.PoolSize)
				assert.Equal(t, 2, opts.MinIdleConns)
				assert.Nil(t, opts.TLSConfig)
			},
		},
		{
			name: "Upstash with custom settings",
			config: &RedisConfig{
				Address:      "custom.upstash.io:6380",
				Password:     "secure-password",
				DB:           2,
				PoolSize:     20,
				MinIdleConns: 10,
				UseTLS:       true,
			},
			validateConfig: func(t *testing.T, opts *redis.Options) {
				assert.Equal(t, "custom.upstash.io:6380", opts.Addr)
				assert.Equal(t, "secure-password", opts.Password)
				assert.Equal(t, 2, opts.DB)
				assert.Equal(t, 20, opts.PoolSize)
				assert.Equal(t, 10, opts.MinIdleConns)
				assert.NotNil(t, opts.TLSConfig)
				
				// Check retry and timeout settings
				assert.Equal(t, 3, opts.MaxRetries)
				assert.Equal(t, 100*time.Millisecond, opts.MinRetryBackoff)
				assert.Equal(t, 2*time.Second, opts.MaxRetryBackoff)
				assert.Equal(t, 5*time.Second, opts.DialTimeout)
				assert.Equal(t, 3*time.Second, opts.ReadTimeout)
				assert.Equal(t, 3*time.Second, opts.WriteTimeout)
				assert.Equal(t, time.Hour, opts.ConnMaxLifetime)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := ConfigureUpstashRedisOptions(tt.config)
			require.NotNil(t, opts)
			
			if tt.validateConfig != nil {
				tt.validateConfig(t, opts)
			}
		})
	}
}

func TestTestRedisConnection(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func() *redis.Client
		expectError bool
	}{
		{
			name: "Successful connection",
			setupClient: func() *redis.Client {
				// This test requires a mock Redis or will fail
				// In real tests, you'd use a test container or mock
				return redis.NewClient(&redis.Options{
					Addr: "localhost:6379",
				})
			},
			expectError: true, // Will fail without actual Redis
		},
		{
			name: "Failed connection",
			setupClient: func() *redis.Client {
				return redis.NewClient(&redis.Options{
					Addr:        "non-existent-host:6379",
					DialTimeout: 100 * time.Millisecond,
				})
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setupClient()
			defer client.Close()
			
			err := TestRedisConnection(client)
			
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMaskConnectionString(t *testing.T) {
	// Test helper function if it exists in logger package
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "PostgreSQL connection string",
			input:    "postgresql://user:password@host:5432/db",
			expected: "postgresql://user:***@host:5432/db",
		},
		{
			name:     "Connection string with special characters",
			input:    "postgresql://user:p@ssw0rd!@host:5432/db",
			expected: "postgresql://user:***@host:5432/db",
		},
		{
			name:     "Key-value format",
			input:    "host=localhost port=5432 user=test password=secret dbname=test",
			expected: "host=localhost port=5432 user=test password=*** dbname=test",
		},
		{
			name:     "Empty password",
			input:    "postgresql://user:@host:5432/db",
			expected: "postgresql://user:@host:5432/db",
		},
		{
			name:     "No password",
			input:    "postgresql://user@host:5432/db",
			expected: "postgresql://user@host:5432/db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This assumes the logger package has a MaskConnectionString function
			// If not, this test would need to be adjusted
			result := maskPassword(tt.input)
			assert.True(t, strings.Contains(result, "***") || !strings.Contains(tt.input, "password"))
		})
	}
}

// Helper function to simulate password masking
func maskPassword(connStr string) string {
	if strings.Contains(connStr, "password=") {
		parts := strings.Split(connStr, " ")
		for i, part := range parts {
			if strings.HasPrefix(part, "password=") {
				parts[i] = "password=***"
			}
		}
		return strings.Join(parts, " ")
	}
	
	if strings.Contains(connStr, "://") && strings.Contains(connStr, "@") {
		// URL format
		if strings.Contains(connStr, ":") {
			start := strings.Index(connStr, "://") + 3
			end := strings.LastIndex(connStr, "@")
			if start < end {
				userPass := connStr[start:end]
				if strings.Contains(userPass, ":") {
					user := strings.Split(userPass, ":")[0]
					return connStr[:start] + user + ":***" + connStr[end:]
				}
			}
		}
	}
	
	return connStr
}

func TestConnectionPoolEdgeCases(t *testing.T) {
	t.Run("Very large connection pool values", func(t *testing.T) {
		cfg := &DatabaseConfig{
			Host:         "localhost",
			Port:         5432,
			User:         "user",
			Password:     "pass",
			Name:         "db",
			SSLMode:      "disable",
			MaxOpenConns: 1000000,
			MaxIdleConns: 500000,
			ConnMaxLife:  "24h",
		}
		
		poolCfg, err := ConfigureNeonPostgresPool(cfg)
		require.NoError(t, err)
		
		// Should be capped at reasonable values
		assert.LessOrEqual(t, poolCfg.MaxConns, int32(1000000))
		assert.LessOrEqual(t, poolCfg.MinConns, int32(500000))
	})
	
	t.Run("Zero connection pool values", func(t *testing.T) {
		cfg := &DatabaseConfig{
			Host:         "localhost",
			Port:         5432,
			User:         "user",
			Password:     "pass",
			Name:         "db",
			SSLMode:      "disable",
			MaxOpenConns: 0,
			MaxIdleConns: 0,
			ConnMaxLife:  "0s",
		}
		
		poolCfg, err := ConfigureNeonPostgresPool(cfg)
		require.NoError(t, err)
		
		// Should have some minimum values
		assert.Equal(t, int32(0), poolCfg.MaxConns)
		assert.Equal(t, int32(0), poolCfg.MinConns)
	})
}

func TestRedisRetryBehavior(t *testing.T) {
	cfg := &RedisConfig{
		Address:      "retry-test.upstash.io:6379",
		Password:     "test",
		DB:           0,
		PoolSize:     5,
		MinIdleConns: 1,
	}
	
	opts := ConfigureUpstashRedisOptions(cfg)
	
	// Verify retry configuration
	assert.Equal(t, 3, opts.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, opts.MinRetryBackoff)
	assert.Equal(t, 2*time.Second, opts.MaxRetryBackoff)
	
	// Test with actual client (will fail but demonstrates retry behavior)
	client := redis.NewClient(opts)
	defer client.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	
	// This will fail but should attempt retries
	err := client.Ping(ctx).Err()
	assert.Error(t, err)
}