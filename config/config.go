// Package config handles loading and validation of application configuration
// from environment variables and potentially configuration files.
package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Environment represents the application's running environment (development or production).
type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvProduction  Environment = "production"

	// Validation constants
	minJWTLength      = 32
	minPasswordLength = 16
	minKeyLength      = 8
)

// ServerConfig holds server-specific configuration.
type ServerConfig struct {
	Environment    Environment `mapstructure:"ENVIRONMENT" yaml:"environment"`
	Port           string      `mapstructure:"PORT" yaml:"port"`
	AllowedOrigins []string    `mapstructure:"ALLOWED_ORIGINS" yaml:"allowed_origins"`
	Version        string      `mapstructure:"VERSION" yaml:"version"`
	JwtSecretKey   string      `mapstructure:"JWT_SECRET_KEY" yaml:"jwt_secret_key"`
	FrontendURL    string      `mapstructure:"FRONTEND_URL" yaml:"frontend_url"`
	// TrustedProxies is a list of CIDR ranges or IPs of trusted reverse proxies.
	// If empty, X-Forwarded-For headers are ignored entirely (safe default).
	// Examples: ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"]
	TrustedProxies []string `mapstructure:"TRUSTED_PROXIES" yaml:"trusted_proxies"`
}

// DatabaseConfig holds PostgreSQL database connection details.
type DatabaseConfig struct {
	Host           string `mapstructure:"HOST" yaml:"host"`
	Port           int    `mapstructure:"PORT" yaml:"port"`
	User           string `mapstructure:"USER" yaml:"user"`
	Password       string `mapstructure:"PASSWORD" yaml:"password"`
	Name           string `mapstructure:"NAME" yaml:"name"`
	MaxConnections int    `mapstructure:"MAX_CONNECTIONS" yaml:"max_connections"`
	SSLMode        string `mapstructure:"SSL_MODE" yaml:"ssl_mode"`
	MaxOpenConns   int    `mapstructure:"MAX_OPEN_CONNS" yaml:"max_open_conns"`
	MaxIdleConns   int    `mapstructure:"MAX_IDLE_CONNS" yaml:"max_idle_conns"`
	ConnMaxLife    string `mapstructure:"CONN_MAX_LIFE" yaml:"conn_max_life"`
}

// URL returns a postgres:// connection URL suitable for golang-migrate and other
// URL-based database tools.
func (c *DatabaseConfig) URL() string {
	sslmode := c.SSLMode
	if sslmode == "" {
		sslmode = "disable"
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		url.QueryEscape(c.User),
		url.QueryEscape(c.Password),
		c.Host,
		c.Port,
		c.Name,
		sslmode,
	)
}

// RedisConfig holds Redis connection details.
type RedisConfig struct {
	Address      string `mapstructure:"ADDRESS" yaml:"address"`
	Password     string `mapstructure:"PASSWORD" yaml:"password"`
	DB           int    `mapstructure:"DB" yaml:"db"`
	UseTLS       bool   `mapstructure:"USE_TLS" yaml:"use_tls"`
	PoolSize     int    `mapstructure:"POOL_SIZE" yaml:"pool_size"`
	MinIdleConns int    `mapstructure:"MIN_IDLE_CONNS" yaml:"min_idle_conns"`
}

// ExternalServices holds API keys and URLs for external services.
type ExternalServices struct {
	GeoapifyKey        string `mapstructure:"GEOAPIFY_KEY"`
	PexelsAPIKey       string `mapstructure:"PEXELS_API_KEY"`
	SupabaseAnonKey    string `mapstructure:"SUPABASE_ANON_KEY"`
	SupabaseServiceKey string `mapstructure:"SUPABASE_SERVICE_KEY"`
	SupabaseURL        string `mapstructure:"SUPABASE_URL"`
	SupabaseJWTSecret  string `mapstructure:"SUPABASE_JWT_SECRET"`
	EmailFromAddress   string `mapstructure:"EMAIL_FROM_ADDRESS"`
	EmailFromName      string `mapstructure:"EMAIL_FROM_NAME"`
	EmailBaseURL       string `mapstructure:"EMAIL_BASE_URL" default:"https://api.mailchannels.net"`
}

// EmailConfig holds configuration for sending emails.
type EmailConfig struct {
	FromAddress  string `mapstructure:"FROM_ADDRESS" yaml:"from_address"`
	FromName     string `mapstructure:"FROM_NAME" yaml:"from_name"`
	BaseURL      string `mapstructure:"BASE_URL" yaml:"base_url"`
	ResendAPIKey string `mapstructure:"RESEND_API_KEY" yaml:"resend_api_key"`
}

