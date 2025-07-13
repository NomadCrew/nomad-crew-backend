package postgres

import (
	"context"
	"encoding/json" // Added for JSON handling
	"errors"        // Added for error checking
	"fmt"           // Added for error formatting
	"strings"       // Added for string joining

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"              // Added for custom errors
	internal_store "github.com/NomadCrew/nomad-crew-backend/internal/store" // Use alias to avoid conflict
	"github.com/NomadCrew/nomad-crew-backend/logger"                        // Added for logging
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v4" // Added for pgx.Tx
	"github.com/jackc/pgx/v4/pgxpool"
)

// Ensure pgTripStore implements internal_store.TripStore.
var _ internal_store.TripStore = (*pgTripStore)(nil)

type pgTripStore struct {
	pool *pgxpool.Pool
	// Potentially add a logger instance if needed
}

// NewPgTripStore creates a new PostgreSQL trip store.
func NewPgTripStore(pool *pgxpool.Pool) internal_store.TripStore {
	return &pgTripStore{pool: pool}
}

// --- Implement internal_store.TripStore methods below ---

// GetPool implements internal_store.TripStore.
func (s *pgTripStore) GetPool() *pgxpool.Pool {
	// TODO: Confirm if exposing the raw pool is desired in the new pattern.
	// If transactions are handled differently, this might be removed.
	return s.pool
}

// CreateTrip implements internal_store.TripStore.
// It inserts a new trip record and adds the creator as an owner member within a transaction.
func (s *pgTripStore) CreateTrip(ctx context.Context, trip types.Trip) (string, error) {
	log := logger.GetLogger()
	var tripID string

	// Log the type and value of CreatedBy before the insert
	if trip.CreatedBy != nil {
		log.Infow("[DEBUG] Type and value of trip.CreatedBy before insert", "type", fmt.Sprintf("%T", trip.CreatedBy), "value", *trip.CreatedBy)
	} else {
		log.Infow("[DEBUG] trip.CreatedBy is nil before insert")
	}

	// Use the WithTx helper (assuming it's moved or accessible)
	err := WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		// Create trip
		err := tx.QueryRow(ctx, `
            INSERT INTO trips (
                name, description, start_date, end_date,
                destination_place_id, destination_address, destination_name, destination_latitude, destination_longitude, 
                created_by, status, background_image_url
            )
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
            RETURNING id`,
			trip.Name,
			trip.Description,
			trip.StartDate,
			trip.EndDate,
			trip.DestinationPlaceID,
			trip.DestinationAddress,
			trip.DestinationName,
			trip.DestinationLatitude,
			trip.DestinationLongitude,
			trip.CreatedBy,
			string(trip.Status), // Status is already types.TripStatus, direct string conversion is fine
			trip.BackgroundImageURL,
		).Scan(&tripID)

		if err != nil {
			log.Errorw("Failed to create trip in transaction", "error", err)
			// TODO: Wrap error with more context? e.g., using custom error types
			return fmt.Errorf("failed to insert trip: %w", err)
		}

		// Add creator as admin
		_, err = tx.Exec(ctx, `
            INSERT INTO trip_memberships (trip_id, user_id, role, status)
            VALUES ($1, $2, $3, $4)`,
			tripID,
			trip.CreatedBy,
			types.MemberRoleOwner,        // Assuming types.MemberRoleOwner is still the intended role
			types.MembershipStatusActive, // Assuming types.MembershipStatusActive is still the intended status
		)
		if err != nil {
			log.Errorw("Failed to add creator as owner member in transaction", "tripId", tripID, "userId", trip.CreatedBy, "error", err)
			// TODO: Wrap error with more context?
			return fmt.Errorf("failed to add creator membership: %w", err)
		}

		log.Infow("Successfully created trip and added owner in transaction", "tripId", tripID, "userId", trip.CreatedBy)
		return nil
	})

	if err != nil {
		// Log the final error after transaction attempt
		log.Errorw("CreateTrip transaction failed", "error", err)
		// Return the wrapped error from the transaction helper
		return "", err
	}

	log.Infow("Successfully created trip", "tripId", tripID)
	return tripID, nil
}

