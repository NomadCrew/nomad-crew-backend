// Package config handles loading and validation of application configuration
// from environment variables and potentially configuration files.
package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/spf13/viper"
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
	GeoapifyKey       string `mapstructure:"GEOAPIFY_KEY"`
	PexelsAPIKey      string `mapstructure:"PEXELS_API_KEY"`
	SupabaseAnonKey   string `mapstructure:"SUPABASE_ANON_KEY"`
	SupabaseURL       string `mapstructure:"SUPABASE_URL"`
	SupabaseJWTSecret string `mapstructure:"SUPABASE_JWT_SECRET"`
	EmailFromAddress  string `mapstructure:"EMAIL_FROM_ADDRESS"`
	EmailFromName     string `mapstructure:"EMAIL_FROM_NAME"`
	EmailBaseURL      string `mapstructure:"EMAIL_BASE_URL" default:"https://api.mailchannels.net"`
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

// Config aggregates all application configuration sections.
type Config struct {
	Server           ServerConfig       `mapstructure:"SERVER" yaml:"server"`
	Database         DatabaseConfig     `mapstructure:"DATABASE" yaml:"database"`
	Redis            RedisConfig        `mapstructure:"REDIS" yaml:"redis"`
	Email            EmailConfig        `mapstructure:"EMAIL" yaml:"email"`
	ExternalServices ExternalServices   `mapstructure:"EXTERNAL_SERVICES" yaml:"external_services"`
	EventService     EventServiceConfig `mapstructure:"EVENT_SERVICE" yaml:"event_service"` // +++ Add EventService config field +++
}

// IsDevelopment returns true if the application is running in development environment.
func (c *Config) IsDevelopment() bool {
	return c.Server.Environment == EnvDevelopment
}

