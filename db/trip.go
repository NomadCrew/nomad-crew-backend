// Package db provides database access functionality.
//
// DEPRECATED: This package and its contents are deprecated and will be removed in a future version.
// All functionality has been migrated to the new store pattern in store/postgres/trip_store_pg.go.
//
// Migration Guide:
// - Replace db.NewTripDB() with store/postgres.NewPgTripStore()
// - Update imports to use internal_store.TripStore instead of db.TripDB
// - All methods have equivalent implementations in the new store
// - Transaction handling is now more explicit with BeginTx(), Commit(), and Rollback()
// - Error handling has been standardized with custom error types
//
// This file will be removed in version 2.0.0.
package db

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

// TripDB provides methods for interacting with trip data in the database.
// It uses a DatabaseClient for managing the connection pool.
type TripDB struct {
	client *DatabaseClient
}

// NewTripDB creates a new instance of TripDB.
func NewTripDB(client *DatabaseClient) *TripDB {
	return &TripDB{client: client}
}

// GetPool returns the underlying pgxpool.Pool used by the TripDB client.
// This might be useful for operations requiring direct pool access (like transactions).
func (tdb *TripDB) GetPool() *pgxpool.Pool {
	return tdb.client.GetPool()
}

// CreateTrip inserts a new trip record into the database and adds the creator
// as the initial owner/admin member in a single transaction.
// Returns the newly created trip's ID or an error.
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

// GetTrip retrieves a single, non-soft-deleted trip by its ID.
func (tdb *TripDB) GetTrip(ctx context.Context, id string) (*types.Trip, error) {
	log := logger.GetLogger()
	query := `
        SELECT t.id, t.name, t.description, t.start_date, t.end_date,
               t.destination, t.status, t.created_by, t.created_at, t.updated_at, t.background_image_url
        FROM trips t
        WHERE t.id = $1
        AND NOT EXISTS (
            SELECT 1 FROM metadata m 
            WHERE m.table_name = 'trips' 
            AND m.record_id = t.id 
            AND m.deleted_at IS NOT NULL
        )`

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
		&trip.BackgroundImageURL,
	)
	if err != nil {
		log.Errorw("Failed to get trip", "tripId", id, "error", err)
		return nil, err
	}

	log.Infow("Fetched trip data", "trip", trip)
	return &trip, nil
}

// UpdateTrip updates specified fields of an existing trip.
// It dynamically builds the UPDATE query based on non-nil fields in the update struct.
// It also validates status transitions before applying the update.
// Returns the updated trip details by calling GetTrip upon success.
func (tdb *TripDB) UpdateTrip(ctx context.Context, id string, update types.TripUpdate) (*types.Trip, error) {
	log := logger.GetLogger()

	// Retrieve the current status for validation
	var currentStatusStr string
	err := tdb.client.GetPool().QueryRow(ctx, "SELECT status FROM trips WHERE id = $1", id).Scan(&currentStatusStr)
	if err != nil {
		log.Errorw("Failed to fetch current status for trip", "tripId", id, "error", err)
		return nil, fmt.Errorf("unable to fetch current status for trip %s: %v", id, err)
	}

	currentStatus := types.TripStatus(currentStatusStr)

	// Ensure status transition is valid
	if update.Status != "" && !currentStatus.IsValidTransition(update.Status) {
		log.Errorw("Invalid status transition", "tripId", id, "currentStatus", currentStatus, "requestedStatus", update.Status)
		return nil, fmt.Errorf("invalid status transition: %s -> %s", currentStatus, update.Status)
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
			return nil, fmt.Errorf("failed to marshal destination: %w", err)
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
		return nil, nil
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
		return nil, err
	}

	// Verify status matches expected value
	if update.Status != "" && updatedStatusStr != string(update.Status) {
		log.Errorw("Mismatch in updated status", "tripId", id, "expected", update.Status, "got", updatedStatusStr)
		return nil, fmt.Errorf("status mismatch: expected %s, got %s", update.Status, updatedStatusStr)
	}

	log.Infow("Trip updated successfully", "tripId", id, "newStatus", updatedStatusStr)
	return tdb.GetTrip(ctx, id)
}

// SoftDeleteTrip marks a trip as deleted by adding a record to the metadata table.
// It uses INSERT ... ON CONFLICT DO UPDATE to handle potential re-deletions idempotently.
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

