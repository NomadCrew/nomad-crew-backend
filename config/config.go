package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/spf13/viper"
)

type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvProduction  Environment = "production"

	// Validation constants
	minJWTLength      = 32
	minPasswordLength = 16
	minKeyLength      = 8
)

type ServerConfig struct {
	Environment    Environment `mapstructure:"ENVIRONMENT" yaml:"environment"`
	Port           string      `mapstructure:"PORT" yaml:"port"`
	AllowedOrigins []string    `mapstructure:"ALLOWED_ORIGINS" yaml:"allowed_origins"`
	Version        string      `mapstructure:"VERSION" yaml:"version"`
	JwtSecretKey   string      `mapstructure:"JWT_SECRET_KEY" yaml:"jwt_secret_key"`
	FrontendURL    string      `mapstructure:"FRONTEND_URL" yaml:"frontend_url"`
}

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

type RedisConfig struct {
	Address      string `mapstructure:"ADDRESS" yaml:"address"`
	Password     string `mapstructure:"PASSWORD" yaml:"password"`
	DB           int    `mapstructure:"DB" yaml:"db"`
	UseTLS       bool   `mapstructure:"USE_TLS" yaml:"use_tls"`
	PoolSize     int    `mapstructure:"POOL_SIZE" yaml:"pool_size"`
	MinIdleConns int    `mapstructure:"MIN_IDLE_CONNS" yaml:"min_idle_conns"`
}

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

type EmailConfig struct {
	FromAddress  string `mapstructure:"FROM_ADDRESS" yaml:"from_address"`
	FromName     string `mapstructure:"FROM_NAME" yaml:"from_name"`
	BaseURL      string `mapstructure:"BASE_URL" yaml:"base_url"`
	ResendAPIKey string `mapstructure:"RESEND_API_KEY" yaml:"resend_api_key"`
}

type Config struct {
	Server           ServerConfig     `mapstructure:"SERVER" yaml:"server"`
	Database         DatabaseConfig   `mapstructure:"DATABASE" yaml:"database"`
	Redis            RedisConfig      `mapstructure:"REDIS" yaml:"redis"`
	Email            EmailConfig      `mapstructure:"EMAIL" yaml:"email"`
	ExternalServices ExternalServices `mapstructure:"EXTERNAL_SERVICES" yaml:"external_services"`
}

func (c *Config) IsDevelopment() bool {
	return c.Server.Environment == EnvDevelopment
}

func (c *Config) IsProduction() bool {
	return c.Server.Environment == EnvProduction
}

