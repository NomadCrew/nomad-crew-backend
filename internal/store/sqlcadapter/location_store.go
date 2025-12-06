package sqlcadapter

import (
	"context"
	"errors"
	"fmt"
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/sqlc"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Ensure sqlcLocationStore implements store.LocationStore
var _ store.LocationStore = (*sqlcLocationStore)(nil)

type sqlcLocationStore struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

// NewSqlcLocationStore creates a new SQLC-based location store
func NewSqlcLocationStore(pool *pgxpool.Pool) store.LocationStore {
	return &sqlcLocationStore{
		pool:    pool,
		queries: sqlc.New(pool),
	}
}

// UpdateLocation updates a user's location and publishes an event.
// It first verifies the user is part of an active trip.
// Returns the newly created Location object or an error.
func (s *sqlcLocationStore) UpdateLocation(ctx context.Context, userID string, update types.LocationUpdate) (*types.Location, error) {
	log := logger.GetLogger()

	// Convert timestamp from milliseconds to time.Time
	timestamp := time.UnixMilli(update.Timestamp)

	// First, check if the user is a member of any active trips using raw query
	// since SQLC doesn't have this specific query
	var tripID string
	err := s.pool.QueryRow(ctx, `
		SELECT tm.trip_id
		FROM trip_memberships tm
		JOIN trips t ON tm.trip_id = t.id
		WHERE tm.user_id = $1
		AND tm.status = 'ACTIVE'
		AND t.status = 'ACTIVE'
		ORDER BY t.start_date DESC
		LIMIT 1
	`, userID).Scan(&tripID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Warnw("User is not a member of any active trips, cannot update location", "userID", userID)
			return nil, apperrors.ValidationFailed("no_active_trip", "User is not currently a member of any active trip.")
		}
		log.Errorw("Failed to check user trip membership", "userID", userID, "error", err)
		return nil, apperrors.NewDatabaseError(fmt.Errorf("failed to check user trip membership: %w", err))
	}

	// Create the location using SQLC
	locationID, err := s.queries.CreateLocation(ctx, sqlc.CreateLocationParams{
		TripID:       tripID,
		UserID:       userID,
		Latitude:     update.Latitude,
		Longitude:    update.Longitude,
		Accuracy:     Float64ToFloat64Ptr(update.Accuracy),
		Timestamp:    TimeToPgTimestamptz(timestamp),
		LocationName: nil,
		LocationType: nil,
		Notes:        nil,
		Status:       nil,
	})
	if err != nil {
		log.Errorw("Failed to insert location", "userID", userID, "error", err)
		return nil, apperrors.NewDatabaseError(fmt.Errorf("failed to insert location: %w", err))
	}

	// Return the created location
	location := &types.Location{
		ID:        locationID,
		TripID:    tripID,
		UserID:    userID,
		Latitude:  update.Latitude,
		Longitude: update.Longitude,
		Accuracy:  update.Accuracy,
		Timestamp: timestamp,
		CreatedAt: timestamp,
		UpdatedAt: timestamp,
	}

	return location, nil
}

// GetTripMemberLocations retrieves the most recent location (within the last 24 hours)
// for each active member of the specified trip.
// It joins with user and membership tables to include user display name and role.
func (s *sqlcLocationStore) GetTripMemberLocations(ctx context.Context, tripID string) ([]types.MemberLocation, error) {
	log := logger.GetLogger()

	rows, err := s.queries.GetTripMemberLocations(ctx, tripID)
	if err != nil {
		log.Errorw("Failed to get trip member locations", "tripID", tripID, "error", err)
		return nil, apperrors.NewDatabaseError(fmt.Errorf("failed to query trip member locations: %w", err))
	}

	locations := make([]types.MemberLocation, 0, len(rows))
	for _, row := range rows {
		loc := types.MemberLocation{
			UserName: row.UserName,
			UserRole: string(row.UserRole),
		}
		// Set embedded Location fields
		loc.Location.ID = row.ID
		loc.Location.TripID = row.TripID
		loc.Location.UserID = row.UserID
		loc.Location.Latitude = row.Latitude
		loc.Location.Longitude = row.Longitude

		// Handle optional fields
		if row.Accuracy != nil {
			loc.Location.Accuracy = *row.Accuracy
		}
		if row.Timestamp.Valid {
			loc.Location.Timestamp = row.Timestamp.Time
		}
		if row.CreatedAt.Valid {
			loc.Location.CreatedAt = row.CreatedAt.Time
		}
		if row.UpdatedAt.Valid {
			loc.Location.UpdatedAt = row.UpdatedAt.Time
		}

		locations = append(locations, loc)
	}

	log.Infow("Successfully retrieved trip member locations", "tripID", tripID, "count", len(locations))
	return locations, nil
}

// GetUserRole retrieves the role of a user within a specific trip.
func (s *sqlcLocationStore) GetUserRole(ctx context.Context, tripID string, userID string) (types.MemberRole, error) {
	log := logger.GetLogger()

	// Use the existing SQLC query for getting user role
	role, err := s.queries.GetUserRole(ctx, sqlc.GetUserRoleParams{
		TripID: tripID,
		UserID: userID,
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperrors.NotFound("membership", fmt.Sprintf("active user %s in trip %s", userID, tripID))
		}
		log.Errorw("Failed to get user role from database", "tripID", tripID, "userID", userID, "error", err)
		return "", apperrors.NewDatabaseError(err)
	}

	// Validate the retrieved role
	memberRole := types.MemberRole(role)
	if !memberRole.IsValid() {
		log.Errorw("Invalid role found in database for active member", "role", role, "tripID", tripID, "userID", userID)
		return "", apperrors.InternalServerError(fmt.Sprintf("Invalid role '%s' found for user %s in trip %s", role, userID, tripID))
	}

	return memberRole, nil
}
