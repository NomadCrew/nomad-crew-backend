package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Environment represents a deployment environment
type EnvType string

const (
	// Development environment
	Development EnvType = "dev"

	// Staging environment
	Staging EnvType = "staging"

	// Production environment
	Production EnvType = "production"
)

// LoadConfig loads configuration for a specific environment
func LoadConfigForEnv(environment string) (*Config, error) {
	env := EnvType(environment)

	// Determine config path based on environment
	configPath, err := getConfigPath(env)
	if err != nil {
		return nil, err
	}

	return LoadConfigFromFile(configPath)
}

// getConfigPath determines the path to the environment-specific config
func getConfigPath(env EnvType) (string, error) {
	// Base config directory
	configDir := "config"

	// Check if running in a container (different path structure)
	if os.Getenv("CONTAINER") == "true" {
		configDir = "/app/config"
	}

	// Environment-specific filename
	var filename string
	switch env {
	case Development:
		filename = "config.dev.yaml"
	case Staging:
		filename = "config.staging.yaml"
	case Production:
		filename = "config.prod.yaml"
	default:
		return "", fmt.Errorf("unknown environment: %s", env)
	}

	// Full path
	path := filepath.Join(configDir, filename)

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("configuration file not found: %s", path)
	}

	return path, nil
}

// LoadConfigFromFile loads configuration from a specific file
func LoadConfigFromFile(path string) (*Config, error) {
	// Load using existing mechanism
	// This assumes you have a function like LoadConfigFromFile
	// Adjust based on your actual implementation
	config, err := LoadConfigFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from %s: %w", path, err)
	}

	return config, nil
}

// Create template environment-specific config files
func CreateConfigTemplateForEnvironment(env EnvType) error {
	// Base config directory
	configDir := "config"

	// Environment-specific filename
	var filename string
	switch env {
	case Development:
		filename = "config.dev.yaml"
	case Staging:
		filename = "config.staging.yaml"
	case Production:
		filename = "config.prod.yaml"
	default:
		return fmt.Errorf("unknown environment: %s", env)
	}

	// Full path
	path := filepath.Join(configDir, filename)

	// Check if file already exists
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("configuration file already exists: %s", path)
	}

	// Create template based on environment
	template := getConfigTemplate(env)

	// Write to file
	err := os.WriteFile(path, []byte(template), 0644)
	if err != nil {
		return fmt.Errorf("failed to write config template: %w", err)
	}

	return nil
}

// getConfigTemplate returns a config template for the given environment
func getConfigTemplate(env EnvType) string {
	baseTemplate := `# Config for %s environment
server:
  port: 8080
  host: "0.0.0.0"
  frontend_url: "%s"
  jwt_secret_key: "${JWT_SECRET}" # Use environment variable

database:
  host: "${DB_HOST}"
  port: 5432
  user: "${DB_USER}"
  password: "${DB_PASSWORD}"
  dbname: "${DB_NAME}"
  sslmode: "%s"

external_services:
  supabase_url: "${SUPABASE_URL}"
  supabase_anon_key: "${SUPABASE_ANON_KEY}"
  supabase_jwt_secret: "${SUPABASE_JWT_SECRET}"
`

	var frontendURL, sslMode string

	switch env {
	case Development:
		frontendURL = "http://localhost:3000"
		sslMode = "disable"
	case Staging:
		frontendURL = "https://staging.nomadcrew.uk"
		sslMode = "require"
	case Production:
		frontendURL = "https://nomadcrew.uk"
		sslMode = "require"
	}

	return fmt.Sprintf(baseTemplate, env, frontendURL, sslMode)
}