func LoadConfig() (*Config, error) {
	v := viper.New()
	log := logger.GetLogger()

	// Set defaults
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
	v.SetDefault("REDIS.USE_TLS", true) // Upstash requires TLS
	v.SetDefault("REDIS.POOL_SIZE", 3)  // Conservative for free tier
	v.SetDefault("REDIS.MIN_IDLE_CONNS", 1)
	v.SetDefault("LOG_LEVEL", "info")

	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Server config bindings
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

	// Database config bindings
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
	if err := v.BindEnv("DATABASE.MAX_OPEN_CONNS", "DB_MAX_OPEN_CONNS"); err != nil {
		return nil, fmt.Errorf("failed to bind DATABASE.MAX_OPEN_CONNS: %w", err)
	}
	if err := v.BindEnv("DATABASE.MAX_IDLE_CONNS", "DB_MAX_IDLE_CONNS"); err != nil {
		return nil, fmt.Errorf("failed to bind DATABASE.MAX_IDLE_CONNS: %w", err)
	}
	if err := v.BindEnv("DATABASE.CONN_MAX_LIFE", "DB_CONN_MAX_LIFE"); err != nil {
		return nil, fmt.Errorf("failed to bind DATABASE.CONN_MAX_LIFE: %w", err)
	}

	// Redis config bindings
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
	if err := v.BindEnv("REDIS.POOL_SIZE", "REDIS_POOL_SIZE"); err != nil {
		return nil, fmt.Errorf("failed to bind REDIS.POOL_SIZE: %w", err)
	}
	if err := v.BindEnv("REDIS.MIN_IDLE_CONNS", "REDIS_MIN_IDLE_CONNS"); err != nil {
		return nil, fmt.Errorf("failed to bind REDIS.MIN_IDLE_CONNS: %w", err)
	}

	// External services bindings
	if err := v.BindEnv("EXTERNAL_SERVICES.GEOAPIFY_KEY", "GEOAPIFY_KEY"); err != nil {
		return nil, fmt.Errorf("failed to bind EXTERNAL_SERVICES.GEOAPIFY_KEY: %w", err)
	}
	if err := v.BindEnv("EXTERNAL_SERVICES.PEXELS_API_KEY", "PEXELS_API_KEY"); err != nil {
		return nil, fmt.Errorf("failed to bind EXTERNAL_SERVICES.PEXELS_API_KEY: %w", err)
	}
	if err := v.BindEnv("EXTERNAL_SERVICES.SUPABASE_ANON_KEY", "SUPABASE_ANON_KEY"); err != nil {
		return nil, fmt.Errorf("failed to bind EXTERNAL_SERVICES.SUPABASE_ANON_KEY: %w", err)
	}

	if err := v.BindEnv("EXTERNAL_SERVICES.EMAIL_FROM_ADDRESS", "EMAIL_FROM_ADDRESS"); err != nil {
		return nil, fmt.Errorf("failed to bind EMAIL_FROM_ADDRESS: %w", err)
	}
	if err := v.BindEnv("EXTERNAL_SERVICES.EMAIL_FROM_NAME", "EMAIL_FROM_NAME"); err != nil {
		return nil, fmt.Errorf("failed to bind EMAIL_FROM_NAME: %w", err)
	}
	if err := v.BindEnv("EXTERNAL_SERVICES.EMAIL_BASE_URL", "EMAIL_BASE_URL"); err != nil {
		return nil, fmt.Errorf("failed to bind EMAIL_BASE_URL: %w", err)
	}

	// Add debug logging
	log.Infof("Environment variables loaded: %+v", map[string]interface{}{
		"SERVER_PORT":     v.GetString("SERVER.PORT"),
		"SERVER_ENV":      v.GetString("SERVER.ENVIRONMENT"),
		"DB_HOST":         v.GetString("DATABASE.HOST"),
		"ALLOWED_ORIGINS": v.GetString("SERVER.ALLOWED_ORIGINS"),
	})

	// Try to read config file based on environment
	env := v.GetString("SERVER.ENVIRONMENT")
	v.SetConfigName("config." + strings.ToLower(env))
	v.AddConfigPath("./config")
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
		log.Info("No config file found, using environment variables and defaults")
	}

	// Load local secrets in development only
	if env == string(EnvDevelopment) {
		if err := loadLocalSecrets(v); err != nil {
			return nil, fmt.Errorf("failed to load local secrets: %w", err)
		}
	} else {
		log.Info("Running in non-development environment, secrets expected from environment variables")
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config unmarshal failed: %w", err)
	}

	// Validate configuration
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	// Email config bindings
	if err := v.BindEnv("EMAIL.FROM_ADDRESS", "EMAIL_FROM_ADDRESS"); err != nil {
		return nil, fmt.Errorf("failed to bind EMAIL.FROM_ADDRESS: %w", err)
	}
	if err := v.BindEnv("EMAIL.FROM_NAME", "EMAIL_FROM_NAME"); err != nil {
		return nil, fmt.Errorf("failed to bind EMAIL.FROM_NAME: %w", err)
	}
	if err := v.BindEnv("EMAIL.RESEND_API_KEY", "RESEND_API_KEY"); err != nil {
		return nil, fmt.Errorf("failed to bind EMAIL.RESEND_API_KEY: %w", err)
	}

	if len(cfg.Email.ResendAPIKey) < minKeyLength {
		return nil, fmt.Errorf("RESEND_API_KEY is invalid or too short")
	}
	if cfg.Email.FromAddress == "" {
		return nil, fmt.Errorf("FROM_ADDRESS is required")
	}

	return &cfg, nil
}