// GetTrip implements internal_store.TripStore.
// Retrieves a single, non-soft-deleted trip by its ID.
func (s *pgTripStore) GetTrip(ctx context.Context, id string) (*types.Trip, error) {
	log := logger.GetLogger()
	query := `
        SELECT t.id, t.name, t.description, t.start_date, t.end_date,
               t.destination_place_id, t.destination_address, t.destination_name, 
               t.destination_latitude, t.destination_longitude,
               t.status, t.created_by, t.created_at, t.updated_at, t.deleted_at, t.background_image_url
        FROM trips t
        WHERE t.id = $1 AND t.deleted_at IS NULL`

	log.Debugw("Executing GetTrip query", "query", query, "tripId", id)

	var trip types.Trip
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&trip.ID,
		&trip.Name,
		&trip.Description,
		&trip.StartDate,
		&trip.EndDate,
		&trip.DestinationPlaceID,
		&trip.DestinationAddress,
		&trip.DestinationName,
		&trip.DestinationLatitude,
		&trip.DestinationLongitude,
		&trip.Status,
		&trip.CreatedBy,
		&trip.CreatedAt,
		&trip.UpdatedAt,
		&trip.DeletedAt,
		&trip.BackgroundImageURL,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Warnw("Trip not found or soft-deleted", "tripId", id)
			return nil, apperrors.NotFound("trip", id)
		}
		// Log the unexpected database error
		log.Errorw("Failed to get trip from database", "tripId", id, "error", err)
		// Wrap the error for upstream handling
		return nil, fmt.Errorf("failed to execute GetTrip query: %w", err)
	}

	log.Infow("Fetched trip data successfully", "tripId", id)
	return &trip, nil
}

// UpdateTrip implements internal_store.TripStore.
// Updates specified fields of an existing trip, validates status transitions,
// and returns the updated trip details.
func (s *pgTripStore) UpdateTrip(ctx context.Context, id string, update types.TripUpdate) (*types.Trip, error) {
	log := logger.GetLogger()

	// Retrieve the current status for validation
	var currentStatusStr string
	// Also fetch deleted_at to ensure we don't update a deleted trip
	var deletedAtCheck interface{} // Use interface{} to scan potentially NULL value
	err := s.pool.QueryRow(ctx, "SELECT status, deleted_at FROM trips WHERE id = $1", id).Scan(&currentStatusStr, &deletedAtCheck)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Warnw("Trip not found during update attempt", "tripId", id)
			return nil, apperrors.NotFound("trip", id)
		}
		log.Errorw("Failed to fetch current status for trip update", "tripId", id, "error", err)
		return nil, fmt.Errorf("unable to fetch current status for trip %s: %w", id, err)
	}

	if deletedAtCheck != nil { // If deleted_at is not NULL
		log.Warnw("Attempted to update an already soft-deleted trip", "tripId", id)
		return nil, apperrors.NotFound("trip", id) // Or a specific error like "Conflict" or "Gone"
	}

	currentStatus := types.TripStatus(currentStatusStr)

	// Ensure status transition is valid if a new status is provided
	if update.Status != nil && *update.Status != "" { // Dereference pointer for check
		if !currentStatus.IsValidTransition(*update.Status) { // Dereference pointer for method call
			log.Warnw("Invalid status transition attempted", "tripId", id, "currentStatus", currentStatus, "requestedStatus", *update.Status)
			return nil, apperrors.InvalidStatusTransition(string(currentStatus), string(*update.Status)) // Dereference pointer for string conversion
		}
	}

	var setFields []string
	var args []interface{}
	argPosition := 1

	// Dynamically build the SET clause for the UPDATE statement
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

	// New destination fields
	if update.DestinationPlaceID != nil {
		setFields = append(setFields, fmt.Sprintf("destination_place_id = $%d", argPosition))
		args = append(args, *update.DestinationPlaceID)
		argPosition++
	}
	if update.DestinationAddress != nil {
		setFields = append(setFields, fmt.Sprintf("destination_address = $%d", argPosition))
		args = append(args, *update.DestinationAddress)
		argPosition++
	}
	if update.DestinationName != nil {
		setFields = append(setFields, fmt.Sprintf("destination_name = $%d", argPosition))
		args = append(args, *update.DestinationName)
		argPosition++
	}
	if update.DestinationLatitude != nil {
		setFields = append(setFields, fmt.Sprintf("destination_latitude = $%d", argPosition))
		args = append(args, *update.DestinationLatitude)
		argPosition++
	}
	if update.DestinationLongitude != nil {
		setFields = append(setFields, fmt.Sprintf("destination_longitude = $%d", argPosition))
		args = append(args, *update.DestinationLongitude)
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
	if update.Status != nil && *update.Status != "" { // Dereference pointer
		setFields = append(setFields, fmt.Sprintf("status = $%d", argPosition))
		args = append(args, string(*update.Status)) // Dereference pointer
		argPosition++
	}

	// Always update the updated_at timestamp
	setFields = append(setFields, "updated_at = CURRENT_TIMESTAMP")

	if len(args) == 0 {
		log.Infow("No update fields provided for trip", "tripId", id)
		return s.GetTrip(ctx, id)
	}

	query := fmt.Sprintf(`
        UPDATE trips
        SET %s
        WHERE id = $%d AND deleted_at IS NULL
        RETURNING id`,
		strings.Join(setFields, ", "),
		argPosition,
	)

	args = append(args, id)

	var updatedID string
	err = s.pool.QueryRow(ctx, query, args...).Scan(&updatedID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Warnw("Trip not found during final update execution or was already deleted", "tripId", id)
			return nil, apperrors.NotFound("trip", id)
		}
		log.Errorw("Failed to execute update trip query", "tripId", id, "query", query, "args", args, "error", err)
		return nil, fmt.Errorf("database error updating trip: %w", err)
	}

	if updatedID != id {
		log.Errorw("Update query returned unexpected ID", "expectedId", id, "returnedId", updatedID)
		return nil, fmt.Errorf("internal error during trip update: ID mismatch")
	}

	log.Infow("Trip updated successfully in database", "tripId", id)
	return s.GetTrip(ctx, id)
}

