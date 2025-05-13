package dbutils

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrations(t *testing.T) {
	// Skip if we're not in a test environment
	if os.Getenv("DB_TEST") != "true" {
		t.Skip("Skipping migration test. Set DB_TEST=true to run")
	}

	// Get database connection from environment variable
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping migration test")
	}

	// Connect to the database
	db, err := sql.Open("pgx", dbURL)
	require.NoError(t, err)
	defer db.Close()

	// Setup the migration driver
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	require.NoError(t, err)

	// Get the migration directory path
	migrationPath := "file://../migrations"

	// Create a new migrate instance
	m, err := migrate.NewWithDatabaseInstance(migrationPath, "postgres", driver)
	require.NoError(t, err)

	// First drop all tables to ensure a clean state
	err = m.Drop()
	if err != nil && err != migrate.ErrNoChange {
		t.Logf("Migration drop error (may be expected if first run): %v", err)
	}

	// Run migrations up
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		t.Fatalf("Migration up error: %v", err)
	}

	// Verify tables exist by querying the information schema
	tables := []string{
		"users", "trips", "todos", "notifications",
		"chat_groups", "chat_messages", "chat_message_reactions",
	}

	for _, table := range tables {
		var exists bool
		query := `SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = $1
		)`

		err := db.QueryRow(query, table).Scan(&exists)
		assert.NoError(t, err)
		assert.True(t, exists, fmt.Sprintf("Table %s should exist", table))
	}

	// Test migration down
	err = m.Down()
	if err != nil && err != migrate.ErrNoChange {
		t.Fatalf("Migration down error: %v", err)
	}

	// Verify tables don't exist after down migration
	for _, table := range tables {
		var exists bool
		query := `SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = $1
		)`

		err := db.QueryRow(query, table).Scan(&exists)
		assert.NoError(t, err)
		assert.False(t, exists, fmt.Sprintf("Table %s should not exist after down migration", table))
	}

	t.Log("Migration test completed successfully")
}
