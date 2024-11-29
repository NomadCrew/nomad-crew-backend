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

package integration

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/require"
    "github.com/NomadCrew/nomad-crew-backend/db"
    "github.com/NomadCrew/nomad-crew-backend/types"
)

func TestTripDB_Integration(t *testing.T) {
    dbClient, cleanup := setupTestDatabase(t)
    defer cleanup()

    ctx := context.Background()
    tripDB := db.NewTripDB(dbClient)
    userDB := db.NewUserDB(dbClient)

    // First create a user for our trips
    user := &types.User{
        Username:     "testuser",
        Email:        "test@example.com",
        PasswordHash: "hashedpassword",
        FirstName:    "Test",
        LastName:     "User",
    }
    err := userDB.SaveUser(ctx, user)
    require.NoError(t, err)
    require.NotZero(t, user.ID)

    t.Run("Create and Get Trip", func(t *testing.T) {
        trip := types.Trip{
            Name:        "Test Trip",
            Description: "Test Description",
            Destination: "Test Location",
            StartDate:   time.Now().Add(24 * time.Hour),
            EndDate:     time.Now().Add(48 * time.Hour),
            CreatedBy:   user.ID,
        }

        // Test creation
        id, err := tripDB.CreateTrip(ctx, trip)
        require.NoError(t, err)
        require.NotZero(t, id)

        // Test retrieval
        fetchedTrip, err := tripDB.GetTrip(ctx, id)
        require.NoError(t, err)
        require.Equal(t, trip.Name, fetchedTrip.Name)
        require.Equal(t, trip.Description, fetchedTrip.Description)
        require.Equal(t, trip.Destination, fetchedTrip.Destination)
    })

    t.Run("Update Trip", func(t *testing.T) {
        // Create a trip first
        trip := types.Trip{
            Name:        "Update Test Trip",
            Description: "Original Description",
            Destination: "Original Location",
            StartDate:   time.Now().Add(24 * time.Hour),
            EndDate:     time.Now().Add(48 * time.Hour),
            CreatedBy:   user.ID,
        }

        id, err := tripDB.CreateTrip(ctx, trip)
        require.NoError(t, err)

        // Update the trip
        update := types.TripUpdate{
            Name:        "Updated Trip",
            Description: "Updated Description",
            Destination: "Updated Location",
        }

        err = tripDB.UpdateTrip(ctx, id, update)
        require.NoError(t, err)

        // Verify update
        fetchedTrip, err := tripDB.GetTrip(ctx, id)
        require.NoError(t, err)
        require.Equal(t, update.Name, fetchedTrip.Name)
        require.Equal(t, update.Description, fetchedTrip.Description)
        require.Equal(t, update.Destination, fetchedTrip.Destination)
    })

    t.Run("List User Trips", func(t *testing.T) {
        // Create multiple trips
        for i := 0; i < 3; i++ {
            trip := types.Trip{
                Name:        fmt.Sprintf("List Test Trip %d", i),
                Description: "Test Description",
                Destination: "Test Location",
                StartDate:   time.Now().Add(24 * time.Hour),
                EndDate:     time.Now().Add(48 * time.Hour),
                CreatedBy:   user.ID,
            }
            _, err := tripDB.CreateTrip(ctx, trip)
            require.NoError(t, err)
        }

        // List trips
        trips, err := tripDB.ListUserTrips(ctx, user.ID)
        require.NoError(t, err)
        require.GreaterOrEqual(t, len(trips), 3)
    })

    t.Run("Soft Delete Trip", func(t *testing.T) {
        // Create a trip
        trip := types.Trip{
            Name:        "Delete Test Trip",
            Description: "Test Description",
            Destination: "Test Location",
            StartDate:   time.Now().Add(24 * time.Hour),
            EndDate:     time.Now().Add(48 * time.Hour),
            CreatedBy:   user.ID,
        }

        id, err := tripDB.CreateTrip(ctx, trip)
        require.NoError(t, err)

        // Delete the trip
        err = tripDB.SoftDeleteTrip(ctx, id)
        require.NoError(t, err)

        // Verify it's not retrievable
        _, err = tripDB.GetTrip(ctx, id)
        require.Error(t, err)
    })

    t.Run("Search Trips", func(t *testing.T) {
        // Create some searchable trips
        locations := []string{"Paris", "London", "New York"}
        for _, loc := range locations {
            trip := types.Trip{
                Name:        fmt.Sprintf("Trip to %s", loc),
                Description: "Test Description",
                Destination: loc,
                StartDate:   time.Now().Add(24 * time.Hour),
                EndDate:     time.Now().Add(48 * time.Hour),
                CreatedBy:   user.ID,
            }
            _, err := tripDB.CreateTrip(ctx, trip)
            require.NoError(t, err)
        }

        // Search for specific location
        criteria := types.TripSearchCriteria{
            Destination: "Paris",
        }
        results, err := tripDB.SearchTrips(ctx, criteria)
        require.NoError(t, err)
        require.NotEmpty(t, results)
        require.Contains(t, results[0].Destination, "Paris")
    })
}