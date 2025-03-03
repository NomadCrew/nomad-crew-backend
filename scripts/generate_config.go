package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Environment    string   `yaml:"environment"`
		Port           string   `yaml:"port"`
		AllowedOrigins []string `yaml:"allowed_origins"`
		JwtSecretKey   string   `yaml:"jwt_secret_key"`
		FrontendURL    string   `yaml:"frontend_url"`
		LogLevel       string   `yaml:"log_level"`
	} `yaml:"server"`

	Database struct {
		Host           string `yaml:"host"`
		Port           string `yaml:"port"`
		User           string `yaml:"user"`
		Password       string `yaml:"password"`
		Name           string `yaml:"name"`
		MaxConnections int    `yaml:"max_connections"`
		SSLMode        string `yaml:"ssl_mode"`
	} `yaml:"database"`

	Redis struct {
		Address  string `yaml:"address"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
	} `yaml:"redis"`

	Email struct {
		FromAddress  string `yaml:"from_address"`
		FromName     string `yaml:"from_name"`
		ResendAPIKey string `yaml:"resend_api_key"`
	} `yaml:"email"`

	ExternalServices struct {
		GeoapifyKey        string `yaml:"geoapify_key"`
		PexelsAPIKey       string `yaml:"pexels_api_key"`
		SupabaseAnonKey    string `yaml:"supabase_anon_key"`
		SupabaseServiceKey string `yaml:"supabase_service_key"`
		SupabaseURL        string `yaml:"supabase_url"`
		SupabaseJWTSecret  string `yaml:"supabase_jwt_secret"`
		JwtSecretKey       string `yaml:"jwt_secret_key"`
	} `yaml:"external_services"`
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func validateRequiredEnv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("ERROR: %s environment variable is not set in your .env file. Please set it and try again", key)
	}
	if len(value) < 8 {
		return "", fmt.Errorf("ERROR: %s value is too short. It must be at least 8 characters long. Current length: %d", key, len(value))
	}
	return value, nil
}

func main() {
	// Check if .env file exists
	if _, err := os.Stat(".env"); os.IsNotExist(err) {
		fmt.Println("ERROR: .env file not found!")
		fmt.Println("Please create a .env file by copying .env.example and filling in the required values:")
		fmt.Println("cp .env.example .env")
		os.Exit(1)
	}

	config := Config{}

	// Server configuration
	config.Server.Environment = getEnvOrDefault("SERVER_ENVIRONMENT", "development")
	config.Server.Port = getEnvOrDefault("PORT", "8080")
	config.Server.AllowedOrigins = strings.Split(getEnvOrDefault("ALLOWED_ORIGINS", "*"), ",")

	jwtKey, err := validateRequiredEnv("JWT_SECRET_KEY")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	config.Server.JwtSecretKey = jwtKey

	config.Server.FrontendURL = getEnvOrDefault("FRONTEND_URL", "https://nomadcrew.uk")
	config.Server.LogLevel = getEnvOrDefault("LOG_LEVEL", "debug")

	// Database configuration
	config.Database.Host = getEnvOrDefault("DB_HOST", "postgres")
	config.Database.Port = getEnvOrDefault("DB_PORT", "5432")
	config.Database.User = getEnvOrDefault("DB_USER", "postgres")

	dbPass, err := validateRequiredEnv("DB_PASSWORD")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	config.Database.Password = dbPass

	config.Database.Name = getEnvOrDefault("DB_NAME", "nomadcrew")
	config.Database.MaxConnections = 20
	config.Database.SSLMode = getEnvOrDefault("DB_SSL_MODE", "disable")

	// Redis configuration
	config.Redis.Address = getEnvOrDefault("REDIS_ADDRESS", "redis:6379")

	redisPass, err := validateRequiredEnv("REDIS_PASSWORD")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	config.Redis.Password = redisPass

	config.Redis.DB = 0

	// Email configuration
	config.Email.FromAddress = getEnvOrDefault("EMAIL_FROM_ADDRESS", "welcome@nomadcrew.uk")
	config.Email.FromName = getEnvOrDefault("EMAIL_FROM_NAME", "NomadCrew")

	resendKey, err := validateRequiredEnv("RESEND_API_KEY")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	config.Email.ResendAPIKey = resendKey

	// External services configuration - validate all required keys
	requiredKeys := []string{
		"GEOAPIFY_KEY",
		"PEXELS_API_KEY",
		"SUPABASE_ANON_KEY",
		"SUPABASE_SERVICE_KEY",
		"SUPABASE_URL",
		"SUPABASE_JWT_SECRET",
	}

	for _, key := range requiredKeys {
		value, err := validateRequiredEnv(key)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		switch key {
		case "GEOAPIFY_KEY":
			config.ExternalServices.GeoapifyKey = value
		case "PEXELS_API_KEY":
			config.ExternalServices.PexelsAPIKey = value
		case "SUPABASE_ANON_KEY":
			config.ExternalServices.SupabaseAnonKey = value
		case "SUPABASE_SERVICE_KEY":
			config.ExternalServices.SupabaseServiceKey = value
		case "SUPABASE_URL":
			config.ExternalServices.SupabaseURL = value
		case "SUPABASE_JWT_SECRET":
			config.ExternalServices.SupabaseJWTSecret = value
		}
	}
	config.ExternalServices.JwtSecretKey = config.Server.JwtSecretKey

	// Generate YAML
	yamlData, err := yaml.Marshal(&config)
	if err != nil {
		fmt.Printf("Error marshaling YAML: %v\n", err)
		os.Exit(1)
	}

	// Get the environment name from command line args or use default
	env := "development"
	if len(os.Args) > 1 {
		env = os.Args[1]
	}

	// Create config directory if it doesn't exist
	err = os.MkdirAll("config", 0755)
	if err != nil {
		fmt.Printf("Error creating config directory: %v\n", err)
		os.Exit(1)
	}

	// Write to file
	filename := fmt.Sprintf("config/config.%s.yaml", env)
	err = os.WriteFile(filename, yamlData, 0644)
	if err != nil {
		fmt.Printf("Error writing config file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully generated %s\n", filename)
}