// EventServiceConfig holds configuration for the Redis-based event service.
type EventServiceConfig struct {
	// Timeout for publishing a single event to Redis (in seconds)
	PublishTimeoutSeconds int `mapstructure:"PUBLISH_TIMEOUT_SECONDS" yaml:"publish_timeout_seconds"`
	// Timeout for establishing a subscription connection via Redis (in seconds)
	SubscribeTimeoutSeconds int `mapstructure:"SUBSCRIBE_TIMEOUT_SECONDS" yaml:"subscribe_timeout_seconds"`
	// Buffer size for the channel delivering events to a single subscriber
	EventBufferSize int `mapstructure:"EVENT_BUFFER_SIZE" yaml:"event_buffer_size"`
}

// RateLimitConfig holds configuration for rate limiting.
type RateLimitConfig struct {
	// Maximum requests per minute for auth endpoints (login, register, password reset)
	AuthRequestsPerMinute int `mapstructure:"AUTH_REQUESTS_PER_MINUTE" yaml:"auth_requests_per_minute"`
	// Window duration in seconds for rate limiting
	WindowSeconds int `mapstructure:"WINDOW_SECONDS" yaml:"window_seconds"`
}

// NotificationConfig holds configuration for the external notification facade API.
type NotificationConfig struct {
	// Enabled indicates whether the notification service is enabled
	Enabled bool `mapstructure:"ENABLED" yaml:"enabled"`
	// APIUrl is the URL of the notification facade API
	APIUrl string `mapstructure:"API_URL" yaml:"api_url"`
	// APIKey is the API key for authenticating with the notification facade
	APIKey string `mapstructure:"API_KEY" yaml:"api_key"`
	// TimeoutSeconds is the HTTP client timeout for notification requests
	TimeoutSeconds int `mapstructure:"TIMEOUT_SECONDS" yaml:"timeout_seconds"`
}

// WorkerPoolConfig holds configuration for the notification worker pool.
type WorkerPoolConfig struct {
	// MaxWorkers is the number of concurrent workers (default: 10)
	MaxWorkers int `mapstructure:"MAX_WORKERS" yaml:"max_workers"`
	// QueueSize is the maximum number of pending jobs (default: 1000)
	QueueSize int `mapstructure:"QUEUE_SIZE" yaml:"queue_size"`
	// ShutdownTimeoutSeconds is the max time to wait for workers during shutdown (default: 30)
	ShutdownTimeoutSeconds int `mapstructure:"SHUTDOWN_TIMEOUT_SECONDS" yaml:"shutdown_timeout_seconds"`
}

// Config aggregates all application configuration sections.
type Config struct {
	Server           ServerConfig       `mapstructure:"SERVER" yaml:"server"`
	Database         DatabaseConfig     `mapstructure:"DATABASE" yaml:"database"`
	Redis            RedisConfig        `mapstructure:"REDIS" yaml:"redis"`
	Email            EmailConfig        `mapstructure:"EMAIL" yaml:"email"`
	ExternalServices ExternalServices   `mapstructure:"EXTERNAL_SERVICES" yaml:"external_services"`
	EventService     EventServiceConfig `mapstructure:"EVENT_SERVICE" yaml:"event_service"`
	RateLimit        RateLimitConfig    `mapstructure:"RATE_LIMIT" yaml:"rate_limit"`
	Notification     NotificationConfig `mapstructure:"NOTIFICATION" yaml:"notification"`
	WorkerPool       WorkerPoolConfig   `mapstructure:"WORKER_POOL" yaml:"worker_pool"`
}

// IsDevelopment returns true if the application is running in development environment.
func (c *Config) IsDevelopment() bool {
	return c.Server.Environment == EnvDevelopment
}

// IsProduction returns true if the application is running in production environment.
func (c *Config) IsProduction() bool {
	return c.Server.Environment == EnvProduction
}

// bindEnvVars binds multiple environment variables to config keys.
// Format: []{configKey, envVar}
func bindEnvVars(v *viper.Viper, bindings [][2]string) error {
	for _, b := range bindings {
		if err := v.BindEnv(b[0], b[1]); err != nil {
			return fmt.Errorf("failed to bind %s: %w", b[0], err)
		}
	}
	return nil
}

