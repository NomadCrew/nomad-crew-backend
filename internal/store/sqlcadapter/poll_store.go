package sqlcadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	internal_store "github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Ensure sqlcPollStore implements internal_store.PollStore
var _ internal_store.PollStore = (*sqlcPollStore)(nil)

type sqlcPollStore struct {
	pool *pgxpool.Pool
}

// NewSqlcPollStore creates a new SQLC-based poll store
func NewSqlcPollStore(pool *pgxpool.Pool) internal_store.PollStore {
	return &sqlcPollStore{
		pool: pool,
	}
}

// CreatePoll creates a new poll in the database
func (s *sqlcPollStore) CreatePoll(ctx context.Context, poll *types.Poll) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO polls (trip_id, question, poll_type, is_blind, allow_multiple_votes, created_by, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		poll.TripID, poll.Question, poll.PollType, poll.IsBlind, poll.AllowMultipleVotes, poll.CreatedBy, poll.ExpiresAt,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to create poll: %w", err)
	}
	return id, nil
}

// CreatePollWithOptions creates a poll and all its options in a single transaction
func (s *sqlcPollStore) CreatePollWithOptions(ctx context.Context, poll *types.Poll, options []*types.PollOption) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var pollID string
	err = tx.QueryRow(ctx,
		`INSERT INTO polls (trip_id, question, poll_type, is_blind, allow_multiple_votes, created_by, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		poll.TripID, poll.Question, poll.PollType, poll.IsBlind, poll.AllowMultipleVotes, poll.CreatedBy, poll.ExpiresAt,
	).Scan(&pollID)
	if err != nil {
		return "", fmt.Errorf("failed to create poll: %w", err)
	}

	for _, opt := range options {
		var metadataJSON []byte
		if opt.OptionMetadata != nil {
			metadataJSON, err = json.Marshal(opt.OptionMetadata)
			if err != nil {
				return "", fmt.Errorf("failed to marshal option metadata: %w", err)
			}
		}
		_, err = tx.Exec(ctx,
			`INSERT INTO poll_options (poll_id, text, position, created_by, option_metadata)
			VALUES ($1, $2, $3, $4, $5)`,
			pollID, opt.Text, opt.Position, opt.CreatedBy, metadataJSON,
		)
		if err != nil {
			return "", fmt.Errorf("failed to create poll option: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("failed to commit poll creation: %w", err)
	}

	return pollID, nil
}

// GetPoll retrieves a poll by ID and trip ID (IDOR prevention: always requires trip_id)
func (s *sqlcPollStore) GetPoll(ctx context.Context, id, tripID string) (*types.Poll, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, trip_id, question, poll_type, is_blind, allow_multiple_votes, status, created_by,
		        closed_by, closed_at, expires_at, created_at, updated_at
		FROM polls
		WHERE id = $1 AND trip_id = $2 AND deleted_at IS NULL`,
		id, tripID,
	)

	return s.scanPoll(row)
}

// ListPolls retrieves polls for a trip with pagination
func (s *sqlcPollStore) ListPolls(ctx context.Context, tripID string, limit, offset int) ([]*types.Poll, int, error) {
	// Get total count
	var total int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM polls WHERE trip_id = $1 AND deleted_at IS NULL`,
		tripID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count polls: %w", err)
	}

	// Get paginated results
	rows, err := s.pool.Query(ctx,
		`SELECT id, trip_id, question, poll_type, is_blind, allow_multiple_votes, status, created_by,
		        closed_by, closed_at, expires_at, created_at, updated_at
		FROM polls
		WHERE trip_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`,
		tripID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list polls: %w", err)
	}
	defer rows.Close()

	polls := make([]*types.Poll, 0)
	for rows.Next() {
		poll, err := s.scanPollRows(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan poll: %w", err)
		}
		polls = append(polls, poll)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("poll rows iteration error: %w", err)
	}

	return polls, total, nil
}

// UpdatePollQuestion updates a poll's question
func (s *sqlcPollStore) UpdatePollQuestion(ctx context.Context, id, tripID, question string) (*types.Poll, error) {
	row := s.pool.QueryRow(ctx,
		`UPDATE polls
		SET question = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND trip_id = $2 AND deleted_at IS NULL
		RETURNING id, trip_id, question, poll_type, is_blind, allow_multiple_votes, status, created_by,
		          closed_by, closed_at, expires_at, created_at, updated_at`,
		id, tripID, question,
	)

	poll, err := s.scanPoll(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("poll", id)
		}
		return nil, fmt.Errorf("failed to update poll question: %w", err)
	}
	return poll, nil
}

// ClosePoll closes an active poll
func (s *sqlcPollStore) ClosePoll(ctx context.Context, id, tripID, closedBy string) (*types.Poll, error) {
	row := s.pool.QueryRow(ctx,
		`UPDATE polls
		SET status = 'CLOSED', closed_by = $3, closed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND trip_id = $2 AND deleted_at IS NULL AND status = 'ACTIVE'
		RETURNING id, trip_id, question, poll_type, is_blind, allow_multiple_votes, status, created_by,
		          closed_by, closed_at, expires_at, created_at, updated_at`,
		id, tripID, closedBy,
	)

	poll, err := s.scanPoll(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("poll", id)
		}
		return nil, fmt.Errorf("failed to close poll: %w", err)
	}
	return poll, nil
}

// SoftDeletePoll soft-deletes a poll
func (s *sqlcPollStore) SoftDeletePoll(ctx context.Context, id, tripID string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE polls
		SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND trip_id = $2 AND deleted_at IS NULL`,
		id, tripID,
	)
	if err != nil {
		return fmt.Errorf("failed to soft-delete poll: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("poll", id)
	}
	return nil
}

