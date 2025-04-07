package postgres

import (
	"context"
	"fmt"

	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/NomadCrew/nomad-crew-backend/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/pkg/errors"
)

// Ensure pgUserStore implements store.UserStore.
var _ store.UserStore = (*pgUserStore)(nil)

type pgUserStore struct {
	pool *pgxpool.Pool
}

// NewPgUserStore creates a new PostgreSQL user store.
// NOTE: This assumes a local 'users' table mirroring models.User exists.
// A migration would be needed to create this table.
func NewPgUserStore(pool *pgxpool.Pool) store.UserStore {
	return &pgUserStore{pool: pool}
}

// GetUserByID retrieves basic user details from the assumed 'users' table.
func (s *pgUserStore) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	query := `SELECT id, username, first_name, last_name, email, created_at, updated_at
	          FROM users
	          WHERE id = $1`

	u := &models.User{}
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.Username, &u.FirstName, &u.LastName, &u.Email, &u.CreatedAt, &u.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user with id %s not found: %w", id, store.ErrNotFound)
		}
		// TODO: Add specific logging here
		return nil, errors.Wrap(err, "failed to get user by id")
	}
	return u, nil
}

// Add other UserStore methods implementations here...
