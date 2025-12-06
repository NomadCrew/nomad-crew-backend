-- Trip Invitations Operations

-- name: CreateInvitation :one
INSERT INTO trip_invitations
    (id, trip_id, inviter_id, invitee_email, role, token, status, expires_at)
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id;

-- name: GetInvitation :one
SELECT
    id, trip_id, inviter_id, invitee_email, role, token, status,
    expires_at, created_at, updated_at
FROM trip_invitations
WHERE id = $1;

-- name: GetInvitationByToken :one
SELECT
    id, trip_id, inviter_id, invitee_email, role, token, status,
    expires_at, created_at, updated_at
FROM trip_invitations
WHERE token = $1;

-- name: GetInvitationsByTripID :many
SELECT
    id, trip_id, inviter_id, invitee_email, role, token, status,
    expires_at, created_at, updated_at
FROM trip_invitations
WHERE trip_id = $1
ORDER BY created_at DESC;

-- name: GetPendingInvitationsForUser :many
SELECT
    id, trip_id, inviter_id, invitee_email, role, token, status,
    expires_at, created_at, updated_at
FROM trip_invitations
WHERE invitee_email = $1 AND status = 'PENDING' AND expires_at > CURRENT_TIMESTAMP
ORDER BY created_at DESC;

-- name: UpdateInvitationStatus :exec
UPDATE trip_invitations
SET status = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $1;

-- name: DeleteExpiredInvitations :exec
UPDATE trip_invitations
SET status = 'DECLINED', updated_at = CURRENT_TIMESTAMP
WHERE status = 'PENDING' AND expires_at <= CURRENT_TIMESTAMP;

-- name: CountPendingInvitationsForTrip :one
SELECT COUNT(*) as count
FROM trip_invitations
WHERE trip_id = $1 AND status = 'PENDING';
