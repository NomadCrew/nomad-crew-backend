package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/store/postgres"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

var (
	testPool  *pgxpool.Pool
	tripStore store.TripStore
)

func setupTestDB(t *testing.T) {
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
	migrationSQL, err := os.ReadFile("../../db/migrations/000001_init.up.sql")
	require.NoError(t, err)

	_, err = testPool.Exec(ctx, string(migrationSQL))
	require.NoError(t, err)

	tripStore = postgres.NewPgTripStore(testPool)
}

func teardownTestDB(t *testing.T) {
	if testPool != nil {
		testPool.Close()
	}
}

func TestTripStore(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	ctx := context.Background()
	testUserID := uuid.New().String()

	// Test CreateTrip
	t.Run("CreateTrip", func(t *testing.T) {
		trip := types.Trip{
			Name:        "Test Trip",
			Description: "Test Description",
			StartDate:   time.Now(),
			EndDate:     time.Now().Add(24 * time.Hour),
			Status:      "PLANNING",
			CreatedBy:   testUserID,
		}

		tripID, err := tripStore.CreateTrip(ctx, trip)
		require.NoError(t, err)
		assert.NotEmpty(t, tripID)
	})

	// Test GetTrip
	t.Run("GetTrip", func(t *testing.T) {
		trip := types.Trip{
			Name:        "Test Trip",
			Description: "Test Description",
			StartDate:   time.Now(),
			EndDate:     time.Now().Add(24 * time.Hour),
			Status:      "PLANNING",
			CreatedBy:   testUserID,
		}

		tripID, err := tripStore.CreateTrip(ctx, trip)
		require.NoError(t, err)

		retrievedTrip, err := tripStore.GetTrip(ctx, tripID)
		require.NoError(t, err)
		assert.Equal(t, tripID, retrievedTrip.ID)
		assert.Equal(t, trip.Name, retrievedTrip.Name)
	})

	// Test UpdateTrip
	t.Run("UpdateTrip", func(t *testing.T) {
		trip := types.Trip{
			Name:        "Test Trip",
			Description: "Test Description",
			StartDate:   time.Now(),
			EndDate:     time.Now().Add(24 * time.Hour),
			Status:      "PLANNING",
			CreatedBy:   testUserID,
		}

		tripID, err := tripStore.CreateTrip(ctx, trip)
		require.NoError(t, err)

		update := types.TripUpdate{
			Name: stringPtr("Updated Trip"),
		}
		updatedTrip, err := tripStore.UpdateTrip(ctx, tripID, update)
		require.NoError(t, err)
		assert.Equal(t, "Updated Trip", updatedTrip.Name)
	})

	// Test DeleteTrip
	t.Run("DeleteTrip", func(t *testing.T) {
		trip := types.Trip{
			Name:        "Test Trip",
			Description: "Test Description",
			StartDate:   time.Now(),
			EndDate:     time.Now().Add(24 * time.Hour),
			Status:      "PLANNING",
			CreatedBy:   testUserID,
		}

		tripID, err := tripStore.CreateTrip(ctx, trip)
		require.NoError(t, err)

		err = tripStore.SoftDeleteTrip(ctx, tripID)
		require.NoError(t, err)

		_, err = tripStore.GetTrip(ctx, tripID)
		assert.Error(t, err)
	})

	// Test ListUserTrips
	t.Run("ListUserTrips", func(t *testing.T) {
		trip := types.Trip{
			Name:        "Test Trip",
			Description: "Test Description",
			StartDate:   time.Now(),
			EndDate:     time.Now().Add(24 * time.Hour),
			Status:      "PLANNING",
			CreatedBy:   testUserID,
		}

		_, err := tripStore.CreateTrip(ctx, trip)
		require.NoError(t, err)

		trips, err := tripStore.ListUserTrips(ctx, testUserID)
		require.NoError(t, err)
		assert.NotEmpty(t, trips)
	})

	// Test SearchTrips
	t.Run("SearchTrips", func(t *testing.T) {
		criteria := types.TripSearchCriteria{
			UserID:        testUserID,
			StartDateFrom: time.Now().Add(-24 * time.Hour),
			StartDateTo:   time.Now().Add(24 * time.Hour),
			Limit:         10,
			Offset:        0,
		}
		trips, err := tripStore.SearchTrips(ctx, criteria)
		require.NoError(t, err)
		assert.NotNil(t, trips)
	})

	// Test AddMember
	t.Run("AddMember", func(t *testing.T) {
		trip := types.Trip{
			Name:        "Test Trip",
			Description: "Test Description",
			StartDate:   time.Now(),
			EndDate:     time.Now().Add(24 * time.Hour),
			Status:      "PLANNING",
			CreatedBy:   testUserID,
		}

		tripID, err := tripStore.CreateTrip(ctx, trip)
		require.NoError(t, err)

		memberID := uuid.New().String()
		membership := &types.TripMembership{
			TripID: tripID,
			UserID: memberID,
			Role:   types.MemberRoleMember,
			Status: types.MembershipStatusActive,
		}
		err = tripStore.AddMember(ctx, membership)
		require.NoError(t, err)

		members, err := tripStore.GetTripMembers(ctx, tripID)
		require.NoError(t, err)
		assert.NotEmpty(t, members)
	})

	// Test UpdateMemberRole
	t.Run("UpdateMemberRole", func(t *testing.T) {
		trip := types.Trip{
			Name:        "Test Trip",
			Description: "Test Description",
			StartDate:   time.Now(),
			EndDate:     time.Now().Add(24 * time.Hour),
			Status:      "PLANNING",
			CreatedBy:   testUserID,
		}

		tripID, err := tripStore.CreateTrip(ctx, trip)
		require.NoError(t, err)

		memberID := uuid.New().String()
		membership := &types.TripMembership{
			TripID: tripID,
			UserID: memberID,
			Role:   types.MemberRoleMember,
			Status: types.MembershipStatusActive,
		}
		err = tripStore.AddMember(ctx, membership)
		require.NoError(t, err)

		err = tripStore.UpdateMemberRole(ctx, tripID, memberID, types.MemberRoleOwner)
		require.NoError(t, err)

		members, err := tripStore.GetTripMembers(ctx, tripID)
		require.NoError(t, err)
		assert.Equal(t, types.MemberRoleOwner, members[0].Role)
	})

	// Test RemoveMember
	t.Run("RemoveMember", func(t *testing.T) {
		trip := types.Trip{
			Name:        "Test Trip",
			Description: "Test Description",
			StartDate:   time.Now(),
			EndDate:     time.Now().Add(24 * time.Hour),
			Status:      "PLANNING",
			CreatedBy:   testUserID,
		}

		tripID, err := tripStore.CreateTrip(ctx, trip)
		require.NoError(t, err)

		memberID := uuid.New().String()
		membership := &types.TripMembership{
			TripID: tripID,
			UserID: memberID,
			Role:   types.MemberRoleMember,
			Status: types.MembershipStatusActive,
		}
		err = tripStore.AddMember(ctx, membership)
		require.NoError(t, err)

		err = tripStore.RemoveMember(ctx, tripID, memberID)
		require.NoError(t, err)

		members, err := tripStore.GetTripMembers(ctx, tripID)
		require.NoError(t, err)

		// Check that the specific member was removed
		for _, member := range members {
			require.NotEqual(t, memberID, member.UserID, "Member should have been removed")
		}
	})

	// Test Transaction Handling
	t.Run("TransactionHandling", func(t *testing.T) {
		tx, err := tripStore.BeginTx(ctx)
		require.NoError(t, err)

		trip := types.Trip{
			Name:        "Test Trip",
			Description: "Test Description",
			StartDate:   time.Now(),
			EndDate:     time.Now().Add(24 * time.Hour),
			Status:      "PLANNING",
			CreatedBy:   testUserID,
		}

		tripID, err := tripStore.CreateTrip(ctx, trip)
		require.NoError(t, err)

		err = tx.Commit()
		require.NoError(t, err)

		retrievedTrip, err := tripStore.GetTrip(ctx, tripID)
		require.NoError(t, err)
		assert.Equal(t, tripID, retrievedTrip.ID)
	})
}

func stringPtr(s string) *string {
	return &s
}
