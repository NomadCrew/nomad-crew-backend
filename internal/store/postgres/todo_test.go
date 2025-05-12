package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	testPool *pgxpool.Pool
)

func setupTestDB(t *testing.T) (*pgxpool.Pool, uuid.UUID, uuid.UUID) {
	// Skip container tests on Windows to avoid rootless Docker issues
	if os.Getenv("OS") == "Windows_NT" {
		t.Skip("Skipping container tests on Windows due to rootless Docker issues")
	}

	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "postgres:15",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "testdb",
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpass",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	connStr := fmt.Sprintf("postgres://testuser:testpass@%s:%s/testdb?sslmode=disable",
		host, port.Port())

	// Add a short delay to ensure the database is fully ready
	time.Sleep(2 * time.Second)

	config, err := pgxpool.ParseConfig(connStr)
	require.NoError(t, err)

	testPool, err = pgxpool.ConnectConfig(ctx, config)
	require.NoError(t, err)

	// Run migrations
	// First try to read from relative path
	initSQL, err := os.ReadFile("../../../db/migrations/000001_init.up.sql")
	if err != nil {
		// If relative path fails, try absolute path based on git repo root detection
		wd, _ := os.Getwd()
		repoRoot := findRepoRoot(wd)

		initSQL, err = os.ReadFile(filepath.Join(repoRoot, "db/migrations/000001_init.up.sql"))
		require.NoError(t, err, "Failed to read init migration file")
	}

	_, err = testPool.Exec(ctx, string(initSQL))
	require.NoError(t, err, "Failed to execute init migration")

	// No need to try to find missing migrations since they're now included in the main migration file
	// All table creation is now in 000001_init.up.sql

	// Create test user and trip
	userID := uuid.New()
	tripID := uuid.New()

	_, err = testPool.Exec(ctx, `
		INSERT INTO users (id, email, name, created_at, updated_at)
		VALUES ($1, 'test@example.com', 'Test User', NOW(), NOW())`,
		userID,
	)
	require.NoError(t, err)

	_, err = testPool.Exec(ctx, `
		INSERT INTO trips (id, name, description, start_date, end_date, status, created_by, created_at, updated_at)
		VALUES ($1, 'Test Trip', 'Test Description', NOW(), NOW() + INTERVAL '1 day', 'PLANNING', $2, NOW(), NOW())`,
		tripID,
		userID,
	)
	require.NoError(t, err)

	// Create trip membership linking user and trip
	_, err = testPool.Exec(ctx, `
		INSERT INTO trip_memberships (trip_id, user_id, role, status, created_at, updated_at)
		VALUES ($1, $2, 'OWNER', 'ACTIVE', NOW(), NOW())`,
		tripID,
		userID,
	)
	require.NoError(t, err)

	return testPool, userID, tripID
}

func teardownTestDB(t *testing.T) {
	if testPool != nil {
		testPool.Close()
	}
}

func stringPtr(s string) *string {
	return &s
}