// ListUserTrips retrieves all non-soft-deleted trips created by a specific user,
// ordered by start date descending.
func (tdb *TripDB) ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error) {
	log := logger.GetLogger()
	query := `
    SELECT t.id, t.name, t.description, t.start_date, t.end_date,
           t.destination, t.status, t.created_by, t.created_at, t.updated_at, t.background_image_url
    FROM trips t
    WHERE t.created_by = $1
    AND NOT EXISTS (
        SELECT 1 FROM metadata m 
        WHERE m.table_name = 'trips' 
        AND m.record_id = t.id 
        AND m.deleted_at IS NOT NULL
    )
    ORDER BY t.start_date DESC`

	rows, err := tdb.client.GetPool().Query(ctx, query, userID)
	if err != nil {
		log.Errorw("Failed to query user trips", "userID", userID, "error", err)
		return nil, errors.NewDatabaseError(fmt.Errorf("failed to list trips for user %s: %w", userID, err))
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
			log.Errorw("Failed to scan user trip row", "userID", userID, "error", err)
			// Return partial results
			return trips, errors.NewDatabaseError(fmt.Errorf("failed scanning trip for user %s: %w", userID, err))
		}
		trips = append(trips, &trip)
	}

	if err := rows.Err(); err != nil {
		log.Errorw("Error iterating user trip rows", "userID", userID, "error", err)
		return trips, errors.NewDatabaseError(fmt.Errorf("error iterating trips for user %s: %w", userID, err))
	}

	return trips, nil
}

// SearchTrips retrieves non-soft-deleted trips matching the provided criteria.
// It dynamically builds a WHERE clause based on the criteria fields (Destination, StartDate, EndDate).
// Supports partial destination matching (case-insensitive) and date range filtering.
func (tdb *TripDB) SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error) {
	log := logger.GetLogger()
	var conditions []string
	var args []interface{}
	argCount := 1

	// Base query selects non-deleted trips.
	baseQuery := `
        SELECT t.id, t.name, t.description, t.start_date, t.end_date,
               t.destination, t.status, t.created_by, t.created_at, t.updated_at, t.background_image_url
        FROM trips t
        WHERE NOT EXISTS (
            SELECT 1 FROM metadata m 
            WHERE m.table_name = 'trips' AND m.record_id = t.id AND m.deleted_at IS NOT NULL
        )`

	// Add conditions based on search criteria.
	if criteria.Destination != "" { // Filter based on Destination (address field within JSONB)
		conditions = append(conditions, fmt.Sprintf("t.destination->>'address' ILIKE $%d", argCount))
		args = append(args, "%"+criteria.Destination+"%") // Case-insensitive partial match
		argCount++
	}
	// Removed Status filter as it caused linter errors (field likely missing from criteria struct)
	if !criteria.StartDate.IsZero() {
		conditions = append(conditions, fmt.Sprintf("t.start_date >= $%d", argCount))
		args = append(args, criteria.StartDate)
		argCount++
	}
	if !criteria.EndDate.IsZero() {
		conditions = append(conditions, fmt.Sprintf("t.end_date <= $%d", argCount))
		args = append(args, criteria.EndDate)
		argCount++
	}

	// Combine base query with conditions.
	finalQuery := baseQuery
	if len(conditions) > 0 {
		finalQuery += " AND " + strings.Join(conditions, " AND ")
	}
	finalQuery += " ORDER BY t.start_date DESC" // Default ordering

	log.Debugw("Executing SearchTrips query", "query", finalQuery, "args", args)

	rows, err := tdb.client.GetPool().Query(ctx, finalQuery, args...)
	if err != nil {
		log.Errorw("Failed to execute search trips query", "criteria", criteria, "error", err)
		return nil, errors.NewDatabaseError(fmt.Errorf("failed searching trips: %w", err))
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
			log.Errorw("Failed to scan search trip row", "criteria", criteria, "error", err)
			return trips, errors.NewDatabaseError(fmt.Errorf("failed scanning searched trip: %w", err))
		}
		trips = append(trips, &trip)
	}

	if err := rows.Err(); err != nil {
		log.Errorw("Error iterating search trip rows", "criteria", criteria, "error", err)
		return trips, errors.NewDatabaseError(fmt.Errorf("error iterating searched trips: %w", err))
	}

	return trips, nil
}

