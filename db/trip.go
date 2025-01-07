package db

import (
	"context"
	"strconv"
	"strings"

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
	query := `
        INSERT INTO trips (
            name, description, start_date, end_date, 
            destination, created_by
        ) 
        VALUES ($1, $2, $3, $4, $5, $6) 
        RETURNING id`

	var tripID string
	err := tdb.client.GetPool().QueryRow(ctx, query,
		trip.Name,
		trip.Description,
		trip.StartDate,
		trip.EndDate,
		trip.Destination,
		trip.CreatedBy,
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
	query := `
        UPDATE trips 
        SET name = COALESCE($1, name),
            description = COALESCE($2, description),
            destination = COALESCE($3, destination),
            start_date = COALESCE($4, start_date),
            end_date = COALESCE($5, end_date),
            updated_at = CURRENT_TIMESTAMP
        WHERE id = $6`

	result, err := tdb.client.GetPool().Exec(ctx, query,
		update.Name,
		update.Description,
		update.Destination,
		update.StartDate,
		update.EndDate,
		id,
	)

	if err != nil {
		log.Errorw("Failed to update trip", "tripId", id, "error", err)
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.NotFound("Trip", id)
	}

	// Update metadata
	metadataQuery := `
        UPDATE metadata
        SET updated_at = CURRENT_TIMESTAMP
        WHERE table_name = 'trips' AND record_id = $1`

	_, err = tdb.client.GetPool().Exec(ctx, metadataQuery, id)
	if err != nil {
		log.Errorw("Failed to update trip metadata", "error", err)
		return err
	}

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
               t.destination, t.created_by, t.created_at, t.updated_at
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

	// Build base query with params
	query := `
        SELECT t.id, t.name, t.description, t.start_date, t.end_date,
               t.destination, t.created_by, t.created_at, t.updated_at
        FROM trips t
        LEFT JOIN metadata m ON m.table_name = 'trips' AND m.record_id = t.id
        WHERE m.deleted_at IS NULL`

	params := make([]interface{}, 0)
	paramCount := 1
	var conditions []string

	if criteria.Destination != "" {
		conditions = append(conditions, `t.destination ILIKE $`+strconv.Itoa(paramCount))
		params = append(params, "%"+criteria.Destination+"%")
	}

	if !criteria.StartDateFrom.IsZero() {
		conditions = append(conditions, `t.start_date >= $`+strconv.Itoa(paramCount))
		params = append(params, criteria.StartDateFrom)
		paramCount++ //nolint:ineffassign
	}

	if !criteria.StartDateTo.IsZero() {
		conditions = append(conditions, `t.start_date <= $`+strconv.Itoa(paramCount))
		params = append(params, criteria.StartDateTo)
		paramCount++ //nolint:ineffassign
	}

	if len(conditions) > 0 {
		query += ` AND ` + strings.Join(conditions, " AND ")
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
