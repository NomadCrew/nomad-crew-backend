package config

import (
	"errors"
	"os"

	"github.com/NomadCrew/nomad-crew-backend/user-service/logger"
)

type Config struct {
	DatabaseConnectionString string
	JwtSecretKey             string
	Port                     string
	serviceAccountKeyPath    string
}

func LoadConfig() (*Config, error) {

	log := logger.GetLogger()

	cfg := &Config{
		DatabaseConnectionString: os.Getenv("DB_CONNECTION_STRING"),
		JwtSecretKey:             os.Getenv("JWT_SECRET_KEY"),
		Port:                     os.Getenv("PORT"),
		serviceAccountKeyPath:    os.Getenv("SERVICE_ACCOUNT_KEY_PATH"),
	}

	if cfg.DatabaseConnectionString == "" || cfg.JwtSecretKey == "" || cfg.Port == "" {
		log.Fatal("Error: one or more environment variables are not set")
		return nil, errors.New("one or more environment variables are not set")
	}

	return cfg, nil
}
