// Package db provides implementations for data access interfaces defined in internal/store.
// It interacts with the PostgreSQL database using pgxpool and potentially other external services like Supabase.
package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
)

// getSchemaPath calculates the absolute path to the initial database schema file
// (expected to be migrations/000001_init.up.sql relative to this file's location).
// It performs security checks to ensure the path is within the expected directory.
func getSchemaPath() (string, error) {
	// Get the directory of the current file.
	_, b, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot get caller information")
	}
	basePath := filepath.Dir(b)

	// Construct the relative path to the schema file.
	schemaRelPath := filepath.Join("migrations", "000001_init.up.sql")
	schemaPath := filepath.Join(basePath, schemaRelPath)

	// Clean the path and get the absolute path.
	absSchemaPath, err := filepath.Abs(filepath.Clean(schemaPath))
	if err != nil {
		return "", fmt.Errorf("failed to get absolute schema path for %s: %w", schemaPath, err)
	}

	// Security check: Ensure the resolved absolute path starts with the base path
	// of this source file's directory to prevent path traversal issues.
	if !strings.HasPrefix(absSchemaPath, basePath) {
		return "", fmt.Errorf("resolved schema path %s is outside of the base directory %s", absSchemaPath, basePath)
	}

	return absSchemaPath, nil
}

// SetupTestDB connects to a test database using the provided connection string,
// executes the initial schema migration (000001_init.up.sql), and returns
// a DatabaseClient wrapping the connection pool.
// Intended for use in integration tests, often with a containerized database.
func SetupTestDB(connectionString string) (*DatabaseClient, error) {
	ctx := context.Background()
	config, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection config: %w", err)
	}
	config.ConnConfig.PreferSimpleProtocol = true

	pool, err := pgxpool.ConnectConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to test database: %w", err)
	}

	schemaPath, err := getSchemaPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get schema path for test setup: %w", err)
	}

	// Read the schema file content. The path is validated in getSchemaPath.
	// #nosec G304 -- Path is constructed relative to the source file and validated.
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file at %s: %w", schemaPath, err)
	}

	// Execute the schema SQL script.
	_, err = pool.Exec(ctx, string(schema))
	if err != nil {
		pool.Close() // Close pool if schema execution fails
		return nil, fmt.Errorf("failed to execute schema: %w", err)
	}

	// Return the client wrapping the initialized pool.
	return NewDatabaseClient(pool), nil
}

// CleanupTestDB drops all known tables from the test database connected via the DatabaseClient.
// This is used to reset the database state after tests.
// Warning: This drops tables unconditionally. Only use on test databases.
func CleanupTestDB(db *DatabaseClient) error {
	// Use a background context as this is for cleanup.
	ctx := context.Background()
	pool := db.GetPool()
	if pool == nil {
		return fmt.Errorf("database client pool is nil, cannot cleanup")
	}

	// List all known tables to be dropped.
	// Ensure this list is kept up-to-date with schema changes.
	tables := []string{
		"chat_message_reactions",
		"chat_last_read",
		"chat_group_members",
		"chat_messages",
		"chat_groups",
		"trip_invitations",
		"todos",
		"trip_memberships",
		"notifications", // Added based on likely schema
		"metadata",
		"relationships",
		"categories",
		"locations",
		"expenses",
		"trips",
		"users",
	}

	dropQuery := ""
	for _, table := range tables {
		dropQuery += fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE;\n", table)
	}

	_, err := pool.Exec(ctx, dropQuery)
	if err != nil {
		return fmt.Errorf("failed to drop tables during cleanup: %w", err)
	}

	// Optionally, close the pool after cleanup if the test runner manages pool lifecycle per test.
	// pool.Close()

	return nil
}
