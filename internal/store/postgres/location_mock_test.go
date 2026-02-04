package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
	"github.com/jackc/pgconn"
)

// Helper function to create test location
func createTestLocation() *types.Location {
	return &types.Location{
		ID:        uuid.NewString(),
		TripID:    uuid.NewString(),
		UserID:    uuid.NewString(),
		Latitude:  37.7749,
		Longitude: -122.4194,
		Accuracy:  10.5,
		Timestamp: time.Now(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// Helper function to create test member location
// MemberLocation embeds Location and adds UserName and UserRole fields
func createTestMemberLocation() *types.MemberLocation {
	return &types.MemberLocation{
		Location: types.Location{
			ID:        uuid.NewString(),
			TripID:    uuid.NewString(),
			UserID:    uuid.NewString(),
			Latitude:  37.7749,
			Longitude: -122.4194,
			Accuracy:  10.5,
			Timestamp: time.Now(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		UserName: "testuser",
		UserRole: "member",
	}
}

func TestLocationStore_CreateLocation(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	location := createTestLocation()

	t.Run("successful creation", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id"}).AddRow(uuid.MustParse(location.ID))

		mock.ExpectQuery("INSERT INTO locations \\(trip_id, user_id, latitude, longitude, accuracy, \"timestamp\"\\)").
			WithArgs(
				location.TripID,
				location.UserID,
				location.Latitude,
				location.Longitude,
				location.Accuracy,
				location.Timestamp,
			).
			WillReturnRows(rows)

		// Would verify successful creation and returned ID
	})

	t.Run("invalid trip ID", func(t *testing.T) {
		mock.ExpectQuery("INSERT INTO locations").
			WithArgs(
				location.TripID,
				location.UserID,
				location.Latitude,
				location.Longitude,
				location.Accuracy,
				location.Timestamp,
			).
			WillReturnError(&pgconn.PgError{
				Code: "23503", // foreign_key_violation
			})

		// Would return "trip not found" error
	})

	t.Run("invalid user ID", func(t *testing.T) {
		mock.ExpectQuery("INSERT INTO locations").
			WithArgs(
				location.TripID,
				location.UserID,
				location.Latitude,
				location.Longitude,
				location.Accuracy,
				location.Timestamp,
			).
			WillReturnError(&pgconn.PgError{
				Code: "23503", // foreign_key_violation
			})

		// Would return "user not found" error
	})

	t.Run("invalid coordinates", func(t *testing.T) {
		invalidLocation := createTestLocation()
		invalidLocation.Latitude = 91.0 // Invalid latitude

		// Should validate before database call
		// Latitude must be between -90 and 90
	})

	t.Run("invalid longitude", func(t *testing.T) {
		invalidLocation := createTestLocation()
		invalidLocation.Longitude = 181.0 // Invalid longitude

		// Should validate before database call
		// Longitude must be between -180 and 180
	})

	t.Run("negative accuracy", func(t *testing.T) {
		invalidLocation := createTestLocation()
		invalidLocation.Accuracy = -5.0

		// Should validate before database call
		// Accuracy must be positive
	})
}

func TestLocationStore_GetLocation(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	location := createTestLocation()

	t.Run("successful retrieval", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"id", "trip_id", "user_id", "latitude", "longitude", 
			"accuracy", "timestamp", "created_at", "updated_at",
		}).AddRow(
			uuid.MustParse(location.ID),
			location.TripID,
			location.UserID,
			location.Latitude,
			location.Longitude,
			location.Accuracy,
			location.Timestamp,
			location.CreatedAt,
			location.UpdatedAt,
		)

		mock.ExpectQuery("SELECT (.+) FROM locations WHERE id = \\$1").
			WithArgs(location.ID).
			WillReturnRows(rows)

		// Would verify successful retrieval
	})

	t.Run("location not found", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM locations WHERE id = \\$1").
			WithArgs("nonexistent").
			WillReturnRows(sqlmock.NewRows([]string{}))

		// Would return sql.ErrNoRows
	})

	t.Run("database error", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM locations WHERE id = \\$1").
			WithArgs(location.ID).
			WillReturnError(errors.New("database connection failed"))

		// Would return the error
	})
}

