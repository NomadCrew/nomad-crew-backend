package db

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v4/pgxpool"
	"io/ioutil"
	"path/filepath"
)

// SetupTestDB initializes a test database with schema
func SetupTestDB(connectionString string) (*DatabaseClient, error) {
	ctx := context.Background()
	pool, err := pgxpool.Connect(ctx, connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	// Read and execute schema file
	schemaPath := filepath.Join(".", "..", "db", "migrations", "init.sql")
	schema, err := ioutil.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %v", err)
	}

	_, err = pool.Exec(ctx, string(schema))
	if err != nil {
		return nil, fmt.Errorf("failed to execute schema: %v", err)
	}

	return NewDatabaseClient(pool), nil
}

// CleanupTestDB drops all tables in the test database
func CleanupTestDB(db *DatabaseClient) error {
	_, err := db.GetPool().Exec(context.Background(), `
		DROP TABLE IF EXISTS metadata CASCADE;
		DROP TABLE IF EXISTS relationships CASCADE;
		DROP TABLE IF EXISTS categories CASCADE;
		DROP TABLE IF EXISTS locations CASCADE;
		DROP TABLE IF EXISTS expenses CASCADE;
		DROP TABLE IF EXISTS trips CASCADE;
		DROP TABLE IF EXISTS users CASCADE;
	`)
	return err
}