// SoftDeleteTrip implements internal_store.TripStore.
// Marks a trip as deleted by setting the deleted_at timestamp.
func (s *pgTripStore) SoftDeleteTrip(ctx context.Context, id string) error {
	log := logger.GetLogger()
	query := `
        UPDATE trips
        SET deleted_at = CURRENT_TIMESTAMP
        WHERE id = $1 AND deleted_at IS NULL
        RETURNING id` // Returning ID to confirm the operation affected a row

	var returnedID string
	err := s.pool.QueryRow(ctx, query, id).Scan(&returnedID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Warnw("Attempted to soft-delete non-existent or already deleted trip", "tripId", id)
			// Consider if NotFound or no error is more appropriate.
			// If it's already deleted, is it an error? Or idempotent success?
			// Returning NotFound if no row was updated (either not found or already deleted).
			return apperrors.NotFound("trip", id)
		}
		log.Errorw("Failed to soft-delete trip in database", "tripId", id, "error", err)
		return fmt.Errorf("database error during soft-delete: %w", err)
	}

	if returnedID != id {
		log.Errorw("Soft-delete returned unexpected ID", "expectedId", id, "returnedId", returnedID)
		return fmt.Errorf("internal error during soft-delete: ID mismatch")
	}

	log.Infow("Successfully soft-deleted trip", "tripId", id)
	return nil
}

// ListUserTrips implements internal_store.TripStore.
// Retrieves all non-soft-deleted trips created by a specific user,
// ordered by start date descending.
func (s *pgTripStore) ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error) {
	log := logger.GetLogger()
	query := `
        SELECT t.id, t.name, t.description, t.start_date, t.end_date,
               t.destination_place_id, t.destination_address, t.destination_name,
               t.destination_latitude, t.destination_longitude,
               t.status, t.created_by, t.created_at, t.updated_at, t.deleted_at, t.background_image_url
        FROM trips t
        WHERE t.created_by = $1 AND t.deleted_at IS NULL
        ORDER BY t.start_date DESC`

	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Infow("No trips found for user", "userID", userID)
			return []*types.Trip{}, nil
		}
		log.Errorw("Failed to query user trips from database", "userID", userID, "error", err)
		return nil, fmt.Errorf("database error listing user trips: %w", err)
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
			&trip.DestinationPlaceID,
			&trip.DestinationAddress,
			&trip.DestinationName,
			&trip.DestinationLatitude,
			&trip.DestinationLongitude,
			&trip.Status,
			&trip.CreatedBy,
			&trip.CreatedAt,
			&trip.UpdatedAt,
			&trip.DeletedAt,
			&trip.BackgroundImageURL,
		)
		if err != nil {
			log.Errorw("Failed to scan user trip row during list", "userID", userID, "error", err)
			return trips, fmt.Errorf("database error scanning trip row: %w", err)
		}
		trips = append(trips, &trip)
	}

	if err := rows.Err(); err != nil {
		log.Errorw("Error iterating user trip rows", "userID", userID, "error", err)
		return trips, fmt.Errorf("database error iterating trip rows: %w", err)
	}

	log.Infow("Successfully listed trips for user", "userID", userID, "count", len(trips))
	return trips, nil
}