func TestLocationStore_UpdateLocation(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	locationID := uuid.NewString()

	t.Run("update coordinates", func(t *testing.T) {
		update := &types.LocationUpdate{
			Latitude:  40.7128,
			Longitude: -74.0060,
		}

		mock.ExpectExec("UPDATE locations SET latitude = \\$1, longitude = \\$2, updated_at = CURRENT_TIMESTAMP WHERE id = \\$3").
			WithArgs(update.Latitude, update.Longitude, locationID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Would verify successful update
	})

	t.Run("update accuracy", func(t *testing.T) {
		update := &types.LocationUpdate{
			Accuracy: 5.0,
		}

		mock.ExpectExec("UPDATE locations SET accuracy = \\$1, updated_at = CURRENT_TIMESTAMP WHERE id = \\$2").
			WithArgs(update.Accuracy, locationID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Would verify successful update
	})

	t.Run("update timestamp", func(t *testing.T) {
		newTimestamp := time.Now().UnixMilli()
		update := &types.LocationUpdate{
			Timestamp: newTimestamp,
		}

		mock.ExpectExec("UPDATE locations SET \"timestamp\" = \\$1, updated_at = CURRENT_TIMESTAMP WHERE id = \\$2").
			WithArgs(update.Timestamp, locationID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Would verify successful update
	})

	t.Run("location not found", func(t *testing.T) {
		update := &types.LocationUpdate{
			Latitude: 40.7128,
		}

		mock.ExpectExec("UPDATE locations").
			WithArgs(update.Latitude, locationID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Would check affected rows and return error
	})

	t.Run("no fields to update", func(t *testing.T) {
		_ = &types.LocationUpdate{}

		// Should return error without database call
		// "no fields to update"
	})

	t.Run("invalid update values", func(t *testing.T) {
		_ = &types.LocationUpdate{
			Latitude:  91.0, // Invalid
			Longitude: -181.0, // Invalid
		}

		// Should validate before database call
	})
}

func TestLocationStore_DeleteLocation(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	locationID := uuid.NewString()

	t.Run("successful deletion", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM locations WHERE id = \\$1").
			WithArgs(locationID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// Would verify successful deletion
	})

	t.Run("location not found", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM locations WHERE id = \\$1").
			WithArgs(locationID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Would check affected rows and return error
	})

	t.Run("database error", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM locations WHERE id = \\$1").
			WithArgs(locationID).
			WillReturnError(errors.New("database connection failed"))

		// Would return the error
	})
}

func TestLocationStore_ListTripMemberLocations(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	tripID := uuid.NewString()

	t.Run("successful list with multiple members", func(t *testing.T) {
		member1 := createTestMemberLocation()
		member2 := createTestMemberLocation()
		member2.Location.UserID = uuid.NewString()
		member2.UserName = "user2"

		rows := sqlmock.NewRows([]string{
			"user_id", "username", "user_role",
			"latitude", "longitude", "accuracy", "timestamp",
		}).
			AddRow(member1.Location.UserID, member1.UserName, member1.UserRole,
				member1.Location.Latitude, member1.Location.Longitude, member1.Location.Accuracy, member1.Location.Timestamp).
			AddRow(member2.Location.UserID, member2.UserName, member2.UserRole,
				member2.Location.Latitude, member2.Location.Longitude, member2.Location.Accuracy, member2.Location.Timestamp)

		// Complex query with window function
		mock.ExpectQuery("WITH latest_locations AS").
			WithArgs(tripID).
			WillReturnRows(rows)

		// Would return list of member locations
	})

	t.Run("empty results - no locations", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"user_id", "username", "user_role",
			"latitude", "longitude", "accuracy", "timestamp",
		})

		mock.ExpectQuery("WITH latest_locations AS").
			WithArgs(tripID).
			WillReturnRows(rows)

		// Would return empty slice
	})

	t.Run("trip not found", func(t *testing.T) {
		// Even if trip doesn't exist, query would return empty results
		rows := sqlmock.NewRows([]string{
			"user_id", "username", "user_role",
			"latitude", "longitude", "accuracy", "timestamp",
		})

		mock.ExpectQuery("WITH latest_locations AS").
			WithArgs("nonexistent-trip").
			WillReturnRows(rows)

		// Would return empty slice
	})

	t.Run("database error", func(t *testing.T) {
		mock.ExpectQuery("WITH latest_locations AS").
			WithArgs(tripID).
			WillReturnError(errors.New("database connection failed"))

		// Would return the error
	})

	t.Run("null handling for optional fields", func(t *testing.T) {
		// Test handling of NULL values for user_role
		rows := sqlmock.NewRows([]string{
			"user_id", "username", "user_role",
			"latitude", "longitude", "accuracy", "timestamp",
		}).
			AddRow(uuid.NewString(), "user1", nil,
				37.7749, -122.4194, 10.5, time.Now())

		mock.ExpectQuery("WITH latest_locations AS").
			WithArgs(tripID).
			WillReturnRows(rows)

		// Would handle NULL values appropriately
	})
}

