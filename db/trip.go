package db

import (
    "context"
    "github.com/jackc/pgx/v4/pgxpool"
    "github.com/NomadCrew/nomad-crew-backend/types"
    "github.com/NomadCrew/nomad-crew-backend/logger"
    "github.com/NomadCrew/nomad-crew-backend/models"
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

func (tdb *TripDB) CreateTrip(ctx context.Context, trip types.Trip) (int64, error) {
    log := logger.GetLogger()
    query := `INSERT INTO trips (name, description, start_date, end_date, destination, created_by) 
              VALUES ($1, $2, $3, $4, $5, $6) 
              RETURNING id`

    var tripID int64
    err := tdb.client.GetPool().QueryRow(ctx, query,
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

func (tdb *TripDB) GetTrip(ctx context.Context, id int64) (*types.Trip, error) {
    return models.GetTripByID(ctx, tdb, id)
}

func (tdb *TripDB) UpdateTrip(ctx context.Context, tripID int64, update types.TripUpdate) error {
    log := logger.GetLogger()
    query := `UPDATE trips 
               SET name = COALESCE($1, name), 
                   description = COALESCE($2, description), 
                   destination = COALESCE($3, destination), 
                   start_date = COALESCE($4, start_date), 
                   end_date = COALESCE($5, end_date) 
               WHERE id = $6`

    _, err := tdb.client.GetPool().Exec(ctx, query, 
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

func (tdb *TripDB) SoftDeleteTrip(ctx context.Context, tripID int64) error {
    log := logger.GetLogger()
    query := `UPDATE metadata 
               SET deleted_at = CURRENT_TIMESTAMP 
               WHERE table_name = 'trips' AND record_id = $1`

    _, err := tdb.client.GetPool().Exec(ctx, query, tripID)
    if err!= nil {
        log.Errorf("Failed to delete trip: %v", err)
        return err
    }
    return nil
}