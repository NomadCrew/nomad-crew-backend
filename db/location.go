// Package db provides implementations for data access interfaces defined in internal/store.
// It interacts with the PostgreSQL database using pgxpool and potentially other external services like Supabase.
package db

import (
	"context"
	"fmt"
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/store" // Added import for store package
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// LocationDB provides database operations related to user and trip locations.
// It interacts with the database via a DatabaseClient.
type LocationDB struct {
	client *DatabaseClient
}

// NewLocationDB creates a new instance of LocationDB.
func NewLocationDB(client *DatabaseClient) *LocationDB {
	return &LocationDB{
		client: client,
	}
}

// Ensure LocationDB implements the store.LocationStore interface.
var _ store.LocationStore = (*LocationDB)(nil)

// UpdateLocation saves a new location record for a user.
// It first verifies the user is part of an active trip.
// Returns the newly created Location object or an error.
func (ldb *LocationDB) UpdateLocation(ctx context.Context, userID string, update types.LocationUpdate) (*types.Location, error) {
	log := logger.GetLogger()

	// Convert timestamp from milliseconds to time.Time
	timestamp := time.UnixMilli(update.Timestamp)

	// Generate a new UUID for the location
	locationID := uuid.New().String()

	// First, check if the user is a member of any active trips
	var tripID string
	err := ldb.client.GetPool().QueryRow(ctx, `
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
		if err == pgx.ErrNoRows {
			log.Warnw("User is not a member of any active trips, cannot update location", "userID", userID)
			// Return a specific application error instead of fmt.Errorf
			return nil, apperrors.ValidationFailed("no_active_trip", "User is not currently a member of any active trip.")
		}
		log.Errorw("Failed to check user trip membership", "userID", userID, "error", err)
		// Return a database error
		return nil, apperrors.NewDatabaseError(fmt.Errorf("failed to check user trip membership: %w", err))
	}

	// Insert the location into the database
	_, err = ldb.client.GetPool().Exec(ctx, `
		INSERT INTO locations (
			id, trip_id, user_id, latitude, longitude, accuracy, 
			location_name, location_type, notes, status, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, 
			$7, $8, $9, $10, $11, $12
		)
	`,
		locationID,
		tripID,
		userID,
		update.Latitude,
		update.Longitude,
		update.Accuracy,
		"",        // location_name
		"user",    // location_type
		"",        // notes
		"active",  // status
		timestamp, // created_at
		timestamp, // updated_at
	)

	if err != nil {
		log.Errorw("Failed to insert location", "userID", userID, "error", err)
		// Return a database error
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
func (ldb *LocationDB) GetTripMemberLocations(ctx context.Context, tripID string) ([]types.MemberLocation, error) {
	log := logger.GetLogger()

	// Use a Common Table Expression (CTE) to find the latest location for each user,
	// then join with user details and filter by timestamp.
	rows, err := ldb.client.GetPool().Query(ctx, `
		WITH latest_locations AS (
			SELECT DISTINCT ON (user_id)
				l.id, l.trip_id, l.user_id, l.latitude, l.longitude, l.accuracy,
				l.created_at as timestamp, l.created_at, l.updated_at,
				u.display_name as user_name, tm.role as user_role
			FROM locations l
			JOIN trip_memberships tm ON l.user_id = tm.user_id AND l.trip_id = tm.trip_id
			LEFT JOIN users u ON l.user_id = u.id -- Use LEFT JOIN if user might not exist in users table
			WHERE l.trip_id = $1
			AND tm.status = 'ACTIVE'
			ORDER BY l.user_id, l.created_at DESC
		)
		SELECT id, trip_id, user_id, latitude, longitude, accuracy,
		       timestamp, created_at, updated_at, user_name, user_role
		FROM latest_locations
		WHERE timestamp > NOW() - INTERVAL '24 hours' -- Filter for recent locations
	`, tripID)

	if err != nil {
		log.Errorw("Failed to query trip member locations", "tripID", tripID, "error", err)
		// Return database error
		return nil, apperrors.NewDatabaseError(fmt.Errorf("failed to query trip member locations: %w", err))
	}
	defer rows.Close()

	var locations []types.MemberLocation
	for rows.Next() {
		var loc types.MemberLocation
		var userName, userRole *string // Use pointers for nullable fields from JOINs

		err := rows.Scan(
			&loc.ID, &loc.TripID, &loc.UserID, &loc.Latitude, &loc.Longitude, &loc.Accuracy,
			&loc.Timestamp, &loc.CreatedAt, &loc.UpdatedAt, &userName, &userRole,
		)

		if err != nil {
			log.Errorw("Failed to scan member location row", "error", err)
			// Decide whether to skip this row or return an error for the whole query
			continue // Skipping problematic row for now
		}

		// Assign values from pointers, handling potential NULLs
		if userName != nil {
			loc.UserName = *userName
		}
		if userRole != nil {
			loc.UserRole = *userRole
		}
		locations = append(locations, loc)
	}

	if err := rows.Err(); err != nil {
		log.Errorw("Error iterating member location rows", "error", err)
		// Return database error
		return nil, apperrors.NewDatabaseError(fmt.Errorf("error iterating member location rows: %w", err))
	}

	return locations, nil
}

// GetUserRole retrieves the role of a user within a specific trip.
// This method might be intended to satisfy an interface like store.LocationStore.
func (ldb *LocationDB) GetUserRole(ctx context.Context, tripID string, userID string) (types.MemberRole, error) {
	log := logger.GetLogger()
	var role string
	err := ldb.client.GetPool().QueryRow(ctx,
		"SELECT role FROM trip_memberships WHERE trip_id = $1 AND user_id = $2 AND status = 'ACTIVE'",
		tripID, userID).Scan(&role)

	if err != nil {
		if err == pgx.ErrNoRows {
			// User is not an active member of this trip.
			return "", apperrors.NotFound("membership", fmt.Sprintf("active user %s in trip %s", userID, tripID))
		}
		log.Errorw("Failed to get user role from database", "tripID", tripID, "userID", userID, "error", err)
		return "", apperrors.NewDatabaseError(err)
	}

	// Validate the retrieved role.
	memberRole := types.MemberRole(role)
	if !memberRole.IsValid() {
		log.Errorw("Invalid role found in database for active member", "role", role, "tripID", tripID, "userID", userID)
		// Return an internal server error as the data state is unexpected.
		return "", apperrors.InternalServerError(fmt.Sprintf("Invalid role '%s' found for user %s in trip %s", role, userID, tripID))
	}

	return memberRole, nil
}
