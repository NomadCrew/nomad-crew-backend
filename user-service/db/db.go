package db

import (
    "context"

    "github.com/jackc/pgx/v4/pgxpool"
    "github.com/NomadCrew/nomad-crew-backend/user-service/db/dbutils"
    "github.com/NomadCrew/nomad-crew-backend/user-service/logger"
)

func ConnectToDB(connectionString string, ctx context.Context) *pgxpool.Pool {
    log := logger.GetLogger()

    pool, err := pgxpool.Connect(ctx, connectionString)
    if err != nil {
        log.Fatalf("Unable to connect to database: %v", err)
    }

    log.Info("Successfully connected to database")
    return pool
}

func EnsureUserTableExists(pool *pgxpool.Pool, ctx context.Context) error {
    return dbutils.EnsureTableExists(ctx, pool, "users")
}
