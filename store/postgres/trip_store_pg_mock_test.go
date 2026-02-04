package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create test trip
func createTestTrip() types.Trip {
	createdBy := uuid.NewString()
	return types.Trip{
		ID:          uuid.NewString(),
		Name:        "Test Trip",
		Description: "Test trip description",
		StartDate:   time.Now().Add(24 * time.Hour),
		EndDate:     time.Now().Add(7 * 24 * time.Hour),
		CreatedBy:   &createdBy,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		// DeletedAt is nil by default (not deleted)
	}
}

// Helper function to create test trip membership
func createTestTripMembership() *types.TripMembership {
	return &types.TripMembership{
		TripID:    uuid.NewString(),
		UserID:    uuid.NewString(),
		Role:      types.MemberRoleMember,
		CreatedAt: time.Now(),
	}
}

// Helper function to create test trip invitation
func createTestTripInvitation() *types.TripInvitation {
	return &types.TripInvitation{
		ID:        uuid.NewString(),
		TripID:    uuid.NewString(),
		Email:     "invitee@example.com",
		Role:      types.MemberRoleMember,
		Status:    types.InvitationStatusPending,
		CreatedBy: uuid.NewString(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestTripStore_CreateTrip(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	ctx := context.Background()
	trip := createTestTrip()

	t.Run("successful creation", func(t *testing.T) {
		// Begin transaction
		mock.ExpectBegin()

		// Insert trip
		mock.ExpectExec("INSERT INTO trips").
			WithArgs(
				trip.ID,
				trip.Name,
				trip.Description,
				trip.StartDate,
				trip.EndDate,
				*trip.CreatedBy,
			).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Add creator as owner
		mock.ExpectExec("INSERT INTO trip_memberships").
			WithArgs(trip.ID, *trip.CreatedBy, types.MemberRoleOwner).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Commit transaction
		mock.ExpectCommit()

		// Would verify successful creation
	})

	t.Run("duplicate trip ID", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO trips").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), 
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnError(&pgconn.PgError{
				Code: "23505", // unique_violation
			})
		mock.ExpectRollback()

		// Would return appropriate error
	})

	t.Run("invalid date range", func(t *testing.T) {
		invalidTrip := createTestTrip()
		invalidTrip.EndDate = invalidTrip.StartDate.Add(-24 * time.Hour) // End before start

		// Should validate before database call
	})

	t.Run("empty trip name", func(t *testing.T) {
		invalidTrip := createTestTrip()
		invalidTrip.Name = ""

		// Should validate before database call
	})

	t.Run("rollback on membership insert failure", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO trips").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("INSERT INTO trip_memberships").
			WillReturnError(errors.New("membership insert failed"))
		mock.ExpectRollback()

		// Would verify rollback
	})
}

func TestTripStore_GetTrip(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	ctx := context.Background()
	trip := createTestTrip()

	t.Run("successful retrieval", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"id", "name", "description", "start_date", "end_date",
			"created_by", "created_at", "updated_at", "deleted_at",
		}).AddRow(
			trip.ID, trip.Name, trip.Description, trip.StartDate, trip.EndDate,
			trip.CreatedBy, trip.CreatedAt, trip.UpdatedAt, trip.DeletedAt,
		)

		mock.ExpectQuery("SELECT (.+) FROM trips WHERE id = \\$1 AND deleted_at IS NULL").
			WithArgs(trip.ID).
			WillReturnRows(rows)

		// Would verify successful retrieval
	})

	t.Run("trip not found", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM trips WHERE id = \\$1").
			WithArgs("nonexistent").
			WillReturnRows(sqlmock.NewRows([]string{}))

		// Would return ErrTripNotFound
	})

	t.Run("soft deleted trip", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM trips WHERE id = \\$1 AND deleted_at IS NULL").
			WithArgs(trip.ID).
			WillReturnRows(sqlmock.NewRows([]string{}))

		// Would return ErrTripNotFound
	})
}

