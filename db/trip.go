package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

type TripDB struct {
	client *DatabaseClient
}

func NewTripDB(client *DatabaseClient) *TripDB {
	return &TripDB{client: client}
}

func (tdb *TripDB) GetPool() *pgxpool.Pool {
	return tdb.client.GetPool()
}

func (tdb *TripDB) CreateTrip(ctx context.Context, trip types.Trip) (string, error) {
	log := logger.GetLogger()
	var tripID string

	err := WithTx(ctx, tdb.GetPool(), func(tx pgx.Tx) error {
		// Create trip
		err := tx.QueryRow(ctx, `
            INSERT INTO trips (
                name, description, start_date, end_date, 
                destination, created_by, status, background_image_url
            ) 
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8) 
            RETURNING id`,
			trip.Name,
			trip.Description,
			trip.StartDate,
			trip.EndDate,
			trip.Destination,
			trip.CreatedBy,
			string(trip.Status),
			trip.BackgroundImageURL,
		).Scan(&tripID)

		if err != nil {
			log.Errorw("Failed to create trip", "error", err)
			return err
		}

		// Add creator as admin
		_, err = tx.Exec(ctx, `
            INSERT INTO trip_memberships (trip_id, user_id, role, status)
            VALUES ($1, $2, $3, $4)`,
			tripID,
			trip.CreatedBy,
			types.MemberRoleOwner,
			types.MembershipStatusActive,
		)
		if err != nil {
			log.Errorw("Failed to add creator as admin", "error", err)
			return err
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return tripID, nil
}

func (tdb *TripDB) GetTrip(ctx context.Context, id string) (*types.Trip, error) {
	log := logger.GetLogger()
	query := `
        SELECT t.id, t.name, t.description, t.start_date, t.end_date,
               t.destination, t.status, t.created_by, t.created_at, t.updated_at
        FROM trips t
        LEFT JOIN metadata m ON m.table_name = 'trips' AND m.record_id = t.id
        WHERE t.id = $1 AND m.deleted_at IS NULL`

	log.Debugw("Executing GetTrip query", "query", query, "tripId", id)

	var trip types.Trip
	err := tdb.client.GetPool().QueryRow(ctx, query, id).Scan(
		&trip.ID,
		&trip.Name,
		&trip.Description,
		&trip.StartDate,
		&trip.EndDate,
		&trip.Destination,
		&trip.Status,
		&trip.CreatedBy,
		&trip.CreatedAt,
		&trip.UpdatedAt,
	)
	if err != nil {
		log.Errorw("Failed to get trip", "tripId", id, "error", err)
		return nil, err
	}

	log.Infow("Fetched trip data", "trip", trip)
	return &trip, nil
}

func (tdb *TripDB) UpdateTrip(ctx context.Context, id string, update types.TripUpdate) error {
	log := logger.GetLogger()

	// Retrieve the current status for validation
	var currentStatusStr string
	err := tdb.client.GetPool().QueryRow(ctx, "SELECT status FROM trips WHERE id = $1", id).Scan(&currentStatusStr)
	if err != nil {
		log.Errorw("Failed to fetch current status for trip", "tripId", id, "error", err)
		return fmt.Errorf("unable to fetch current status for trip %s: %v", id, err)
	}

	currentStatus := types.TripStatus(currentStatusStr)

	// Ensure status transition is valid
	if update.Status != "" && !currentStatus.IsValidTransition(update.Status) {
		log.Errorw("Invalid status transition", "tripId", id, "currentStatus", currentStatus, "requestedStatus", update.Status)
		return fmt.Errorf("invalid status transition: %s -> %s", currentStatus, update.Status)
	}

	var setFields []string
	var args []interface{}
	argPosition := 1

	// Update fields - need to check for nil pointers
	if update.Name != nil && *update.Name != "" {
		setFields = append(setFields, fmt.Sprintf("name = $%d", argPosition))
		args = append(args, *update.Name)
		argPosition++
	}
	if update.Description != nil && *update.Description != "" {
		setFields = append(setFields, fmt.Sprintf("description = $%d", argPosition))
		args = append(args, *update.Description)
		argPosition++
	}
	if update.Destination != nil {
		// Handle JSONB destination
		destJSON, err := json.Marshal(update.Destination)
		if err != nil {
			return fmt.Errorf("failed to marshal destination: %w", err)
		}
		setFields = append(setFields, fmt.Sprintf("destination = $%d", argPosition))
		args = append(args, destJSON)
		argPosition++
	}
	if update.StartDate != nil && !update.StartDate.IsZero() {
		setFields = append(setFields, fmt.Sprintf("start_date = $%d", argPosition))
		args = append(args, *update.StartDate)
		argPosition++
	}
	if update.EndDate != nil && !update.EndDate.IsZero() {
		setFields = append(setFields, fmt.Sprintf("end_date = $%d", argPosition))
		args = append(args, *update.EndDate)
		argPosition++
	}
	if update.Status != "" {
		setFields = append(setFields, fmt.Sprintf("status = $%d", argPosition))
		args = append(args, string(update.Status))
		argPosition++
	}

	setFields = append(setFields, "updated_at = CURRENT_TIMESTAMP")

	if len(setFields) == 0 {
		return nil
	}

	query := fmt.Sprintf(`
        UPDATE trips 
        SET %s
        WHERE id = $%d
        RETURNING status;`,
		strings.Join(setFields, ", "),
		argPosition,
	)

	args = append(args, id)

	// Execute query and validate result
	var updatedStatusStr string
	err = tdb.client.GetPool().QueryRow(ctx, query, args...).Scan(&updatedStatusStr)
	if err != nil {
		log.Errorw("Failed to update trip", "tripId", id, "error", err)
		return err
	}

	// Verify status matches expected value
	if update.Status != "" && updatedStatusStr != string(update.Status) {
		log.Errorw("Mismatch in updated status", "tripId", id, "expected", update.Status, "got", updatedStatusStr)
		return fmt.Errorf("status mismatch: expected %s, got %s", update.Status, updatedStatusStr)
	}

	log.Infow("Trip updated successfully", "tripId", id, "newStatus", updatedStatusStr)
	return nil
}

func (tdb *TripDB) SoftDeleteTrip(ctx context.Context, id string) error {
	log := logger.GetLogger()
	query := `
        INSERT INTO metadata (table_name, record_id, deleted_at)
        VALUES ('trips', $1, CURRENT_TIMESTAMP)
        ON CONFLICT (table_name, record_id) 
        DO UPDATE SET deleted_at = CURRENT_TIMESTAMP
        RETURNING record_id`

	var returnedID string
	err := tdb.client.GetPool().QueryRow(ctx, query, id).Scan(&returnedID)
	if err != nil {
		log.Errorw("Failed to delete trip", "tripId", id, "error", err)
		return errors.NotFound("Trip", id)
	}

	return nil
}

func (tdb *TripDB) ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error) {
	log := logger.GetLogger()
	query := `
    SELECT t.id, t.name, t.description, t.start_date, t.end_date,
           t.destination, t.status, t.created_by, t.created_at, t.updated_at, t.background_image_url
    FROM trips t
    LEFT JOIN metadata m ON m.table_name = 'trips' AND m.record_id = t.id
    WHERE t.created_by = $1 AND m.deleted_at IS NULL
    ORDER BY t.start_date DESC`

	rows, err := tdb.client.GetPool().Query(ctx, query, userID)
	if err != nil {
		log.Errorw("Failed to list user trips", "userId", userID, "error", err)
		return nil, err
	}
	defer rows.Close()

	var trips []*types.Trip
	for rows.Next() {
		var trip types.Trip
		err := rows.Scan(
			&trip.ID,
			&trip.Name,
			&trip.Description,
			&trip.StartDate,
			&trip.EndDate,
			&trip.Destination,
			&trip.Status,
			&trip.CreatedBy,
			&trip.CreatedAt,
			&trip.UpdatedAt,
			&trip.BackgroundImageURL,
		)
		if err != nil {
			log.Errorw("Failed to scan trip row", "error", err)
			return nil, err
		}
		trips = append(trips, &trip)
	}

	if err = rows.Err(); err != nil {
		log.Errorw("Error iterating trip rows", "error", err)
		return nil, err
	}

	return trips, nil
}

func (tdb *TripDB) SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error) {
	log := logger.GetLogger()

	baseQuery := `
        SELECT t.id, t.name, t.description, t.start_date, t.end_date,
               t.destination, t.status, t.created_by, t.created_at, t.updated_at, t.background_image_url
        FROM trips t
        LEFT JOIN metadata m ON m.table_name = 'trips' AND m.record_id::uuid = t.id
        WHERE m.deleted_at IS NULL`

	var conditions []string
	params := make([]interface{}, 0)
	paramCount := 1

	if criteria.Destination != "" {
		conditions = append(conditions, fmt.Sprintf("t.destination ILIKE $%d", paramCount))
		params = append(params, "%"+criteria.Destination+"%")
		paramCount++ // nolint:ineffassign
	}

	if !criteria.StartDateFrom.IsZero() {
		conditions = append(conditions, fmt.Sprintf("t.start_date >= $%d", paramCount))
		params = append(params, criteria.StartDateFrom)
		paramCount++ // nolint:ineffassign
	}

	if !criteria.StartDateTo.IsZero() {
		conditions = append(conditions, fmt.Sprintf("t.start_date <= $%d", paramCount))
		params = append(params, criteria.StartDateTo)
		paramCount++ // nolint:ineffassign
	}

	// Add conditions to base query
	query := baseQuery
	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY t.start_date DESC"

	log.Debugw("Executing search query", "query", query, "params", params)

	rows, err := tdb.client.GetPool().Query(ctx, query, params...)
	if err != nil {
		log.Errorw("Failed to search trips", "error", err, "query", query)
		return nil, err
	}
	defer rows.Close()

	var trips []*types.Trip
	for rows.Next() {
		var trip types.Trip
		err := rows.Scan(
			&trip.ID,
			&trip.Name,
			&trip.Description,
			&trip.StartDate,
			&trip.EndDate,
			&trip.Destination,
			&trip.Status,
			&trip.CreatedBy,
			&trip.CreatedAt,
			&trip.UpdatedAt,
			&trip.BackgroundImageURL,
		)
		if err != nil {
			log.Errorw("Failed to scan trip row", "error", err)
			return nil, err
		}
		trips = append(trips, &trip)
	}

	if err = rows.Err(); err != nil {
		log.Errorw("Error iterating trip rows", "error", err)
		return nil, err
	}

	return trips, nil
}

// AddMember adds a new member to a trip
func (tdb *TripDB) AddMember(ctx context.Context, membership *types.TripMembership) error {
	log := logger.GetLogger()
	query := `
        INSERT INTO trip_memberships (trip_id, user_id, role, status)
        VALUES ($1, $2, $3, $4)
        RETURNING id`

	err := tdb.GetPool().QueryRow(ctx, query,
		membership.TripID,
		membership.UserID,
		membership.Role,
		types.MembershipStatusActive,
	).Scan(&membership.ID)

	if err != nil {
		log.Errorw("Failed to add trip member", "error", err)
		return fmt.Errorf("failed to add member: %w", err)
	}

	return nil
}

// UpdateMemberRole updates a member's role in a trip
func (tdb *TripDB) UpdateMemberRole(ctx context.Context, tripID string, userID string, role types.MemberRole) error {
	log := logger.GetLogger()
	query := `
        UPDATE trip_memberships
        SET role = $1, updated_at = CURRENT_TIMESTAMP
        WHERE trip_id = $2 AND user_id = $3
        RETURNING id`

	var id string
	err := tdb.GetPool().QueryRow(ctx, query, role, tripID, userID).Scan(&id)
	if err != nil {
		log.Errorw("Failed to update member role",
			"tripId", tripID,
			"userId", userID,
			"error", err)
		return fmt.Errorf("failed to update member role: %w", err)
	}

	return nil
}

// RemoveMember removes a member from a trip (soft delete by setting status to INACTIVE)
func (tdb *TripDB) RemoveMember(ctx context.Context, tripID string, userID string) error {
	log := logger.GetLogger()
	query := `
        UPDATE trip_memberships
        SET status = $1, updated_at = CURRENT_TIMESTAMP
        WHERE trip_id = $2 AND user_id = $3
        RETURNING id`

	var id string
	err := tdb.GetPool().QueryRow(ctx, query,
		types.MembershipStatusInactive,
		tripID,
		userID,
	).Scan(&id)

	if err != nil {
		log.Errorw("Failed to remove trip member",
			"tripId", tripID,
			"userId", userID,
			"error", err)
		return fmt.Errorf("failed to remove member: %w", err)
	}

	return nil
}

// GetTripMembers gets all active members of a trip
func (tdb *TripDB) GetTripMembers(ctx context.Context, tripID string) ([]types.TripMembership, error) {
	log := logger.GetLogger()
	query := `
        SELECT id, trip_id, user_id, role, status, created_at, updated_at
        FROM trip_memberships
        WHERE trip_id = $1 AND status = $2
        ORDER BY created_at ASC`

	rows, err := tdb.GetPool().Query(ctx, query, tripID, types.MembershipStatusActive)
	if err != nil {
		log.Errorw("Failed to get trip members", "tripId", tripID, "error", err)
		return nil, fmt.Errorf("failed to get trip members: %w", err)
	}
	defer rows.Close()

	var members []types.TripMembership
	for rows.Next() {
		var member types.TripMembership
		err := rows.Scan(
			&member.ID,
			&member.TripID,
			&member.UserID,
			&member.Role,
			&member.Status,
			&member.CreatedAt,
			&member.UpdatedAt,
		)
		if err != nil {
			log.Errorw("Failed to scan trip member", "error", err)
			return nil, fmt.Errorf("failed to scan member: %w", err)
		}
		members = append(members, member)
	}

	if err = rows.Err(); err != nil {
		log.Errorw("Error iterating trip members", "error", err)
		return nil, fmt.Errorf("error iterating members: %w", err)
	}

	return members, nil
}

// GetUserRole gets a user's role in a trip
func (tdb *TripDB) GetUserRole(ctx context.Context, tripID string, userID string) (types.MemberRole, error) {
	log := logger.GetLogger()
	query := `
        SELECT role
        FROM trip_memberships
        WHERE trip_id = $1 AND user_id = $2 AND status = $3`

	var role types.MemberRole
	err := tdb.GetPool().QueryRow(ctx, query,
		tripID,
		userID,
		types.MembershipStatusActive,
	).Scan(&role)

	if err != nil {
		log.Errorw("Failed to get user role",
			"tripId", tripID,
			"userId", userID,
			"error", err)
		return "", fmt.Errorf("failed to get user role: %w", err)
	}

	return role, nil
}