// LoadConfig loads configuration from environment variables using Viper,
// sets default values, binds environment variables to config struct fields,
// unmarshals the configuration, and validates it.
func LoadConfig() (*Config, error) {
	v := viper.New()
	log := logger.GetLogger()

	v.SetDefault("SERVER.ENVIRONMENT", EnvDevelopment)
	v.SetDefault("SERVER.PORT", "8080")
	v.SetDefault("SERVER.ALLOWED_ORIGINS", []string{"*"})
	v.SetDefault("SERVER.TRUSTED_PROXIES", []string{}) // Empty = trust no one (safe default)
	v.SetDefault("DATABASE.MAX_CONNECTIONS", 20)
	v.SetDefault("DATABASE.MAX_OPEN_CONNS", 5) // Conservative for free tier
	v.SetDefault("DATABASE.MAX_IDLE_CONNS", 2) // Conservative for free tier
	v.SetDefault("DATABASE.CONN_MAX_LIFE", "1h")
	v.SetDefault("DATABASE.HOST", "localhost")
	v.SetDefault("DATABASE.PORT", 5432)
	v.SetDefault("DATABASE.USER", "postgres")
	v.SetDefault("DATABASE.PASSWORD", "")
	v.SetDefault("DATABASE.NAME", "nomadcrew_dev")
	v.SetDefault("DATABASE.SSL_MODE", "disable")
	v.SetDefault("REDIS.DB", 0)
	v.SetDefault("REDIS.ADDRESS", "localhost:6379")
	v.SetDefault("REDIS.PASSWORD", "")
	v.SetDefault("REDIS.USE_TLS", false) // Only enable TLS for Upstash
	v.SetDefault("REDIS.POOL_SIZE", 3)   // Conservative for free tier
	v.SetDefault("REDIS.MIN_IDLE_CONNS", 1)
	v.SetDefault("LOG_LEVEL", "info")
	// +++ Set EventService defaults +++
	v.SetDefault("EVENT_SERVICE.PUBLISH_TIMEOUT_SECONDS", 5)
	v.SetDefault("EVENT_SERVICE.SUBSCRIBE_TIMEOUT_SECONDS", 10)
	v.SetDefault("EVENT_SERVICE.EVENT_BUFFER_SIZE", 100)
	// +++ Set RateLimit defaults +++
	v.SetDefault("RATE_LIMIT.AUTH_REQUESTS_PER_MINUTE", 10)
	v.SetDefault("RATE_LIMIT.WINDOW_SECONDS", 60)
	// +++ Set Notification defaults +++
	v.SetDefault("NOTIFICATION.ENABLED", false)
	v.SetDefault("NOTIFICATION.API_URL", "https://ilqhxd37y4.execute-api.us-east-1.amazonaws.com/dev/notify")
	v.SetDefault("NOTIFICATION.API_KEY", "")
	v.SetDefault("NOTIFICATION.TIMEOUT_SECONDS", 10)
	// +++ Set WorkerPool defaults +++
	v.SetDefault("WORKER_POOL.MAX_WORKERS", 10)
	v.SetDefault("WORKER_POOL.QUEUE_SIZE", 1000)
	v.SetDefault("WORKER_POOL.SHUTDOWN_TIMEOUT_SECONDS", 30)

	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Bind environment variables
	envBindings := [][2]string{
		// Server config
		{"SERVER.ENVIRONMENT", "SERVER_ENVIRONMENT"},
		{"SERVER.PORT", "PORT"},
		{"SERVER.ALLOWED_ORIGINS", "ALLOWED_ORIGINS"},
		{"SERVER.JWT_SECRET_KEY", "JWT_SECRET_KEY"},
		{"SERVER.TRUSTED_PROXIES", "TRUSTED_PROXIES"},
		// Database config
		{"DATABASE.HOST", "DB_HOST"},
		{"DATABASE.PORT", "DB_PORT"},
		{"DATABASE.USER", "DB_USER"},
		{"DATABASE.PASSWORD", "DB_PASSWORD"},
		{"DATABASE.NAME", "DB_NAME"},
		{"DATABASE.SSL_MODE", "DB_SSL_MODE"},
		// Redis config
		{"REDIS.ADDRESS", "REDIS_ADDRESS"},
		{"REDIS.PASSWORD", "REDIS_PASSWORD"},
		{"REDIS.DB", "REDIS_DB"},
		{"REDIS.USE_TLS", "REDIS_USE_TLS"},
		// External services
		{"EXTERNAL_SERVICES.GEOAPIFY_KEY", "GEOAPIFY_KEY"},
		{"EXTERNAL_SERVICES.PEXELS_API_KEY", "PEXELS_API_KEY"},
		{"EXTERNAL_SERVICES.SUPABASE_ANON_KEY", "SUPABASE_ANON_KEY"},
		{"EXTERNAL_SERVICES.SUPABASE_SERVICE_KEY", "SUPABASE_SERVICE_KEY"},
		{"EXTERNAL_SERVICES.SUPABASE_URL", "SUPABASE_URL"},
		{"EXTERNAL_SERVICES.SUPABASE_JWT_SECRET", "SUPABASE_JWT_SECRET"},
		// Email config
		{"EMAIL.FROM_ADDRESS", "EMAIL_FROM_ADDRESS"},
		{"EMAIL.FROM_NAME", "EMAIL_FROM_NAME"},
		{"EMAIL.RESEND_API_KEY", "RESEND_API_KEY"},
		// Event service config
		{"EVENT_SERVICE.PUBLISH_TIMEOUT_SECONDS", "EVENT_SERVICE_PUBLISH_TIMEOUT_SECONDS"},
		{"EVENT_SERVICE.SUBSCRIBE_TIMEOUT_SECONDS", "EVENT_SERVICE_SUBSCRIBE_TIMEOUT_SECONDS"},
		{"EVENT_SERVICE.EVENT_BUFFER_SIZE", "EVENT_SERVICE_EVENT_BUFFER_SIZE"},
		// Rate limit config
		{"RATE_LIMIT.AUTH_REQUESTS_PER_MINUTE", "RATE_LIMIT_AUTH_REQUESTS_PER_MINUTE"},
		{"RATE_LIMIT.WINDOW_SECONDS", "RATE_LIMIT_WINDOW_SECONDS"},
		// Notification config
		{"NOTIFICATION.ENABLED", "NOTIFICATION_ENABLED"},
		{"NOTIFICATION.API_URL", "NOTIFICATION_API_URL"},
		{"NOTIFICATION.API_KEY", "NOTIFICATION_API_KEY"},
		{"NOTIFICATION.TIMEOUT_SECONDS", "NOTIFICATION_TIMEOUT_SECONDS"},
		// WorkerPool config
		{"WORKER_POOL.MAX_WORKERS", "WORKER_POOL_MAX_WORKERS"},
		{"WORKER_POOL.QUEUE_SIZE", "WORKER_POOL_QUEUE_SIZE"},
		{"WORKER_POOL.SHUTDOWN_TIMEOUT_SECONDS", "WORKER_POOL_SHUTDOWN_TIMEOUT_SECONDS"},
	}

	if err := bindEnvVars(v, envBindings); err != nil {
		return nil, err
	}

	env := v.GetString("SERVER.ENVIRONMENT")
	log.Infow("Configuration loaded",
		"environment", env,
		"server_port", v.GetString("SERVER.PORT"),
		"db_host", v.GetString("DATABASE.HOST"),
		"allowed_origins", v.GetString("SERVER.ALLOWED_ORIGINS"),
		"trusted_proxies", v.GetStringSlice("SERVER.TRUSTED_PROXIES"),
		// +++ Log EventService config +++
		"event_service_publish_timeout", v.GetInt("EVENT_SERVICE.PUBLISH_TIMEOUT_SECONDS"),
		"event_service_subscribe_timeout", v.GetInt("EVENT_SERVICE.SUBSCRIBE_TIMEOUT_SECONDS"),
		"event_service_buffer_size", v.GetInt("EVENT_SERVICE.EVENT_BUFFER_SIZE"),
	)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config unmarshal failed: %w", err)
	}

	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	log.Info("Configuration validated successfully")
	return &cfg, nil
}

