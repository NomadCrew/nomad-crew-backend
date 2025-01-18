package db

import (
	"context"
	"strings"
	"fmt"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v4/pgxpool"
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
	if trip.Status == "" {
        trip.Status = types.TripStatusPlanning
    }
	query := `
        INSERT INTO trips (
            name, description, start_date, end_date, 
            destination, created_by, status
        ) 
        VALUES ($1, $2, $3, $4, $5, $6, $7) 
        RETURNING id`

	var tripID string
	err := tdb.client.GetPool().QueryRow(ctx, query,
		trip.Name,
		trip.Description,
		trip.StartDate,
		trip.EndDate,
		trip.Destination,
		trip.CreatedBy,
		string(trip.Status),
	).Scan(&tripID)

	if err != nil {
		log.Errorw("Failed to create trip", "error", err)
		return "", err
	}

	// Insert metadata
	metadataQuery := `
        INSERT INTO metadata (table_name, record_id, created_at, updated_at)
        VALUES ($1, $2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`

	_, err = tdb.client.GetPool().Exec(ctx, metadataQuery, "trips", tripID)
	if err != nil {
		log.Errorw("Failed to create trip metadata", "error", err)
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

    // Update fields
    if update.Name != "" {
        setFields = append(setFields, fmt.Sprintf("name = $%d", argPosition))
        args = append(args, update.Name)
        argPosition++
    }
    if update.Description != "" {
        setFields = append(setFields, fmt.Sprintf("description = $%d", argPosition))
        args = append(args, update.Description)
        argPosition++
    }
    if update.Destination != "" {
        setFields = append(setFields, fmt.Sprintf("destination = $%d", argPosition))
        args = append(args, update.Destination)
        argPosition++
    }
    if !update.StartDate.IsZero() {
        setFields = append(setFields, fmt.Sprintf("start_date = $%d", argPosition))
        args = append(args, update.StartDate)
        argPosition++
    }
    if !update.EndDate.IsZero() {
        setFields = append(setFields, fmt.Sprintf("end_date = $%d", argPosition))
        args = append(args, update.EndDate)
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
        UPDATE metadata 
        SET deleted_at = CURRENT_TIMESTAMP 
        WHERE table_name = 'trips' AND record_id = $1`

	result, err := tdb.client.GetPool().Exec(ctx, query, id)
	if err != nil {
		log.Errorw("Failed to delete trip", "tripId", id, "error", err)
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.NotFound("Trip", id)
	}

	return nil
}

func (tdb *TripDB) ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error) {
	log := logger.GetLogger()
	query := `
    SELECT t.id, t.name, t.description, t.start_date, t.end_date,
           t.destination, t.status, t.created_by, t.created_at, t.updated_at
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
               t.destination, t.status, t.created_by, t.created_at, t.updated_at
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
