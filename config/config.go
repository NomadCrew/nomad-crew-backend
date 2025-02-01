package config

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/jackc/pgx/v4/pgxpool"
)

type RedisConfig struct {
	Address  string
	Password string
	DB       int
}

type SSEConfig struct {
	ApiKey string
}

type Config struct {
	DatabaseConnectionString string
	JwtSecretKey             string
	SupabaseAnonKey          string
	Port                     string
	PexelsAPIKey             string
	Redis                    RedisConfig
	SSE                      SSEConfig
}

func LoadConfig() (*Config, error) {
	log := logger.GetLogger()

	redisDBStr := os.Getenv("REDIS_DB")
	redisDB, err := strconv.Atoi(redisDBStr)
	if err != nil {
		log.Fatalf("Error converting REDIS_DB to int: %v", err)
		return nil, errors.New("invalid REDIS_DB environment variable")
	}

	cfg := &Config{
		DatabaseConnectionString: os.Getenv("DB_CONNECTION_STRING"),
		SupabaseAnonKey:          os.Getenv("SUPABASE_ANON_KEY"),
		JwtSecretKey:             os.Getenv("JWT_SECRET_KEY"),
		Port:                     os.Getenv("PORT"),
		PexelsAPIKey:             os.Getenv("PEXELS_API_KEY"),
		Redis: RedisConfig{
			Address:  os.Getenv("REDIS_ADDRESS"),
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       redisDB,
		},
		SSE: SSEConfig{
			ApiKey: os.Getenv("SUPABASE_ANON_KEY"),
		},
	}

	if cfg.DatabaseConnectionString == "" || cfg.SupabaseAnonKey == "" || cfg.JwtSecretKey == "" || cfg.Port == "" {
		log.Fatal("Error: one or more environment variables are not set")
		return nil, errors.New("one or more environment variables are not set")
	}

	if cfg.SSE.ApiKey == "" {
		log.Fatal("SSE_API_KEY environment variable is required")
		return nil, errors.New("SSE_API_KEY not set")
	}

	log.Infow("Configuration loaded",
		"database_connection", maskSensitiveURL(cfg.DatabaseConnectionString),
		"jwt_key_length", len(cfg.JwtSecretKey),
		"port", cfg.Port,
	)
	return cfg, nil
}

func maskSensitiveURL(url string) string {
	if url == "" {
		return ""
	}
	parts := strings.Split(url, "@")
	if len(parts) != 2 {
		return "invalid-url-format"
	}
	credentials := strings.Split(parts[0], "://")
	if len(credentials) != 2 {
		return "invalid-url-format"
	}
	return credentials[0] + "://*****:*****@" + parts[1]
}
