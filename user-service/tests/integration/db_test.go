package integration

import (
    "context"
    "testing"
    "time"
    "fmt"
    "github.com/stretchr/testify/require"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
    "github.com/NomadCrew/nomad-crew-backend/user-service/db"
    "github.com/NomadCrew/nomad-crew-backend/user-service/types"
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

    // Build the connection string using the container's host and mapped port
    connectionString := fmt.Sprintf("postgresql://test:test@%s:%s/testdb?sslmode=disable", host, mappedPort.Port())

    // Add a short delay to ensure the database is fully ready
    time.Sleep(2 * time.Second)

    dbClient, err := db.SetupTestDB(connectionString)
    require.NoError(t, err)

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