// IsProduction returns true if the application is running in production environment.
func (c *Config) IsProduction() bool {
	return c.Server.Environment == EnvProduction
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
	v.SetDefault("DATABASE.MAX_CONNECTIONS", 20)
	v.SetDefault("DATABASE.MAX_OPEN_CONNS", 5) // Conservative for free tier
	v.SetDefault("DATABASE.MAX_IDLE_CONNS", 2) // Conservative for free tier
	v.SetDefault("DATABASE.CONN_MAX_LIFE", "1h")
	v.SetDefault("DATABASE.HOST", "ep-blue-sun-a8kj1qdc-pooler.eastus2.azure.neon.tech")
	v.SetDefault("DATABASE.PORT", 5432)
	v.SetDefault("DATABASE.USER", "neondb_owner")
	v.SetDefault("DATABASE.NAME", "neondb")
	v.SetDefault("DATABASE.SSL_MODE", "require")
	v.SetDefault("REDIS.DB", 0)
	v.SetDefault("REDIS.ADDRESS", "actual-serval-57447.upstash.io:6379")
	v.SetDefault("REDIS.USE_TLS", false) // Only enable TLS for Upstash
	v.SetDefault("REDIS.POOL_SIZE", 3)   // Conservative for free tier
	v.SetDefault("REDIS.MIN_IDLE_CONNS", 1)
	v.SetDefault("LOG_LEVEL", "info")
	// +++ Set EventService defaults +++
	v.SetDefault("EVENT_SERVICE.PUBLISH_TIMEOUT_SECONDS", 5)
	v.SetDefault("EVENT_SERVICE.SUBSCRIBE_TIMEOUT_SECONDS", 10)
	v.SetDefault("EVENT_SERVICE.EVENT_BUFFER_SIZE", 100)

	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Bind environment variables
	if err := v.BindEnv("SERVER.ENVIRONMENT", "SERVER_ENVIRONMENT"); err != nil {
		return nil, fmt.Errorf("failed to bind SERVER.ENVIRONMENT: %w", err)
	}
	if err := v.BindEnv("SERVER.PORT", "PORT"); err != nil {
		return nil, fmt.Errorf("failed to bind SERVER.PORT: %w", err)
	}
	if err := v.BindEnv("SERVER.ALLOWED_ORIGINS", "ALLOWED_ORIGINS"); err != nil {
		return nil, fmt.Errorf("failed to bind SERVER.ALLOWED_ORIGINS: %w", err)
	}
	if err := v.BindEnv("SERVER.JWT_SECRET_KEY", "JWT_SECRET_KEY"); err != nil {
		return nil, fmt.Errorf("failed to bind SERVER.JWT_SECRET_KEY: %w", err)
	}

	if err := v.BindEnv("DATABASE.HOST", "DB_HOST"); err != nil {
		return nil, fmt.Errorf("failed to bind DATABASE.HOST: %w", err)
	}
	if err := v.BindEnv("DATABASE.PORT", "DB_PORT"); err != nil {
		return nil, fmt.Errorf("failed to bind DATABASE.PORT: %w", err)
	}
	if err := v.BindEnv("DATABASE.USER", "DB_USER"); err != nil {
		return nil, fmt.Errorf("failed to bind DATABASE.USER: %w", err)
	}
	if err := v.BindEnv("DATABASE.PASSWORD", "DB_PASSWORD"); err != nil {
		return nil, fmt.Errorf("failed to bind DATABASE.PASSWORD: %w", err)
	}
	if err := v.BindEnv("DATABASE.NAME", "DB_NAME"); err != nil {
		return nil, fmt.Errorf("failed to bind DATABASE.NAME: %w", err)
	}
	if err := v.BindEnv("DATABASE.SSL_MODE", "DB_SSL_MODE"); err != nil {
		return nil, fmt.Errorf("failed to bind DATABASE.SSL_MODE: %w", err)
	}

	if err := v.BindEnv("REDIS.ADDRESS", "REDIS_ADDRESS"); err != nil {
		return nil, fmt.Errorf("failed to bind REDIS.ADDRESS: %w", err)
	}
	if err := v.BindEnv("REDIS.PASSWORD", "REDIS_PASSWORD"); err != nil {
		return nil, fmt.Errorf("failed to bind REDIS.PASSWORD: %w", err)
	}
	if err := v.BindEnv("REDIS.DB", "REDIS_DB"); err != nil {
		return nil, fmt.Errorf("failed to bind REDIS.DB: %w", err)
	}
	if err := v.BindEnv("REDIS.USE_TLS", "REDIS_USE_TLS"); err != nil {
		return nil, fmt.Errorf("failed to bind REDIS.USE_TLS: %w", err)
	}

	if err := v.BindEnv("EXTERNAL_SERVICES.GEOAPIFY_KEY", "GEOAPIFY_KEY"); err != nil {
		return nil, fmt.Errorf("failed to bind EXTERNAL_SERVICES.GEOAPIFY_KEY: %w", err)
	}
	if err := v.BindEnv("EXTERNAL_SERVICES.PEXELS_API_KEY", "PEXELS_API_KEY"); err != nil {
		return nil, fmt.Errorf("failed to bind EXTERNAL_SERVICES.PEXELS_API_KEY: %w", err)
	}
	if err := v.BindEnv("EXTERNAL_SERVICES.SUPABASE_ANON_KEY", "SUPABASE_ANON_KEY"); err != nil {
		return nil, fmt.Errorf("failed to bind EXTERNAL_SERVICES.SUPABASE_ANON_KEY: %w", err)
	}
	if err := v.BindEnv("EXTERNAL_SERVICES.SUPABASE_JWT_SECRET", "SUPABASE_JWT_SECRET"); err != nil {
		return nil, fmt.Errorf("failed to bind EXTERNAL_SERVICES.SUPABASE_JWT_SECRET: %w", err)
	}
	if err := v.BindEnv("EXTERNAL_SERVICES.SUPABASE_URL", "SUPABASE_URL"); err != nil {
		return nil, fmt.Errorf("failed to bind EXTERNAL_SERVICES.SUPABASE_URL: %w", err)
	}

	if err := v.BindEnv("EMAIL.FROM_ADDRESS", "EMAIL_FROM_ADDRESS"); err != nil {
		return nil, fmt.Errorf("failed to bind EMAIL.FROM_ADDRESS: %w", err)
	}
	if err := v.BindEnv("EMAIL.FROM_NAME", "EMAIL_FROM_NAME"); err != nil {
		return nil, fmt.Errorf("failed to bind EMAIL.FROM_NAME: %w", err)
	}
	if err := v.BindEnv("EMAIL.RESEND_API_KEY", "RESEND_API_KEY"); err != nil {
		return nil, fmt.Errorf("failed to bind EMAIL.RESEND_API_KEY: %w", err)
	}

	// +++ Bind EventService environment variables +++
	if err := v.BindEnv("EVENT_SERVICE.PUBLISH_TIMEOUT_SECONDS", "EVENT_SERVICE_PUBLISH_TIMEOUT_SECONDS"); err != nil {
		return nil, fmt.Errorf("failed to bind EVENT_SERVICE.PUBLISH_TIMEOUT_SECONDS: %w", err)
	}
	if err := v.BindEnv("EVENT_SERVICE.SUBSCRIBE_TIMEOUT_SECONDS", "EVENT_SERVICE_SUBSCRIBE_TIMEOUT_SECONDS"); err != nil {
		return nil, fmt.Errorf("failed to bind EVENT_SERVICE.SUBSCRIBE_TIMEOUT_SECONDS: %w", err)
	}
	if err := v.BindEnv("EVENT_SERVICE.EVENT_BUFFER_SIZE", "EVENT_SERVICE_EVENT_BUFFER_SIZE"); err != nil {
		return nil, fmt.Errorf("failed to bind EVENT_SERVICE.EVENT_BUFFER_SIZE: %w", err)
	}

	resendAPIKey := v.GetString("EMAIL.RESEND_API_KEY")
	log.Infow("RESEND_API_KEY check",
		"present", resendAPIKey != "",
		"length", len(resendAPIKey),
		"first_chars", func() string {
			if len(resendAPIKey) > 3 {
				return resendAPIKey[:3] + "..."
			}
			return ""
		}())

	env := v.GetString("SERVER.ENVIRONMENT")
	log.Infow("Configuration loaded",
		"environment", env,
		"server_port", v.GetString("SERVER.PORT"),
		"db_host", v.GetString("DATABASE.HOST"),
		"allowed_origins", v.GetString("SERVER.ALLOWED_ORIGINS"),
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
	if services.SupabaseURL == "" {
		return fmt.Errorf("supabase URL is required")
	}
	if len(services.SupabaseJWTSecret) < minJWTLength {
		return fmt.Errorf("supabase JWT secret must be at least %d characters long", minJWTLength)
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
