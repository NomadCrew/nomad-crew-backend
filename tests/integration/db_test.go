package integration

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/NomadCrew/nomad-crew-backend/db"
	"github.com/NomadCrew/nomad-crew-backend/store/postgres"
	"github.com/NomadCrew/nomad-crew-backend/tests/testutil"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
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

func setupTestDatabase(t *testing.T) (*db.DatabaseClient, func()) {
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

	dbClient, err := db.SetupTestDB(connectionString)
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

	return dbClient, func() {
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

func TestTripStore_Integration(t *testing.T) {
	// Skip if running on Windows
	if runtime.GOOS == "windows" {
		t.Skip("Skipping integration test on Windows - rootless Docker is not supported")
	}

	dbClient, cleanup := setupTestDatabase(t)
	ctx := context.Background()
	tripStore := postgres.NewPgTripStore(dbClient.GetPool())
	testUserUUID := authIDtoUUID(testAuthID)

	// Insert the main test user for this test suite
	err := testutil.InsertTestUser(ctx, dbClient.GetPool(), uuid.MustParse(testUserUUID), "integration-testuser@example.com", "testuser1")
	require.NoError(t, err, "Failed to insert main test user for TestTripStore_Integration")

	// Setup cleanup to run at the end
	defer func() {
		t.Run("Cleanup", func(t *testing.T) {
			conn := dbClient.GetPool()
			tx, err := conn.Begin(ctx)
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
				"locations",
				"expenses",
				"trips",
				"categories",
				"user_profiles",
				"auth.users",
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
			Name:                 "Test Trip",
			Description:          "Test Description",
			DestinationAddress:   ptr("Test Location Address"),
			DestinationPlaceID:   ptr("test-place-id-create"),
			DestinationLatitude:  10.0,
			DestinationLongitude: 20.0,
			StartDate:            time.Now().Add(24 * time.Hour),
			EndDate:              time.Now().Add(48 * time.Hour),
			CreatedBy:            &testUserUUID,
			Status:               types.TripStatusPlanning,
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
		require.Equal(t, *trip.DestinationAddress, *fetchedTrip.DestinationAddress)
		require.Equal(t, *trip.DestinationPlaceID, *fetchedTrip.DestinationPlaceID)
		require.Equal(t, trip.DestinationLatitude, fetchedTrip.DestinationLatitude)
		require.Equal(t, trip.DestinationLongitude, fetchedTrip.DestinationLongitude)
		require.Equal(t, *trip.CreatedBy, *fetchedTrip.CreatedBy)
	})

	t.Run("Update Trip", func(t *testing.T) {
		trip := types.Trip{
			Name:                 "Update Test Trip",
			Description:          "Original Description",
			DestinationAddress:   ptr("Original Location Address"),
			DestinationPlaceID:   ptr("original-place-id-update"),
			DestinationLatitude:  30.0,
			DestinationLongitude: 40.0,
			StartDate:            time.Now().Add(24 * time.Hour),
			EndDate:              time.Now().Add(48 * time.Hour),
			CreatedBy:            &testUserUUID,
			Status:               types.TripStatusPlanning,
		}

		id, err := tripStore.CreateTrip(ctx, trip)
		require.NoError(t, err)

		update := types.TripUpdate{
			Name:                 ptr("Updated Trip"),
			Description:          ptr("Updated Description"),
			DestinationAddress:   ptr("Updated Location Address"),
			DestinationPlaceID:   ptr("updated-place-id"),
			DestinationLatitude:  float64Ptr(50.0),
			DestinationLongitude: float64Ptr(60.0),
		}

		updatedTripFull, err := tripStore.UpdateTrip(ctx, id, update)
		require.NoError(t, err)
		require.NotNil(t, updatedTripFull)

		fetchedTrip, err := tripStore.GetTrip(ctx, id)
		require.NoError(t, err)
		require.Equal(t, *update.Name, fetchedTrip.Name)
		require.Equal(t, *update.Description, fetchedTrip.Description)
		require.Equal(t, *update.DestinationAddress, *fetchedTrip.DestinationAddress)
		require.Equal(t, *update.DestinationPlaceID, *fetchedTrip.DestinationPlaceID)
		require.Equal(t, *update.DestinationLatitude, fetchedTrip.DestinationLatitude)
		require.Equal(t, *update.DestinationLongitude, fetchedTrip.DestinationLongitude)
	})

	t.Run("List User Trips", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			trip := types.Trip{
				Name:                 fmt.Sprintf("List Test Trip %d", i),
				Description:          "Test Description",
				DestinationAddress:   ptr(fmt.Sprintf("Test Location Address List %d", i)),
				DestinationPlaceID:   ptr(fmt.Sprintf("list-place-id-%d", i)),
				DestinationLatitude:  11.0 + float64(i),
				DestinationLongitude: 21.0 + float64(i),
				StartDate:            time.Now().Add(24 * time.Hour),
				EndDate:              time.Now().Add(48 * time.Hour),
				CreatedBy:            &testUserUUID,
				Status:               types.TripStatusPlanning,
			}
			_, err := tripStore.CreateTrip(ctx, trip)
			require.NoError(t, err)
		}

		trips, err := tripStore.ListUserTrips(ctx, testUserUUID)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(trips), 3)

		for _, trip := range trips {
			require.NotNil(t, trip.CreatedBy)
			require.Equal(t, testUserUUID, *trip.CreatedBy)
		}
	})

	t.Run("Status Transitions", func(t *testing.T) {
		trip := types.Trip{
			Name:                 "Status Test Trip",
			Description:          "Testing status transitions",
			DestinationAddress:   ptr("Status Test Location"),
			DestinationPlaceID:   ptr("status-place-id"),
			DestinationLatitude:  12.0,
			DestinationLongitude: 22.0,
			StartDate:            time.Now().Add(24 * time.Hour),
			EndDate:              time.Now().Add(48 * time.Hour),
			CreatedBy:            &testUserUUID,
			Status:               types.TripStatusPlanning,
		}

		id, err := tripStore.CreateTrip(ctx, trip)
		require.NoError(t, err)

		// Transition to Active
		activeStatus := types.TripStatusActive
		update := types.TripUpdate{
			Status: &activeStatus,
		}
		_, err = tripStore.UpdateTrip(ctx, id, update)
		require.NoError(t, err, "Expected status transition to ACTIVE to succeed")

		fetchedTrip, err := tripStore.GetTrip(ctx, id)
		require.NoError(t, err)
		require.Equal(t, types.TripStatusActive, fetchedTrip.Status)
	})

	t.Run("Soft Delete Functionality", func(t *testing.T) {
		trip1 := types.Trip{
			Name:                 "Trip to Delete",
			Description:          "This trip will be deleted",
			DestinationAddress:   ptr("Deletion Test Address 1"),
			DestinationPlaceID:   ptr("delete-place-id-1"),
			DestinationLatitude:  13.0,
			DestinationLongitude: 23.0,
			StartDate:            time.Now().Add(24 * time.Hour),
			EndDate:              time.Now().Add(48 * time.Hour),
			CreatedBy:            &testUserUUID,
			Status:               types.TripStatusPlanning,
		}

		trip2 := types.Trip{
			Name:                 "Trip to Keep",
			Description:          "This trip will remain",
			DestinationAddress:   ptr("Deletion Test Address 2"),
			DestinationPlaceID:   ptr("delete-place-id-2"),
			DestinationLatitude:  14.0,
			DestinationLongitude: 24.0,
			StartDate:            time.Now().Add(24 * time.Hour),
			EndDate:              time.Now().Add(48 * time.Hour),
			CreatedBy:            &testUserUUID,
			Status:               types.TripStatusPlanning,
		}

		id1, err := tripStore.CreateTrip(ctx, trip1)
		require.NoError(t, err)
		id2, err := tripStore.CreateTrip(ctx, trip2)
		require.NoError(t, err)

		err = tripStore.SoftDeleteTrip(ctx, id1)
		require.NoError(t, err)

		_, err = tripStore.GetTrip(ctx, id1)
		require.Error(t, err)

		fetchedTrip2, err := tripStore.GetTrip(ctx, id2)
		require.NoError(t, err)
		require.Equal(t, trip2.Name, fetchedTrip2.Name)

		trips, err := tripStore.ListUserTrips(ctx, testUserUUID)
		require.NoError(t, err)
		for _, trip := range trips {
			require.NotEqual(t, id1, trip.ID, "Deleted trip should not appear in list")
		}
	})

	t.Run("Search Functionality", func(t *testing.T) {
		searchTrips := []types.Trip{
			{
				Name:                 "Paris Summer Trip",
				Description:          "Summer vacation in Paris",
				DestinationAddress:   ptr("Paris"),
				DestinationPlaceID:   ptr("paris-summer-place-id"),
				DestinationLatitude:  48.8566,
				DestinationLongitude: 2.3522,
				StartDate:            time.Now().Add(30 * 24 * time.Hour),
				EndDate:              time.Now().Add(37 * 24 * time.Hour),
				CreatedBy:            &testUserUUID,
				Status:               types.TripStatusPlanning,
			},
			{
				Name:                 "London Business Trip",
				Description:          "Business meeting in London",
				DestinationAddress:   ptr("London"),
				DestinationPlaceID:   ptr("london-business-place-id"),
				DestinationLatitude:  51.5074,
				DestinationLongitude: 0.1278,
				StartDate:            time.Now().Add(60 * 24 * time.Hour),
				EndDate:              time.Now().Add(63 * 24 * time.Hour),
				CreatedBy:            &testUserUUID,
				Status:               types.TripStatusPlanning,
			},
			{
				Name:                 "Paris Winter Trip",
				Description:          "Winter in Paris",
				DestinationAddress:   ptr("Paris"),
				DestinationPlaceID:   ptr("paris-winter-place-id"),
				DestinationLatitude:  48.8566,
				DestinationLongitude: 2.3522,
				StartDate:            time.Now().Add(180 * 24 * time.Hour),
				EndDate:              time.Now().Add(187 * 24 * time.Hour),
				CreatedBy:            &testUserUUID,
				Status:               types.TripStatusPlanning,
			},
		}

		for _, trip := range searchTrips {
			_, err := tripStore.CreateTrip(ctx, trip)
			require.NoError(t, err)
		}

		t.Run("Search by Destination", func(t *testing.T) {
			criteria := types.TripSearchCriteria{
				Destination: "Paris",
			}
			results, err := tripStore.SearchTrips(ctx, criteria)
			require.NoError(t, err)
			require.Len(t, results, 2)
			for _, trip := range results {
				require.NotNil(t, trip.DestinationAddress)
				require.Equal(t, "Paris", *trip.DestinationAddress)
			}
		})

		t.Run("Search by Date Range", func(t *testing.T) {
			criteria := types.TripSearchCriteria{
				StartDateFrom: time.Now().Add(20 * 24 * time.Hour),
				StartDateTo:   time.Now().Add(40 * 24 * time.Hour),
			}
			results, err := tripStore.SearchTrips(ctx, criteria)
			require.NoError(t, err)
			require.Len(t, results, 1)
			require.Equal(t, "Paris Summer Trip", results[0].Name)
		})

		t.Run("Search with Multiple Criteria", func(t *testing.T) {
			criteria := types.TripSearchCriteria{
				Destination:   "Paris",
				StartDateFrom: time.Now().Add(150 * 24 * time.Hour),
			}
			results, err := tripStore.SearchTrips(ctx, criteria)
			require.NoError(t, err)
			require.Len(t, results, 1)
			require.Equal(t, "Paris Winter Trip", results[0].Name)
		})

		t.Run("Search with No Results", func(t *testing.T) {
			criteria := types.TripSearchCriteria{
				Destination: "Tokyo",
			}
			results, err := tripStore.SearchTrips(ctx, criteria)
			require.NoError(t, err)
			require.Empty(t, results)
		})
	})
}

// Helper function to validate UUIDs
func isValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil && strings.Contains(u, "-")
}

// Helper to create string pointers
func ptr(s string) *string {
	return &s
}

func float64Ptr(f float64) *float64 {
	return &f
}