// validateConfig checks if the loaded configuration values are valid.
func validateConfig(cfg *Config) error {
	log := logger.GetLogger()

	// Validate Server Config
	if cfg.Server.Port == "" {
		return fmt.Errorf("server port is required")
	}
	if len(cfg.Server.JwtSecretKey) < minJWTLength {
		return fmt.Errorf("JWT secret key must be at least %d characters long", minJWTLength)
	}
	// Validate AllowedOrigins format if not wildcard
	if !containsWildcard(cfg.Server.AllowedOrigins) {
		for _, origin := range cfg.Server.AllowedOrigins {
			if _, err := url.ParseRequestURI(origin); err != nil {
				return fmt.Errorf("invalid allowed origin '%s': %w", origin, err)
			}
		}
	}

	// Validate Database Config
	if cfg.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if cfg.Database.User == "" {
		return fmt.Errorf("database user is required")
	}
	if cfg.Database.Password == "" {
		log.Warn("Database password is not set. Ensure this is intended (e.g., using trusted auth).")
	}
	if cfg.Database.Name == "" {
		return fmt.Errorf("database name is required")
	}

	// Validate Redis Config
	if cfg.Redis.Address == "" {
		return fmt.Errorf("redis address is required")
	}
	if cfg.Redis.Password == "" && cfg.Redis.UseTLS {
		// Upstash typically requires a password with TLS
		log.Warn("Redis password is not set, but TLS is enabled. Ensure this is correct for your Redis provider.")
	}

	// Validate External Services
	if err := validateExternalServices(&cfg.ExternalServices); err != nil {
		return err
	}

	// Validate Email Config
	if cfg.Email.FromAddress == "" {
		return fmt.Errorf("email from address is required")
	}
	if cfg.Email.ResendAPIKey == "" {
		return fmt.Errorf("resend API key is required")
	}

	// +++ Validate EventService config +++
	if cfg.EventService.PublishTimeoutSeconds <= 0 {
		return fmt.Errorf("event service publish timeout must be positive")
	}
	if cfg.EventService.SubscribeTimeoutSeconds <= 0 {
		return fmt.Errorf("event service subscribe timeout must be positive")
	}
	if cfg.EventService.EventBufferSize <= 0 {
		return fmt.Errorf("event service buffer size must be positive")
	}

	// +++ Validate RateLimit config +++
	if cfg.RateLimit.AuthRequestsPerMinute <= 0 {
		return fmt.Errorf("rate limit auth requests per minute must be positive")
	}
	if cfg.RateLimit.WindowSeconds <= 0 {
		return fmt.Errorf("rate limit window seconds must be positive")
	}

	// +++ Validate Notification config +++
	if err := validateNotificationConfig(&cfg.Notification, log); err != nil {
		return err
	}

	// +++ Validate WorkerPool config +++
	if cfg.WorkerPool.MaxWorkers <= 0 {
		return fmt.Errorf("worker pool max workers must be positive")
	}
	if cfg.WorkerPool.QueueSize <= 0 {
		return fmt.Errorf("worker pool queue size must be positive")
	}
	if cfg.WorkerPool.ShutdownTimeoutSeconds <= 0 {
		return fmt.Errorf("worker pool shutdown timeout must be positive")
	}

	return nil
}

