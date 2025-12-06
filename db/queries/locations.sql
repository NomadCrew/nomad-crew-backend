-- Locations Operations

-- name: CreateLocation :one
INSERT INTO locations (
    trip_id, user_id, latitude, longitude, accuracy, timestamp,
    location_name, location_type, notes, status,
    is_sharing_enabled, sharing_expires_at, privacy
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id;

-- name: GetLocation :one
SELECT
    id, trip_id, user_id, latitude, longitude, accuracy, timestamp,
    COALESCE(location_name, '') as location_name,
    COALESCE(location_type, '') as location_type,
    COALESCE(notes, '') as notes,
    COALESCE(status, 'planned') as status,
    is_sharing_enabled, sharing_expires_at, privacy,
    created_at, updated_at
FROM locations
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserLocationInTrip :one
-- Get the latest location for a user in a specific trip
SELECT
    id, trip_id, user_id, latitude, longitude, accuracy, timestamp,
    COALESCE(location_name, '') as location_name,
    COALESCE(location_type, '') as location_type,
    COALESCE(notes, '') as notes,
    COALESCE(status, 'planned') as status,
    is_sharing_enabled, sharing_expires_at, privacy,
    created_at, updated_at
FROM locations
WHERE trip_id = $1 AND user_id = $2 AND deleted_at IS NULL
ORDER BY timestamp DESC
LIMIT 1;

-- name: GetTripMemberLocations :many
-- Get latest locations for all members of a trip with user info
SELECT DISTINCT ON (l.user_id)
    l.id, l.trip_id, l.user_id, l.latitude, l.longitude, l.accuracy, l.timestamp,
    COALESCE(l.location_name, '') as location_name,
    COALESCE(l.location_type, '') as location_type,
    COALESCE(l.notes, '') as notes,
    COALESCE(l.status, 'planned') as status,
    l.is_sharing_enabled, l.sharing_expires_at, l.privacy,
    l.created_at, l.updated_at,
    COALESCE(up.first_name || ' ' || up.last_name, up.username, '') as user_name,
    tm.role as user_role
FROM locations l
INNER JOIN trip_memberships tm ON l.trip_id = tm.trip_id AND l.user_id = tm.user_id
LEFT JOIN user_profiles up ON l.user_id = up.id
WHERE l.trip_id = $1
    AND tm.status = 'ACTIVE'
    AND l.deleted_at IS NULL
    AND l.is_sharing_enabled = true
ORDER BY l.user_id, l.timestamp DESC;

-- name: UpdateLocation :exec
UPDATE locations
SET
    latitude = $2,
    longitude = $3,
    accuracy = $4,
    timestamp = $5,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateLocationSharing :exec
UPDATE locations
SET
    is_sharing_enabled = $2,
    sharing_expires_at = $3,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateLocationPrivacy :exec
UPDATE locations
SET
    privacy = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL;

-- name: SoftDeleteLocation :exec
UPDATE locations
SET deleted_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL;

-- name: DisableExpiredSharing :exec
-- Disable location sharing that has expired
UPDATE locations
SET
    is_sharing_enabled = false,
    updated_at = CURRENT_TIMESTAMP
WHERE is_sharing_enabled = true
    AND sharing_expires_at IS NOT NULL
    AND sharing_expires_at <= CURRENT_TIMESTAMP
    AND deleted_at IS NULL;
