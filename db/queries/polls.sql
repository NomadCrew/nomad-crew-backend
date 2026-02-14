-- Poll Operations

-- name: CreatePoll :one
INSERT INTO polls (trip_id, question, allow_multiple_votes, created_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetPoll :one
SELECT * FROM polls
WHERE id = $1 AND trip_id = $2 AND deleted_at IS NULL;

-- name: ListPollsByTrip :many
SELECT * FROM polls
WHERE trip_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountPollsByTrip :one
SELECT COUNT(*) as count FROM polls
WHERE trip_id = $1 AND deleted_at IS NULL;

-- name: UpdatePollQuestion :one
UPDATE polls
SET question = $3, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND trip_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: ClosePoll :one
UPDATE polls
SET status = 'CLOSED', closed_by = $3, closed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND trip_id = $2 AND deleted_at IS NULL AND status = 'ACTIVE'
RETURNING *;

-- name: SoftDeletePoll :exec
UPDATE polls
SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND trip_id = $2 AND deleted_at IS NULL;

-- name: CreatePollOption :one
INSERT INTO poll_options (poll_id, text, position, created_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListPollOptions :many
SELECT * FROM poll_options
WHERE poll_id = $1
ORDER BY position ASC;

-- name: CastVote :exec
INSERT INTO poll_votes (poll_id, option_id, user_id)
VALUES ($1, $2, $3)
ON CONFLICT (poll_id, option_id, user_id) DO NOTHING;

-- name: RemoveVote :exec
DELETE FROM poll_votes
WHERE poll_id = $1 AND option_id = $2 AND user_id = $3;

-- name: RemoveAllUserVotesForPoll :exec
DELETE FROM poll_votes
WHERE poll_id = $1 AND user_id = $2;

-- name: GetUserVotesForPoll :many
SELECT * FROM poll_votes
WHERE poll_id = $1 AND user_id = $2;

-- name: GetVoteCountsByPoll :many
SELECT option_id, COUNT(*) as vote_count
FROM poll_votes
WHERE poll_id = $1
GROUP BY option_id;

-- name: ListVotesByPoll :many
SELECT * FROM poll_votes
WHERE poll_id = $1;
