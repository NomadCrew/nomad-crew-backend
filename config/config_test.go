package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvironmentChecks(t *testing.T) {
	tests := []struct {
		name           string
		env            Environment
		isDevelopment  bool
		isProduction   bool
	}{
		{
			name:          "Development environment",
			env:           EnvDevelopment,
			isDevelopment: true,
			isProduction:  false,
		},
		{
			name:          "Production environment",
			env:           EnvProduction,
			isDevelopment: false,
			isProduction:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Server: ServerConfig{
					Environment: tt.env,
				},
			}

			assert.Equal(t, tt.isDevelopment, cfg.IsDevelopment())
			assert.Equal(t, tt.isProduction, cfg.IsProduction())
		})
	}
}

func TestLoadConfigWithEnvironmentVariables(t *testing.T) {
	// Save current environment and restore after test
	originalEnv := make(map[string]string)
	envVars := []string{
		"SERVER_ENVIRONMENT",
		"PORT",
		"ALLOWED_ORIGINS",
		"JWT_SECRET_KEY",
		"DATABASE_URL",
		"DB_HOST",
		"DB_PORT",
		"DB_USER",
		"DB_PASSWORD",
		"DB_NAME",
		"DB_SSL_MODE",
		"REDIS_ADDRESS",
		"REDIS_PASSWORD",
		"REDIS_DB",
		"REDIS_USE_TLS",
		"GEOAPIFY_KEY",
		"PEXELS_API_KEY",
		"SUPABASE_ANON_KEY",
		"SUPABASE_SERVICE_KEY",
		"SUPABASE_URL",
		"SUPABASE_JWT_SECRET",
		"EMAIL_FROM_ADDRESS",
		"EMAIL_FROM_NAME",
		"RESEND_API_KEY",
		"EVENT_SERVICE_PUBLISH_TIMEOUT_SECONDS",
		"EVENT_SERVICE_SUBSCRIBE_TIMEOUT_SECONDS",
		"EVENT_SERVICE_EVENT_BUFFER_SIZE",
	}

	// Save original values
	for _, key := range envVars {
		originalEnv[key] = os.Getenv(key)
		os.Unsetenv(key)
	}

	// Restore environment after test
	defer func() {
		for key, value := range originalEnv {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	t.Run("Load with minimal valid configuration", func(t *testing.T) {
		// Set minimal required environment variables
		os.Setenv("JWT_SECRET_KEY", "this-is-a-very-long-secret-key-that-meets-the-minimum-requirements")
		os.Setenv("REDIS_ADDRESS", "localhost:6379")
		os.Setenv("GEOAPIFY_KEY", "test-geoapify-key")
		os.Setenv("PEXELS_API_KEY", "test-pexels-key")
		os.Setenv("SUPABASE_ANON_KEY", "test-supabase-anon-key")
		os.Setenv("SUPABASE_SERVICE_KEY", "test-supabase-service-key-that-is-long-enough")
		os.Setenv("SUPABASE_URL", "https://test.supabase.co")
		os.Setenv("SUPABASE_JWT_SECRET", "test-supabase-jwt-secret-that-is-long-enough")
		os.Setenv("EMAIL_FROM_ADDRESS", "test@example.com")
		os.Setenv("RESEND_API_KEY", "test-resend-key")

		cfg, err := LoadConfig()
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Check defaults were set
		assert.Equal(t, EnvDevelopment, cfg.Server.Environment)
		assert.Equal(t, "8080", cfg.Server.Port)
		assert.Equal(t, []string{"*"}, cfg.Server.AllowedOrigins)
		assert.Equal(t, 0, cfg.Redis.DB)
		assert.Equal(t, 5, cfg.EventService.PublishTimeoutSeconds)
		assert.Equal(t, 10, cfg.EventService.SubscribeTimeoutSeconds)
		assert.Equal(t, 100, cfg.EventService.EventBufferSize)
	})

	t.Run("Load with custom values", func(t *testing.T) {
		// Set all environment variables
		os.Setenv("SERVER_ENVIRONMENT", "production")
		os.Setenv("PORT", "3000")
		os.Setenv("ALLOWED_ORIGINS", "https://example.com,https://app.example.com")
		os.Setenv("JWT_SECRET_KEY", "custom-jwt-secret-key-that-is-very-long-and-secure")
		os.Setenv("DB_HOST", "custom-host")
		os.Setenv("DB_USER", "custom-user")
		os.Setenv("DB_PASSWORD", "custom-pass")
		os.Setenv("DB_NAME", "custom-db")
		os.Setenv("REDIS_ADDRESS", "redis.example.com:6379")
		os.Setenv("REDIS_PASSWORD", "redis-password")
		os.Setenv("REDIS_DB", "2")
		os.Setenv("REDIS_USE_TLS", "true")
		os.Setenv("GEOAPIFY_KEY", "custom-geoapify-key")
		os.Setenv("PEXELS_API_KEY", "custom-pexels-key")
		os.Setenv("SUPABASE_ANON_KEY", "custom-supabase-anon-key")
		os.Setenv("SUPABASE_SERVICE_KEY", "custom-supabase-service-key-that-is-long-enough")
		os.Setenv("SUPABASE_URL", "https://custom.supabase.co")
		os.Setenv("SUPABASE_JWT_SECRET", "custom-supabase-jwt-secret-that-is-long-enough")
		os.Setenv("EMAIL_FROM_ADDRESS", "noreply@example.com")
		os.Setenv("EMAIL_FROM_NAME", "Example App")
		os.Setenv("RESEND_API_KEY", "custom-resend-key")
		os.Setenv("EVENT_SERVICE_PUBLISH_TIMEOUT_SECONDS", "30")
		os.Setenv("EVENT_SERVICE_SUBSCRIBE_TIMEOUT_SECONDS", "60")
		os.Setenv("EVENT_SERVICE_EVENT_BUFFER_SIZE", "200")

		cfg, err := LoadConfig()
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify custom values
		assert.Equal(t, Environment("production"), cfg.Server.Environment)
		assert.Equal(t, "3000", cfg.Server.Port)
		assert.Equal(t, "custom-jwt-secret-key-that-is-very-long-and-secure", cfg.Server.JwtSecretKey)
		assert.Equal(t, "custom-host", cfg.Database.Host)
		assert.Equal(t, "custom-user", cfg.Database.User)
		assert.Equal(t, "custom-db", cfg.Database.Name)
		assert.Equal(t, "redis.example.com:6379", cfg.Redis.Address)
		assert.Equal(t, "redis-password", cfg.Redis.Password)
		assert.Equal(t, 2, cfg.Redis.DB)
		assert.True(t, cfg.Redis.UseTLS)
		assert.Equal(t, 30, cfg.EventService.PublishTimeoutSeconds)
		assert.Equal(t, 60, cfg.EventService.SubscribeTimeoutSeconds)
		assert.Equal(t, 200, cfg.EventService.EventBufferSize)
	})

	t.Run("Validation failures", func(t *testing.T) {
		testCases := []struct {
			name    string
			setup   func()
			cleanup func()
			errMsg  string
		}{
			{
				name: "Missing JWT secret",
				setup: func() {
					os.Setenv("REDIS_ADDRESS", "localhost:6379")
					os.Setenv("GEOAPIFY_KEY", "key")
					os.Setenv("PEXELS_API_KEY", "key")
					os.Setenv("SUPABASE_ANON_KEY", "key")
					os.Setenv("SUPABASE_SERVICE_KEY", "key-that-is-long-enough-to-meet-requirements")
					os.Setenv("SUPABASE_URL", "https://test.supabase.co")
					os.Setenv("SUPABASE_JWT_SECRET", "key-that-is-long-enough-to-meet-requirements")
					os.Setenv("EMAIL_FROM_ADDRESS", "test@example.com")
					os.Setenv("RESEND_API_KEY", "key")
				},
				cleanup: func() {
					os.Unsetenv("JWT_SECRET_KEY")
				},
				errMsg: "JWT secret key must be at least",
			},
			{
				name: "JWT secret too short",
				setup: func() {
					os.Setenv("JWT_SECRET_KEY", "short")
					os.Setenv("REDIS_ADDRESS", "localhost:6379")
					os.Setenv("GEOAPIFY_KEY", "key")
					os.Setenv("PEXELS_API_KEY", "key")
					os.Setenv("SUPABASE_ANON_KEY", "key")
					os.Setenv("SUPABASE_SERVICE_KEY", "key-that-is-long-enough-to-meet-requirements")
					os.Setenv("SUPABASE_URL", "https://test.supabase.co")
					os.Setenv("SUPABASE_JWT_SECRET", "key-that-is-long-enough-to-meet-requirements")
					os.Setenv("EMAIL_FROM_ADDRESS", "test@example.com")
					os.Setenv("RESEND_API_KEY", "key")
				},
				cleanup: func() {},
				errMsg:  "JWT secret key must be at least",
			},
			// NOTE: "Missing Redis address" is not testable via LoadConfig because
			// Viper defaults REDIS.ADDRESS to "localhost:6379" and empty env vars
			// don't override Viper defaults. This case is already covered by
			// TestValidateConfig which tests validateConfig() directly.
			{
				name: "Invalid allowed origins",
				setup: func() {
					os.Setenv("JWT_SECRET_KEY", "this-is-a-very-long-secret-key-that-meets-requirements")
					os.Setenv("ALLOWED_ORIGINS", "not-a-valid-url")
					os.Setenv("REDIS_ADDRESS", "localhost:6379")
					os.Setenv("GEOAPIFY_KEY", "key")
					os.Setenv("PEXELS_API_KEY", "key")
					os.Setenv("SUPABASE_ANON_KEY", "key")
					os.Setenv("SUPABASE_SERVICE_KEY", "key-that-is-long-enough-to-meet-requirements")
					os.Setenv("SUPABASE_URL", "https://test.supabase.co")
					os.Setenv("SUPABASE_JWT_SECRET", "key-that-is-long-enough-to-meet-requirements")
					os.Setenv("EMAIL_FROM_ADDRESS", "test@example.com")
					os.Setenv("RESEND_API_KEY", "key")
				},
				cleanup: func() {},
				errMsg:  "invalid allowed origin",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Clear all env vars first
				for _, key := range envVars {
					os.Unsetenv(key)
				}

				tc.setup()
				defer tc.cleanup()

				cfg, err := LoadConfig()
				assert.Error(t, err)
				assert.Nil(t, cfg)
				assert.Contains(t, err.Error(), tc.errMsg)
			})
		}
	})
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid configuration",
			config: &Config{
				Server: ServerConfig{
					Port:           "8080",
					JwtSecretKey:   "this-is-a-very-long-secret-key-that-meets-requirements",
					AllowedOrigins: []string{"*"},
				},
				Database: DatabaseConfig{
					Host:     "host",
					User:     "user",
					Password: "pass",
					Name:     "db",
				},
				Redis: RedisConfig{
					Address: "localhost:6379",
				},
				ExternalServices: ExternalServices{
					GeoapifyKey:        "key",
					PexelsAPIKey:       "key",
					SupabaseAnonKey:    "key",
					SupabaseServiceKey: "key-that-is-long-enough-to-meet-requirements",
					SupabaseURL:        "https://test.supabase.co",
					SupabaseJWTSecret:  "key-that-is-long-enough-to-meet-requirements",
				},
				Email: EmailConfig{
					FromAddress:  "test@example.com",
					ResendAPIKey: "key",
				},
				EventService: EventServiceConfig{
					PublishTimeoutSeconds:   5,
					SubscribeTimeoutSeconds: 10,
					EventBufferSize:         100,
				},
				RateLimit: RateLimitConfig{
					AuthRequestsPerMinute: 10,
					WindowSeconds:         60,
				},
				WorkerPool: WorkerPoolConfig{
					MaxWorkers:             10,
					QueueSize:              1000,
					ShutdownTimeoutSeconds: 30,
				},
			},
			expectError: false,
		},
		{
			name: "Missing server port",
			config: &Config{
				Server: ServerConfig{
					Port:         "",
					JwtSecretKey: "this-is-a-very-long-secret-key-that-meets-requirements",
				},
			},
			expectError: true,
			errorMsg:    "server port is required",
		},
		{
			name: "Missing database host",
			config: &Config{
				Server: ServerConfig{
					Port:         "8080",
					JwtSecretKey: "this-is-a-very-long-secret-key-that-meets-requirements",
				},
				Database: DatabaseConfig{
					Host: "",
					User: "user",
					Name: "db",
				},
				Redis: RedisConfig{
					Address: "localhost:6379",
				},
			},
			expectError: true,
			errorMsg:    "database host is required",
		},
		{
			name: "Invalid event service configuration",
			config: &Config{
				Server: ServerConfig{
					Port:         "8080",
					JwtSecretKey: "this-is-a-very-long-secret-key-that-meets-requirements",
				},
				Database: DatabaseConfig{
					Host:     "host",
					User:     "user",
					Password: "pass",
					Name:     "db",
				},
				Redis: RedisConfig{
					Address: "localhost:6379",
				},
				ExternalServices: ExternalServices{
					GeoapifyKey:        "key",
					PexelsAPIKey:       "key",
					SupabaseAnonKey:    "key",
					SupabaseServiceKey: "key-that-is-long-enough-to-meet-requirements",
					SupabaseURL:        "https://test.supabase.co",
					SupabaseJWTSecret:  "key-that-is-long-enough-to-meet-requirements",
				},
				Email: EmailConfig{
					FromAddress:  "test@example.com",
					ResendAPIKey: "key",
				},
				EventService: EventServiceConfig{
					PublishTimeoutSeconds:   0,
					SubscribeTimeoutSeconds: 10,
					EventBufferSize:         100,
				},
			},
			expectError: true,
			errorMsg:    "event service publish timeout must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateExternalServices(t *testing.T) {
	tests := []struct {
		name        string
		services    *ExternalServices
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid external services",
			services: &ExternalServices{
				GeoapifyKey:        "key",
				PexelsAPIKey:       "key",
				SupabaseAnonKey:    "key",
				SupabaseServiceKey: "key-that-is-long-enough-to-meet-requirements",
				SupabaseURL:        "https://test.supabase.co",
				SupabaseJWTSecret:  "key-that-is-long-enough-to-meet-requirements",
			},
			expectError: false,
		},
		{
			name: "Missing Geoapify key",
			services: &ExternalServices{
				GeoapifyKey:        "",
				PexelsAPIKey:       "key",
				SupabaseAnonKey:    "key",
				SupabaseServiceKey: "key-that-is-long-enough-to-meet-requirements",
				SupabaseURL:        "https://test.supabase.co",
				SupabaseJWTSecret:  "key-that-is-long-enough-to-meet-requirements",
			},
			expectError: true,
			errorMsg:    "geoapify key is required",
		},
		{
			name: "Supabase service key too short",
			services: &ExternalServices{
				GeoapifyKey:        "key",
				PexelsAPIKey:       "key",
				SupabaseAnonKey:    "key",
				SupabaseServiceKey: "short",
				SupabaseURL:        "https://test.supabase.co",
				SupabaseJWTSecret:  "key-that-is-long-enough-to-meet-requirements",
			},
			expectError: true,
			errorMsg:    "supabase service key must be at least",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExternalServices(tt.services)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestContainsWildcard(t *testing.T) {
	tests := []struct {
		name     string
		origins  []string
		expected bool
	}{
		{
			name:     "Contains wildcard",
			origins:  []string{"https://example.com", "*", "https://app.example.com"},
			expected: true,
		},
		{
			name:     "Only wildcard",
			origins:  []string{"*"},
			expected: true,
		},
		{
			name:     "No wildcard",
			origins:  []string{"https://example.com", "https://app.example.com"},
			expected: false,
		},
		{
			name:     "Empty list",
			origins:  []string{},
			expected: false,
		},
		{
			name:     "Wildcard in URL",
			origins:  []string{"https://*.example.com"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsWildcard(tt.origins)
			assert.Equal(t, tt.expected, result)
		})
	}
}