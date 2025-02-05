package db

import (
	"context"
	"fmt"
	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v4"
)

type TodoDB struct {
	client *DatabaseClient
}

func NewTodoDB(client *DatabaseClient) *TodoDB {
	return &TodoDB{client: client}
}

func (tdb *TodoDB) CreateTodo(ctx context.Context, todo *types.Todo) error {
	query := `
        INSERT INTO trip_todos (trip_id, text, created_by, status)
        VALUES ($1, $2, $3, $4)
        RETURNING id, created_at`

	err := tdb.client.GetPool().QueryRow(
		ctx, query,
		todo.TripID,
		todo.Text,
		todo.CreatedBy,
		types.TodoStatusIncomplete,
	).Scan(&todo.ID, &todo.CreatedAt)

	if err != nil {
		return errors.NewDatabaseError(err)
	}

	return nil
}

func (tdb *TodoDB) ListTodos(ctx context.Context, tripID string, limit int, offset int) ([]*types.Todo, int, error) {
	// First get total count
	var total int
	countQuery := `
        SELECT COUNT(*) 
        FROM trip_todos t
        LEFT JOIN metadata m ON m.table_name = 'trip_todos' AND m.record_id = t.id
        WHERE t.trip_id = $1 AND m.deleted_at IS NULL`

	err := tdb.client.GetPool().QueryRow(ctx, countQuery, tripID).Scan(&total)
	if err != nil {
		return nil, 0, errors.NewDatabaseError(err)
	}

	// Then get paginated results
	query := `
        SELECT t.id, t.trip_id, t.text, t.status, t.created_by, t.created_at, t.updated_at 
        FROM trip_todos t
        LEFT JOIN metadata m ON m.table_name = 'trip_todos' AND m.record_id = t.id
        WHERE t.trip_id = $1 AND m.deleted_at IS NULL 
        ORDER BY t.status = 'COMPLETE', t.created_at DESC
        LIMIT $2 OFFSET $3`

	rows, err := tdb.client.GetPool().Query(ctx, query, tripID, limit, offset)
	if err != nil {
		return nil, 0, errors.NewDatabaseError(err)
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
			return nil, 0, errors.NewDatabaseError(err)
		}
		todos = append(todos, &todo)
	}

	return todos, total, nil
}

func (tdb *TodoDB) UpdateTodo(ctx context.Context, id string, update *types.TodoUpdate) error {
	query := `
        UPDATE trip_todos 
        SET updated_at = CURRENT_TIMESTAMP`

	var args []interface{}
	args = append(args, id)
	paramCount := 1

	if update.Status != nil {
		paramCount++
		query += fmt.Sprintf(", status = $%d", paramCount)
		args = append(args, *update.Status)
	}

	if update.Text != nil {
		paramCount++
		query += fmt.Sprintf(", text = $%d", paramCount)
		args = append(args, *update.Text)
	}

	query += " WHERE id = $1 RETURNING id"

	var returnedID string
	err := tdb.client.GetPool().QueryRow(ctx, query, args...).Scan(&returnedID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return errors.NotFound("Todo", id)
		}
		return errors.NewDatabaseError(err)
	}

	return nil
}

func (tdb *TodoDB) DeleteTodo(ctx context.Context, id string, userID string) error {
	query := `
        WITH deleted AS (
            DELETE FROM trip_todos 
            WHERE id = $1
            RETURNING id
        )
        INSERT INTO metadata (table_name, record_id, deleted_by, deleted_at)
        SELECT 'trip_todos', id, $2, NOW()
        FROM deleted
    `
	_, err := tdb.client.GetPool().Exec(ctx, query, id, userID)
	return err
}

func (tdb *TodoDB) GetTodosByCreator(ctx context.Context, tripID string, userID string) ([]*types.Todo, error) {
	query := `
        SELECT t.id, t.trip_id, t.text, t.status, t.created_by, t.created_at, t.updated_at 
        FROM trip_todos t
        LEFT JOIN metadata m ON m.table_name = 'trip_todos' AND m.record_id = t.id
        WHERE t.trip_id = $1 
        AND t.created_by = $2
        AND m.deleted_at IS NULL
        ORDER BY t.status = 'COMPLETE', t.created_at DESC`

	rows, err := tdb.client.GetPool().Query(ctx, query, tripID, userID)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
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
			return nil, errors.NewDatabaseError(err)
		}
		todos = append(todos, &todo)
	}

	return todos, nil
}

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
		return nil, errors.NewDatabaseError(err)
	}

	return &todo, nil
}