// SearchTrips implements internal_store.TripStore.
// Retrieves non-soft-deleted trips matching the provided criteria (Destination, StartDate, EndDate).
func (s *pgTripStore) SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error) {
	log := logger.GetLogger()
	var conditions []string
	var args []interface{}
	argCount := 1

	baseQuery := `
        SELECT t.id, t.name, t.description, t.start_date, t.end_date,
               t.destination_place_id, t.destination_address, t.destination_name,
               t.destination_latitude, t.destination_longitude,
               t.status, t.created_by, t.created_at, t.updated_at, t.deleted_at, t.background_image_url
        FROM trips t
        WHERE t.deleted_at IS NULL`

	if criteria.Destination != "" {
		// Search against destination_name or destination_address. Let's use destination_name for now.
		conditions = append(conditions, fmt.Sprintf("(t.destination_name ILIKE $%d OR t.destination_address ILIKE $%d)", argCount, argCount))
		args = append(args, "%"+criteria.Destination+"%")
		// argCount will be incremented after adding the argument.
		// No, the same argCount should be used for both parts of OR if using the same criteria value
		argCount++
	}

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

	if !criteria.StartDateFrom.IsZero() {
		conditions = append(conditions, fmt.Sprintf("t.start_date >= $%d", argCount))
		args = append(args, criteria.StartDateFrom)
		argCount++
	}
	if !criteria.StartDateTo.IsZero() {
		conditions = append(conditions, fmt.Sprintf("t.start_date <= $%d", argCount))
		args = append(args, criteria.StartDateTo)
		// argCount++ // No increment if it's the last condition using args
	}

	finalQuery := baseQuery
	if len(conditions) > 0 {
		finalQuery += " AND " + strings.Join(conditions, " AND ")
	}
	finalQuery += " ORDER BY t.start_date DESC"

	log.Debugw("Executing SearchTrips query", "query", finalQuery, "args", args)

	rows, err := s.pool.Query(ctx, finalQuery, args...)
	if err != nil {
		log.Errorw("Failed to execute search trips query", "criteria", criteria, "error", err)
		return nil, fmt.Errorf("database error searching trips: %w", err)
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
			&trip.DestinationPlaceID,
			&trip.DestinationAddress,
			&trip.DestinationName,
			&trip.DestinationLatitude,
			&trip.DestinationLongitude,
			&trip.Status,
			&trip.CreatedBy,
			&trip.CreatedAt,
			&trip.UpdatedAt,
			&trip.DeletedAt,
			&trip.BackgroundImageURL,
		)
		if err != nil {
			log.Errorw("Failed to scan search trip row", "criteria", criteria, "error", err)
			return trips, fmt.Errorf("database error scanning searched trip: %w", err)
		}
		trips = append(trips, &trip)
	}

	if err := rows.Err(); err != nil {
		log.Errorw("Error iterating search trip rows", "criteria", criteria, "error", err)
		return trips, fmt.Errorf("database error iterating searched trips: %w", err)
	}

	log.Infow("Successfully searched trips", "criteria", criteria, "count", len(trips))
	return trips, nil
}