// AddMember adds a new membership record to the trip_memberships table.
// It uses INSERT ... ON CONFLICT DO UPDATE to handle cases where the user
// might already have an inactive membership, reactivating it with the specified role.
func (tdb *TripDB) AddMember(ctx context.Context, membership *types.TripMembership) error {
	log := logger.GetLogger()
	query := `
        INSERT INTO trip_memberships (trip_id, user_id, role, status)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (trip_id, user_id) 
        DO UPDATE SET 
            role = EXCLUDED.role,
            status = EXCLUDED.status,
            updated_at = CURRENT_TIMESTAMP
        WHERE trip_memberships.status != 'ACTIVE' -- Only update if not already active
    `
	_, err := tdb.client.GetPool().Exec(ctx, query,
		membership.TripID,
		membership.UserID,
		membership.Role,
		membership.Status,
	)
	if err != nil {
		log.Errorw("Failed to add or update trip member", "tripID", membership.TripID, "userID", membership.UserID, "error", err)
		return errors.NewDatabaseError(fmt.Errorf("failed to add/update member: %w", err))
	}
	return nil
}

// UpdateMemberRole updates the role of an existing active member in a trip.
func (tdb *TripDB) UpdateMemberRole(ctx context.Context, tripID string, userID string, role types.MemberRole) error {
	log := logger.GetLogger()
	query := `
        UPDATE trip_memberships 
        SET role = $1, updated_at = CURRENT_TIMESTAMP
        WHERE trip_id = $2 AND user_id = $3 AND status = 'ACTIVE'
        RETURNING trip_id -- Return something to check affected rows
    `
	var returnedID string
	err := tdb.client.GetPool().QueryRow(ctx, query, role, tripID, userID).Scan(&returnedID)
	if err != nil {
		if err == pgx.ErrNoRows {
			log.Warnw("Failed to update role: active member not found", "tripID", tripID, "userID", userID, "role", role)
			return errors.NotFound("Active Trip Member", fmt.Sprintf("user %s in trip %s", userID, tripID))
		}
		log.Errorw("Failed to update member role", "tripID", tripID, "userID", userID, "role", role, "error", err)
		return errors.NewDatabaseError(fmt.Errorf("failed updating role: %w", err))
	}
	return nil
}

// RemoveMember marks a trip membership as inactive.
// This performs a logical delete by updating the status rather than removing the row.
func (tdb *TripDB) RemoveMember(ctx context.Context, tripID string, userID string) error {
	log := logger.GetLogger()
	// Instead of deleting, update status to INACTIVE.
	query := `
        UPDATE trip_memberships 
        SET status = $1, updated_at = CURRENT_TIMESTAMP
        WHERE trip_id = $2 AND user_id = $3 AND status = 'ACTIVE' -- Only deactivate active members
        RETURNING trip_id -- Return something to check affected rows
    `
	var returnedID string
	err := tdb.client.GetPool().QueryRow(ctx, query, types.MembershipStatusInactive, tripID, userID).Scan(&returnedID)
	if err != nil {
		if err == pgx.ErrNoRows {
			// No active member found to remove.
			log.Warnw("Attempted to remove non-existent or inactive member", "tripID", tripID, "userID", userID)
			// Consider this success for idempotency.
			return nil // Idempotent success
		}
		log.Errorw("Failed to remove (deactivate) trip member", "tripID", tripID, "userID", userID, "error", err)
		return errors.NewDatabaseError(fmt.Errorf("failed removing member: %w", err))
	}
	log.Infow("Successfully removed (deactivated) trip member", "tripID", tripID, "userID", userID)
	return nil
}

