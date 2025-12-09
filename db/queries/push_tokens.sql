-- Push Token Operations

-- name: RegisterPushToken :one
-- Upserts a push token for a user (updates if already exists)
INSERT INTO user_push_tokens (user_id, token, device_type, is_active, last_used_at)
VALUES ($1, $2, $3, true, NOW())
ON CONFLICT (user_id, token) DO UPDATE SET
    is_active = true,
    device_type = EXCLUDED.device_type,
    updated_at = NOW(),
    last_used_at = NOW()
RETURNING id, user_id, token, device_type, is_active, created_at, updated_at, last_used_at;

-- name: DeactivatePushToken :exec
-- Deactivates a specific token (e.g., on logout)
UPDATE user_push_tokens
SET is_active = false, updated_at = NOW()
WHERE user_id = $1 AND token = $2;

-- name: DeactivateAllUserTokens :exec
-- Deactivates all tokens for a user (e.g., on account delete or forced logout)
UPDATE user_push_tokens
SET is_active = false, updated_at = NOW()
WHERE user_id = $1;

-- name: GetActiveTokensForUser :many
-- Gets all active push tokens for a user
SELECT id, user_id, token, device_type, is_active, created_at, updated_at, last_used_at
FROM user_push_tokens
WHERE user_id = $1 AND is_active = true
ORDER BY last_used_at DESC NULLS LAST;

-- name: GetActiveTokensForUsers :many
-- Gets all active push tokens for multiple users (for batch sending)
SELECT id, user_id, token, device_type, is_active, created_at, updated_at, last_used_at
FROM user_push_tokens
WHERE user_id = ANY($1::uuid[]) AND is_active = true;

-- name: InvalidateToken :exec
-- Marks a token as invalid (e.g., when Expo reports it as invalid)
UPDATE user_push_tokens
SET is_active = false, updated_at = NOW()
WHERE token = $1;

-- name: UpdateTokenLastUsed :exec
-- Updates the last_used_at timestamp for a token
UPDATE user_push_tokens
SET last_used_at = NOW()
WHERE token = $1 AND is_active = true;

-- name: CleanupOldInactiveTokens :exec
-- Removes inactive tokens older than 30 days
DELETE FROM user_push_tokens
WHERE is_active = false AND updated_at < NOW() - INTERVAL '30 days';

-- name: CountActiveTokens :one
-- Counts active tokens for a user
SELECT COUNT(*) as count
FROM user_push_tokens
WHERE user_id = $1 AND is_active = true;
