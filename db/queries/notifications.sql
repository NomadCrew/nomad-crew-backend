-- Notifications Operations

-- name: CreateNotification :one
INSERT INTO notifications (user_id, type, metadata, is_read)
VALUES ($1, $2, $3, false)
RETURNING id;

-- name: GetNotification :one
SELECT id, user_id, type, metadata, is_read, created_at, updated_at
FROM notifications
WHERE id = $1;

-- name: GetUserNotifications :many
SELECT id, user_id, type, metadata, is_read, created_at, updated_at
FROM notifications
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetUnreadNotifications :many
SELECT id, user_id, type, metadata, is_read, created_at, updated_at
FROM notifications
WHERE user_id = $1 AND is_read = false
ORDER BY created_at DESC;

-- name: MarkNotificationAsRead :exec
UPDATE notifications
SET is_read = true, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND user_id = $2;

-- name: MarkAllNotificationsRead :exec
UPDATE notifications
SET is_read = true, updated_at = CURRENT_TIMESTAMP
WHERE user_id = $1 AND is_read = false;

-- name: DeleteNotification :exec
DELETE FROM notifications
WHERE id = $1 AND user_id = $2;

-- name: DeleteOldNotifications :exec
DELETE FROM notifications
WHERE created_at < CURRENT_TIMESTAMP - INTERVAL '30 days';

-- name: CountUnreadNotifications :one
SELECT COUNT(*) as count
FROM notifications
WHERE user_id = $1 AND is_read = false;
