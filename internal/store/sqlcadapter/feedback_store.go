package sqlcadapter

import (
	"context"
	"fmt"

	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Ensure sqlcFeedbackStore implements store.FeedbackStore
var _ store.FeedbackStore = (*sqlcFeedbackStore)(nil)

type sqlcFeedbackStore struct {
	pool *pgxpool.Pool
}

// NewSqlcFeedbackStore creates a new feedback store backed by pgxpool.
func NewSqlcFeedbackStore(pool *pgxpool.Pool) store.FeedbackStore {
	return &sqlcFeedbackStore{pool: pool}
}

// CreateFeedback inserts a new feedback entry and returns the generated ID.
func (s *sqlcFeedbackStore) CreateFeedback(ctx context.Context, fb *types.Feedback) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO feedback (name, email, message, source) VALUES ($1, $2, $3, $4) RETURNING id`,
		fb.Name, fb.Email, fb.Message, fb.Source,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to create feedback: %w", err)
	}
	return id, nil
}