// validateExternalServices checks the configuration for external services.
func validateExternalServices(services *ExternalServices) error {
	if services.GeoapifyKey == "" {
		return fmt.Errorf("geoapify key is required")
	}
	if services.PexelsAPIKey == "" {
		return fmt.Errorf("pexels API key is required")
	}
	if services.SupabaseAnonKey == "" {
		return fmt.Errorf("supabase anon key is required")
	}
	if services.SupabaseServiceKey == "" {
		return fmt.Errorf("supabase service key is required")
	}
	if len(services.SupabaseServiceKey) < minJWTLength {
		return fmt.Errorf("supabase service key must be at least %d characters long", minJWTLength)
	}
	if services.SupabaseURL == "" {
		return fmt.Errorf("supabase URL is required")
	}
	if len(services.SupabaseJWTSecret) < minJWTLength {
		return fmt.Errorf("supabase JWT secret must be at least %d characters long", minJWTLength)
	}
	return nil
}

// validateNotificationConfig validates the notification facade configuration.
// If enabled but missing API key, it auto-disables the service with a warning.
func validateNotificationConfig(cfg *NotificationConfig, log *zap.SugaredLogger) error {
	// If notifications are disabled, no further validation needed
	if !cfg.Enabled {
		return nil
	}

	// Validate API URL format
	if cfg.APIUrl != "" {
		if _, err := url.ParseRequestURI(cfg.APIUrl); err != nil {
			return fmt.Errorf("invalid notification API URL: %w", err)
		}
	}

	// If enabled but no API key, auto-disable with warning
	if cfg.APIKey == "" {
		log.Warn("Notification API key not set, auto-disabling notification service")
		cfg.Enabled = false
		return nil
	}

	// Validate timeout
	if cfg.TimeoutSeconds <= 0 {
		return fmt.Errorf("notification timeout must be positive")
	}

	return nil
}

// containsWildcard checks if the list of allowed origins contains the wildcard "*".
func containsWildcard(origins []string) bool {
	for _, origin := range origins {
		if origin == "*" {
			return true
		}
	}
	return false
}
