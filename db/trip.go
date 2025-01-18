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
               t.destination, t.created_by, t.created_at, t.updated_at
        FROM trips t
        LEFT JOIN metadata m ON m.table_name = 'trips' AND m.record_id = t.id
        WHERE t.id = $1 AND m.deleted_at IS NULL`

	var trip types.Trip
	err := tdb.client.GetPool().QueryRow(ctx, query, id).Scan(
		&trip.ID,
		&trip.Name,
		&trip.Description,
		&trip.StartDate,
		&trip.EndDate,
		&trip.Destination,
		&trip.CreatedBy,
		&trip.CreatedAt,
		&trip.UpdatedAt,
	)

	if err != nil {
		log.Errorw("Failed to get trip", "tripId", id, "error", err)
		return nil, err
	}

	return &trip, nil
}

func (tdb *TripDB) UpdateTrip(ctx context.Context, id string, update types.TripUpdate) error {
    log := logger.GetLogger()
    
    // Build the query dynamically based on what fields are being updated
    var setFields []string
    var args []interface{}
    argPosition := 1

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
		log.Debugw("Adding status update to query", "status", update.Status)
    }

    // Always update updated_at timestamp
    setFields = append(setFields, "updated_at = CURRENT_TIMESTAMP")

    // If no fields to update, return early
    if len(setFields) == 0 {
        return nil
    }

    query := fmt.Sprintf(`
        UPDATE trips 
        SET %s, updated_at = CURRENT_TIMESTAMP
        WHERE id = $%d
        RETURNING status`,
        strings.Join(setFields, ", "),
        argPosition,
	)

    args = append(args, id)

    var updatedStatus string
    err := tdb.client.GetPool().QueryRow(ctx, query, args...).Scan(&updatedStatus)
    if err != nil {
        log.Errorw("Failed to update trip", "tripId", id, "error", err)
        return err
    }

    log.Debugw("Trip status updated", "tripId", id, "status", updatedStatus)

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

    query := `
        SELECT t.id, t.name, t.description, t.start_date, t.end_date,
               t.destination, t.created_by, t.created_at, t.updated_at, trip.status
        FROM trips t
        LEFT JOIN metadata m ON m.table_name = 'trips' AND m.record_id = t.id
        WHERE m.deleted_at IS NULL`

    params := make([]interface{}, 0)
    paramCount := 1

    if criteria.Destination != "" {
        query += fmt.Sprintf(" AND t.destination ILIKE $%d", paramCount)
        params = append(params, "%"+criteria.Destination+"%")
        paramCount++  // nolint:ineffassign
    }

    if !criteria.StartDateFrom.IsZero() {
        query += fmt.Sprintf(" AND t.start_date >= $%d::timestamp", paramCount)
        params = append(params, criteria.StartDateFrom)
        paramCount++ // nolint:ineffassign
    }

    if !criteria.StartDateTo.IsZero() {
        query += fmt.Sprintf(" AND t.start_date <= $%d::timestamp", paramCount)
        params = append(params, criteria.StartDateTo)
        paramCount++ // nolint:ineffassign
    }

    query += ` ORDER BY t.start_date DESC`

    rows, err := tdb.client.GetPool().Query(ctx, query, params...)
    if err != nil {
        log.Errorw("Failed to search trips", "error", err)
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
			&trip.CreatedBy,
			&trip.CreatedAt,
			&trip.UpdatedAt,
			&trip.Status,
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
