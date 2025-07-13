package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// pgxTxWrapper wraps pgx.Tx to implement store.Transaction
type pgxTxWrapper struct {
	tx pgx.Tx
}

func (w *pgxTxWrapper) Commit() error {
	return w.tx.Commit(context.Background())
}

func (w *pgxTxWrapper) Rollback() error {
	return w.tx.Rollback(context.Background())
}

type LocationStore struct {
	pool *pgxpool.Pool
}

func NewLocationStore(pool *pgxpool.Pool) store.LocationStore {
	return &LocationStore{pool: pool}
}

func (s *LocationStore) CreateLocation(ctx context.Context, location *types.Location) (string, error) {
	query := `
		INSERT INTO locations (trip_id, user_id, latitude, longitude, accuracy, "timestamp")
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`

	var id uuid.UUID // Expect UUID from the database
	err := s.pool.QueryRow(ctx, query,
		location.TripID,
		location.UserID,
		location.Latitude,
		location.Longitude,
		location.Accuracy,
		location.Timestamp,
	).Scan(&id)

	if err != nil {
		return "", err
	}

	return id.String(), nil // Return the string representation of the UUID
}

func (s *LocationStore) GetLocation(ctx context.Context, id string) (*types.Location, error) {
	query := `
		SELECT id, trip_id, user_id, latitude, longitude, accuracy, "timestamp", created_at, updated_at
		FROM locations
		WHERE id = $1
	`

	location := &types.Location{}
	var locationID uuid.UUID // Scan into UUID
	var tripID uuid.UUID
	var userID uuid.UUID
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&locationID,
		&tripID,
		&userID,
		&location.Latitude,
		&location.Longitude,
		&location.Accuracy,
		&location.Timestamp,
		&location.CreatedAt,
		&location.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}

	// Assign scanned UUIDs as strings to the struct
	location.ID = locationID.String()
	location.TripID = tripID.String()
	location.UserID = userID.String()

	return location, nil
}

func (s *LocationStore) UpdateLocation(ctx context.Context, id string, update *types.LocationUpdate) (*types.Location, error) {
	query := `
		UPDATE locations
		SET latitude = $1,
			longitude = $2,
			accuracy = $3,
			timestamp = $4,
			updated_at = NOW()
		WHERE id = $5
		RETURNING id, trip_id, user_id, latitude, longitude, accuracy, timestamp, created_at, updated_at
	`

	location := &types.Location{}
	var locationID uuid.UUID // Scan into UUID
	var tripID uuid.UUID
	var userID uuid.UUID
	err := s.pool.QueryRow(ctx, query,
		update.Latitude,
		update.Longitude,
		update.Accuracy,
		time.UnixMilli(update.Timestamp).UTC(), // Ensure UTC
		id,                                     // The location ID to update
	).Scan(
		&locationID,
		&tripID,
		&userID,
		&location.Latitude,
		&location.Longitude,
		&location.Accuracy,
		&location.Timestamp,
		&location.CreatedAt,
		&location.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}

	// Assign scanned UUIDs as strings to the struct
	location.ID = locationID.String()
	location.TripID = tripID.String()
	location.UserID = userID.String()

	return location, nil
}

func (s *LocationStore) DeleteLocation(ctx context.Context, id string) error {
	query := `DELETE FROM locations WHERE id = $1`
	result, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return sql.ErrNoRows // Return ErrNoRows if ID didn't exist
	}
	return nil
}

func (s *LocationStore) ListTripMemberLocations(ctx context.Context, tripID string) ([]*types.MemberLocation, error) {
	query := `
		SELECT l.id, l.trip_id, l.user_id, l.latitude, l.longitude, l.accuracy, l.timestamp, l.created_at, l.updated_at,
			   u.name as user_name, tm.role as user_role
		FROM locations l
		JOIN trip_memberships tm ON l.user_id = tm.user_id AND l.trip_id = tm.trip_id
		JOIN users u ON l.user_id = u.id
		WHERE l.trip_id = $1
		ORDER BY l.timestamp DESC
	`

	rows, err := s.pool.Query(ctx, query, tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var locations []*types.MemberLocation
	for rows.Next() {
		location := &types.MemberLocation{}
		var locationID uuid.UUID
		var locTripID uuid.UUID
		var userID uuid.UUID
		err := rows.Scan(
			&locationID,
			&locTripID, // Use different var name to avoid shadowing tripID parameter
			&userID,
			&location.Latitude,
			&location.Longitude,
			&location.Accuracy,
			&location.Timestamp,
			&location.CreatedAt,
			&location.UpdatedAt,
			&location.UserName,
			&location.UserRole,
		)
		if err != nil {
			return nil, err
		}
		location.ID = locationID.String()
		location.TripID = locTripID.String()
		location.UserID = userID.String()
		locations = append(locations, location)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return locations, nil
}

func (s *LocationStore) BeginTx(ctx context.Context) (store.Transaction, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &pgxTxWrapper{tx: tx}, nil
}

// UpdateUserRoleInTrip updates a user's role in a trip.
// ... existing code ...
