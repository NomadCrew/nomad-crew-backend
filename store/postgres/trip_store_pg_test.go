package postgres

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Test constants and helpers
const (
	testAuthID = "auth0|123456789" // Auth0/Supabase style ID
)

// Helper function to convert Auth0 ID to UUID
func authIDtoUUID(authID string) string {
	// Create a deterministic UUID v5 from the auth ID
	// using DNS namespace for simplicity
	namespace := uuid.NameSpaceDNS
	return uuid.NewSHA1(namespace, []byte(authID)).String()
}

func setupTestDatabase(t *testing.T) (*pgxpool.Pool, func()) {
	// Skip if running on Windows
	if runtime.GOOS == "windows" {
		t.Skip("Skipping integration test on Windows - rootless Docker is not supported")
	}

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
		Started:          true,
	})
	require.NoError(t, err)

	mappedPort, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	connectionString := fmt.Sprintf("postgresql://test:test@%s:%s/testdb?sslmode=disable", host, mappedPort.Port())

	// Add a short delay to ensure the database is fully ready
	time.Sleep(2 * time.Second)

	pool, err := pgxpool.Connect(ctx, connectionString)
	if err != nil {
		// Enhanced error logging
		t.Logf("Database setup failed: %v", err)
		t.Logf("Current working directory: %s", mustGetwd(t))
		t.Logf("Container status: host=%s, port=%s", host, mappedPort.Port())

		logs, _ := container.Logs(ctx)
		if logs != nil {
			content, _ := io.ReadAll(logs)
			t.Logf("Container logs:\n%s", string(content))
		}

		require.NoError(t, err)
	}

	// Run migrations
	migrationSQL, err := os.ReadFile("../../db/migrations/000001_init.up.sql")
	require.NoError(t, err, "Failed to read migration file")
	_, err = pool.Exec(ctx, string(migrationSQL))
	require.NoError(t, err, "Failed to apply migration")

	return pool, func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %s", err)
		}
	}
}

func mustGetwd(t *testing.T) string {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	return dir
}

