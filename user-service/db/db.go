package db

import (
	"context"
	"log"

	"github.com/jackc/pgx/v4/pgxpool"
)

var DbPool *pgxpool.Pool

func ConnectToDB(connectionString string) *pgxpool.Pool {
	pool, err := pgxpool.Connect(context.Background(), connectionString)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	ensureUserTableExists(pool)
	return pool
}

func ensureUserTableExists(pool *pgxpool.Pool) {
	ctx := context.Background()
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(50),
		email VARCHAR(255) UNIQUE,
		password_hash VARCHAR(255)
	);
	`
	_, err := pool.Exec(ctx, query)
	if err != nil {
		log.Fatalf("Unable to ensure table exists: %v\n", err)
	}
}
