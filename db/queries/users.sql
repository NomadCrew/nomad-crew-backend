-- Users Operations (using user_profiles table)

-- name: GetUserProfileByID :one
SELECT
    id, email, username,
    COALESCE(first_name, '') as first_name,
    COALESCE(last_name, '') as last_name,
    COALESCE(avatar_url, '') as avatar_url,
    contact_email,
    created_at, updated_at
FROM user_profiles
WHERE id = $1;

-- name: GetUserProfileByEmail :one
SELECT
    id, email, username,
    COALESCE(first_name, '') as first_name,
    COALESCE(last_name, '') as last_name,
    COALESCE(avatar_url, '') as avatar_url,
    contact_email,
    created_at, updated_at
FROM user_profiles
WHERE email = $1;

-- name: GetUserProfileByUsername :one
SELECT
    id, email, username,
    COALESCE(first_name, '') as first_name,
    COALESCE(last_name, '') as last_name,
    COALESCE(avatar_url, '') as avatar_url,
    contact_email,
    created_at, updated_at
FROM user_profiles
WHERE username = $1;

-- name: CreateUserProfile :one
INSERT INTO user_profiles (id, email, username, first_name, last_name, avatar_url)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id;

-- name: UpsertUserProfile :one
INSERT INTO user_profiles (id, email, username, first_name, last_name, avatar_url)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id)
DO UPDATE SET
    email = EXCLUDED.email,
    first_name = EXCLUDED.first_name,
    last_name = EXCLUDED.last_name,
    avatar_url = EXCLUDED.avatar_url,
    updated_at = CURRENT_TIMESTAMP
RETURNING id;

-- name: UpdateUserProfile :exec
UPDATE user_profiles
SET
    first_name = COALESCE(sqlc.narg('first_name'), first_name),
    last_name = COALESCE(sqlc.narg('last_name'), last_name),
    avatar_url = COALESCE(sqlc.narg('avatar_url'), avatar_url),
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1;

-- name: ListUserProfiles :many
SELECT
    id, email, username,
    COALESCE(first_name, '') as first_name,
    COALESCE(last_name, '') as last_name,
    COALESCE(avatar_url, '') as avatar_url,
    contact_email,
    created_at, updated_at
FROM user_profiles
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: SearchUserProfiles :many
-- Fuzzy search across username, email, contact_email, first_name, and last_name
-- Results are ordered by relevance: exact prefix matches first, then partial matches
SELECT
    id, email, username,
    COALESCE(first_name, '') as first_name,
    COALESCE(last_name, '') as last_name,
    COALESCE(avatar_url, '') as avatar_url,
    contact_email,
    created_at, updated_at
FROM user_profiles
WHERE
    username ILIKE '%' || $1 || '%'
    OR email ILIKE '%' || $1 || '%'
    OR contact_email ILIKE '%' || $1 || '%'
    OR first_name ILIKE '%' || $1 || '%'
    OR last_name ILIKE '%' || $1 || '%'
ORDER BY
    -- Prioritize exact prefix matches on username
    CASE WHEN username ILIKE $1 || '%' THEN 0 ELSE 1 END,
    -- Then exact prefix matches on email/contact_email
    CASE WHEN email ILIKE $1 || '%' OR contact_email ILIKE $1 || '%' THEN 0 ELSE 1 END,
    username ASC
LIMIT $2;

-- name: CheckUsernameExists :one
SELECT EXISTS(SELECT 1 FROM user_profiles WHERE username = $1) as exists;

-- name: CheckEmailExists :one
SELECT EXISTS(SELECT 1 FROM user_profiles WHERE email = $1) as exists;

-- name: UpdateUserContactEmail :exec
UPDATE user_profiles
SET contact_email = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $1;

-- name: GetUserProfileByContactEmail :one
-- Find user by their discoverable contact_email (for invitation lookups)
SELECT
    id, email, username,
    COALESCE(first_name, '') as first_name,
    COALESCE(last_name, '') as last_name,
    COALESCE(avatar_url, '') as avatar_url,
    contact_email,
    created_at, updated_at
FROM user_profiles
WHERE contact_email = $1;
