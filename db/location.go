package db

import (
	"context"
	"fmt"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
)

// LocationDB handles database operations for locations
type LocationDB struct {
	client *DatabaseClient
}

// NewLocationDB creates a new LocationDB instance
func NewLocationDB(client *DatabaseClient) *LocationDB {
	return &LocationDB{
		client: client,
	}
}

// UpdateLocation stores a user's location in the database
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
			log.Warnw("User is not a member of any active trips", "userID", userID)
			return nil, fmt.Errorf("user is not a member of any active trips")
		}
		log.Errorw("Failed to check user trip membership", "userID", userID, "error", err)
		return nil, fmt.Errorf("failed to check user trip membership: %w", err)
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
		return nil, fmt.Errorf("failed to insert location: %w", err)
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

// GetTripMemberLocations retrieves the latest locations for all members of a trip
func (ldb *LocationDB) GetTripMemberLocations(ctx context.Context, tripID string) ([]types.MemberLocation, error) {
	log := logger.GetLogger()

	// Query to get the latest location for each member of the trip
	rows, err := ldb.client.GetPool().Query(ctx, `
		WITH latest_locations AS (
			SELECT DISTINCT ON (user_id) 
				l.id, l.trip_id, l.user_id, l.latitude, l.longitude, l.accuracy, 
				l.created_at as timestamp, l.created_at, l.updated_at,
				u.display_name as user_name, tm.role as user_role
			FROM locations l
			JOIN trip_memberships tm ON l.user_id = tm.user_id AND l.trip_id = tm.trip_id
			LEFT JOIN users u ON l.user_id = u.id
			WHERE l.trip_id = $1
			AND tm.status = 'ACTIVE'
			ORDER BY l.user_id, l.created_at DESC
		)
		SELECT * FROM latest_locations
		WHERE created_at > NOW() - INTERVAL '24 hours'
	`, tripID)

	if err != nil {
		log.Errorw("Failed to query trip member locations", "tripID", tripID, "error", err)
		return nil, fmt.Errorf("failed to query trip member locations: %w", err)
	}
	defer rows.Close()

	var locations []types.MemberLocation
	for rows.Next() {
		var loc types.MemberLocation
		var userName, userRole string

		err := rows.Scan(
			&loc.ID, &loc.TripID, &loc.UserID, &loc.Latitude, &loc.Longitude, &loc.Accuracy,
			&loc.Timestamp, &loc.CreatedAt, &loc.UpdatedAt, &userName, &userRole,
		)

		if err != nil {
			log.Errorw("Failed to scan location row", "error", err)
			continue
		}

		loc.UserName = userName
		loc.UserRole = userRole
		locations = append(locations, loc)
	}

	if err := rows.Err(); err != nil {
		log.Errorw("Error iterating location rows", "error", err)
		return nil, fmt.Errorf("error iterating location rows: %w", err)
	}

	return locations, nil
}
