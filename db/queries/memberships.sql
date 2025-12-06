-- Trip Memberships Operations

-- name: CreateMembership :exec
INSERT INTO trip_memberships (trip_id, user_id, role, status)
VALUES ($1, $2, $3, $4);

-- name: UpsertMembership :exec
INSERT INTO trip_memberships (trip_id, user_id, role, status)
VALUES ($1, $2, $3, $4)
ON CONFLICT (trip_id, user_id)
DO UPDATE SET
    role = EXCLUDED.role,
    status = EXCLUDED.status,
    updated_at = CURRENT_TIMESTAMP
WHERE trip_memberships.status != EXCLUDED.status;

-- name: GetMembership :one
SELECT id, trip_id, user_id, role, status, created_at, updated_at
FROM trip_memberships
WHERE trip_id = $1 AND user_id = $2;

-- name: GetActiveMembership :one
SELECT id, trip_id, user_id, role, status, created_at, updated_at
FROM trip_memberships
WHERE trip_id = $1 AND user_id = $2 AND status = 'ACTIVE';

-- name: GetTripMembers :many
SELECT id, trip_id, user_id, role, status, created_at, updated_at
FROM trip_memberships
WHERE trip_id = $1 AND status = 'ACTIVE'
ORDER BY created_at ASC;

-- name: GetUserRole :one
SELECT role
FROM trip_memberships
WHERE trip_id = $1 AND user_id = $2 AND status = 'ACTIVE';

-- name: UpdateMemberRole :exec
UPDATE trip_memberships
SET role = $3, updated_at = CURRENT_TIMESTAMP
WHERE trip_id = $1 AND user_id = $2 AND status = 'ACTIVE';

-- name: DeactivateMembership :exec
UPDATE trip_memberships
SET status = 'INACTIVE', updated_at = CURRENT_TIMESTAMP
WHERE trip_id = $1 AND user_id = $2 AND status = 'ACTIVE';

-- name: CountTripMembers :one
SELECT COUNT(*) as count
FROM trip_memberships
WHERE trip_id = $1 AND status = 'ACTIVE';

-- name: IsUserMember :one
SELECT EXISTS(
    SELECT 1 FROM trip_memberships
    WHERE trip_id = $1 AND user_id = $2 AND status = 'ACTIVE'
) as is_member;

-- name: GetMemberWithUserDetails :many
-- Get members with user profile information
SELECT
    tm.id,
    tm.trip_id,
    tm.user_id,
    tm.role,
    tm.status,
    tm.created_at,
    tm.updated_at,
    up.email,
    up.username,
    COALESCE(up.first_name, '') as first_name,
    COALESCE(up.last_name, '') as last_name,
    COALESCE(up.avatar_url, '') as avatar_url
FROM trip_memberships tm
INNER JOIN user_profiles up ON tm.user_id = up.id
WHERE tm.trip_id = $1 AND tm.status = 'ACTIVE'
ORDER BY tm.created_at ASC;