// CreatePollOption creates a new poll option
func (s *sqlcPollStore) CreatePollOption(ctx context.Context, option *types.PollOption) (string, error) {
	var metadataJSON []byte
	if option.OptionMetadata != nil {
		var err error
		metadataJSON, err = json.Marshal(option.OptionMetadata)
		if err != nil {
			return "", fmt.Errorf("failed to marshal option metadata: %w", err)
		}
	}
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO poll_options (poll_id, text, position, created_by, option_metadata)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		option.PollID, option.Text, option.Position, option.CreatedBy, metadataJSON,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to create poll option: %w", err)
	}
	return id, nil
}

// ListPollOptions retrieves all options for a poll
func (s *sqlcPollStore) ListPollOptions(ctx context.Context, pollID string) ([]*types.PollOption, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, poll_id, text, position, created_by, created_at, option_metadata
		FROM poll_options
		WHERE poll_id = $1
		ORDER BY position ASC`,
		pollID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list poll options: %w", err)
	}
	defer rows.Close()

	options := make([]*types.PollOption, 0)
	for rows.Next() {
		var opt types.PollOption
		var createdAt pgtype.Timestamptz
		var metadataRaw []byte
		if err := rows.Scan(&opt.ID, &opt.PollID, &opt.Text, &opt.Position, &opt.CreatedBy, &createdAt, &metadataRaw); err != nil {
			return nil, fmt.Errorf("failed to scan poll option: %w", err)
		}
		opt.CreatedAt = PgTimestamptzToTime(createdAt)
		if err := opt.UnmarshalOptionMetadata(metadataRaw); err != nil {
			return nil, fmt.Errorf("failed to unmarshal option metadata: %w", err)
		}
		options = append(options, &opt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("poll options rows iteration error: %w", err)
	}

	return options, nil
}

// CastVote inserts a vote with ON CONFLICT DO NOTHING for dedup
func (s *sqlcPollStore) CastVote(ctx context.Context, pollID, optionID, userID string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO poll_votes (poll_id, option_id, user_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (poll_id, option_id, user_id) DO NOTHING`,
		pollID, optionID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to cast vote: %w", err)
	}
	return nil
}

// SwapVote atomically removes all existing votes for a user on a poll and casts a new one.
// Used for single-choice polls to ensure the delete+insert is transactional.
func (s *sqlcPollStore) SwapVote(ctx context.Context, pollID, optionID, userID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Remove all existing votes for this user in this poll
	_, err = tx.Exec(ctx,
		`DELETE FROM poll_votes WHERE poll_id = $1 AND user_id = $2`,
		pollID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to remove existing votes: %w", err)
	}

	// Cast the new vote
	_, err = tx.Exec(ctx,
		`INSERT INTO poll_votes (poll_id, option_id, user_id)
		VALUES ($1, $2, $3)`,
		pollID, optionID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to cast vote: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit vote swap: %w", err)
	}

	return nil
}

// RemoveVote removes a specific vote
func (s *sqlcPollStore) RemoveVote(ctx context.Context, pollID, optionID, userID string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM poll_votes
		WHERE poll_id = $1 AND option_id = $2 AND user_id = $3`,
		pollID, optionID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to remove vote: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("vote", fmt.Sprintf("poll=%s option=%s user=%s", pollID, optionID, userID))
	}
	return nil
}

// RemoveAllUserVotesForPoll removes all votes by a user for a poll
func (s *sqlcPollStore) RemoveAllUserVotesForPoll(ctx context.Context, pollID, userID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM poll_votes WHERE poll_id = $1 AND user_id = $2`,
		pollID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to remove user votes: %w", err)
	}
	return nil
}

