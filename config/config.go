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

	// Secrets path
	secretPath = "nomadcrew/prod/secrets"

	// Validation constants
	minJWTLength      = 32
	minPasswordLength = 16
	minKeyLength      = 8
)

type ServerConfig struct {
	Environment    Environment `mapstructure:"ENVIRONMENT"`
	Port           string      `mapstructure:"PORT"`
	AllowedOrigins []string    `mapstructure:"ALLOWED_ORIGINS"`
	Version        string      `mapstructure:"VERSION"`
	JwtSecretKey   string      `mapstructure:"JWT_SECRET_KEY"`
}

type DatabaseConfig struct {
	ConnectionString string `mapstructure:"DB_CONNECTION_STRING"`
	MaxConnections   int    `mapstructure:"DB_MAX_CONNECTIONS"`
}

type RedisConfig struct {
	Address  string `mapstructure:"REDIS_ADDRESS"`
	Password string `mapstructure:"REDIS_PASSWORD"`
	DB       int    `mapstructure:"REDIS_DB"`
}

type ExternalServices struct {
	GeoapifyKey     string `mapstructure:"GEOAPIFY_KEY"`
	PexelsAPIKey    string `mapstructure:"PEXELS_API_KEY"`
	SupabaseAnonKey string `mapstructure:"SUPABASE_ANON_KEY"`
}

type Config struct {
	Server           ServerConfig     `mapstructure:",squash"`
	Database         DatabaseConfig   `mapstructure:",squash"`
	Redis            RedisConfig      `mapstructure:",squash"`
	ExternalServices ExternalServices `mapstructure:",squash"`
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

	// Set defaults for development
	v.SetDefault("ENVIRONMENT", EnvDevelopment)
	v.SetDefault("PORT", "8080")
	v.SetDefault("ALLOWED_ORIGINS", []string{"*"})
	v.SetDefault("VERSION", "0.0.0-dev")
	v.SetDefault("DB_MAX_CONNECTIONS", 20)
	v.SetDefault("REDIS_DB", 0)

	// Configuration sources priority:
	// 1. Environment variables
	// 2. Config file (config.<environment>.yaml)
	// 3. Defaults

	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

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
	secretID := secretPath
	secret, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &secretID,
	})
	if err != nil {
		return fmt.Errorf("failed to load secret %s: %w", secretPath, err)
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

		if err := validateConnectionString(cfg.Database.ConnectionString); err != nil {
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
	if cfg.Server.Port == "" {
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