// AddMember implements internal_store.TripStore.
// Adds a new membership record or updates an existing inactive one.
func (s *pgTripStore) AddMember(ctx context.Context, membership *types.TripMembership) error {
	log := logger.GetLogger()
	query := `
        INSERT INTO trip_memberships (trip_id, user_id, role, status)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (trip_id, user_id)
        DO UPDATE SET
            role = EXCLUDED.role,
            status = EXCLUDED.status,
            updated_at = CURRENT_TIMESTAMP
        WHERE trip_memberships.status != $4 -- Only update if not already active (using the passed status)
        `
	// Use Exec, as we don't need to return rows
	_, err := s.pool.Exec(ctx, query,
		membership.TripID,
		membership.UserID,
		membership.Role,
		membership.Status, // Use the status from the input struct for comparison
	)

	if err != nil {
		log.Errorw("Failed to add or update trip member in database", "tripID", membership.TripID, "userID", membership.UserID, "role", membership.Role, "status", membership.Status, "error", err)
		// TODO: Check for specific DB errors (e.g., foreign key violations) if needed
		// return apperrors.NewDatabaseError(fmt.Errorf("failed to add/update member: %w", err))
		return fmt.Errorf("database error adding/updating member: %w", err)
	}

	log.Infow("Successfully added or updated trip member", "tripID", membership.TripID, "userID", membership.UserID, "role", membership.Role, "status", membership.Status)
	return nil
}

// UpdateMemberRole implements internal_store.TripStore.
// Updates the role of an existing active member in a trip.
func (s *pgTripStore) UpdateMemberRole(ctx context.Context, tripID string, userID string, role types.MemberRole) error {
	log := logger.GetLogger()
	query := `
        UPDATE trip_memberships
        SET role = $1, updated_at = CURRENT_TIMESTAMP
        WHERE trip_id = $2 AND user_id = $3 AND status = $4
        RETURNING trip_id -- Return something to check affected rows
    `
	var returnedID string
	err := s.pool.QueryRow(ctx, query, role, tripID, userID, types.MembershipStatusActive).Scan(&returnedID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Warnw("Failed to update role: active member not found", "tripID", tripID, "userID", userID, "role", role)
			return apperrors.NotFound("Active Trip Member", fmt.Sprintf("user %s in trip %s", userID, tripID))
		}
		log.Errorw("Failed to update member role in database", "tripID", tripID, "userID", userID, "role", role, "error", err)
		// return apperrors.NewDatabaseError(fmt.Errorf("failed updating role: %w", err))
		return fmt.Errorf("database error updating member role: %w", err)
	}

	// Sanity check
	if returnedID != tripID {
		log.Errorw("UpdateMemberRole returned unexpected trip ID", "expected", tripID, "returned", returnedID)
		return fmt.Errorf("internal error during role update: ID mismatch")
	}

	log.Infow("Successfully updated member role", "tripID", tripID, "userID", userID, "newRole", role)
	return nil
}

// RemoveMember implements internal_store.TripStore.
// Marks a trip membership as inactive (logical delete).
func (s *pgTripStore) RemoveMember(ctx context.Context, tripID string, userID string) error {
	log := logger.GetLogger()
	query := `
        UPDATE trip_memberships
        SET status = $1, updated_at = CURRENT_TIMESTAMP
        WHERE trip_id = $2 AND user_id = $3 AND status = $4 -- Only deactivate active members
        RETURNING trip_id -- Return something to check affected rows
    `
	var returnedID string
	err := s.pool.QueryRow(ctx, query, types.MembershipStatusInactive, tripID, userID, types.MembershipStatusActive).Scan(&returnedID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No active member found to remove.
			log.Warnw("Attempted to remove non-existent or inactive member", "tripID", tripID, "userID", userID)
			// Considered success for idempotency in the original code.
			return nil
		}
		log.Errorw("Failed to remove (deactivate) trip member in database", "tripID", tripID, "userID", userID, "error", err)
		// return apperrors.NewDatabaseError(fmt.Errorf("failed removing member: %w", err))
		return fmt.Errorf("database error removing member: %w", err)
	}

	// Sanity check
	if returnedID != tripID {
		log.Errorw("RemoveMember returned unexpected trip ID", "expected", tripID, "returned", returnedID)
		return fmt.Errorf("internal error during member removal: ID mismatch")
	}

	log.Infow("Successfully removed (deactivated) trip member", "tripID", tripID, "userID", userID)
	return nil
}

