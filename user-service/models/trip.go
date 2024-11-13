package models

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/user-service/db"
)

type Trip struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Description string `json:"description"`
	Destination string `json:"destination"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	CreatedBy int64    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TripUpdate struct {
    Name        string    `json:"name"`
    Description string    `json:"description"`
    Destination string    `json:"destination"`
    StartDate   time.Time `json:"start_date"`
    EndDate     time.Time `json:"end_date"`
}

// GetTripByID retrieves a trip by ID
func GetTripByID(ctx context.Context, tripDB *db.TripDB, id int64) (*Trip, error) {
    const query = `SELECT id, name, description, start_date, end_date, destination, created_by FROM trips WHERE id = $1`
    var trip Trip
    err := tripDB.pool.QueryRow(ctx, query, id).Scan(
        &trip.ID,
        &trip.Name,
        &trip.Description,
        &trip.StartDate,
        &trip.EndDate,
        &trip.Destination,
        &trip.CreatedBy,
    )
    if err!= nil {
        if err == sql.ErrNoRows {
            return nil, errors.New("trip not found")
        }
        return nil, err
    }
    return &trip, nil
}

// CreateTrip creates a new trip