// GetTripMembers retrieves all active memberships for a specific trip.
// Note: This only returns membership details, not joined user information.
func (tdb *TripDB) GetTripMembers(ctx context.Context, tripID string) ([]types.TripMembership, error) {
	log := logger.GetLogger()
	// Reverted query to select only from trip_memberships
	query := `
        SELECT id, trip_id, user_id, role, status, created_at, updated_at
        FROM trip_memberships
        WHERE trip_id = $1 AND status = $2
        ORDER BY created_at ASC
    `
	rows, err := tdb.client.GetPool().Query(ctx, query, tripID, types.MembershipStatusActive)
	if err != nil {
		log.Errorw("Failed to query active trip members", "tripID", tripID, "error", err)
		return nil, errors.NewDatabaseError(fmt.Errorf("failed getting members for trip %s: %w", tripID, err))
	}
	defer rows.Close()

	var members []types.TripMembership
	for rows.Next() {
		var member types.TripMembership
		// Reverted scan to match the reverted query
		err := rows.Scan(
			&member.ID, // Assuming ID field exists
			&member.TripID,
			&member.UserID,
			&member.Role,
			&member.Status,
			&member.CreatedAt, // Assuming CreatedAt exists
			&member.UpdatedAt, // Assuming UpdatedAt exists
		)
		if err != nil {
			log.Errorw("Failed to scan trip member row", "tripID", tripID, "error", err)
			return members, errors.NewDatabaseError(fmt.Errorf("failed scanning member for trip %s: %w", tripID, err))
		}
		// Removed population of member.User as it's not selected and likely doesn't exist
		members = append(members, member)
	}

	if err := rows.Err(); err != nil {
		log.Errorw("Error iterating trip member rows", "tripID", tripID, "error", err)
		return members, errors.NewDatabaseError(fmt.Errorf("error iterating members for trip %s: %w", tripID, err))
	}

	return members, nil
}

// GetUserRole retrieves the role of a specific user within a specific trip.
// Duplicate of the method in location.go, likely intended to satisfy an interface here too.
// TODO: Consolidate GetUserRole implementations if possible.
func (tdb *TripDB) GetUserRole(ctx context.Context, tripID string, userID string) (types.MemberRole, error) {
	log := logger.GetLogger()
	var role string
	err := tdb.client.GetPool().QueryRow(ctx,
		"SELECT role FROM trip_memberships WHERE trip_id = $1 AND user_id = $2 AND status = 'ACTIVE'",
		tripID, userID).Scan(&role)

	if err != nil {
		if err == pgx.ErrNoRows {
			// User is not an active member of this trip.
			return "", errors.NotFound("membership", fmt.Sprintf("active user %s in trip %s", userID, tripID))
		}
		log.Errorw("Failed to get user role from database", "tripID", tripID, "userID", userID, "error", err)
		return "", errors.NewDatabaseError(fmt.Errorf("failed getting role for user %s in trip %s: %w", userID, tripID, err))
	}

	// Validate the retrieved role.
	memberRole := types.MemberRole(role)
	if !memberRole.IsValid() {
		log.Errorw("Invalid role found in database for active member", "role", role, "tripID", tripID, "userID", userID)
		// Return an internal server error as the data state is unexpected.
		return "", errors.InternalServerError(fmt.Sprintf("Invalid role '%s' found for user %s in trip %s", role, userID, tripID))
	}

	return memberRole, nil
}

// LookupUserByEmail finds a user in the local users table by their email address.
// Note: Assumes a local 'users' table exists and is kept in sync with authentication provider.
// Consider security implications: avoid exposing sensitive user data unnecessarily.
func (tdb *TripDB) LookupUserByEmail(ctx context.Context, email string) (*types.SupabaseUser, error) {
	log := logger.GetLogger()
	query := `SELECT id, email, user_metadata FROM users WHERE email = $1 LIMIT 1`

	var user types.SupabaseUser // Using SupabaseUser, adjust if a different local type is used
	var userMetadataJSON []byte

	err := tdb.client.GetPool().QueryRow(ctx, query, email).Scan(&user.ID, &user.Email, &userMetadataJSON)
	if err != nil {
		if err == pgx.ErrNoRows {
			log.Infow("User not found by email in local table", "email", email)
			return nil, errors.NotFound("User", email)
		}
		log.Errorw("Failed to lookup user by email", "email", email, "error", err)
		return nil, errors.NewDatabaseError(fmt.Errorf("failed looking up email %s: %w", email, err))
	}

	// Unmarshal user_metadata if needed
	if len(userMetadataJSON) > 0 {
		if err := json.Unmarshal(userMetadataJSON, &user.UserMetadata); err != nil {
			log.Errorw("Failed to unmarshal user metadata during email lookup", "email", email, "error", err)
			// Return user data found so far, but log the metadata issue
			return &user, fmt.Errorf("failed unmarshalling metadata for user %s: %w", email, err)
		}
	}

	return &user, nil
}