// GetTripMembers implements internal_store.TripStore.
// Retrieves all active memberships for a specific trip.
func (s *pgTripStore) GetTripMembers(ctx context.Context, tripID string) ([]types.TripMembership, error) {
	log := logger.GetLogger()
	query := `
        SELECT id, trip_id, user_id, role, status, created_at, updated_at
        FROM trip_memberships
        WHERE trip_id = $1 AND status = $2
        ORDER BY created_at ASC
    `
	rows, err := s.pool.Query(ctx, query, tripID, types.MembershipStatusActive)
	if err != nil {
		log.Errorw("Failed to query active trip members from database", "tripID", tripID, "error", err)
		// return nil, apperrors.NewDatabaseError(fmt.Errorf("failed getting members for trip %s: %w", tripID, err))
		return nil, fmt.Errorf("database error getting trip members: %w", err)
	}
	defer rows.Close()

	members := make([]types.TripMembership, 0)
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
			log.Errorw("Failed to scan trip member row", "tripID", tripID, "error", err)
			// return members, apperrors.NewDatabaseError(fmt.Errorf("failed scanning member for trip %s: %w", tripID, err))
			return members, fmt.Errorf("database error scanning member row: %w", err)
		}
		members = append(members, member)
	}

	if err := rows.Err(); err != nil {
		log.Errorw("Error iterating trip member rows", "tripID", tripID, "error", err)
		// return members, apperrors.NewDatabaseError(fmt.Errorf("error iterating members for trip %s: %w", tripID, err))
		return members, fmt.Errorf("database error iterating members: %w", err)
	}

	log.Infow("Successfully retrieved trip members", "tripID", tripID, "count", len(members))
	return members, nil
}

// GetUserRole implements internal_store.TripStore.
// Retrieves the role of a specific active user within a specific trip.
func (s *pgTripStore) GetUserRole(ctx context.Context, tripID string, userID string) (types.MemberRole, error) {
	log := logger.GetLogger()
	var role string
	query := `SELECT role FROM trip_memberships WHERE trip_id = $1 AND user_id = $2 AND status = $3`
	err := s.pool.QueryRow(ctx, query, tripID, userID, types.MembershipStatusActive).Scan(&role)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// User is not an active member of this trip.
			log.Warnw("Active membership not found for user role lookup", "tripID", tripID, "userID", userID)
			return "", apperrors.NotFound("active membership", fmt.Sprintf("user %s in trip %s", userID, tripID))
		}
		log.Errorw("Failed to get user role from database", "tripID", tripID, "userID", userID, "error", err)
		// return "", apperrors.NewDatabaseError(fmt.Errorf("failed getting role for user %s in trip %s: %w", userID, tripID, err))
		return "", fmt.Errorf("database error getting user role: %w", err)
	}

	// Validate the retrieved role.
	memberRole := types.MemberRole(role)
	if !memberRole.IsValid() {
		log.Errorw("Invalid role found in database for active member", "role", role, "tripID", tripID, "userID", userID)
		// Return an internal server error as the data state is unexpected.
		// return "", apperrors.InternalServerError(fmt.Sprintf("Invalid role '%s' found for user %s in trip %s", role, userID, tripID))
		return "", fmt.Errorf("invalid role '%s' found in database for user %s, trip %s", role, userID, tripID)
	}

	log.Debugw("Successfully retrieved user role", "tripID", tripID, "userID", userID, "role", memberRole)
	return memberRole, nil
}

// LookupUserByEmail implements internal_store.TripStore.
// Finds a user in the local users table by their email address.
// TODO: Move this method to UserStore once it's implemented following the new pattern.
func (s *pgTripStore) LookupUserByEmail(ctx context.Context, email string) (*types.SupabaseUser, error) {
	log := logger.GetLogger()
	// Note: Assumes a local 'users' table mirroring Supabase/Auth users.
	query := `SELECT id, email, user_metadata FROM users WHERE email = $1 LIMIT 1`

	var user types.SupabaseUser // Using SupabaseUser type for now
	var userMetadataJSON []byte

	err := s.pool.QueryRow(ctx, query, email).Scan(&user.ID, &user.Email, &userMetadataJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Infow("User not found by email in local table", "email", email)
			return nil, apperrors.NotFound("User with email", email)
		}
		log.Errorw("Failed to lookup user by email in database", "email", email, "error", err)
		// return nil, apperrors.NewDatabaseError(fmt.Errorf("failed looking up email %s: %w", email, err))
		return nil, fmt.Errorf("database error looking up email %s: %w", email, err)
	}

	// Unmarshal user_metadata if it exists
	if len(userMetadataJSON) > 0 {
		if err := json.Unmarshal(userMetadataJSON, &user.UserMetadata); err != nil {
			log.Errorw("Failed to unmarshal user metadata during email lookup", "userId", user.ID, "email", email, "error", err)
			// Return user data found so far, but also the error
			return &user, fmt.Errorf("failed unmarshalling metadata for user %s: %w", email, err)
		}
	}

	log.Infow("Successfully looked up user by email", "email", email, "userId", user.ID)
	return &user, nil
}

