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

	// Insert a test user for trip foreign key constraints
	testUserUUID := authIDtoUUID(testAuthID) // Ensure this is the same as used in tests
	_, err = pool.Exec(ctx, "INSERT INTO users (id, supabase_id, email, username, name, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, NOW(), NOW())",
		testUserUUID, uuid.New().String(), "testuser@example.com", "testuser1", "Test User Integration")
	require.NoError(t, err, "Failed to insert test user")

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
				"locations",
				"expenses",
				"trips",
				"categories",
				"users", // Added users table for cleanup
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

	// Test sub-suites
	t.Run("Create and Get Trip", func(t *testing.T) {
		trip := types.Trip{
			Name:                 "Test Trip",
			Description:          "Test Description",
			DestinationAddress:   stringPtr("Test Location"),
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
		require.Equal(t, trip.DestinationAddress, fetchedTrip.DestinationAddress)
		require.Equal(t, trip.DestinationLatitude, fetchedTrip.DestinationLatitude)
		require.Equal(t, trip.DestinationLongitude, fetchedTrip.DestinationLongitude)
		require.Equal(t, &testUserUUID, fetchedTrip.CreatedBy)
	})

	t.Run("Update Trip", func(t *testing.T) {
		trip := types.Trip{
			Name:                 "Update Test Trip",
			Description:          "Original Description",
			DestinationAddress:   stringPtr("Original Location"),
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
			Name:                 stringPtr("Updated Trip"),
			Description:          stringPtr("Updated Description"),
			DestinationAddress:   stringPtr("Updated Location"),
			DestinationLatitude:  float64Ptr(35.0),
			DestinationLongitude: float64Ptr(45.0),
		}

		_, err = tripStore.UpdateTrip(ctx, id, update)
		require.NoError(t, err)

		fetchedTrip, err := tripStore.GetTrip(ctx, id)
		require.NoError(t, err)
		require.Equal(t, *update.Name, fetchedTrip.Name)
		require.Equal(t, *update.Description, fetchedTrip.Description)
		require.NotNil(t, fetchedTrip.DestinationAddress)
		require.Equal(t, *update.DestinationAddress, *fetchedTrip.DestinationAddress)
		require.Equal(t, *update.DestinationLatitude, fetchedTrip.DestinationLatitude)
		require.Equal(t, *update.DestinationLongitude, fetchedTrip.DestinationLongitude)
	})

	t.Run("List User Trips", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			trip := types.Trip{
				Name:                 fmt.Sprintf("List Test Trip %d", i),
				Description:          "Test Description",
				DestinationAddress:   stringPtr("Test Location"),
				DestinationLatitude:  float64(i + 10.0),
				DestinationLongitude: float64(i + 20.0),
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
			require.Equal(t, &testUserUUID, trip.CreatedBy)
		}
	})

	t.Run("Status Transitions", func(t *testing.T) {
		trip := types.Trip{
			Name:                 "Status Test Trip",
			Description:          "Testing status transitions",
			DestinationAddress:   stringPtr("Test Location"),
			DestinationLatitude:  50.0,
			DestinationLongitude: 60.0,
			StartDate:            time.Now().Add(24 * time.Hour),
			EndDate:              time.Now().Add(48 * time.Hour),
			CreatedBy:            &testUserUUID,
			Status:               types.TripStatusPlanning,
		}

		id, err := tripStore.CreateTrip(ctx, trip)
		require.NoError(t, err)

		// Transition to Active
		activeStatus := types.TripStatusActive
		updateActive := types.TripUpdate{
			Status: &activeStatus,
		}
		_, err = tripStore.UpdateTrip(ctx, id, updateActive)
		require.NoError(t, err)
		fetchedTrip, _ := tripStore.GetTrip(ctx, id)
		require.Equal(t, types.TripStatusActive, fetchedTrip.Status)

		// Transition to Completed
		completedStatus := types.TripStatusCompleted
		updateCompleted := types.TripUpdate{
			Status: &completedStatus,
		}
		_, err = tripStore.UpdateTrip(ctx, id, updateCompleted)
		require.NoError(t, err)
		fetchedTrip, _ = tripStore.GetTrip(ctx, id)
		require.Equal(t, types.TripStatusCompleted, fetchedTrip.Status)
	})

	t.Run("Soft Delete Trip", func(t *testing.T) {
		trip1 := types.Trip{
			Name:                 "Trip to Delete",
			Description:          "Will be soft deleted",
			DestinationAddress:   stringPtr("Test Location"),
			DestinationLatitude:  10.0,
			DestinationLongitude: 20.0,
			StartDate:            time.Now().Add(24 * time.Hour),
			EndDate:              time.Now().Add(48 * time.Hour),
			CreatedBy:            &testUserUUID,
			Status:               types.TripStatusPlanning,
		}

		trip2 := types.Trip{
			Name:                 "Trip to Keep",
			Description:          "Will remain active",
			DestinationAddress:   stringPtr("Test Location"),
			DestinationLatitude:  10.0,
			DestinationLongitude: 20.0,
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
		// Create some trips for searching
		trip1 := types.Trip{
			Name:                 "Paris Adventure",
			Description:          "Exploring the city of lights",
			DestinationAddress:   stringPtr("Paris, France"),
			DestinationLatitude:  48.8566,
			DestinationLongitude: 2.3522,
			StartDate:            time.Now().AddDate(0, 1, 0), // Next month
			EndDate:              time.Now().AddDate(0, 1, 7),
			CreatedBy:            &testUserUUID,
			Status:               types.TripStatusPlanning,
		}
		_, err := tripStore.CreateTrip(ctx, trip1)
		require.NoError(t, err)

		trip2 := types.Trip{
			Name:                 "Beach Getaway",
			Description:          "Relaxing by the sea",
			DestinationAddress:   stringPtr("Malibu, California"),
			DestinationLatitude:  34.0259,
			DestinationLongitude: -118.7798,
			StartDate:            time.Now().AddDate(0, 2, 0), // Two months from now
			EndDate:              time.Now().AddDate(0, 2, 10),
			CreatedBy:            &testUserUUID,
			Status:               types.TripStatusActive,
		}
		_, err = tripStore.CreateTrip(ctx, trip2)
		require.NoError(t, err)

		// Search by destination name (partial match)
		criteria1 := types.TripSearchCriteria{
			Destination: "Paris",
		}
		results1, err := tripStore.SearchTrips(ctx, criteria1)
		require.NoError(t, err)
		require.Len(t, results1, 1)
		require.Equal(t, "Paris Adventure", results1[0].Name)

		// Search by date range
		startDate := time.Now().AddDate(0, 1, -5) // Slightly before trip1 starts
		endDate := time.Now().AddDate(0, 1, 5)    // During trip1
		criteria2 := types.TripSearchCriteria{
			StartDateFrom: startDate,
			StartDateTo:   endDate,
		}
		results2, err := tripStore.SearchTrips(ctx, criteria2)
		require.NoError(t, err)
		require.Len(t, results2, 1)
		require.Equal(t, "Paris Adventure", results2[0].Name)

		// Search by status
		criteria3 := types.TripSearchCriteria{
			Destination: "Malibu",
		}
		results3, err := tripStore.SearchTrips(ctx, criteria3)
		require.NoError(t, err)
		require.Len(t, results3, 1)
		require.Equal(t, "Beach Getaway", results3[0].Name)
	})

	t.Run("Trip Membership", func(t *testing.T) {
		trip := types.Trip{
			Name:                 "Membership Test Trip",
			Description:          "Testing membership functions",
			DestinationAddress:   stringPtr("Membership Location"),
			DestinationLatitude:  70.0,
			DestinationLongitude: 80.0,
			StartDate:            time.Now().Add(24 * time.Hour),
			EndDate:              time.Now().Add(48 * time.Hour),
			CreatedBy:            &testUserUUID,
			Status:               types.TripStatusPlanning,
		}
		tripID, err := tripStore.CreateTrip(ctx, trip)
		require.NoError(t, err)

		memberUUID := authIDtoUUID("auth0|member123")

		// Insert the member user into the users table first
		_, err = pool.Exec(ctx, "INSERT INTO users (id, supabase_id, email, username, name, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, NOW(), NOW())",
			memberUUID, uuid.New().String(), "member@example.com", "member123", "Test Member")
		require.NoError(t, err, "Failed to insert test member user")

		// Add member
		membership := &types.TripMembership{
			TripID: tripID,
			UserID: memberUUID,
			Role:   types.MemberRoleMember,
			Status: types.MembershipStatusActive,
		}
		err = tripStore.AddMember(ctx, membership)
		require.NoError(t, err)

		// Get user role
		role, err := tripStore.GetUserRole(ctx, tripID, memberUUID)
		require.NoError(t, err)
		require.Equal(t, types.MemberRoleMember, role)

		// Update member role
		err = tripStore.UpdateMemberRole(ctx, tripID, memberUUID, types.MemberRoleAdmin)
		require.NoError(t, err)
		role, _ = tripStore.GetUserRole(ctx, tripID, memberUUID)
		require.Equal(t, types.MemberRoleAdmin, role)

		// Get trip members
		members, err := tripStore.GetTripMembers(ctx, tripID)
		require.NoError(t, err)
		// Owner (creator) + added member
		require.Len(t, members, 2)

		// Remove member
		err = tripStore.RemoveMember(ctx, tripID, memberUUID)
		require.NoError(t, err)
		_, err = tripStore.GetUserRole(ctx, tripID, memberUUID)
		// Expect an error because the user should no longer be an active member
		require.Error(t, err)
	})
}

// Helper to check if a string is a valid UUID
func isValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}

// Helper to get a pointer to a string
func stringPtr(s string) *string {
	return &s
}

// Helper to get a pointer to a float64
func float64Ptr(f float64) *float64 {
	return &f
}
