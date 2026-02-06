package sqlcadapter

import (
	"context"
	"errors"
	"fmt"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/sqlc"
	internal_store "github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Ensure sqlcTodoStore implements internal_store.TodoStore
var _ internal_store.TodoStore = (*sqlcTodoStore)(nil)

type sqlcTodoStore struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

// NewSqlcTodoStore creates a new SQLC-based todo store
func NewSqlcTodoStore(pool *pgxpool.Pool) internal_store.TodoStore {
	return &sqlcTodoStore{
		pool:    pool,
		queries: sqlc.New(pool),
	}
}

// CreateTodo creates a new todo item in the database
func (s *sqlcTodoStore) CreateTodo(ctx context.Context, todo *types.Todo) (string, error) {
	log := logger.GetLogger()

	todoID, err := s.queries.CreateTodo(ctx, sqlc.CreateTodoParams{
		TripID:    todo.TripID,
		Text:      todo.Text,
		Status:    TodoStatusToSqlc(todo.Status),
		CreatedBy: todo.CreatedBy,
	})
	if err != nil {
		return "", fmt.Errorf("failed to insert todo: %w", err)
	}

	log.Infow("Successfully created todo", "todoID", todoID)
	return todoID, nil
}

// GetTodo retrieves a todo item by its ID
func (s *sqlcTodoStore) GetTodo(ctx context.Context, id string) (*types.Todo, error) {
	row, err := s.queries.GetTodo(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("todo", id)
		}
		return nil, fmt.Errorf("failed to get todo: %w", err)
	}

	return GetTodoRowToTodo(row), nil
}

// ListTodos retrieves all todos for a specific trip
func (s *sqlcTodoStore) ListTodos(ctx context.Context, tripID string) ([]*types.Todo, error) {
	log := logger.GetLogger()

	rows, err := s.queries.ListTodosByTrip(ctx, tripID)
	if err != nil {
		return nil, fmt.Errorf("failed to list todos: %w", err)
	}

	todos := make([]*types.Todo, 0, len(rows))
	for _, row := range rows {
		todos = append(todos, ListTodosByTripRowToTodo(row))
	}

	log.Infow("Successfully listed todos", "tripID", tripID, "count", len(todos))
	return todos, nil
}

// UpdateTodo updates an existing todo item
func (s *sqlcTodoStore) UpdateTodo(ctx context.Context, id string, update *types.TodoUpdate) (*types.Todo, error) {
	log := logger.GetLogger()

	// First check if the todo exists
	_, err := s.queries.GetTodo(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("todo", id)
		}
		return nil, fmt.Errorf("failed to get todo: %w", err)
	}

	// Apply updates using individual update methods
	if update.Text != nil && *update.Text != "" {
		if err := s.queries.UpdateTodoText(ctx, sqlc.UpdateTodoTextParams{
			ID:   id,
			Text: *update.Text,
		}); err != nil {
			return nil, fmt.Errorf("failed to update todo text: %w", err)
		}
	}

	if update.Status != nil {
		if err := s.queries.UpdateTodoStatus(ctx, sqlc.UpdateTodoStatusParams{
			ID:     id,
			Status: TodoStatusToSqlc(*update.Status),
		}); err != nil {
			return nil, fmt.Errorf("failed to update todo status: %w", err)
		}
	}

	log.Infow("Todo updated successfully", "todoID", id)
	return s.GetTodo(ctx, id)
}

// DeleteTodo removes a todo item from the database (soft delete)
func (s *sqlcTodoStore) DeleteTodo(ctx context.Context, id string) error {
	log := logger.GetLogger()

	if err := s.queries.SoftDeleteTodo(ctx, id); err != nil {
		return fmt.Errorf("failed to soft-delete todo: %w", err)
	}

	log.Infow("Successfully soft-deleted todo", "todoID", id)
	return nil
}

// BeginTx starts a new database transaction
func (s *sqlcTodoStore) BeginTx(ctx context.Context) (types.DatabaseTransaction, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return &txWrapper{tx: tx}, nil
}