func TestTodoStore(t *testing.T) {
	pool, userID, tripID := setupTestDB(t)
	defer teardownTestDB(t)

	ctx := context.Background()
	store := NewTodoStore(pool)
	now := time.Now().UTC()

	t.Run("CreateTodo", func(t *testing.T) {
		todo := &types.Todo{
			TripID:    tripID.String(),
			Text:      "Test todo",
			Status:    types.TodoStatusIncomplete,
			CreatedBy: userID.String(),
			CreatedAt: now,
			UpdatedAt: now,
		}

		id, err := store.CreateTodo(ctx, todo)
		require.NoError(t, err)
		assert.NotEmpty(t, id)
		todo.ID = id

		// Verify created todo
		created, err := store.GetTodo(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, todo.TripID, created.TripID)
		assert.Equal(t, todo.Text, created.Text)
		assert.Equal(t, todo.Status, created.Status)
		assert.Equal(t, todo.CreatedBy, created.CreatedBy)
	})

	t.Run("GetTodo - Not Found", func(t *testing.T) {
		_, err := store.GetTodo(ctx, uuid.New().String())
		assert.Error(t, err)
		assert.Equal(t, ErrNotFound, err)
	})

	t.Run("ListTodos", func(t *testing.T) {
		// Create another todo for the same trip
		todo2 := &types.Todo{
			TripID:    tripID.String(),
			Text:      "Another test todo",
			Status:    types.TodoStatusIncomplete,
			CreatedBy: userID.String(),
			CreatedAt: now,
			UpdatedAt: now,
		}

		id, err := store.CreateTodo(ctx, todo2)
		require.NoError(t, err)
		todo2.ID = id

		// List todos for the trip
		todos, err := store.ListTodos(ctx, tripID.String())
		require.NoError(t, err)
		assert.Len(t, todos, 2)

		// Verify todos are ordered by created_at DESC
		assert.Equal(t, todo2.ID, todos[0].ID)
	})

	t.Run("UpdateTodo", func(t *testing.T) {
		// Create a todo to update
		todo := &types.Todo{
			TripID:    tripID.String(),
			Text:      "Todo to update",
			Status:    types.TodoStatusIncomplete,
			CreatedBy: userID.String(),
			CreatedAt: now,
			UpdatedAt: now,
		}

		id, err := store.CreateTodo(ctx, todo)
		require.NoError(t, err)
		todo.ID = id

		// Update the todo
		update := &types.TodoUpdate{
			Text:   stringPtr("Updated text"),
			Status: types.TodoStatusComplete.Ptr(),
		}

		updated, err := store.UpdateTodo(ctx, id, update)
		require.NoError(t, err)
		assert.Equal(t, "Updated text", updated.Text)
		assert.Equal(t, types.TodoStatusComplete, updated.Status)
		assert.True(t, updated.UpdatedAt.After(now))
	})

	t.Run("UpdateTodo - Not Found", func(t *testing.T) {
		update := &types.TodoUpdate{
			Text: stringPtr("Should fail"),
		}

		_, err := store.UpdateTodo(ctx, uuid.New().String(), update)
		assert.Error(t, err)
		assert.Equal(t, ErrNotFound, err)
	})

	t.Run("DeleteTodo", func(t *testing.T) {
		// Create a todo to delete
		todo := &types.Todo{
			TripID:    tripID.String(),
			Text:      "Todo to delete",
			Status:    types.TodoStatusIncomplete,
			CreatedBy: userID.String(),
			CreatedAt: now,
			UpdatedAt: now,
		}

		id, err := store.CreateTodo(ctx, todo)
		require.NoError(t, err)

		// Delete the todo
		err = store.DeleteTodo(ctx, id)
		require.NoError(t, err)

		// Verify it's deleted
		_, err = store.GetTodo(ctx, id)
		assert.Error(t, err)
		assert.Equal(t, ErrNotFound, err)
	})

	t.Run("DeleteTodo - Not Found", func(t *testing.T) {
		err := store.DeleteTodo(ctx, uuid.New().String())
		assert.Error(t, err)
		assert.Equal(t, ErrNotFound, err)
	})

	t.Run("Transaction", func(t *testing.T) {
		// Start a transaction
		tx, err := store.BeginTx(ctx)
		require.NoError(t, err)

		// Create a todo in the transaction
		todo := &types.Todo{
			TripID:    tripID.String(),
			Text:      "Transaction todo",
			Status:    types.TodoStatusIncomplete,
			CreatedBy: userID.String(),
			CreatedAt: now,
			UpdatedAt: now,
		}

		id, err := store.CreateTodo(ctx, todo)
		require.NoError(t, err)

		// Verify the todo exists in the transaction
		_, err = store.GetTodo(ctx, id)
		require.NoError(t, err)

		// Rollback the transaction
		err = tx.Rollback()
		require.NoError(t, err)

		// Create a new store instance to verify the rollback
		newStore := NewTodoStore(pool)
		_, err = newStore.GetTodo(ctx, id)
		assert.Error(t, err)
		assert.Equal(t, ErrNotFound, err)
	})
}

// Helper function to find Git repository root
func findRepoRoot(dir string) string {
	for i := 0; i < 5; i++ { // Limit search to 5 levels up
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// We've reached the root of the filesystem
			break
		}
		dir = parent
	}

	// Fallback to current directory if repo root not found
	return "."
}
