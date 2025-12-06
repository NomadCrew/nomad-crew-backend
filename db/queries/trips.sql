-- Trips CRUD Operations

-- name: CreateTrip :one
INSERT INTO trips (
    name, description, start_date, end_date,
    destination_place_id, destination_address, destination_name,
    destination_latitude, destination_longitude,
    created_by, status, background_image_url
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING id;

-- name: GetTrip :one
SELECT
    id, name, description, start_date, end_date,
    destination_place_id, destination_address, destination_name,
    destination_latitude, destination_longitude,
    status, created_by, created_at, updated_at, deleted_at,
    COALESCE(background_image_url, '') as background_image_url
FROM trips
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetTripForMember :one
-- Get a trip that a specific user is a member of
SELECT
    t.id, t.name, t.description, t.start_date, t.end_date,
    t.destination_place_id, t.destination_address, t.destination_name,
    t.destination_latitude, t.destination_longitude,
    t.status, t.created_by, t.created_at, t.updated_at, t.deleted_at,
    COALESCE(t.background_image_url, '') as background_image_url
FROM trips t
INNER JOIN trip_memberships tm ON t.id = tm.trip_id
WHERE t.id = $1
    AND tm.user_id = $2
    AND tm.status = 'ACTIVE'
    AND t.deleted_at IS NULL;

-- name: ListUserTrips :many
-- Get all trips created by a specific user
SELECT
    id, name, description, start_date, end_date,
    destination_place_id, destination_address, destination_name,
    destination_latitude, destination_longitude,
    status, created_by, created_at, updated_at, deleted_at,
    COALESCE(background_image_url, '') as background_image_url
FROM trips
WHERE created_by = $1 AND deleted_at IS NULL
ORDER BY start_date DESC;

-- name: ListMemberTrips :many
-- Get all trips where user is a member (active membership)
SELECT
    t.id, t.name, t.description, t.start_date, t.end_date,
    t.destination_place_id, t.destination_address, t.destination_name,
    t.destination_latitude, t.destination_longitude,
    t.status, t.created_by, t.created_at, t.updated_at, t.deleted_at,
    COALESCE(t.background_image_url, '') as background_image_url
FROM trips t
INNER JOIN trip_memberships tm ON t.id = tm.trip_id
WHERE tm.user_id = $1
    AND tm.status = 'ACTIVE'
    AND t.deleted_at IS NULL
ORDER BY t.start_date DESC;

-- name: SoftDeleteTrip :exec
UPDATE trips
SET deleted_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetTripStatus :one
SELECT status, deleted_at
FROM trips
WHERE id = $1;

-- name: UpdateTripName :exec
UPDATE trips
SET name = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateTripDescription :exec
UPDATE trips
SET description = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateTripDates :exec
UPDATE trips
SET start_date = $2, end_date = $3, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateTripStatus :exec
UPDATE trips
SET status = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateTripDestination :exec
UPDATE trips
SET
    destination_place_id = $2,
    destination_address = $3,
    destination_name = $4,
    destination_latitude = $5,
    destination_longitude = $6,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL;

-- name: SearchTrips :many
-- Search trips by destination name/address with optional date filters
SELECT
    id, name, description, start_date, end_date,
    destination_place_id, destination_address, destination_name,
    destination_latitude, destination_longitude,
    status, created_by, created_at, updated_at, deleted_at,
    COALESCE(background_image_url, '') as background_image_url
FROM trips
WHERE deleted_at IS NULL
    AND (
        COALESCE(sqlc.narg('destination')::text, '') = ''
        OR destination_name ILIKE '%' || sqlc.narg('destination')::text || '%'
        OR destination_address ILIKE '%' || sqlc.narg('destination')::text || '%'
    )
    AND (sqlc.narg('start_date_from')::date IS NULL OR start_date >= sqlc.narg('start_date_from')::date)
    AND (sqlc.narg('start_date_to')::date IS NULL OR start_date <= sqlc.narg('start_date_to')::date)
    AND (sqlc.narg('end_date_from')::date IS NULL OR end_date >= sqlc.narg('end_date_from')::date)
    AND (sqlc.narg('end_date_to')::date IS NULL OR end_date <= sqlc.narg('end_date_to')::date)
ORDER BY start_date DESC;
