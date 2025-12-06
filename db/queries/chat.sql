-- Chat Operations

-- name: CreateChatGroup :one
INSERT INTO chat_groups (trip_id, name, description, created_by)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: GetChatGroup :one
SELECT
    id, trip_id, name,
    COALESCE(description, '') as description,
    created_by, created_at, updated_at
FROM chat_groups
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetChatGroupByTrip :one
SELECT
    id, trip_id, name,
    COALESCE(description, '') as description,
    created_by, created_at, updated_at
FROM chat_groups
WHERE trip_id = $1 AND deleted_at IS NULL
LIMIT 1;

-- name: CreateChatMessage :one
INSERT INTO chat_messages (group_id, user_id, content, content_type)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: GetChatMessage :one
SELECT
    id, group_id, user_id, content, content_type, created_at, updated_at
FROM chat_messages
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetChatMessages :many
SELECT
    id, group_id, user_id, content, content_type, created_at, updated_at
FROM chat_messages
WHERE group_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetChatMessagesAfter :many
-- Get messages after a specific timestamp (for real-time updates)
SELECT
    id, group_id, user_id, content, content_type, created_at, updated_at
FROM chat_messages
WHERE group_id = $1 AND created_at > $2 AND deleted_at IS NULL
ORDER BY created_at ASC;

-- name: SoftDeleteChatMessage :exec
UPDATE chat_messages
SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: AddChatGroupMember :exec
INSERT INTO chat_group_members (group_id, user_id)
VALUES ($1, $2)
ON CONFLICT (group_id, user_id) DO NOTHING;

-- name: RemoveChatGroupMember :exec
DELETE FROM chat_group_members
WHERE group_id = $1 AND user_id = $2;

-- name: GetChatGroupMembers :many
SELECT
    cgm.group_id, cgm.user_id, cgm.joined_at, cgm.last_read_message_id
FROM chat_group_members cgm
WHERE cgm.group_id = $1;

-- name: UpdateLastReadMessage :exec
UPDATE chat_group_members
SET last_read_message_id = $3
WHERE group_id = $1 AND user_id = $2;

-- name: GetLastReadMessageID :one
SELECT last_read_message_id
FROM chat_group_members
WHERE group_id = $1 AND user_id = $2;

-- name: AddMessageReaction :exec
INSERT INTO chat_message_reactions (message_id, user_id, reaction)
VALUES ($1, $2, $3)
ON CONFLICT (message_id, user_id, reaction) DO NOTHING;

-- name: RemoveMessageReaction :exec
DELETE FROM chat_message_reactions
WHERE message_id = $1 AND user_id = $2 AND reaction = $3;

-- name: GetMessageReactions :many
SELECT message_id, user_id, reaction, created_at
FROM chat_message_reactions
WHERE message_id = $1;

-- name: CountUnreadMessages :one
SELECT COUNT(*) as count
FROM chat_messages cm
INNER JOIN chat_group_members cgm ON cm.group_id = cgm.group_id
WHERE cgm.group_id = $1
    AND cgm.user_id = $2
    AND cm.deleted_at IS NULL
    AND (cgm.last_read_message_id IS NULL OR cm.id > cgm.last_read_message_id);
