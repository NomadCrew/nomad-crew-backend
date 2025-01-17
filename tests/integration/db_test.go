package integration

import (
    "context"
    "testing"
    "time"
    "fmt"
    "io"
    "os"
    "github.com/stretchr/testify/require"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
    "github.com/NomadCrew/nomad-crew-backend/db"
    "github.com/NomadCrew/nomad-crew-backend/types"
)

// Test constants to represent Supabase user IDs
const (
    testUserID = "auth0|123456789"  // Format similar to Supabase/Auth0 user IDs
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

func TestTripDB_Integration(t *testing.T) {
    dbClient, cleanup := setupTestDatabase(t)
    defer cleanup()

    ctx := context.Background()
    tripDB := db.NewTripDB(dbClient)

    t.Run("Create and Get Trip", func(t *testing.T) {
        trip := types.Trip{
            Name:        "Test Trip",
            Description: "Test Description",
            Destination: "Test Location",
            StartDate:   time.Now().Add(24 * time.Hour),
            EndDate:     time.Now().Add(48 * time.Hour),
            CreatedBy:   testUserID,
            Status:      types.TripStatusPlanning,
        }

        // Test creation
        id, err := tripDB.CreateTrip(ctx, trip)
        require.NoError(t, err)
        require.NotEmpty(t, id)

        // Test retrieval
        fetchedTrip, err := tripDB.GetTrip(ctx, id)
        require.NoError(t, err)
        require.Equal(t, trip.Name, fetchedTrip.Name)
        require.Equal(t, trip.Description, fetchedTrip.Description)
        require.Equal(t, trip.Destination, fetchedTrip.Destination)
        require.Equal(t, testUserID, fetchedTrip.CreatedBy)
    })

    t.Run("Update Trip", func(t *testing.T) {
        // Create a trip first
        trip := types.Trip{
            Name:        "Update Test Trip",
            Description: "Original Description",
            Destination: "Original Location",
            StartDate:   time.Now().Add(24 * time.Hour),
            EndDate:     time.Now().Add(48 * time.Hour),
            CreatedBy:   testUserID,
            Status:      types.TripStatusPlanning,
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
                CreatedBy:   testUserID,
                Status:      types.TripStatusPlanning,
            }
            _, err := tripDB.CreateTrip(ctx, trip)
            require.NoError(t, err)
        }

        // List trips
        trips, err := tripDB.ListUserTrips(ctx, testUserID)
        require.NoError(t, err)
        require.GreaterOrEqual(t, len(trips), 3)

        // Verify all trips belong to test user
        for _, trip := range trips {
            require.Equal(t, testUserID, trip.CreatedBy)
        }
    })

    // Add cleanup verification
    t.Run("Cleanup", func(t *testing.T) {
        err := db.CleanupTestDB(dbClient)
        require.NoError(t, err)
    })

    // Add these test cases to TestTripDB_Integration

    t.Run("Status Transitions", func(t *testing.T) {
        // Create a trip in PLANNING status
        trip := types.Trip{
            Name:        "Status Test Trip",
            Description: "Testing status transitions",
            Destination: "Test Location",
            StartDate:   time.Now().Add(24 * time.Hour),
            EndDate:     time.Now().Add(48 * time.Hour),
            CreatedBy:   testUserID,
            Status:      types.TripStatusPlanning,
        }

        id, err := tripDB.CreateTrip(ctx, trip)
        require.NoError(t, err)

        // Test valid transition: PLANNING -> ACTIVE
        update := types.TripUpdate{
            Status: types.TripStatusActive,
        }
        err = tripDB.UpdateTrip(ctx, id, update)
        require.NoError(t, err)

        // Verify status change
        fetchedTrip, err := tripDB.GetTrip(ctx, id)
        require.NoError(t, err)
        require.Equal(t, types.TripStatusActive, fetchedTrip.Status)

        // Test valid transition: ACTIVE -> COMPLETED
        update.Status = types.TripStatusCompleted
        err = tripDB.UpdateTrip(ctx, id, update)
        require.NoError(t, err)

        // Verify final status
        fetchedTrip, err = tripDB.GetTrip(ctx, id)
        require.NoError(t, err)
        require.Equal(t, types.TripStatusCompleted, fetchedTrip.Status)
    })

    t.Run("Soft Delete Functionality", func(t *testing.T) {
        // Create two trips
        trip1 := types.Trip{
            Name:        "Trip to Delete",
            Description: "This trip will be deleted",
            Destination: "Deletion Test",
            StartDate:   time.Now().Add(24 * time.Hour),
            EndDate:     time.Now().Add(48 * time.Hour),
            CreatedBy:   testUserID,
            Status:      types.TripStatusPlanning,
        }

        trip2 := types.Trip{
            Name:        "Trip to Keep",
            Description: "This trip will remain",
            Destination: "Deletion Test",
            StartDate:   time.Now().Add(24 * time.Hour),
            EndDate:     time.Now().Add(48 * time.Hour),
            CreatedBy:   testUserID,
            Status:      types.TripStatusPlanning,
        }

        // Create both trips
        id1, err := tripDB.CreateTrip(ctx, trip1)
        require.NoError(t, err)
        id2, err := tripDB.CreateTrip(ctx, trip2)
        require.NoError(t, err)

        // Soft delete the first trip
        err = tripDB.SoftDeleteTrip(ctx, id1)
        require.NoError(t, err)

        // Verify first trip is not retrieved directly
        _, err = tripDB.GetTrip(ctx, id1)
        require.Error(t, err) // Should return NotFound error

        // Verify second trip is still retrievable
        fetchedTrip2, err := tripDB.GetTrip(ctx, id2)
        require.NoError(t, err)
        require.Equal(t, trip2.Name, fetchedTrip2.Name)

        // Verify first trip doesn't appear in list
        trips, err := tripDB.ListUserTrips(ctx, testUserID)
        require.NoError(t, err)
        for _, trip := range trips {
            require.NotEqual(t, id1, trip.ID, "Deleted trip should not appear in list")
        }
    })

    t.Run("Search Functionality", func(t *testing.T) {
        // Create trips with different characteristics for search testing
        searchTrips := []types.Trip{
            {
                Name:        "Paris Summer Trip",
                Description: "Summer vacation in Paris",
                Destination: "Paris",
                StartDate:   time.Now().Add(30 * 24 * time.Hour),  // 30 days from now
                EndDate:     time.Now().Add(37 * 24 * time.Hour),
                CreatedBy:   testUserID,
                Status:      types.TripStatusPlanning,
            },
            {
                Name:        "London Business Trip",
                Description: "Business meeting in London",
                Destination: "London",
                StartDate:   time.Now().Add(60 * 24 * time.Hour),  // 60 days from now
                EndDate:     time.Now().Add(63 * 24 * time.Hour),
                CreatedBy:   testUserID,
                Status:      types.TripStatusPlanning,
            },
            {
                Name:        "Paris Winter Trip",
                Description: "Winter in Paris",
                Destination: "Paris",
                StartDate:   time.Now().Add(180 * 24 * time.Hour), // 180 days from now
                EndDate:     time.Now().Add(187 * 24 * time.Hour),
                CreatedBy:   testUserID,
                Status:      types.TripStatusPlanning,
            },
        }

        // Create all test trips
        for _, trip := range searchTrips {
            _, err := tripDB.CreateTrip(ctx, trip)
            require.NoError(t, err)
        }

        t.Run("Search by Destination", func(t *testing.T) {
            criteria := types.TripSearchCriteria{
                Destination: "Paris",
            }
            results, err := tripDB.SearchTrips(ctx, criteria)
            require.NoError(t, err)
            require.Len(t, results, 2) // Should find both Paris trips
            for _, trip := range results {
                require.Equal(t, "Paris", trip.Destination)
            }
        })

        t.Run("Search by Date Range", func(t *testing.T) {
            criteria := types.TripSearchCriteria{
                StartDateFrom: time.Now().Add(20 * 24 * time.Hour),
                StartDateTo:   time.Now().Add(40 * 24 * time.Hour),
            }
            results, err := tripDB.SearchTrips(ctx, criteria)
            require.NoError(t, err)
            require.Len(t, results, 1) // Should find only the first Paris trip
            require.Equal(t, "Paris Summer Trip", results[0].Name)
        })

        t.Run("Search with Multiple Criteria", func(t *testing.T) {
            criteria := types.TripSearchCriteria{
                Destination:   "Paris",
                StartDateFrom: time.Now().Add(150 * 24 * time.Hour),
            }
            results, err := tripDB.SearchTrips(ctx, criteria)
            require.NoError(t, err)
            require.Len(t, results, 1) // Should find only the Paris Winter trip
            require.Equal(t, "Paris Winter Trip", results[0].Name)
        })

        t.Run("Search with No Results", func(t *testing.T) {
            criteria := types.TripSearchCriteria{
                Destination: "Tokyo", // No trips to Tokyo in test data
            }
            results, err := tripDB.SearchTrips(ctx, criteria)
            require.NoError(t, err)
            require.Empty(t, results)
        })
    })
}