package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
)

func getSchemaPath() (string, error) {
	_, b, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(b)
	schemaPath := filepath.Join(baseDir, "migrations", "init.sql")

	// Ensure the path is absolute
	if !filepath.IsAbs(schemaPath) {
		return "", fmt.Errorf("schema path must be absolute: %s", schemaPath)
	}

	// Validate the path is inside the migrations directory
	expectedDir := filepath.Join(baseDir, "migrations")
	if !strings.HasPrefix(filepath.Clean(schemaPath), filepath.Clean(expectedDir)) {
		return "", errors.New("schema path is outside the migrations directory")
	}

	return schemaPath, nil
}

func SetupTestDB(connectionString string) (*DatabaseClient, error) {
	ctx := context.Background()
	pool, err := pgxpool.Connect(ctx, connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	schemaPath, err := getSchemaPath()
	if err != nil {
		return nil, fmt.Errorf("invalid schema path: %v", err)
	}

	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file at %s: %v", schemaPath, err)
	}

	_, err = pool.Exec(ctx, string(schema))
	if err != nil {
		return nil, fmt.Errorf("failed to execute schema: %v", err)
	}

	return NewDatabaseClient(pool), nil
}

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