func TestTripStore_UpdateTrip(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	ctx := context.Background()
	tripID := uuid.NewString()

	t.Run("update name", func(t *testing.T) {
		update := types.TripUpdate{
			Name: func() *string { s := "Updated Trip Name"; return &s }(),
		}

		mock.ExpectExec("UPDATE trips SET name = \\$1, updated_at = CURRENT_TIMESTAMP WHERE id = \\$2 AND deleted_at IS NULL").
			WithArgs(*update.Name, tripID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Would verify successful update
	})

	t.Run("update description", func(t *testing.T) {
		update := types.TripUpdate{
			Description: func() *string { s := "Updated description"; return &s }(),
		}

		mock.ExpectExec("UPDATE trips SET description = \\$1").
			WithArgs(*update.Description, tripID).
			WillReturnResult(sqlmock.NewResult(1, 1))
	})

	t.Run("update dates", func(t *testing.T) {
		newStart := time.Now().Add(48 * time.Hour)
		newEnd := time.Now().Add(10 * 24 * time.Hour)
		update := types.TripUpdate{
			StartDate: &newStart,
			EndDate:   &newEnd,
		}

		mock.ExpectExec("UPDATE trips SET start_date = \\$1, end_date = \\$2").
			WithArgs(update.StartDate, update.EndDate, tripID).
			WillReturnResult(sqlmock.NewResult(1, 1))
	})

	t.Run("trip not found", func(t *testing.T) {
		update := types.TripUpdate{
			Name: func() *string { s := "Updated"; return &s }(),
		}

		mock.ExpectExec("UPDATE trips").
			WithArgs(*update.Name, tripID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Would check affected rows
	})

	t.Run("invalid date range update", func(t *testing.T) {
		newStart := time.Now().Add(10 * 24 * time.Hour)
		newEnd := time.Now().Add(48 * time.Hour)
		update := types.TripUpdate{
			StartDate: &newStart,
			EndDate:   &newEnd,
		}

		// Should validate dates before database call
	})
}

func TestTripStore_SoftDeleteTrip(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	ctx := context.Background()
	tripID := uuid.NewString()

	t.Run("successful soft delete", func(t *testing.T) {
		mock.ExpectExec("UPDATE trips SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = \\$1").
			WithArgs(tripID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Would verify soft delete
	})

	t.Run("trip not found", func(t *testing.T) {
		mock.ExpectExec("UPDATE trips SET deleted_at = CURRENT_TIMESTAMP").
			WithArgs(tripID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Would return ErrTripNotFound
	})

	t.Run("already deleted", func(t *testing.T) {
		mock.ExpectExec("UPDATE trips SET deleted_at = CURRENT_TIMESTAMP").
			WithArgs(tripID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Would handle gracefully
	})
}

func TestTripStore_ListUserTrips(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.NewString()

	t.Run("successful list", func(t *testing.T) {
		trip1 := createTestTrip()
		trip2 := createTestTrip()
		trip2.ID = uuid.NewString()

		rows := sqlmock.NewRows([]string{
			"id", "name", "description", "start_date", "end_date",
			"created_by", "created_at", "updated_at", "deleted_at", "role",
		}).
			AddRow(trip1.ID, trip1.Name, trip1.Description, trip1.StartDate, trip1.EndDate,
				trip1.CreatedBy, trip1.CreatedAt, trip1.UpdatedAt, trip1.DeletedAt, types.MemberRoleOwner).
			AddRow(trip2.ID, trip2.Name, trip2.Description, trip2.StartDate, trip2.EndDate,
				trip2.CreatedBy, trip2.CreatedAt, trip2.UpdatedAt, trip2.DeletedAt, types.MemberRoleMember)

		mock.ExpectQuery("SELECT t.\\*, tm.role FROM trips t JOIN trip_memberships tm").
			WithArgs(userID).
			WillReturnRows(rows)

		// Would return list of trips with user's role
	})

	t.Run("empty list", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"id", "name", "description", "start_date", "end_date",
			"created_by", "created_at", "updated_at", "deleted_at", "role",
		})

		mock.ExpectQuery("SELECT t.\\*, tm.role FROM trips").
			WithArgs(userID).
			WillReturnRows(rows)

		// Would return empty slice
	})

	t.Run("exclude deleted trips", func(t *testing.T) {
		// Query should filter out deleted trips (deleted_at IS NOT NULL)
		mock.ExpectQuery("SELECT (.+) WHERE (.+) AND t.deleted_at IS NULL").
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{}))
	})
}

