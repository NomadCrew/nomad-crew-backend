package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
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
}

type DatabaseConfig struct {
	Host           string `mapstructure:"HOST" yaml:"host"`
	Port           int    `mapstructure:"PORT" yaml:"port"`
	User           string `mapstructure:"USER" yaml:"user"`
	Password       string `mapstructure:"PASSWORD" yaml:"password"`
	Name           string `mapstructure:"NAME" yaml:"name"`
	MaxConnections int    `mapstructure:"MAX_CONNECTIONS" yaml:"max_connections"`
	SSLMode        string `mapstructure:"SSL_MODE" yaml:"ssl_mode"`
}

type RedisConfig struct {
	Address  string `mapstructure:"ADDRESS" yaml:"address"`
	Password string `mapstructure:"PASSWORD" yaml:"password"`
	DB       int    `mapstructure:"DB" yaml:"db"`
}

type ExternalServices struct {
	GeoapifyKey     string `mapstructure:"GEOAPIFY_KEY"`
	PexelsAPIKey    string `mapstructure:"PEXELS_API_KEY"`
	SupabaseAnonKey string `mapstructure:"SUPABASE_ANON_KEY"`
}

type Config struct {
	Server           ServerConfig     `mapstructure:"SERVER" yaml:"server"`
	Database         DatabaseConfig   `mapstructure:"DATABASE" yaml:"database"`
	Redis            RedisConfig      `mapstructure:"REDIS" yaml:"redis"`
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
	v.SetDefault("REDIS.DB", 0)

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

	// Add debug logging
	log.Infof("Environment variables loaded: %+v", map[string]interface{}{
		"SERVER_PORT":     v.GetString("SERVER.PORT"),
		"SERVER_ENV":      v.GetString("SERVER.ENVIRONMENT"),
		"DB_HOST":         v.GetString("DATABASE.HOST"),
		"ALLOWED_ORIGINS": v.GetString("SERVER.ALLOWED_ORIGINS"),
	})

	// Try to read config file based on environment
	env := v.GetString("ENVIRONMENT")
	v.SetConfigName("config." + env)
	v.AddConfigPath(".")
	v.AddConfigPath("/etc/nomadcrew/")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
		log.Info("No config file found, using environment variables and defaults")
	}

	// Load AWS secrets in production
	if env == string(EnvProduction) {
		if err := loadAWSSecrets(v); err != nil {
			return nil, fmt.Errorf("failed to load AWS secrets: %w", err)
		}
	}

	// Add AWS secrets path binding
	if err := v.BindEnv("AWS_SECRETS_PATH", "AWS_SECRETS_PATH"); err != nil {
		return nil, fmt.Errorf("failed to bind AWS_SECRETS_PATH: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config unmarshal failed: %w", err)
	}

	// Validate configuration
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func loadAWSSecrets(v *viper.Viper) error {
	ctx := context.Background()

	// Load AWS configuration from environment
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("unable to load AWS config: %w", err)
	}

	// Create Secrets Manager client
	client := secretsmanager.NewFromConfig(cfg)

	// Get secret
	secretID := v.GetString("AWS_SECRETS_PATH")
	if secretID == "" {
		secretID = "nomadcrew/prod/secrets" // Default value if not set
	}
	secret, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &secretID,
	})
	if err != nil {
		return fmt.Errorf("failed to load secret %s: %w", secretID, err)
	}

	// Parse secrets JSON
	var secrets map[string]string
	if err := json.Unmarshal([]byte(*secret.SecretString), &secrets); err != nil {
		return fmt.Errorf("failed to parse secrets: %w", err)
	}

	// Set secrets in Viper
	for key, value := range secrets {
		v.Set(key, value)
	}

	return nil
}

func validateConfig(cfg *Config) error {
	var errors []string

	// Production-specific validations
	if cfg.IsProduction() {
		if containsWildcard(cfg.Server.AllowedOrigins) {
			errors = append(errors, "ALLOWED_ORIGINS should not contain wildcard in production")
		}

		if len(cfg.Server.JwtSecretKey) < minJWTLength {
			errors = append(errors, fmt.Sprintf("JWT_SECRET_KEY must be at least %d characters in production", minJWTLength))
		}

		if err := validateConnectionString(fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Database.Host,
			cfg.Database.Port,
			cfg.Database.User,
			cfg.Database.Password,
			cfg.Database.Name,
			cfg.Database.SSLMode,
		)); err != nil {
			errors = append(errors, err.Error())
		}

		if len(cfg.Redis.Password) < minPasswordLength {
			errors = append(errors, fmt.Sprintf("REDIS_PASSWORD must be at least %d characters in production", minPasswordLength))
		}

		if err := validateExternalServices(&cfg.ExternalServices); err != nil {
			errors = append(errors, err.Error())
		}
	}

	// Common validations
	if cfg.Server.Port == "0" {
		errors = append(errors, "PORT is required")
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed:\n- %s", strings.Join(errors, "\n- "))
	}

	return nil
}

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

func containsWildcard(origins []string) bool {
	for _, o := range origins {
		if o == "*" {
			return true
		}
	}
	return false
}
