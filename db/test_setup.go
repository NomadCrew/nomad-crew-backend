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

func getSchemaPath() (string, error) {
    _, b, _, _ := runtime.Caller(0)
    basePath := filepath.Dir(b)
    schemaPath := filepath.Join(basePath, "migrations", "init.sql")

    // Clean and obtain the absolute path
    absSchemaPath, err := filepath.Abs(filepath.Clean(schemaPath))
    if err != nil {
        return "", fmt.Errorf("failed to get absolute path: %v", err)
    }

    // Ensure the schema path is within the base directory
    if !strings.HasPrefix(absSchemaPath, basePath) {
        return "", fmt.Errorf("schema path %s is outside of the base directory %s", absSchemaPath, basePath)
    }

    return absSchemaPath, nil
}

func SetupTestDB(connectionString string) (*DatabaseClient, error) {
    ctx := context.Background()
    pool, err := pgxpool.Connect(ctx, connectionString)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to database: %v", err)
    }

    schemaPath, err := getSchemaPath()
    if err != nil {
        return nil, fmt.Errorf("failed to get schema path: %v", err)
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
