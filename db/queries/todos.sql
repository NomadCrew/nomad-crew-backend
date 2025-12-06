-- Todos Operations

-- name: CreateTodo :one
INSERT INTO todos (trip_id, text, status, created_by)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: GetTodo :one
SELECT id, trip_id, text, status, created_by, created_at, updated_at
FROM todos
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListTodosByTrip :many
SELECT id, trip_id, text, status, created_by, created_at, updated_at
FROM todos
WHERE trip_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: ListTodosByUser :many
SELECT id, trip_id, text, status, created_by, created_at, updated_at
FROM todos
WHERE created_by = $1 AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: UpdateTodoText :exec
UPDATE todos
SET text = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateTodoStatus :exec
UPDATE todos
SET status = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL;

-- name: SoftDeleteTodo :exec
UPDATE todos
SET deleted_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL;

-- name: CountTodosByTrip :one
SELECT COUNT(*) as count
FROM todos
WHERE trip_id = $1 AND deleted_at IS NULL;

-- name: CountCompletedTodosByTrip :one
SELECT COUNT(*) as count
FROM todos
WHERE trip_id = $1 AND status = 'COMPLETE' AND deleted_at IS NULL;