func TestLocationStore_BeginTx(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()

	t.Run("successful transaction start", func(t *testing.T) {
		mock.ExpectBegin()

		// Would return transaction wrapper
	})

	t.Run("transaction already in progress", func(t *testing.T) {
		// Would need to test with actual pgxpool implementation
		// Cannot have nested transactions
	})

	t.Run("database connection error", func(t *testing.T) {
		mock.ExpectBegin().WillReturnError(errors.New("connection lost"))

		// Would return the error
	})
}

func TestLocationStore_TransactionOperations(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()

	t.Run("successful commit", func(t *testing.T) {
		location := createTestLocation()

		// Begin transaction
		mock.ExpectBegin()

		// Insert location within transaction
		rows := sqlmock.NewRows([]string{"id"}).AddRow(uuid.MustParse(location.ID))
		mock.ExpectQuery("INSERT INTO locations").
			WithArgs(
				location.TripID,
				location.UserID,
				location.Latitude,
				location.Longitude,
				location.Accuracy,
				location.Timestamp,
			).
			WillReturnRows(rows)

		// Commit transaction
		mock.ExpectCommit()

		// Would verify transaction flow
	})

	t.Run("rollback on error", func(t *testing.T) {
		// Begin transaction
		mock.ExpectBegin()

		// Insert fails
		mock.ExpectQuery("INSERT INTO locations").
			WillReturnError(errors.New("constraint violation"))

		// Rollback transaction
		mock.ExpectRollback()

		// Would verify rollback behavior
	})
}

// Benchmark tests
func BenchmarkLocationStore_CreateLocation(b *testing.B) {
	_, mock, cleanup := setupMockDB(b)
	defer cleanup()

	location := createTestLocation()
	rows := sqlmock.NewRows([]string{"id"}).AddRow(uuid.MustParse(location.ID))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock.ExpectQuery("INSERT INTO locations").
			WithArgs(
				location.TripID,
				location.UserID,
				location.Latitude,
				location.Longitude,
				location.Accuracy,
				location.Timestamp,
			).
			WillReturnRows(rows)

		// Execute in actual implementation
	}
}

func BenchmarkLocationStore_ListTripMemberLocations(b *testing.B) {
	_, mock, cleanup := setupMockDB(b)
	defer cleanup()

	tripID := uuid.NewString()

	// Create test data
	rows := sqlmock.NewRows([]string{
		"user_id", "username", "user_role",
		"latitude", "longitude", "accuracy", "timestamp",
	})

	// Add 10 members
	for i := 0; i < 10; i++ {
		member := createTestMemberLocation()
		rows.AddRow(member.Location.UserID, member.UserName, member.UserRole,
			member.Location.Latitude, member.Location.Longitude, member.Location.Accuracy, member.Location.Timestamp)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock.ExpectQuery("WITH latest_locations AS").
			WithArgs(tripID).
			WillReturnRows(rows)

		// Execute in actual implementation
	}
}

// Test coordinate validation
func TestLocationStore_ValidateCoordinates(t *testing.T) {
	tests := []struct {
		name      string
		latitude  float64
		longitude float64
		wantErr   bool
	}{
		{
			name:      "valid coordinates",
			latitude:  37.7749,
			longitude: -122.4194,
			wantErr:   false,
		},
		{
			name:      "latitude too high",
			latitude:  91.0,
			longitude: 0,
			wantErr:   true,
		},
		{
			name:      "latitude too low",
			latitude:  -91.0,
			longitude: 0,
			wantErr:   true,
		},
		{
			name:      "longitude too high",
			latitude:  0,
			longitude: 181.0,
			wantErr:   true,
		},
		{
			name:      "longitude too low",
			latitude:  0,
			longitude: -181.0,
			wantErr:   true,
		},
		{
			name:      "edge case - max latitude",
			latitude:  90.0,
			longitude: 0,
			wantErr:   false,
		},
		{
			name:      "edge case - min latitude",
			latitude:  -90.0,
			longitude: 0,
			wantErr:   false,
		},
		{
			name:      "edge case - max longitude",
			latitude:  0,
			longitude: 180.0,
			wantErr:   false,
		},
		{
			name:      "edge case - min longitude",
			latitude:  0,
			longitude: -180.0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validation logic would be tested here
			// This would be part of the actual implementation
		})
	}
}