// CreateInvitation inserts a new trip invitation record into the database.
func (tdb *TripDB) CreateInvitation(ctx context.Context, invitation *types.TripInvitation) error {
	log := logger.GetLogger()
	query := `
        INSERT INTO trip_invitations 
            (id, trip_id, inviter_id, invitee_email, role, status, expires_at, created_at, updated_at)
        VALUES 
            ($1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
        RETURNING id -- Return ID to confirm insertion
    `
	var returnedID string
	err := tdb.client.GetPool().QueryRow(ctx, query,
		invitation.ID, // Assuming ID is pre-generated (e.g., UUID)
		invitation.TripID,
		invitation.InviterID,
		invitation.InviteeEmail,
		invitation.Role,
		invitation.Status,
		invitation.ExpiresAt,
	).Scan(&returnedID)

	if err != nil {
		log.Errorw("Failed to create trip invitation", "tripID", invitation.TripID, "inviteeEmail", invitation.InviteeEmail, "error", err)
		return errors.NewDatabaseError(fmt.Errorf("failed creating invitation: %w", err))
	}

	if returnedID != invitation.ID {
		log.Errorw("CreateInvitation returned mismatching ID", "expected", invitation.ID, "got", returnedID)
		return errors.InternalServerError("database returned unexpected ID during invitation creation")
	}

	log.Infow("Trip invitation created successfully", "invitationID", invitation.ID)
	return nil
}

// GetInvitation retrieves a specific trip invitation by its ID.
func (tdb *TripDB) GetInvitation(ctx context.Context, invitationID string) (*types.TripInvitation, error) {
	log := logger.GetLogger()
	query := `
        SELECT id, trip_id, inviter_id, invitee_email, role, status, expires_at, created_at
        FROM trip_invitations
        WHERE id = $1
    `
	var inv types.TripInvitation
	err := tdb.client.GetPool().QueryRow(ctx, query, invitationID).Scan(
		&inv.ID,
		&inv.TripID,
		&inv.InviterID,
		&inv.InviteeEmail,
		&inv.Role,
		&inv.Status,
		&inv.ExpiresAt,
		&inv.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			log.Warnw("Trip invitation not found", "invitationID", invitationID)
			return nil, errors.NotFound("Invitation", invitationID)
		}
		log.Errorw("Failed to get trip invitation", "invitationID", invitationID, "error", err)
		return nil, errors.NewDatabaseError(fmt.Errorf("failed getting invitation %s: %w", invitationID, err))
	}

	return &inv, nil
}

// GetInvitationsByTripID retrieves all invitations associated with a specific trip ID.
func (tdb *TripDB) GetInvitationsByTripID(ctx context.Context, tripID string) ([]*types.TripInvitation, error) {
	log := logger.GetLogger()
	query := `
        SELECT id, trip_id, inviter_id, invitee_email, role, status, expires_at, created_at
        FROM trip_invitations
        WHERE trip_id = $1
        ORDER BY created_at DESC
    `
	rows, err := tdb.client.GetPool().Query(ctx, query, tripID)
	if err != nil {
		log.Errorw("Failed to query invitations by trip ID", "tripID", tripID, "error", err)
		return nil, errors.NewDatabaseError(fmt.Errorf("failed getting invitations for trip %s: %w", tripID, err))
	}
	defer rows.Close()

	invitations := make([]*types.TripInvitation, 0)
	for rows.Next() {
		var inv types.TripInvitation
		err := rows.Scan(
			&inv.ID,
			&inv.TripID,
			&inv.InviterID,
			&inv.InviteeEmail,
			&inv.Role,
			&inv.Status,
			&inv.ExpiresAt,
			&inv.CreatedAt,
		)
		if err != nil {
			log.Errorw("Failed to scan trip invitation row", "tripID", tripID, "error", err)
			return invitations, errors.NewDatabaseError(fmt.Errorf("failed scanning invitation for trip %s: %w", tripID, err))
		}
		invitations = append(invitations, &inv)
	}

	if err := rows.Err(); err != nil {
		log.Errorw("Error iterating trip invitation rows", "tripID", tripID, "error", err)
		return invitations, errors.NewDatabaseError(fmt.Errorf("error iterating invitations for trip %s: %w", tripID, err))
	}

	return invitations, nil
}