// GetUserVotesForPoll retrieves all votes by a user for a specific poll
func (s *sqlcPollStore) GetUserVotesForPoll(ctx context.Context, pollID, userID string) ([]*types.PollVote, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, poll_id, option_id, user_id, created_at
		FROM poll_votes
		WHERE poll_id = $1 AND user_id = $2`,
		pollID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get user votes: %w", err)
	}
	defer rows.Close()

	votes := make([]*types.PollVote, 0)
	for rows.Next() {
		var v types.PollVote
		var createdAt pgtype.Timestamptz
		if err := rows.Scan(&v.ID, &v.PollID, &v.OptionID, &v.UserID, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan vote: %w", err)
		}
		v.CreatedAt = PgTimestamptzToTime(createdAt)
		votes = append(votes, &v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("user votes rows iteration error: %w", err)
	}

	return votes, nil
}

// GetVoteCountsByPoll returns a map of optionID -> vote count
func (s *sqlcPollStore) GetVoteCountsByPoll(ctx context.Context, pollID string) (map[string]int, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT option_id, COUNT(*) as vote_count
		FROM poll_votes
		WHERE poll_id = $1
		GROUP BY option_id`,
		pollID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get vote counts: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var optionID string
		var count int64
		if err := rows.Scan(&optionID, &count); err != nil {
			return nil, fmt.Errorf("failed to scan vote count: %w", err)
		}
		counts[optionID] = int(count)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("vote counts rows iteration error: %w", err)
	}

	return counts, nil
}

// ListVotesByPoll retrieves all votes for a poll
func (s *sqlcPollStore) ListVotesByPoll(ctx context.Context, pollID string) ([]*types.PollVote, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, poll_id, option_id, user_id, created_at
		FROM poll_votes
		WHERE poll_id = $1`,
		pollID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list votes: %w", err)
	}
	defer rows.Close()

	votes := make([]*types.PollVote, 0)
	for rows.Next() {
		var v types.PollVote
		var createdAt pgtype.Timestamptz
		if err := rows.Scan(&v.ID, &v.PollID, &v.OptionID, &v.UserID, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan vote: %w", err)
		}
		v.CreatedAt = PgTimestamptzToTime(createdAt)
		votes = append(votes, &v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("votes rows iteration error: %w", err)
	}

	return votes, nil
}

// BeginTx starts a new database transaction
func (s *sqlcPollStore) BeginTx(ctx context.Context) (types.DatabaseTransaction, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return &txWrapper{tx: tx}, nil
}

// scanPoll scans a single poll row from QueryRow
func (s *sqlcPollStore) scanPoll(row pgx.Row) (*types.Poll, error) {
	var p types.Poll
	var pollType string
	var status string
	var closedBy *string
	var closedAt pgtype.Timestamptz
	var expiresAt pgtype.Timestamptz
	var createdAt pgtype.Timestamptz
	var updatedAt pgtype.Timestamptz

	err := row.Scan(
		&p.ID, &p.TripID, &p.Question, &pollType, &p.IsBlind, &p.AllowMultipleVotes, &status, &p.CreatedBy,
		&closedBy, &closedAt, &expiresAt, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("poll", "")
		}
		return nil, fmt.Errorf("failed to scan poll: %w", err)
	}

	p.PollType = types.PollType(pollType)
	p.Status = types.PollStatus(status)
	p.ClosedBy = closedBy
	p.ClosedAt = PgTimestamptzToTimePtr(closedAt)
	p.ExpiresAt = PgTimestamptzToTime(expiresAt)
	p.CreatedAt = PgTimestamptzToTime(createdAt)
	p.UpdatedAt = PgTimestamptzToTime(updatedAt)

	return &p, nil
}

// scanPollRows scans a poll from a Rows iterator
func (s *sqlcPollStore) scanPollRows(rows pgx.Rows) (*types.Poll, error) {
	var p types.Poll
	var pollType string
	var status string
	var closedBy *string
	var closedAt pgtype.Timestamptz
	var expiresAt pgtype.Timestamptz
	var createdAt pgtype.Timestamptz
	var updatedAt pgtype.Timestamptz

	err := rows.Scan(
		&p.ID, &p.TripID, &p.Question, &pollType, &p.IsBlind, &p.AllowMultipleVotes, &status, &p.CreatedBy,
		&closedBy, &closedAt, &expiresAt, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	p.PollType = types.PollType(pollType)
	p.Status = types.PollStatus(status)
	p.ClosedBy = closedBy
	p.ClosedAt = PgTimestamptzToTimePtr(closedAt)
	p.ExpiresAt = PgTimestamptzToTime(expiresAt)
	p.CreatedAt = PgTimestamptzToTime(createdAt)
	p.UpdatedAt = PgTimestamptzToTime(updatedAt)

	return &p, nil
}

// CountUniqueVotersByPoll returns the count of distinct users who voted on a poll
func (s *sqlcPollStore) CountUniqueVotersByPoll(ctx context.Context, pollID string) (int, error) {
	row := s.pool.QueryRow(ctx, "SELECT COUNT(DISTINCT user_id) FROM poll_votes WHERE poll_id = $1", pollID)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("count unique voters: %w", err)
	}
	return count, nil
}
