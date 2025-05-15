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
			Name:                 "Test Trip Create",
			Description:          "Test Description Create",
			DestinationAddress:   stringPtr("Address Create"),
			DestinationPlaceID:   stringPtr("place-id-create"),
			DestinationLatitude:  1.0,
			DestinationLongitude: 1.0,
			StartDate:            time.Now(),
			EndDate:              time.Now().Add(24 * time.Hour),
			Status:               types.TripStatusPlanning,
			CreatedBy:            &testUserID,
		}

		tripID, err := tripStore.CreateTrip(ctx, trip)
		require.NoError(t, err)
		assert.NotEmpty(t, tripID)
	})

	// Test GetTrip
	t.Run("GetTrip", func(t *testing.T) {
		trip := types.Trip{
			Name:                 "Test Trip Get",
			Description:          "Test Description Get",
			DestinationAddress:   stringPtr("Address Get"),
			DestinationPlaceID:   stringPtr("place-id-get"),
			DestinationLatitude:  2.0,
			DestinationLongitude: 2.0,
			StartDate:            time.Now(),
			EndDate:              time.Now().Add(24 * time.Hour),
			Status:               types.TripStatusPlanning,
			CreatedBy:            &testUserID,
		}

		tripID, err := tripStore.CreateTrip(ctx, trip)
		require.NoError(t, err)

		retrievedTrip, err := tripStore.GetTrip(ctx, tripID)
		require.NoError(t, err)
		assert.Equal(t, tripID, retrievedTrip.ID)
		assert.Equal(t, trip.Name, retrievedTrip.Name)
		assert.Equal(t, *trip.DestinationAddress, *retrievedTrip.DestinationAddress)
		assert.NotNil(t, retrievedTrip.CreatedBy)
		assert.Equal(t, testUserID, *retrievedTrip.CreatedBy)
	})

	// Test UpdateTrip
	t.Run("UpdateTrip", func(t *testing.T) {
		trip := types.Trip{
			Name:                 "Test Trip Update Initial",
			Description:          "Test Description Update Initial",
			DestinationAddress:   stringPtr("Address Update Initial"),
			DestinationPlaceID:   stringPtr("place-id-update-initial"),
			DestinationLatitude:  3.0,
			DestinationLongitude: 3.0,
			StartDate:            time.Now(),
			EndDate:              time.Now().Add(24 * time.Hour),
			Status:               types.TripStatusPlanning,
			CreatedBy:            &testUserID,
		}

		tripID, err := tripStore.CreateTrip(ctx, trip)
		require.NoError(t, err)

		update := types.TripUpdate{
			Name:               stringPtr("Updated Trip Name"),
			Description:        stringPtr("Updated Trip Description"),
			DestinationAddress: stringPtr("Updated Address"),
		}
		updatedTrip, err := tripStore.UpdateTrip(ctx, tripID, update)
		require.NoError(t, err)
		assert.Equal(t, "Updated Trip Name", updatedTrip.Name)
		assert.Equal(t, "Updated Trip Description", updatedTrip.Description)
		assert.Equal(t, "Updated Address", *updatedTrip.DestinationAddress)
	})

	// Test DeleteTrip
	t.Run("DeleteTrip", func(t *testing.T) {
		trip := types.Trip{
			Name:                 "Test Trip Delete",
			Description:          "Test Description Delete",
			DestinationAddress:   stringPtr("Address Delete"),
			DestinationPlaceID:   stringPtr("place-id-delete"),
			DestinationLatitude:  4.0,
			DestinationLongitude: 4.0,
			StartDate:            time.Now(),
			EndDate:              time.Now().Add(24 * time.Hour),
			Status:               types.TripStatusPlanning,
			CreatedBy:            &testUserID,
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
		listTestUserID := uuid.New().String()
		trip := types.Trip{
			Name:                 "Test Trip List",
			Description:          "Test Description List",
			DestinationAddress:   stringPtr("Address List"),
			DestinationPlaceID:   stringPtr("place-id-list"),
			DestinationLatitude:  5.0,
			DestinationLongitude: 5.0,
			StartDate:            time.Now(),
			EndDate:              time.Now().Add(24 * time.Hour),
			Status:               types.TripStatusPlanning,
			CreatedBy:            &listTestUserID,
		}

		_, err := tripStore.CreateTrip(ctx, trip)
		require.NoError(t, err)

		trips, err := tripStore.ListUserTrips(ctx, listTestUserID)
		require.NoError(t, err)
		assert.NotEmpty(t, trips)
		assert.Len(t, trips, 1)
		assert.Equal(t, listTestUserID, *trips[0].CreatedBy)
	})

	// Test SearchTrips
	t.Run("SearchTrips", func(t *testing.T) {
		searchUserID := uuid.New().String()
		searchTrip := types.Trip{
			Name:                 "Searchable Trip",
			DestinationAddress:   stringPtr("Searchable Address"),
			DestinationPlaceID:   stringPtr("searchable-place-id"),
			DestinationLatitude:  6.0,
			DestinationLongitude: 6.0,
			StartDate:            time.Now(),
			EndDate:              time.Now().Add(5 * 24 * time.Hour),
			Status:               types.TripStatusPlanning,
			CreatedBy:            &searchUserID,
		}
		_, err := tripStore.CreateTrip(ctx, searchTrip)
		require.NoError(t, err)

		criteria := types.TripSearchCriteria{
			UserID:        searchUserID,
			StartDateFrom: time.Now().Add(-24 * time.Hour),
			StartDateTo:   time.Now().Add(24 * time.Hour),
			Limit:         10,
			Offset:        0,
			Destination:   "Searchable Address",
		}
		trips, err := tripStore.SearchTrips(ctx, criteria)
		require.NoError(t, err)
		assert.NotNil(t, trips)
		assert.GreaterOrEqual(t, len(trips), 1, "Should find at least the created searchable trip")
		if len(trips) > 0 {
			assert.Equal(t, "Searchable Trip", trips[0].Name)
		}
	})

	// Test AddMember
	t.Run("AddMember", func(t *testing.T) {
		addMemberUserID := uuid.New().String()
		trip := types.Trip{
			Name:                 "Test Trip AddMember",
			Description:          "Test Description AddMember",
			DestinationAddress:   stringPtr("Address AddMember"),
			DestinationPlaceID:   stringPtr("place-id-addmember"),
			DestinationLatitude:  7.0,
			DestinationLongitude: 7.0,
			StartDate:            time.Now(),
			EndDate:              time.Now().Add(24 * time.Hour),
			Status:               types.TripStatusPlanning,
			CreatedBy:            &addMemberUserID,
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
		foundAddedMember := false
		for _, m := range members {
			if m.UserID == memberID {
				foundAddedMember = true
				break
			}
		}
		assert.True(t, foundAddedMember, "Added member not found in trip members list")
	})

	// Test UpdateMemberRole
	t.Run("UpdateMemberRole", func(t *testing.T) {
		updateRoleUserID := uuid.New().String()
		trip := types.Trip{
			Name:                 "Test Trip UpdateMemberRole",
			Description:          "Test Description UpdateMemberRole",
			DestinationAddress:   stringPtr("Address UpdateMemberRole"),
			DestinationPlaceID:   stringPtr("place-id-updaterole"),
			DestinationLatitude:  8.0,
			DestinationLongitude: 8.0,
			StartDate:            time.Now(),
			EndDate:              time.Now().Add(24 * time.Hour),
			Status:               types.TripStatusPlanning,
			CreatedBy:            &updateRoleUserID,
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
		foundUpdatedMember := false
		for _, m := range members {
			if m.UserID == memberID && m.Role == types.MemberRoleOwner {
				foundUpdatedMember = true
				break
			}
		}
		assert.True(t, foundUpdatedMember, "Member role was not updated to Owner")
	})

	// Test RemoveMember
	t.Run("RemoveMember", func(t *testing.T) {
		removeMemberUserID := uuid.New().String()
		trip := types.Trip{
			Name:                 "Test Trip RemoveMember",
			Description:          "Test Description RemoveMember",
			DestinationAddress:   stringPtr("Address RemoveMember"),
			DestinationPlaceID:   stringPtr("place-id-removemember"),
			DestinationLatitude:  9.0,
			DestinationLongitude: 9.0,
			StartDate:            time.Now(),
			EndDate:              time.Now().Add(24 * time.Hour),
			Status:               types.TripStatusPlanning,
			CreatedBy:            &removeMemberUserID,
		}

		tripID, err := tripStore.CreateTrip(ctx, trip)
		require.NoError(t, err)

		memberToRemoveID := uuid.New().String()
		membership := &types.TripMembership{
			TripID: tripID,
			UserID: memberToRemoveID,
			Role:   types.MemberRoleMember,
			Status: types.MembershipStatusActive,
		}
		err = tripStore.AddMember(ctx, membership)
		require.NoError(t, err)

		err = tripStore.RemoveMember(ctx, tripID, memberToRemoveID)
		require.NoError(t, err)

		members, err := tripStore.GetTripMembers(ctx, tripID)
		require.NoError(t, err)

		foundRemovedMember := false
		for _, m := range members {
			if m.UserID == memberToRemoveID {
				foundRemovedMember = true
				break
			}
		}
		assert.False(t, foundRemovedMember, "Removed member still found in trip members list")
	})

	// Test Transaction Handling
	t.Run("TransactionHandling", func(t *testing.T) {
		tx, err := tripStore.BeginTx(ctx)
		require.NoError(t, err)

		trip := types.Trip{
			Name:                 "Test Trip Transaction",
			Description:          "Test Description Transaction",
			DestinationAddress:   stringPtr("Address Transaction"),
			DestinationPlaceID:   stringPtr("place-id-transaction"),
			DestinationLatitude:  10.0,
			DestinationLongitude: 10.0,
			StartDate:            time.Now(),
			EndDate:              time.Now().Add(24 * time.Hour),
			Status:               types.TripStatusPlanning,
			CreatedBy:            &testUserID,
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