// CreateInvitation implements internal_store.TripStore.
// Inserts a new trip invitation record into the database.
func (s *pgTripStore) CreateInvitation(ctx context.Context, invitation *types.TripInvitation) error {
	log := logger.GetLogger()
	query := `
        INSERT INTO trip_invitations
            (id, trip_id, inviter_id, invitee_email, role, token, status, expires_at, created_at, updated_at)
        VALUES
            ($1, $2, $3, $4, $5, $6, $7, $8, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
        RETURNING id -- Return ID to confirm insertion
    `
	var returnedID string
	err := s.pool.QueryRow(ctx, query,
		invitation.ID,
		invitation.TripID,
		invitation.InviterID,
		invitation.InviteeEmail,
		invitation.Role,
		invitation.Token,
		invitation.Status,
		invitation.ExpiresAt,
	).Scan(&returnedID)

	if err != nil {
		// TODO: Check for specific errors like unique constraint violations if needed
		log.Errorw("Failed to create trip invitation in database", "tripID", invitation.TripID, "inviteeEmail", invitation.InviteeEmail, "error", err)
		// return apperrors.NewDatabaseError(fmt.Errorf("failed creating invitation: %w", err))
		return fmt.Errorf("database error creating invitation: %w", err)
	}

	if returnedID != invitation.ID {
		log.Errorw("CreateInvitation returned mismatching ID", "expected", invitation.ID, "got", returnedID)
		// return apperrors.InternalServerError("database returned unexpected ID during invitation creation")
		return fmt.Errorf("internal error during invitation creation: ID mismatch")
	}

	log.Infow("Trip invitation created successfully", "invitationID", invitation.ID)
	return nil
}

// GetInvitation implements internal_store.TripStore.
// Retrieves a specific trip invitation by its ID.
func (s *pgTripStore) GetInvitation(ctx context.Context, invitationID string) (*types.TripInvitation, error) {
	log := logger.GetLogger()
	query := `
        SELECT id, trip_id, inviter_id, invitee_email, role, token, status, expires_at, created_at, updated_at
        FROM trip_invitations
        WHERE id = $1
    `
	var inv types.TripInvitation
	err := s.pool.QueryRow(ctx, query, invitationID).Scan(
		&inv.ID,
		&inv.TripID,
		&inv.InviterID,
		&inv.InviteeEmail,
		&inv.Role,
		&inv.Token,
		&inv.Status,
		&inv.ExpiresAt,
		&inv.CreatedAt,
		&inv.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Warnw("Trip invitation not found", "invitationID", invitationID)
			return nil, apperrors.NotFound("Invitation", invitationID)
		}
		log.Errorw("Failed to get trip invitation from database", "invitationID", invitationID, "error", err)
		// return nil, apperrors.NewDatabaseError(fmt.Errorf("failed getting invitation %s: %w", invitationID, err))
		return nil, fmt.Errorf("database error getting invitation %s: %w", invitationID, err)
	}

	log.Debugw("Successfully retrieved trip invitation", "invitationID", invitationID)
	return &inv, nil
}

