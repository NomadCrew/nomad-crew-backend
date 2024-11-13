package db

import (
	"context"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/NomadCrew/nomad-crew-backend/user-service/models"
	"github.com/NomadCrew/nomad-crew-backend/user-service/logger"
)

// TripDB encapsulates trip database operations
type TripDB struct {
	pool *pgxpool.Pool
}

// NewTripDB creates a new instance of TripDB
func NewTripDB(pool *pgxpool.Pool) *TripDB {
	return &TripDB{pool: pool}
}

// CreateTrip inserts a new trip into the database
func (tdb *TripDB) CreateTrip(ctx context.Context, trip models.Trip) (int64, error) {
	log := logger.GetLogger()
	query := `INSERT INTO trips (name, description, start_date, end_date, destination, created_by) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`

	var tripID int64
	err := tdb.pool.QueryRow(ctx, query,
		trip.Name,
		trip.Description,
		trip.StartDate,
		trip.EndDate,
		trip.Destination,
		trip.CreatedBy,
	).Scan(&tripID)

	if err != nil {
		log.Errorf("Failed to insert trip: %v", err)
		return 0, err
	}
	return tripID, nil
}

// GetTrip retrieves a trip by ID
func (tdb *TripDB) GetTrip(ctx context.Context, id int64) (*models.Trip, error) {
	return models.GetTripByID(ctx, tdb.pool, id)
}

func (tdb *TripDB) UpdateTrip(ctx context.Context, tripID int64, update models.TripUpdate) error {
    log := logger.GetLogger()
    query := `UPDATE trips 
               SET name = COALESCE($1, name), 
                   description = COALESCE($2, description), 
                   destination = COALESCE($3, destination), 
                   start_date = COALESCE($4, start_date), 
                   end_date = COALESCE($5, end_date) 
               WHERE id = $6`

    _, err := tdb.pool.Exec(ctx, query, 
        update.Name, 
        update.Description, 
        update.Destination, 
        update.StartDate, 
        update.EndDate, 
        tripID,
    )

    if err!= nil {
        log.Errorf("Failed to update trip: %v", err)
        return err
    }
    return nil
}

// DeleteTrip deletes a trip from the database
func (tdb *TripDB) DeleteTrip(ctx context.Context, tripID int64) error {
    log := logger.GetLogger()
    query := `UPDATE metadata 
               SET deleted_at = CURRENT_TIMESTAMP 
               WHERE table_name = 'trips' AND record_id = $1`

    _, err := tdb.pool.Exec(ctx, query, tripID)
    if err!= nil {
        log.Errorf("Failed to delete trip: %v", err)
        return err
    }
    return nil
}