func loadLocalSecrets(v *viper.Viper) error {
	log := logger.GetLogger()

	// Get secrets path from environment variable
	secretsPath := os.Getenv("APP_SECRETS_PATH")
	if secretsPath == "" {
		log.Info("APP_SECRETS_PATH not set, skipping local secrets loading")
		return nil
	}

	// Check if file exists
	if _, err := os.Stat(secretsPath); os.IsNotExist(err) {
		// If file doesn't exist, just log and continue
		log.Warnf("Secrets file not found at %s, continuing with environment variables", secretsPath)
		return nil
	}

	// Read secrets file
	secretsData, err := os.ReadFile(secretsPath)
	if err != nil {
		return fmt.Errorf("failed to read secrets file: %w", err)
	}

	// Parse secrets JSON
	var secrets map[string]string
	if err := json.Unmarshal(secretsData, &secrets); err != nil {
		return fmt.Errorf("failed to parse secrets: %w", err)
	}

	// Set secrets in Viper
	for key, value := range secrets {
		v.Set(key, value)
	}

	return nil
}

func validateConfig(cfg *Config) error {
	log := logger.GetLogger()

	// Validate server config
	if cfg.Server.JwtSecretKey == "" {
		return fmt.Errorf("JWT_SECRET_KEY is required")
	}

	if len(cfg.Server.JwtSecretKey) < minJWTLength {
		log.Warn("JWT_SECRET_KEY is shorter than recommended length")
	}

	// Validate database config
	if cfg.Database.Host == "" {
		return fmt.Errorf("DATABASE.HOST is required")
	}

	if cfg.Database.User == "" {
		return fmt.Errorf("DATABASE.USER is required")
	}

	if cfg.Database.Password == "" {
		return fmt.Errorf("DATABASE.PASSWORD is required")
	}

	if cfg.Database.Name == "" {
		return fmt.Errorf("DATABASE.NAME is required")
	}

	// Log only non-sensitive configuration information
	log.Infow("Configuration validated",
		"environment", cfg.Server.Environment,
		"database_host", cfg.Database.Host,
		"database_name", cfg.Database.Name,
		"redis_address", cfg.Redis.Address)

	return nil
}

// nolint:unused
func validateConnectionString(connStr string) error {
	if connStr == "" {
		return fmt.Errorf("DB_CONNECTION_STRING is required")
	}

	u, err := url.Parse(connStr)
	if err != nil {
		return fmt.Errorf("invalid DB_CONNECTION_STRING format: %w", err)
	}

	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return fmt.Errorf("DB_CONNECTION_STRING must use postgres:// or postgresql:// scheme")
	}

	if u.User == nil || u.User.Username() == "" {
		return fmt.Errorf("DB_CONNECTION_STRING must contain username")
	}

	_, hasPassword := u.User.Password()
	if !hasPassword {
		return fmt.Errorf("DB_CONNECTION_STRING must contain password")
	}

	return nil
}

// nolint:unused
func validateExternalServices(services *ExternalServices) error {
	if len(services.SupabaseAnonKey) < minKeyLength {
		return fmt.Errorf("SUPABASE_ANON_KEY is invalid or too short")
	}

	if len(services.GeoapifyKey) < minKeyLength {
		return fmt.Errorf("GEOAPIFY_KEY is invalid or too short")
	}

	if len(services.PexelsAPIKey) < minKeyLength {
		return fmt.Errorf("PEXELS_API_KEY is invalid or too short")
	}

	return nil
}

// nolint:unused
func containsWildcard(origins []string) bool {
	for _, o := range origins {
		if o == "*" {
			return true
		}
	}
	return false
}