func TestPgTripStore_Integration(t *testing.T) {
	// Skip if running on Windows
	if runtime.GOOS == "windows" {
		t.Skip("Skipping integration test on Windows - rootless Docker is not supported")
	}

	pool, cleanup := setupTestDatabase(t)
	ctx := context.Background()
	tripStore := NewPgTripStore(pool)
	testUserUUID := authIDtoUUID(testAuthID)

	// Setup cleanup to run at the end
	defer func() {
		t.Run("Cleanup", func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			require.NoError(t, err)

			// Only rollback if commit hasn't happened
			var committed bool
			defer func() {
				if !committed {
					if err := tx.Rollback(ctx); err != nil {
						t.Logf("failed to rollback transaction: %v", err)
					}
				}
			}()

			tables := []string{
				"trip_memberships",
				"metadata",
				"locations",
				"expenses",
				"trips",
				"categories",
			}
			for _, table := range tables {
				_, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s", table))
				require.NoError(t, err)
			}

			err = tx.Commit(ctx)
			require.NoError(t, err)
			committed = true
		})
		cleanup()
	}()

	t.Run("Create and Get Trip", func(t *testing.T) {
		trip := types.Trip{
			Name:        "Test Trip",
			Description: "Test Description",
			Destination: types.Destination{Address: "Test Location"},
			StartDate:   time.Now().Add(24 * time.Hour),
			EndDate:     time.Now().Add(48 * time.Hour),
			CreatedBy:   testUserUUID,
			Status:      types.TripStatusPlanning,
		}

		// Test creation
		id, err := tripStore.CreateTrip(ctx, trip)
		require.NoError(t, err)
		require.True(t, isValidUUID(id), "Expected valid UUID")

		// Test retrieval
		fetchedTrip, err := tripStore.GetTrip(ctx, id)
		require.NoError(t, err)
		require.Equal(t, trip.Name, fetchedTrip.Name)
		require.Equal(t, trip.Description, fetchedTrip.Description)
		require.Equal(t, trip.Destination, fetchedTrip.Destination)
		require.Equal(t, testUserUUID, fetchedTrip.CreatedBy)
	})

	t.Run("Update Trip", func(t *testing.T) {
		trip := types.Trip{
			Name:        "Update Test Trip",
			Description: "Original Description",
			Destination: types.Destination{Address: "Original Location"},
			StartDate:   time.Now().Add(24 * time.Hour),
			EndDate:     time.Now().Add(48 * time.Hour),
			CreatedBy:   testUserUUID,
			Status:      types.TripStatusPlanning,
		}

		id, err := tripStore.CreateTrip(ctx, trip)
		require.NoError(t, err)

		update := types.TripUpdate{
			Name:        ptr("Updated Trip"),
			Description: ptr("Updated Description"),
			Destination: &types.Destination{Address: "Updated Location"},
		}

		_, err = tripStore.UpdateTrip(ctx, id, update)
		require.NoError(t, err)

		fetchedTrip, err := tripStore.GetTrip(ctx, id)
		require.NoError(t, err)
		require.Equal(t, *update.Name, fetchedTrip.Name)
		require.Equal(t, *update.Description, fetchedTrip.Description)
		require.Equal(t, update.Destination.Address, fetchedTrip.Destination.Address)
	})

	t.Run("List User Trips", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			trip := types.Trip{
				Name:        fmt.Sprintf("List Test Trip %d", i),
				Description: "Test Description",
				Destination: types.Destination{Address: "Test Location"},
				StartDate:   time.Now().Add(24 * time.Hour),
				EndDate:     time.Now().Add(48 * time.Hour),
				CreatedBy:   testUserUUID,
				Status:      types.TripStatusPlanning,
			}
			_, err := tripStore.CreateTrip(ctx, trip)
			require.NoError(t, err)
		}

		trips, err := tripStore.ListUserTrips(ctx, testUserUUID)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(trips), 3)

		for _, trip := range trips {
			require.Equal(t, testUserUUID, trip.CreatedBy)
		}
	})

	t.Run("Status Transitions", func(t *testing.T) {
		trip := types.Trip{
			Name:        "Status Test Trip",
			Description: "Testing status transitions",
			Destination: types.Destination{Address: "Test Location"},
			StartDate:   time.Now().Add(24 * time.Hour),
			EndDate:     time.Now().Add(48 * time.Hour),
			CreatedBy:   testUserUUID,
			Status:      types.TripStatusPlanning,
		}

		id, err := tripStore.CreateTrip(ctx, trip)
		require.NoError(t, err)

		// Transition to Active
		update := types.TripUpdate{
			Status: types.TripStatusActive,
		}
		_, err = tripStore.UpdateTrip(ctx, id, update)
		require.NoError(t, err, "Expected status transition to ACTIVE to succeed")

		fetchedTrip, err := tripStore.GetTrip(ctx, id)
		require.NoError(t, err)
		require.Equal(t, types.TripStatusActive, fetchedTrip.Status)
	})

	t.Run("Soft Delete Trip", func(t *testing.T) {
		trip1 := types.Trip{
			Name:        "Trip to Delete",
			Description: "Will be soft deleted",
			Destination: types.Destination{Address: "Test Location"},
			StartDate:   time.Now().Add(24 * time.Hour),
			EndDate:     time.Now().Add(48 * time.Hour),
			CreatedBy:   testUserUUID,
			Status:      types.TripStatusPlanning,
		}

		trip2 := types.Trip{
			Name:        "Trip to Keep",
			Description: "Will remain active",
			Destination: types.Destination{Address: "Test Location"},
			StartDate:   time.Now().Add(24 * time.Hour),
			EndDate:     time.Now().Add(48 * time.Hour),
			CreatedBy:   testUserUUID,
			Status:      types.TripStatusPlanning,
		}

		id1, err := tripStore.CreateTrip(ctx, trip1)
		require.NoError(t, err)
		id2, err := tripStore.CreateTrip(ctx, trip2)
		require.NoError(t, err)

		err = tripStore.SoftDeleteTrip(ctx, id1)
		require.NoError(t, err)

		_, err = tripStore.GetTrip(ctx, id1)
		require.Error(t, err) // Should return NotFound error

		fetchedTrip2, err := tripStore.GetTrip(ctx, id2)
		require.NoError(t, err)
		require.Equal(t, trip2.Name, fetchedTrip2.Name)

		trips, err := tripStore.ListUserTrips(ctx, testUserUUID)
		require.NoError(t, err)
		for _, trip := range trips {
			require.NotEqual(t, id1, trip.ID) // Deleted trip should not appear in list
		}
	})

	t.Run("Search Trips", func(t *testing.T) {
		// Create test trips
		searchTrips := []types.Trip{
			{
				Name:        "Paris Adventure",
				Description: "A trip to Paris",
				Destination: types.Destination{Address: "Paris, France"},
				StartDate:   time.Now().Add(30 * 24 * time.Hour),
				EndDate:     time.Now().Add(35 * 24 * time.Hour),
				CreatedBy:   testUserUUID,
				Status:      types.TripStatusPlanning,
			},
			{
				Name:        "Tokyo Exploration",
				Description: "Exploring Tokyo",
				Destination: types.Destination{Address: "Tokyo, Japan"},
				StartDate:   time.Now().Add(60 * 24 * time.Hour),
				EndDate:     time.Now().Add(65 * 24 * time.Hour),
				CreatedBy:   testUserUUID,
				Status:      types.TripStatusPlanning,
			},
			{
				Name:        "New York City",
				Description: "NYC trip",
				Destination: types.Destination{Address: "New York, USA"},
				StartDate:   time.Now().Add(90 * 24 * time.Hour),
				EndDate:     time.Now().Add(95 * 24 * time.Hour),
				CreatedBy:   testUserUUID,
				Status:      types.TripStatusPlanning,
			},
		}

		for _, trip := range searchTrips {
			_, err := tripStore.CreateTrip(ctx, trip)
			require.NoError(t, err)
		}

		// Test search by destination
		criteria := types.TripSearchCriteria{
			Destination: "Paris",
		}
		results, err := tripStore.SearchTrips(ctx, criteria)
		require.NoError(t, err)
		require.Len(t, results, 1)
		require.Equal(t, "Paris Adventure", results[0].Name)

		// Test search by date range
		criteria = types.TripSearchCriteria{
			StartDateFrom: time.Now().Add(20 * 24 * time.Hour),
			StartDateTo:   time.Now().Add(40 * 24 * time.Hour),
		}
		results, err = tripStore.SearchTrips(ctx, criteria)
		require.NoError(t, err)
		require.Len(t, results, 1)
		require.Equal(t, "Paris Adventure", results[0].Name)

		// Test search by date range (no results)
		criteria = types.TripSearchCriteria{
			StartDateFrom: time.Now().Add(150 * 24 * time.Hour),
		}
		results, err = tripStore.SearchTrips(ctx, criteria)
		require.NoError(t, err)
		require.Empty(t, results)
	})
}

func isValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}

func ptr(s string) *string {
	return &s
}