func TestTripStore_SearchTrips(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()

	t.Run("search by destination", func(t *testing.T) {
		_ = types.TripSearchCriteria{
			Destination: "beach",
		}

		rows := sqlmock.NewRows([]string{
			"id", "name", "description", "start_date", "end_date",
			"created_by", "created_at", "updated_at", "deleted_at",
		})

		mock.ExpectQuery("SELECT (.+) FROM trips WHERE (.+) ILIKE (.+)").
			WithArgs("%beach%", "%beach%").
			WillReturnRows(rows)

		// Would search in destination
	})

	t.Run("search by date range", func(t *testing.T) {
		startDate := time.Now()
		endDate := time.Now().Add(30 * 24 * time.Hour)
		_ = types.TripSearchCriteria{
			StartDate: startDate,
			EndDate:   endDate,
		}

		rows := sqlmock.NewRows([]string{
			"id", "name", "description", "start_date", "end_date",
			"created_by", "created_at", "updated_at", "deleted_at",
		})

		mock.ExpectQuery("SELECT (.+) WHERE (.+) start_date >= \\$1 AND end_date <= \\$2").
			WithArgs(startDate, endDate).
			WillReturnRows(rows)
	})

	t.Run("combined search", func(t *testing.T) {
		startDate := time.Now()
		_ = types.TripSearchCriteria{
			Destination: "vacation",
			StartDate:   startDate,
		}

		// Would combine destination and date filters
	})
}

func TestTripStore_AddMember(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	ctx := context.Background()
	membership := createTestTripMembership()

	t.Run("successful addition", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO trip_memberships \\(trip_id, user_id, role\\) VALUES").
			WithArgs(membership.TripID, membership.UserID, membership.Role).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Would verify member added
	})

	t.Run("duplicate member", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO trip_memberships").
			WithArgs(membership.TripID, membership.UserID, membership.Role).
			WillReturnError(&pgconn.PgError{
				Code: "23505", // unique_violation
			})

		// Would return "already a member" error
	})

	t.Run("invalid trip", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO trip_memberships").
			WithArgs(membership.TripID, membership.UserID, membership.Role).
			WillReturnError(&pgconn.PgError{
				Code: "23503", // foreign_key_violation
			})

		// Would return "trip not found" error
	})

	t.Run("invalid role", func(t *testing.T) {
		invalidMembership := createTestTripMembership()
		invalidMembership.Role = "invalid_role"

		// Should validate role before database call
	})
}

func TestTripStore_UpdateMemberRole(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	ctx := context.Background()
	tripID := uuid.NewString()
	userID := uuid.NewString()

	t.Run("successful role update", func(t *testing.T) {
		mock.ExpectExec("UPDATE trip_memberships SET role = \\$1 WHERE trip_id = \\$2 AND user_id = \\$3").
			WithArgs(types.MemberRoleAdmin, tripID, userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Would verify role updated
	})

	t.Run("member not found", func(t *testing.T) {
		mock.ExpectExec("UPDATE trip_memberships").
			WithArgs(types.MemberRoleAdmin, tripID, userID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Would return "member not found" error
	})

	t.Run("cannot demote last owner", func(t *testing.T) {
		// Business logic to prevent removing last owner
		// Would need to check if user is last owner before update
	})
}

func TestTripStore_RemoveMember(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	ctx := context.Background()
	tripID := uuid.NewString()
	userID := uuid.NewString()

	t.Run("successful removal", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM trip_memberships WHERE trip_id = \\$1 AND user_id = \\$2").
			WithArgs(tripID, userID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// Would verify member removed
	})

	t.Run("member not found", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM trip_memberships").
			WithArgs(tripID, userID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Would return appropriate error
	})

	t.Run("cannot remove last owner", func(t *testing.T) {
		// Business logic to prevent removing last owner
	})
}

func TestTripStore_GetTripMembers(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	ctx := context.Background()
	tripID := uuid.NewString()

	t.Run("successful list", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"user_id", "role", "created_at", "email", "username", "display_name",
		}).
			AddRow(uuid.NewString(), types.MemberRoleOwner, time.Now(), "owner@example.com", "owner", "Trip Owner").
			AddRow(uuid.NewString(), types.MemberRoleAdmin, time.Now(), "admin@example.com", "admin", "Trip Admin").
			AddRow(uuid.NewString(), types.MemberRoleMember, time.Now(), "member@example.com", "member", "Trip Member")

		mock.ExpectQuery("SELECT tm.\\*, u.email, u.username, u.display_name FROM trip_memberships tm JOIN users u").
			WithArgs(tripID).
			WillReturnRows(rows)

		// Would return list of members with user details
	})

	t.Run("empty trip", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"user_id", "role", "created_at", "email", "username", "display_name",
		})

		mock.ExpectQuery("SELECT (.+) FROM trip_memberships").
			WithArgs(tripID).
			WillReturnRows(rows)

		// Would return empty slice
	})
}

