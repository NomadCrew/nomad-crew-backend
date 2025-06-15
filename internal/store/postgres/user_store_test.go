package postgres

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/NomadCrew/nomad-crew-backend/db"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

// setupTestDatabase spins up a disposable Postgres container and returns a *db.DatabaseClient and cleanup func.
func setupTestDatabase(t *testing.T) (*db.DatabaseClient, func()) {
	t.Helper()

	// On Windows GitHub runners rootless Docker isn't supported reliably.
	if runtime.GOOS == "windows" {
		t.Skip("Skipping container-based tests on Windows")
	}

	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "postgres:14",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "testdb",
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
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

	// Give Postgres a moment.
	time.Sleep(2 * time.Second)

	connStr := "postgresql://test:test@" + host + ":" + port.Port() + "/testdb?sslmode=disable"

	dbClient, err := db.SetupTestDB(connStr)
	require.NoError(t, err)

	cleanup := func() {
		_ = container.Terminate(ctx)
	}

	return dbClient, cleanup
}

func TestUserStore_CreateGetAndPreferences(t *testing.T) {
	dbClient, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()

	userStore := NewUserStore(dbClient.GetPool(), "", "")

	// --- Create ---
	u := &types.User{
		ID:       uuid.NewString(),
		Email:    "test@example.com",
		Username: "testuser",
	}
	createdID, err := userStore.CreateUser(ctx, u)
	require.NoError(t, err)
	require.Equal(t, u.ID, createdID)

	// --- Get ---
	fetched, err := userStore.GetUserByID(ctx, createdID)
	require.NoError(t, err)
	require.Equal(t, u.Email, fetched.Email)
	require.Equal(t, u.Username, fetched.Username)

	// --- Update Preferences ---
	prefs := map[string]interface{}{"theme": "dark"}
	err = userStore.UpdateUserPreferences(ctx, createdID, prefs)
	require.NoError(t, err)

	updated, err := userStore.GetUserByID(ctx, createdID)
	require.NoError(t, err)
	require.NotNil(t, updated.Preferences)
	require.Equal(t, "dark", updated.Preferences["theme"])
}

// TestUserStore_DuplicateCreate ensures unique constraint is enforced on user ID/email.
func TestUserStore_DuplicateCreate(t *testing.T) {
	dbClient, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	store := NewUserStore(dbClient.GetPool(), "", "")

	id := uuid.NewString()
	u := &types.User{ID: id, Email: "dup@example.com", Username: "dup"}
	_, err := store.CreateUser(ctx, u)
	require.NoError(t, err)

	// Attempt duplicate insert (same ID should fail with unique violation handled in method)
	_, err = store.CreateUser(ctx, u)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")
}
