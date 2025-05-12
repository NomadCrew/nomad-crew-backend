// Package db provides implementations for data access interfaces defined in internal/store.
// It interacts with the PostgreSQL database using pgxpool and potentially other external services like Supabase.
package db

import (
	"context"
	"fmt"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v4"
)

// TodoDB provides methods for interacting with todo items in the database.
// It uses a DatabaseClient for database connections.
type TodoDB struct {
	client *DatabaseClient
}

// NewTodoDB creates a new instance of TodoDB.
func NewTodoDB(client *DatabaseClient) *TodoDB {
	return &TodoDB{client: client}
}

// CreateTodo inserts a new todo item into the trip_todos table.
// It sets the initial status to Incomplete and populates the ID and CreatedAt fields.
func (tdb *TodoDB) CreateTodo(ctx context.Context, todo *types.Todo) (string, error) {
	query := `
        INSERT INTO trip_todos (trip_id, text, created_by, status)
        VALUES ($1, $2, $3, $4)
        RETURNING id, created_at`

	var todoID string
	err := tdb.client.GetPool().QueryRow(
		ctx, query,
		todo.TripID,
		todo.Text,
		todo.CreatedBy,
		types.TodoStatusIncomplete,
	).Scan(&todoID, &todo.CreatedAt)

	if err != nil {
		return "", errors.NewDatabaseError(fmt.Errorf("failed to create todo: %w", err))
	}

	// Set the ID on the todo object for consistency
	todo.ID = todoID
	todo.Status = types.TodoStatusIncomplete

	return todoID, nil
}

// ListTodos retrieves all non-deleted todos for a specific trip.
// This method matches the interface defined in store.TodoStore.
func (tdb *TodoDB) ListTodos(ctx context.Context, tripID string) ([]*types.Todo, error) {
	// Use the existing paginated method with no limits
	todos, _, err := tdb.ListTodosWithPagination(ctx, tripID, 1000, 0)
	return todos, err
}

// ListTodosWithPagination retrieves a paginated list of non-deleted todos for a specific trip.
// This method is an extended version of ListTodos with pagination support.
func (tdb *TodoDB) ListTodosWithPagination(ctx context.Context, tripID string, limit int, offset int) ([]*types.Todo, int, error) {
	// Get total count of non-deleted todos for the trip.
	var total int
	countQuery := `
        SELECT COUNT(*) 
        FROM trip_todos t
        LEFT JOIN metadata m ON m.table_name = 'trip_todos' AND m.record_id = t.id
        WHERE t.trip_id = $1 AND m.deleted_at IS NULL`

	err := tdb.client.GetPool().QueryRow(ctx, countQuery, tripID).Scan(&total)
	if err != nil {
		return nil, 0, errors.NewDatabaseError(fmt.Errorf("failed to count todos for trip %s: %w", tripID, err))
	}

	// Get the paginated list of non-deleted todos.
	query := `
        SELECT t.id, t.trip_id, t.text, t.status, t.created_by, t.created_at, t.updated_at 
        FROM trip_todos t
        LEFT JOIN metadata m ON m.table_name = 'trip_todos' AND m.record_id = t.id
        WHERE t.trip_id = $1 AND m.deleted_at IS NULL 
        ORDER BY t.status = 'COMPLETE', t.created_at DESC -- Incomplete first, then newest
        LIMIT $2 OFFSET $3`

	rows, err := tdb.client.GetPool().Query(ctx, query, tripID, limit, offset)
	if err != nil {
		return nil, 0, errors.NewDatabaseError(fmt.Errorf("failed to query todos for trip %s: %w", tripID, err))
	}
	defer rows.Close()

	var todos []*types.Todo
	for rows.Next() {
		var todo types.Todo
		err := rows.Scan(
			&todo.ID,
			&todo.TripID,
			&todo.Text,
			&todo.Status,
			&todo.CreatedBy,
			&todo.CreatedAt,
			&todo.UpdatedAt,
		)
		if err != nil {
			// Return partial results along with the error.
			return todos, total, errors.NewDatabaseError(fmt.Errorf("failed to scan todo row: %w", err))
		}
		todos = append(todos, &todo)
	}

	if err = rows.Err(); err != nil {
		// Error during iteration.
		return todos, total, errors.NewDatabaseError(fmt.Errorf("error iterating todo rows: %w", err))
	}

	return todos, total, nil
}

// UpdateTodo updates the status and/or text of an existing todo item.
// It dynamically builds the SQL query based on the fields provided in the update struct.
func (tdb *TodoDB) UpdateTodo(ctx context.Context, id string, update *types.TodoUpdate) (*types.Todo, error) {
	// Start building the query and arguments.
	query := "UPDATE trip_todos SET updated_at = CURRENT_TIMESTAMP"
	var args []interface{}
	args = append(args, id) // $1 will always be the id for the WHERE clause.
	paramCount := 1

	// Append status update if provided.
	if update.Status != nil {
		paramCount++
		query += fmt.Sprintf(", status = $%d", paramCount)
		args = append(args, *update.Status)
	}

	// Append text update if provided.
	if update.Text != nil {
		paramCount++
		query += fmt.Sprintf(", text = $%d", paramCount)
		args = append(args, *update.Text)
	}

	// Update WHERE clause to include RETURNING all fields
	query += ` WHERE id = $1 
	   RETURNING id, trip_id, text, status, created_by, created_at, updated_at`

	var todo types.Todo
	err := tdb.client.GetPool().QueryRow(ctx, query, args...).Scan(
		&todo.ID,
		&todo.TripID,
		&todo.Text,
		&todo.Status,
		&todo.CreatedBy,
		&todo.CreatedAt,
		&todo.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			// The ID did not match any row, likely meaning it doesn't exist or was deleted.
			return nil, errors.NotFound("Todo", id)
		}
		// Other potential database error during update.
		return nil, errors.NewDatabaseError(fmt.Errorf("failed to update todo %s: %w", id, err))
	}

	return &todo, nil
}