// GetInvitationsByTripID implements internal_store.TripStore.
// Retrieves all invitations associated with a specific trip ID.
func (s *pgTripStore) GetInvitationsByTripID(ctx context.Context, tripID string) ([]*types.TripInvitation, error) {
	log := logger.GetLogger()
	query := `
        SELECT id, trip_id, inviter_id, invitee_email, role, token, status, expires_at, created_at, updated_at
        FROM trip_invitations
        WHERE trip_id = $1
        ORDER BY created_at DESC
    `
	rows, err := s.pool.Query(ctx, query, tripID)
	if err != nil {
		log.Errorw("Failed to query invitations by trip ID from database", "tripID", tripID, "error", err)
		// return nil, apperrors.NewDatabaseError(fmt.Errorf("failed getting invitations for trip %s: %w", tripID, err))
		return nil, fmt.Errorf("database error getting invitations for trip %s: %w", tripID, err)
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
			&inv.Token,
			&inv.Status,
			&inv.ExpiresAt,
			&inv.CreatedAt,
			&inv.UpdatedAt,
		)
		if err != nil {
			log.Errorw("Failed to scan trip invitation row", "tripID", tripID, "error", err)
			// return invitations, apperrors.NewDatabaseError(fmt.Errorf("failed scanning invitation for trip %s: %w", tripID, err))
			return invitations, fmt.Errorf("database error scanning invitation row: %w", err)
		}
		invitations = append(invitations, &inv)
	}

	if err := rows.Err(); err != nil {
		log.Errorw("Error iterating trip invitation rows", "tripID", tripID, "error", err)
		// return invitations, apperrors.NewDatabaseError(fmt.Errorf("error iterating invitations for trip %s: %w", tripID, err))
		return invitations, fmt.Errorf("database error iterating invitations: %w", err)
	}

	log.Infow("Successfully retrieved invitations for trip", "tripID", tripID, "count", len(invitations))
	return invitations, nil
}

// UpdateInvitationStatus implements internal_store.TripStore.
// Updates the status of an existing trip invitation.
func (s *pgTripStore) UpdateInvitationStatus(ctx context.Context, invitationID string, status types.InvitationStatus) error {
	log := logger.GetLogger()
	query := `
        UPDATE trip_invitations
        SET status = $1, updated_at = CURRENT_TIMESTAMP
        WHERE id = $2
        RETURNING id -- Return ID to confirm update occurred
    `
	var returnedID string
	err := s.pool.QueryRow(ctx, query, status, invitationID).Scan(&returnedID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Warnw("Failed to update invitation status: invitation not found", "invitationID", invitationID, "status", status)
			return apperrors.NotFound("Invitation", invitationID)
		}
		log.Errorw("Failed to update invitation status in database", "invitationID", invitationID, "status", status, "error", err)
		// return apperrors.NewDatabaseError(fmt.Errorf("failed updating status for invitation %s: %w", invitationID, err))
		return fmt.Errorf("database error updating invitation status: %w", err)
	}

	if returnedID != invitationID {
		log.Errorw("UpdateInvitationStatus returned mismatching ID", "expected", invitationID, "got", returnedID)
		// return apperrors.InternalServerError("database returned unexpected ID during invitation status update")
		return fmt.Errorf("internal error updating invitation status: ID mismatch")
	}

	log.Infow("Invitation status updated successfully", "invitationID", invitationID, "newStatus", status)
	return nil
}

// --- Transaction Methods ---

// txWrapper wraps a pgx.Tx to satisfy the store.Transaction interface.
type txWrapper struct {
	tx pgx.Tx
}

// Commit commits the underlying transaction.
func (t *txWrapper) Commit() error {
	// Using background context consistent with original db/trip.go txWrapper
	return t.tx.Commit(context.Background())
}

// Rollback rolls back the underlying transaction.
func (t *txWrapper) Rollback() error {
	// Using background context consistent with original db/trip.go txWrapper
	return t.tx.Rollback(context.Background())
}

// BeginTx implements internal_store.TripStore.
// Starts a new database transaction.
// TODO: Consider if this manual transaction management is needed or if WithTx is sufficient.
func (s *pgTripStore) BeginTx(ctx context.Context) (internal_store.Transaction, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	// Return the wrapper that implements the Transaction interface
	return &txWrapper{tx: tx}, nil
}


// --- Transaction Helper ---
// TODO: Move this to a shared utility package (e.g., dbutils) if used by other stores.

// TxFn is a function signature for operations to be executed within a transaction.
type TxFn func(tx pgx.Tx) error

// WithTx executes a function within a database transaction.
// It handles begin, commit, and rollback automatically.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn TxFn) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		// Rollback is safe to call even if the transaction was committed.
		// It will just be a no-op in that case.
		if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
			// Log the rollback error, but don't overwrite the original error from fn
			logger.GetLogger().Errorw("Failed to rollback transaction", "error", err)
		}
	}()

	if err := fn(tx); err != nil {
		// If the function returned an error, rollback happens automatically via defer.
		// Return the original error from fn.
		return err
	}

	// If fn was successful, commit the transaction.
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