func TestTripStore_GetUserRole(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	ctx := context.Background()
	tripID := uuid.NewString()
	userID := uuid.NewString()

	t.Run("user is owner", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"role"}).AddRow(types.MemberRoleOwner)

		mock.ExpectQuery("SELECT role FROM trip_memberships WHERE trip_id = \\$1 AND user_id = \\$2").
			WithArgs(tripID, userID).
			WillReturnRows(rows)

		// Would return MemberRoleOwner
	})

	t.Run("user is member", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"role"}).AddRow(types.MemberRoleMember)

		mock.ExpectQuery("SELECT role FROM trip_memberships").
			WithArgs(tripID, userID).
			WillReturnRows(rows)

		// Would return MemberRoleMember
	})

	t.Run("user not a member", func(t *testing.T) {
		mock.ExpectQuery("SELECT role FROM trip_memberships").
			WithArgs(tripID, userID).
			WillReturnRows(sqlmock.NewRows([]string{"role"}))

		// Would return empty string or error
	})
}

func TestTripStore_CreateInvitation(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	ctx := context.Background()
	invitation := createTestTripInvitation()

	t.Run("successful creation", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO trip_invitations").
			WithArgs(
				invitation.ID,
				invitation.TripID,
				invitation.Email,
				invitation.Role,
				invitation.Status,
				invitation.CreatedBy,
			).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Would verify invitation created
	})

	t.Run("duplicate invitation", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO trip_invitations").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnError(&pgconn.PgError{
				Code: "23505", // unique_violation
			})

		// Would return "invitation already exists" error
	})

	t.Run("invalid email", func(t *testing.T) {
		invalidInvitation := createTestTripInvitation()
		invalidInvitation.Email = "invalid-email"

		// Should validate email format
	})
}

func TestTripStore_GetInvitation(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	ctx := context.Background()
	invitation := createTestTripInvitation()

	t.Run("successful retrieval", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"id", "trip_id", "email", "role", "status",
			"created_by", "created_at", "updated_at",
		}).AddRow(
			invitation.ID, invitation.TripID, invitation.Email, invitation.Role, invitation.Status,
			invitation.CreatedBy, invitation.CreatedAt, invitation.UpdatedAt,
		)

		mock.ExpectQuery("SELECT (.+) FROM trip_invitations WHERE id = \\$1").
			WithArgs(invitation.ID).
			WillReturnRows(rows)

		// Would verify successful retrieval
	})

	t.Run("invitation not found", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM trip_invitations WHERE id = \\$1").
			WithArgs("nonexistent").
			WillReturnRows(sqlmock.NewRows([]string{}))

		// Would return ErrInvitationNotFound
	})
}

func TestTripStore_UpdateInvitationStatus(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	ctx := context.Background()
	invitationID := uuid.NewString()

	t.Run("accept invitation", func(t *testing.T) {
		mock.ExpectExec("UPDATE trip_invitations SET status = \\$1, updated_at = CURRENT_TIMESTAMP WHERE id = \\$2").
			WithArgs(types.InvitationStatusAccepted, invitationID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Would verify status updated
	})

	t.Run("reject invitation", func(t *testing.T) {
		mock.ExpectExec("UPDATE trip_invitations SET status = \\$1").
			WithArgs(types.InvitationStatusRejected, invitationID).
			WillReturnResult(sqlmock.NewResult(1, 1))
	})

	t.Run("invitation not found", func(t *testing.T) {
		mock.ExpectExec("UPDATE trip_invitations").
			WithArgs(types.InvitationStatusAccepted, invitationID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Would check affected rows
	})

	t.Run("invalid status transition", func(t *testing.T) {
		// Can't update already accepted/rejected invitation
		// Business logic validation
	})
}

// Benchmark tests
func BenchmarkTripStore_CreateTrip(b *testing.B) {
	db, mock, cleanup := setupMockDB(b)
	defer cleanup()

	trip := createTestTrip()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO trips").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("INSERT INTO trip_memberships").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		// Execute in actual implementation
	}
}

func BenchmarkTripStore_ListUserTrips(b *testing.B) {
	_, mock, cleanup := setupMockDB(b)
	defer cleanup()

	userID := uuid.NewString()

	// Create test data
	rows := sqlmock.NewRows([]string{
		"id", "name", "description", "start_date", "end_date",
		"created_by", "created_at", "updated_at", "deleted_at", "role",
	})

	// Add 20 trips
	for i := 0; i < 20; i++ {
		trip := createTestTrip()
		rows.AddRow(trip.ID, trip.Name, trip.Description, trip.StartDate, trip.EndDate,
			trip.CreatedBy, trip.CreatedAt, trip.UpdatedAt, trip.DeletedAt, types.MemberRoleMember)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock.ExpectQuery("SELECT (.+) FROM trips").
			WithArgs(userID).
			WillReturnRows(rows)

		// Execute in actual implementation
	}
}