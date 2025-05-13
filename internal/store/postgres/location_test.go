package postgres

import (
	"context"
	"database/sql"
	"runtime"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupLocationTest sets up the test database and returns the store, test user ID, test trip ID, and a cleanup function.
func setupLocationTest(t *testing.T) (store.LocationStore, uuid.UUID, uuid.UUID, func()) {
	// Skip tests on Windows since testcontainers don't work properly
	if isWindows() {
		t.Skip("Skipping test on Windows due to Docker compatibility issues")
	}

	pool, userID, tripID := setupTestDB(t)
	store := NewLocationStore(pool)
	cleanup := func() {
		teardownTestDB(t)
	}
	return store, userID, tripID, cleanup
}

// isWindows checks if the current platform is Windows
func isWindows() bool {
	return runtime.GOOS == "windows"
}

func TestLocationStore_CreateLocation(t *testing.T) {
	store, userID, tripID, cleanup := setupLocationTest(t)
	defer cleanup()

	ctx := context.Background()
	location := &types.Location{
		TripID:    tripID.String(),
		UserID:    userID.String(),
		Latitude:  51.5074,
		Longitude: -0.1278,
		Accuracy:  10.5,
		Timestamp: time.Now(),
	}

	id, err := store.CreateLocation(ctx, location)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	// Verify created location
	created, err := store.GetLocation(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, location.TripID, created.TripID)
	assert.Equal(t, location.UserID, created.UserID)
	assert.InDelta(t, location.Latitude, created.Latitude, 0.000001)
	assert.InDelta(t, location.Longitude, created.Longitude, 0.000001)
	assert.InDelta(t, location.Accuracy, created.Accuracy, 0.00001)
}

func TestLocationStore_UpdateLocation(t *testing.T) {
	store, userID, tripID, cleanup := setupLocationTest(t)
	defer cleanup()

	ctx := context.Background()
	initialLocation := &types.Location{
		TripID:    tripID.String(),
		UserID:    userID.String(),
		Latitude:  51.5074,
		Longitude: -0.1278,
		Accuracy:  10.5,
		Timestamp: time.Now().Add(-time.Hour),
	}
	id, err := store.CreateLocation(ctx, initialLocation)
	require.NoError(t, err)
	require.NotEmpty(t, id)

	update := &types.LocationUpdate{
		Latitude:  52.5200,
		Longitude: 13.4050,
		Accuracy:  5.0,
		Timestamp: time.Now().UnixMilli(),
	}

	updated, err := store.UpdateLocation(ctx, id, update)
	require.NoError(t, err)
	assert.Equal(t, id, updated.ID)
	assert.Equal(t, tripID.String(), updated.TripID)
	assert.Equal(t, userID.String(), updated.UserID)
	assert.InDelta(t, update.Latitude, updated.Latitude, 0.000001)
	assert.InDelta(t, update.Longitude, updated.Longitude, 0.000001)
	assert.InDelta(t, update.Accuracy, updated.Accuracy, 0.00001)
	assert.WithinDuration(t, time.UnixMilli(update.Timestamp).UTC(), updated.Timestamp.UTC(), time.Second)
}

func TestLocationStore_DeleteLocation(t *testing.T) {
	store, userID, tripID, cleanup := setupLocationTest(t)
	defer cleanup()

	ctx := context.Background()
	location := &types.Location{
		TripID:    tripID.String(),
		UserID:    userID.String(),
		Latitude:  51.5074,
		Longitude: -0.1278,
		Accuracy:  10.5,
		Timestamp: time.Now(),
	}
	id, err := store.CreateLocation(ctx, location)
	require.NoError(t, err)
	require.NotEmpty(t, id)

	err = store.DeleteLocation(ctx, id)
	require.NoError(t, err)

	_, err = store.GetLocation(ctx, id)
	assert.Error(t, err)
	assert.Equal(t, sql.ErrNoRows, err)
}

func TestLocationStore_ListTripMemberLocations(t *testing.T) {
	store, userID, tripID, cleanup := setupLocationTest(t)
	defer cleanup()

	ctx := context.Background()

	loc1 := &types.Location{
		TripID:    tripID.String(),
		UserID:    userID.String(),
		Latitude:  51.5074,
		Longitude: -0.1278,
		Accuracy:  10.5,
		Timestamp: time.Now().Add(-time.Minute),
	}
	loc2 := &types.Location{
		TripID:    tripID.String(),
		UserID:    userID.String(),
		Latitude:  52.5200,
		Longitude: 13.4050,
		Accuracy:  5.0,
		Timestamp: time.Now(),
	}

	_, err := store.CreateLocation(ctx, loc1)
	require.NoError(t, err)
	_, err = store.CreateLocation(ctx, loc2)
	require.NoError(t, err)

	memberLocations, err := store.ListTripMemberLocations(ctx, tripID.String())
	require.NoError(t, err)
	assert.Len(t, memberLocations, 2)

	assert.InDelta(t, loc2.Latitude, memberLocations[0].Latitude, 0.000001)
	assert.Equal(t, userID.String(), memberLocations[0].UserID)
	assert.NotEmpty(t, memberLocations[0].UserName)
	assert.InDelta(t, loc1.Latitude, memberLocations[1].Latitude, 0.000001)
}

/*
// TODO: Offline location functionality was removed. These tests are no longer valid.
func TestLocationStore_SaveOfflineLocations(t *testing.T) {
	store, userID, tripID, cleanup := setupLocationTest(t)
	defer cleanup()

	ctx := context.Background()
	deviceID := "test-device-id"
	now := time.Now()
	updates := []types.LocationUpdate{
		{
			Latitude:  51.5074,
			Longitude: -0.1278,
			Accuracy:  10.5,
			Timestamp: now.Add(-time.Second).UnixMilli(),
		},
		{
			Latitude:  52.5200,
			Longitude: 13.4050,
			Accuracy:  5.0,
			Timestamp: now.UnixMilli(),
		},
	}

	err := store.SaveOfflineLocations(ctx, userID.String(), tripID.String(), updates, deviceID)
	require.NoError(t, err)

	var count int
	err = testPool.QueryRow(ctx, "SELECT COUNT(*) FROM offline_location_updates WHERE user_id = $1 AND trip_id = $2 AND device_id = $3", userID, tripID, deviceID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestLocationStore_ProcessOfflineLocations(t *testing.T) {
	store, userID, tripID, cleanup := setupLocationTest(t)
	defer cleanup()

	ctx := context.Background()
	deviceID := "test-device-id-process"
	now := time.Now()
	updates := []types.LocationUpdate{
		{
			Latitude:  51.5074,
			Longitude: -0.1278,
			Accuracy:  10.5,
			Timestamp: now.Add(-time.Second).UnixMilli(),
		},
	}

	err := store.SaveOfflineLocations(ctx, userID.String(), tripID.String(), updates, deviceID)
	require.NoError(t, err)

	err = store.ProcessOfflineLocations(ctx, userID.String(), tripID.String())
	require.NoError(t, err)

	var processedCount int
	err = testPool.QueryRow(ctx, "SELECT COUNT(*) FROM offline_location_updates WHERE user_id = $1 AND trip_id = $2 AND device_id = $3 AND processed_at IS NOT NULL", userID, tripID, deviceID).Scan(&processedCount)
	require.NoError(t, err)
	assert.Equal(t, 1, processedCount, "Offline update should be marked as processed")

	var locationCount int
	err = testPool.QueryRow(ctx, "SELECT COUNT(*) FROM locations WHERE user_id = $1 AND trip_id = $2", userID, tripID).Scan(&locationCount)
	require.NoError(t, err)
	assert.Equal(t, 1, locationCount, "Location should be inserted into the main table")
}
*/
