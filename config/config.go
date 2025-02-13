package config

import (
    "context"
    "errors"
    "os"
    "strings"

    "github.com/NomadCrew/nomad-crew-backend/db"
    "github.com/NomadCrew/nomad-crew-backend/logger"
    "github.com/jackc/pgx/v4/pgxpool"
)

type Config struct {
    DatabaseConnectionString string
    JwtSecretKey            string
    SupabaseAnonKey         string 
    Port                    string
    DB                      *db.DatabaseClient
}

func LoadConfig() (*Config, error) {
    log := logger.GetLogger()

    cfg := &Config{
        DatabaseConnectionString: os.Getenv("DB_CONNECTION_STRING"),
        SupabaseAnonKey:          os.Getenv("SUPABASE_ANON_KEY"),
        JwtSecretKey:             os.Getenv("JWT_SECRET_KEY"),
        Port:                     os.Getenv("PORT"),
    }

    log.Debug("DB_CONNECTION_STRING: ", cfg.DatabaseConnectionString)
    log.Debug("SUPABASE_ANON_KEY: ", cfg.SupabaseAnonKey)
    log.Debug("JWT_SECRET_KEY: ",
        strings.Repeat("*", len(cfg.JwtSecretKey)),
    )
    log.Debug("PORT: ", cfg.Port)

    if cfg.DatabaseConnectionString == "" || cfg.SupabaseAnonKey == "" || cfg.JwtSecretKey == "" || cfg.Port == "" {
        log.Fatal("Error: one or more environment variables are not set")
        return nil, errors.New("one or more environment variables are not set")
    }

    pool := connectToDB(cfg.DatabaseConnectionString, context.Background())
    cfg.DB = db.NewDatabaseClient(pool)

    if cfg.DatabaseConnectionString == "" {
        return nil, errors.New("missing DB_CONNECTION_STRING environment variable")
    }
    if cfg.JwtSecretKey == "" {
        return nil, errors.New("missing JWT_SECRET_KEY environment variable")
    }
    if cfg.Port == "" {
        return nil, errors.New("missing PORT environment variable")
    }
    
    log.Infow("Configuration loaded",
        "database_connection", maskSensitiveURL(cfg.DatabaseConnectionString),
        "jwt_key_length", len(cfg.JwtSecretKey),
        "port", cfg.Port,
    )
    return cfg, nil
}

func connectToDB(connectionString string, ctx context.Context) *pgxpool.Pool {
    log := logger.GetLogger()
    pool, err := pgxpool.Connect(ctx, connectionString)
    if err != nil {
        log.Fatalf("Unable to connect to database: %v", err)
    }
    log.Info("Successfully connected to database")
    return pool
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