// DeleteTodo performs a soft delete on a todo item.
// It removes the row from trip_todos and adds a corresponding record to the metadata table.
func (tdb *TodoDB) DeleteTodo(ctx context.Context, id string) error {
	// Use a Common Table Expression (CTE) to delete the row and immediately insert
	// the metadata record in a single atomic operation.
	query := `
        WITH deleted AS (
            DELETE FROM trip_todos 
            WHERE id = $1
            RETURNING id
        )
        INSERT INTO metadata (table_name, record_id, deleted_at)
        SELECT 'trip_todos', id, NOW()
        FROM deleted
        RETURNING record_id -- Return something to check if deletion occurred.
    `
	var deletedID string
	// Use QueryRow(...).Scan(...) to check if the CTE affected any rows.
	err := tdb.client.GetPool().QueryRow(ctx, query, id).Scan(&deletedID)
	if err != nil {
		if err == pgx.ErrNoRows {
			// No rows were returned by the CTE, meaning the todo didn't exist to be deleted.
			return errors.NotFound("Todo", id)
		}
		// Other potential database error during soft delete.
		return errors.NewDatabaseError(fmt.Errorf("failed to soft delete todo %s: %w", id, err))
	}
	return nil
}

// GetTodosByCreator retrieves all non-deleted todos created by a specific user for a given trip.
func (tdb *TodoDB) GetTodosByCreator(ctx context.Context, tripID string, userID string) ([]*types.Todo, error) {
	query := `
        SELECT t.id, t.trip_id, t.text, t.status, t.created_by, t.created_at, t.updated_at 
        FROM trip_todos t
        LEFT JOIN metadata m ON m.table_name = 'trip_todos' AND m.record_id = t.id
        WHERE t.trip_id = $1 
        AND t.created_by = $2
        AND m.deleted_at IS NULL
        ORDER BY t.status = 'COMPLETE', t.created_at DESC -- Incomplete first, then newest`

	rows, err := tdb.client.GetPool().Query(ctx, query, tripID, userID)
	if err != nil {
		return nil, errors.NewDatabaseError(fmt.Errorf("failed to query todos for creator %s in trip %s: %w", userID, tripID, err))
	}
	defer rows.Close()

	var todos []*types.Todo
	for rows.Next() {
		var todo types.Todo
		err := rows.Scan(
			&todo.ID,
			&todo.TripID,
			&todo.Text,
			&todo.Status,
			&todo.CreatedBy,
			&todo.CreatedAt,
			&todo.UpdatedAt,
		)
		if err != nil {
			return todos, errors.NewDatabaseError(fmt.Errorf("failed to scan todo row for creator %s: %w", userID, err))
		}
		todos = append(todos, &todo)
	}

	if err = rows.Err(); err != nil {
		return todos, errors.NewDatabaseError(fmt.Errorf("error iterating todo rows for creator %s: %w", userID, err))
	}

	return todos, nil
}

// GetTodo retrieves a single todo item by its ID, regardless of its deleted status.
// Note: This method currently does not check the metadata table for soft deletion.
func (tdb *TodoDB) GetTodo(ctx context.Context, id string) (*types.Todo, error) {
	query := `
        SELECT id, trip_id, text, status, created_by, created_at, updated_at 
        FROM trip_todos
        WHERE id = $1`

	var todo types.Todo
	err := tdb.client.GetPool().QueryRow(ctx, query, id).Scan(
		&todo.ID,
		&todo.TripID,
		&todo.Text,
		&todo.Status,
		&todo.CreatedBy,
		&todo.CreatedAt,
		&todo.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, errors.NotFound("Todo", id)
	}
	if err != nil {
		return nil, errors.NewDatabaseError(fmt.Errorf("failed to get todo %s: %w", id, err))
	}

	return &todo, nil
}

// BeginTx starts a new database transaction
func (tdb *TodoDB) BeginTx(ctx context.Context) (store.Transaction, error) {
	tx, err := tdb.client.GetPool().Begin(ctx)
	if err != nil {
		return nil, errors.NewDatabaseError(fmt.Errorf("failed to begin transaction: %w", err))
	}

	return &Transaction{tx: tx}, nil
}

// Transaction wraps a pgx.Tx to implement store.Transaction
type Transaction struct {
	tx pgx.Tx
}

func (t *Transaction) Commit() error {
	return t.tx.Commit(context.Background())
}

func (t *Transaction) Rollback() error {
	return t.tx.Rollback(context.Background())
}
