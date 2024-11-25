// user-service/models/trip.go
package models

import (
    "context"
    "database/sql"
    "fmt"
    "strings"

    "github.com/NomadCrew/nomad-crew-backend/user-service/errors"
    "github.com/NomadCrew/nomad-crew-backend/user-service/types"
    "github.com/NomadCrew/nomad-crew-backend/user-service/internal/store"
    "github.com/NomadCrew/nomad-crew-backend/user-service/logger"
)

type TripModel struct {
    store store.TripStore
}

func NewTripModel(store store.TripStore) *TripModel {
    return &TripModel{store: store}
}

func (tm *TripModel) GetStore() store.TripStore {
    return tm.store
}

func (tm *TripModel) GetTripByID(ctx context.Context, id int64) (*types.Trip, error) {
    return GetTripByID(ctx, tm.store, id)
}

// CreateTrip creates a new trip
func (tm *TripModel) CreateTrip(ctx context.Context, trip *types.Trip) error {
    log := logger.GetLogger()

    if err := validateTrip(trip); err != nil {
        return err
    }

    id, err := tm.store.CreateTrip(ctx, *trip)
    if err != nil {
        log.Errorw("Failed to create trip", "error", err)
        return errors.NewDatabaseError(err)
    }

    trip.ID = id
    return nil
}

// GetTripByID retrieves a trip by ID
func GetTripByID(ctx context.Context, store store.TripStore, id int64) (*types.Trip, error) {
    const query = `
        SELECT t.id, t.name, t.description, t.start_date, t.end_date, 
               t.destination, t.created_by, t.created_at, t.updated_at
        FROM trips t
        LEFT JOIN metadata m ON m.table_name = 'trips' AND m.record_id = t.id
        WHERE t.id = $1 AND m.deleted_at IS NULL`

    var trip types.Trip
    err := store.GetPool().QueryRow(ctx, query, id).Scan(
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
        if err == sql.ErrNoRows {
            return nil, errors.NotFound("Trip", id)
        }
        return nil, errors.NewDatabaseError(err)
    }

    return &trip, nil
}

// UpdateTrip updates an existing trip
func (tm *TripModel) UpdateTrip(ctx context.Context, id int64, update *types.TripUpdate) error {
    log := logger.GetLogger()

    if err := validateTripUpdate(update); err != nil {
        return err
    }

    err := tm.store.UpdateTrip(ctx, id, *update)
    if err != nil {
        log.Errorw("Failed to update trip", "tripId", id, "error", err)
        return errors.NewDatabaseError(err)
    }

    return nil
}

// DeleteTrip performs a soft delete of a trip
func (tm *TripModel) DeleteTrip(ctx context.Context, id int64) error {
    log := logger.GetLogger()

    err := tm.store.SoftDeleteTrip(ctx, id)
    if err != nil {
        log.Errorw("Failed to delete trip", "tripId", id, "error", err)
        return errors.NewDatabaseError(err)
    }

    return nil
}

// ListUserTrips gets all trips for a specific user
func (tm *TripModel) ListUserTrips(ctx context.Context, userID int64) ([]*types.Trip, error) {
    log := logger.GetLogger()

    query := `
        SELECT t.id, t.name, t.description, t.start_date, t.end_date, 
               t.destination, t.created_by, t.created_at, t.updated_at
        FROM trips t
        LEFT JOIN metadata m ON m.table_name = 'trips' AND m.record_id = t.id
        WHERE t.created_by = $1 AND m.deleted_at IS NULL
        ORDER BY t.start_date DESC`

    rows, err := tm.store.GetPool().Query(ctx, query, userID)
    if err != nil {
        log.Errorw("Failed to list user trips", "userId", userID, "error", err)
        return nil, errors.NewDatabaseError(err)
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
            return nil, errors.NewDatabaseError(err)
        }
        trips = append(trips, &trip)
    }

    if err = rows.Err(); err != nil {
        log.Errorw("Error iterating trip rows", "error", err)
        return nil, errors.NewDatabaseError(err)
    }

    return trips, nil
}

// Helper functions for validation
func validateTrip(trip *types.Trip) error {
    if trip.Name == "" {
        return errors.ValidationFailed("Trip name is required", "")
    }
    if trip.Destination == "" {
        return errors.ValidationFailed("Trip destination is required", "")
    }
    if trip.StartDate.IsZero() {
        return errors.ValidationFailed("Trip start date is required", "")
    }
    if trip.EndDate.IsZero() {
        return errors.ValidationFailed("Trip end date is required", "")
    }
    if trip.EndDate.Before(trip.StartDate) {
        return errors.ValidationFailed("Trip end date cannot be before start date", "")
    }
    if trip.CreatedBy == 0 {
        return errors.ValidationFailed("Trip creator ID is required", "")
    }
    return nil
}

func validateTripUpdate(update *types.TripUpdate) error {
    if update.EndDate.Before(update.StartDate) {
        return errors.ValidationFailed("Trip end date cannot be before start date", "")
    }
    // Note: We don't validate other fields as they're optional in updates
    return nil
}

// SearchTrips searches for trips based on criteria
func (tm *TripModel) SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error) {
    log := logger.GetLogger()

    query := `
        SELECT t.id, t.name, t.description, t.start_date, t.end_date, 
               t.destination, t.created_by, t.created_at, t.updated_at
        FROM trips t
        LEFT JOIN metadata m ON m.table_name = 'trips' AND m.record_id = t.id
        WHERE m.deleted_at IS NULL`

    var params []interface{}
    var conditions []string

    // Build dynamic query based on search criteria
    if criteria.Destination != "" {
        params = append(params, "%"+criteria.Destination+"%")
        conditions = append(conditions, fmt.Sprintf("t.destination ILIKE $%d", len(params)))
    }

    if !criteria.StartDateFrom.IsZero() {
        params = append(params, criteria.StartDateFrom)
        conditions = append(conditions, fmt.Sprintf("t.start_date >= $%d", len(params)))
    }

    if !criteria.StartDateTo.IsZero() {
        params = append(params, criteria.StartDateTo)
        conditions = append(conditions, fmt.Sprintf("t.start_date <= $%d", len(params)))
    }

    // Add conditions to query if any exist
    if len(conditions) > 0 {
        query += " AND " + strings.Join(conditions, " AND ")
    }

    query += " ORDER BY t.start_date DESC"

    // Execute query
    rows, err := tm.store.GetPool().Query(ctx, query, params...)
    if err != nil {
        log.Errorw("Failed to search trips", "error", err)
        return nil, errors.NewDatabaseError(err)
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
            return nil, errors.NewDatabaseError(err)
        }
        trips = append(trips, &trip)
    }

    if err = rows.Err(); err != nil {
        log.Errorw("Error iterating trip rows", "error", err)
        return nil, errors.NewDatabaseError(err)
    }

    return trips, nil
}