// UpdateInvitationStatus updates the status of an existing trip invitation.
func (tdb *TripDB) UpdateInvitationStatus(ctx context.Context, invitationID string, status types.InvitationStatus) error {
	log := logger.GetLogger()
	query := `
        UPDATE trip_invitations
        SET status = $1, updated_at = CURRENT_TIMESTAMP
        WHERE id = $2
        RETURNING id -- Return ID to confirm update occurred
    `
	var returnedID string
	err := tdb.client.GetPool().QueryRow(ctx, query, status, invitationID).Scan(&returnedID)
	if err != nil {
		if err == pgx.ErrNoRows {
			log.Warnw("Failed to update invitation status: invitation not found", "invitationID", invitationID, "status", status)
			return errors.NotFound("Invitation", invitationID)
		}
		log.Errorw("Failed to update invitation status", "invitationID", invitationID, "status", status, "error", err)
		return errors.NewDatabaseError(fmt.Errorf("failed updating status for invitation %s: %w", invitationID, err))
	}

	if returnedID != invitationID {
		log.Errorw("UpdateInvitationStatus returned mismatching ID", "expected", invitationID, "got", returnedID)
		return errors.InternalServerError("database returned unexpected ID during invitation status update")
	}

	log.Infow("Invitation status updated successfully", "invitationID", invitationID, "newStatus", status)
	return nil
}

// BeginTx starts a new database transaction.
// It returns a Transaction interface which can be used to commit or rollback.
// Note: This seems redundant with the WithTx helper function in db.go.
// Consider using WithTx for managing transactions where possible.
func (tdb *TripDB) BeginTx(ctx context.Context) (store.Transaction, error) {
	tx, err := tdb.GetPool().Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return &txWrapper{tx: tx}, nil
}

// txWrapper wraps a pgx.Tx to satisfy the store.Transaction interface.
type txWrapper struct {
	tx pgx.Tx
}

// Commit commits the underlying transaction.
func (t *txWrapper) Commit() error {
	return t.tx.Commit(context.Background()) // Using background context for commit
}

// Rollback rolls back the underlying transaction.
func (t *txWrapper) Rollback() error {
	return t.tx.Rollback(context.Background()) // Using background context for rollback
}

// Commit is currently a placeholder and does not commit any transaction managed by TripDB.
// Deprecated: This method seems incorrectly placed on TripDB. Use txWrapper.Commit() or WithTx.
func (tdb *TripDB) Commit() error {
	// This method likely needs context or a transaction reference to be meaningful.
	logger.GetLogger().Warn("TripDB.Commit called, but it has no effect.")
	return fmt.Errorf("TripDB.Commit is not implemented")
}

// Rollback is currently a placeholder and does not roll back any transaction managed by TripDB.
// Deprecated: This method seems incorrectly placed on TripDB. Use txWrapper.Rollback() or WithTx.
func (tdb *TripDB) Rollback() error {
	// This method likely needs context or a transaction reference to be meaningful.
	logger.GetLogger().Warn("TripDB.Rollback called, but it has no effect.")
	return fmt.Errorf("TripDB.Rollback is not implemented")
}

// getCallerInfo returns a string representation of the caller's file and line number.
// Useful for debugging purposes.
func getCallerInfo() string {
	// skip = 1 to get the caller of getCallerInfo
	pc, file, line, ok := runtime.Caller(1)
	if !ok {
		return "unknown:0"
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return fmt.Sprintf("%s:%d", filepath.Base(file), line)
	}
	// Return function name along with file and line
	return fmt.Sprintf("%s - %s:%d", filepath.Base(fn.Name()), filepath.Base(file), line)
}

// GetTripByID retrieves a single trip by ID.
// Deprecated: This is likely a duplicate of GetTrip defined earlier in the file.
// Use GetTrip instead.
func (tdb *TripDB) GetTripByID(ctx context.Context, id string) (*types.Trip, error) {
	log := logger.GetLogger()
	log.Warnw("GetTripByID called, likely a duplicate of GetTrip", "tripId", id)
	return tdb.GetTrip(ctx, id) // Delegate to the primary GetTrip method
}
