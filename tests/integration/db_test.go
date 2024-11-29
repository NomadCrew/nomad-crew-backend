package integration

import (
    "context"
    "testing"
    "time"
    "fmt"
	"os"
	"io"
    "github.com/stretchr/testify/require"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
    "github.com/NomadCrew/nomad-crew-backend/db"
    "github.com/NomadCrew/nomad-crew-backend/types"
)

func setupTestDatabase(t *testing.T) (*db.DatabaseClient, func()) {
    ctx := context.Background()

    req := testcontainers.ContainerRequest{
        Image:        "postgres:14",
        ExposedPorts: []string{"5432/tcp"},
        WaitingFor:   wait.ForLog("database system is ready to accept connections"),
        Env: map[string]string{
            "POSTGRES_DB":       "testdb",
            "POSTGRES_USER":     "test",
            "POSTGRES_PASSWORD": "test",
        },
    }

    container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:         true,
    })
    require.NoError(t, err)

    mappedPort, err := container.MappedPort(ctx, "5432")
    require.NoError(t, err)

    host, err := container.Host(ctx)
    require.NoError(t, err)

    connectionString := fmt.Sprintf("postgresql://test:test@%s:%s/testdb?sslmode=disable", host, mappedPort.Port())

    // Add a short delay to ensure the database is fully ready
    time.Sleep(2 * time.Second)

    dbClient, err := db.SetupTestDB(connectionString)
    if err != nil {
        // Enhanced error logging
        t.Logf("Database setup failed: %v", err)
        t.Logf("Current working directory: %s", mustGetwd(t))
        t.Logf("Container status: host=%s, port=%s", host, mappedPort.Port())
        
        // Get container logs for debugging
        logs, _ := container.Logs(ctx)
        if logs != nil {
            content, _ := io.ReadAll(logs)
            t.Logf("Container logs:\n%s", string(content))
        }
        
        require.NoError(t, err)
    }

    cleanup := func() {
        if err := db.CleanupTestDB(dbClient); err != nil {
            t.Logf("failed to cleanup test database: %s", err)
        }
        if err := container.Terminate(ctx); err != nil {
            t.Logf("failed to terminate container: %s", err)
        }
    }

    return dbClient, cleanup
}

func mustGetwd(t *testing.T) string {
    dir, err := os.Getwd()
    if err != nil {
        t.Fatalf("Failed to get working directory: %v", err)
    }
    return dir
}

func TestUserDB_Integration(t *testing.T) {
    dbClient, cleanup := setupTestDatabase(t)
    defer cleanup()

    ctx := context.Background()
    userDB := db.NewUserDB(dbClient)

    t.Run("Create and Get User", func(t *testing.T) {
        user := &types.User{
            Username:     "testuser",
            Email:        "test@example.com",
            PasswordHash: "hashedpassword",
            FirstName:    "Test",
            LastName:     "User",
            CreatedAt:    time.Now(),
            UpdatedAt:    time.Now(),
        }

        err := userDB.SaveUser(ctx, user)
        require.NoError(t, err)
        require.NotZero(t, user.ID)

        fetchedUser, err := userDB.GetUserByID(ctx, user.ID)
        require.NoError(t, err)
        require.Equal(t, user.Username, fetchedUser.Username)
        require.Equal(t, user.Email, fetchedUser.Email)
    })

    // Add cleanup verification
    t.Run("Cleanup", func(t *testing.T) {
        err := db.CleanupTestDB(dbClient)
        require.NoError(t, err)
    })
}