package db

import (
    "context"
    "fmt"
    "os"
    "strings"
    "path/filepath"
    "runtime"
    "github.com/jackc/pgx/v4/pgxpool"
)

func getSchemaPath() string {
    _, b, _, _ := runtime.Caller(0)
    // Explicitly use the init.sql file relative to this file
    return filepath.Join(filepath.Dir(b), "migrations", "init.sql")
}

func SetupTestDB(connectionString string) (*DatabaseClient, error) {
    ctx := context.Background()
    pool, err := pgxpool.Connect(ctx, connectionString)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to database: %v", err)
    }

    schemaPath := getSchemaPath()
    // Validate path is within project root
    if !filepath.IsAbs(schemaPath) {
        return nil, fmt.Errorf("schema path must be absolute: %s", schemaPath)
    }

    // Extra validation to ensure we're only reading from migrations directory
    if !strings.Contains(schemaPath, "migrations") {
        return nil, fmt.Errorf("schema must be in migrations directory")
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