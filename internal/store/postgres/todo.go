package postgres

import (
	"context"
	"errors"

	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

var (
	ErrNotFound = errors.New("record not found")
)

// TodoStore implements the store.TodoStore interface using PostgreSQL
type TodoStore struct {
	pool *pgxpool.Pool
	tx   pgx.Tx
}

// NewTodoStore creates a new TodoStore instance
func NewTodoStore(pool *pgxpool.Pool) *TodoStore {
	return &TodoStore{
		pool: pool,
	}
}

// BeginTx starts a new database transaction
func (s *TodoStore) BeginTx(ctx context.Context) (store.Transaction, error) {
	if s.tx != nil {
		return nil, errors.New("transaction already started")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}

	s.tx = tx
	return &Transaction{tx: tx}, nil
}

// CreateTodo creates a new todo item in the database
func (s *TodoStore) CreateTodo(ctx context.Context, todo *types.Todo) (string, error) {
	query := `
		INSERT INTO todos (trip_id, text, status, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id`

	var id string
	row := s.queryRow(ctx, query,
		todo.TripID,
		todo.Text,
		todo.Status,
		todo.CreatedBy,
	)

	err := row.Scan(&id)
	if err != nil {
		return "", err
	}

	return id, nil
}

// GetTodo retrieves a todo item by its ID
func (s *TodoStore) GetTodo(ctx context.Context, id string) (*types.Todo, error) {
	query := `
		SELECT id, trip_id, text, status, created_by, created_at, updated_at
		FROM todos
		WHERE id = $1 AND deleted_at IS NULL`

	todo := &types.Todo{}
	row := s.queryRow(ctx, query, id)

	err := row.Scan(
		&todo.ID,
		&todo.TripID,
		&todo.Text,
		&todo.Status,
		&todo.CreatedBy,
		&todo.CreatedAt,
		&todo.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return todo, nil
}

// ListTodos retrieves all todos for a specific trip
func (s *TodoStore) ListTodos(ctx context.Context, tripID string) ([]*types.Todo, error) {
	query := `
		SELECT id, trip_id, text, status, created_by, created_at, updated_at
		FROM todos
		WHERE trip_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	rows, err := s.query(ctx, query, tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var todos []*types.Todo
	for rows.Next() {
		todo := &types.Todo{}
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
			return nil, err
		}
		todos = append(todos, todo)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return todos, nil
}

// UpdateTodo updates an existing todo item
func (s *TodoStore) UpdateTodo(ctx context.Context, id string, update *types.TodoUpdate) (*types.Todo, error) {
	query := `
		UPDATE todos
		SET text = COALESCE($1, text),
			status = COALESCE($2, status),
			updated_at = NOW()
		WHERE id = $3 AND deleted_at IS NULL
		RETURNING id, trip_id, text, status, created_by, created_at, updated_at`

	todo := &types.Todo{}
	row := s.queryRow(ctx, query,
		update.Text,
		update.Status,
		id,
	)

	err := row.Scan(
		&todo.ID,
		&todo.TripID,
		&todo.Text,
		&todo.Status,
		&todo.CreatedBy,
		&todo.CreatedAt,
		&todo.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return todo, nil
}

// DeleteTodo removes a todo item from the database (soft delete)
func (s *TodoStore) DeleteTodo(ctx context.Context, id string) error {
	query := `
		UPDATE todos
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`

	result, err := s.exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// Helper methods for database operations

func (s *TodoStore) queryRow(ctx context.Context, query string, args ...interface{}) pgx.Row {
	if s.tx != nil {
		return s.tx.QueryRow(ctx, query, args...)
	}
	return s.pool.QueryRow(ctx, query, args...)
}

func (s *TodoStore) query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	if s.tx != nil {
		return s.tx.Query(ctx, query, args...)
	}
	return s.pool.Query(ctx, query, args...)
}

func (s *TodoStore) exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	if s.tx != nil {
		return s.tx.Exec(ctx, query, args...)
	}
	return s.pool.Exec(ctx, query, args...)
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
