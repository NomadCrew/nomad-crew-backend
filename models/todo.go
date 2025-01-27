package models

import (
    "context"
    "fmt"
    "strings"

    "github.com/NomadCrew/nomad-crew-backend/errors"
    "github.com/NomadCrew/nomad-crew-backend/internal/store"
    "github.com/NomadCrew/nomad-crew-backend/logger"
    "github.com/NomadCrew/nomad-crew-backend/types"
)

type TodoModel struct {
    store store.TodoStore
    tripModel *TripModel
}

func NewTodoModel(store store.TodoStore, tripModel *TripModel) *TodoModel {
    return &TodoModel{
        store: store,
        tripModel: tripModel,
    }
}

func (tm *TodoModel) CreateTodo(ctx context.Context, todo *types.Todo) error {
    log := logger.GetLogger()

    if err := validateTodo(todo); err != nil {
        return err
    }

    // Verify trip exists and user is a member
    if err := tm.verifyTripMembership(ctx, todo.TripID, todo.CreatedBy); err != nil {
        return err
    }

    if err := tm.store.CreateTodo(ctx, todo); err != nil {
        log.Errorw("Failed to create todo", 
            "tripId", todo.TripID,
            "error", err,
        )
        return errors.NewDatabaseError(err)
    }

    return nil
}

func (tm *TodoModel) UpdateTodo(ctx context.Context, id string, userID string, update *types.TodoUpdate) error {
    log := logger.GetLogger()

    // Verify todo exists and user is creator
    todo, err := tm.store.GetTodo(ctx, id)
    if err != nil {
        return err
    }

    if todo.CreatedBy != userID {
        return errors.ValidationFailed(
            "unauthorized",
            "only creator can update todo",
        )
    }

    if err := validateTodoUpdate(update); err != nil {
        return err
    }

    if err := tm.store.UpdateTodo(ctx, id, update); err != nil {
        log.Errorw("Failed to update todo",
            "todoId", id,
            "error", err,
        )
        return err
    }

    return nil
}

func (tm *TodoModel) DeleteTodo(ctx context.Context, id string, userID string) error {
    return tm.store.DeleteTodo(ctx, id, userID)
}

func (tm *TodoModel) ListTripTodos(ctx context.Context, tripID string, userID string, limit int, offset int) (*types.PaginatedResponse, error) {
    // First verify trip access
    if err := tm.verifyTripMembership(ctx, tripID, userID); err != nil {
        return nil, err
    }

    todos, total, err := tm.store.ListTodos(ctx, tripID, limit, offset)
    if err != nil {
        return nil, err
    }

    return &types.PaginatedResponse{
        Data: todos,
        Pagination: types.Pagination{
            Limit:  limit,
            Offset: offset,
            Total:  total,
        },
    }, nil
}

func (tm *TodoModel) GetTodo(ctx context.Context, id string) (*types.Todo, error) {
    todo, err := tm.store.GetTodo(ctx, id)
    if err != nil {
        return nil, errors.NewDatabaseError(err)
    }
    return todo, nil
}

// Helper functions
func validateTodo(todo *types.Todo) error {
    var validationErrors []string

    if todo.TripID == "" {
        validationErrors = append(validationErrors, "trip ID is required")
    }

    if todo.Text == "" {
        validationErrors = append(validationErrors, "todo text is required")
    }

    if len(todo.Text) > 255 {
        validationErrors = append(validationErrors, "todo text exceeds 255 characters")
    }

    if todo.CreatedBy == "" {
        validationErrors = append(validationErrors, "creator ID is required")
    }

    if len(validationErrors) > 0 {
        return errors.ValidationFailed(
            "Invalid todo data",
            strings.Join(validationErrors, "; "),
        )
    }

    return nil
}

func validateTodoUpdate(update *types.TodoUpdate) error {
    if update.Text != nil && *update.Text == "" {
        return errors.ValidationFailed(
            "Invalid update",
            "todo text cannot be empty",
        )
    }

    if update.Text != nil && len(*update.Text) > 255 {
        return errors.ValidationFailed(
            "Invalid update",
            "todo text exceeds 255 characters",
        )
    }

    if update.Status != nil && !isValidTodoStatus(*update.Status) {
        return errors.ValidationFailed(
            "Invalid update",
            fmt.Sprintf("invalid status: %s", *update.Status),
        )
    }

    return nil
}

func (tm *TodoModel) verifyTripMembership(ctx context.Context, tripID string, userID string) error {
    role, err := tm.tripModel.GetUserRole(ctx, tripID, userID)
    if err != nil {
        return err
    }
    if role == "" {
        return errors.ValidationFailed(
            "unauthorized",
            "user is not a trip member",
        )
    }
    return nil
}

func isValidTodoStatus(status types.TodoStatus) bool {
    return status == types.TodoStatusComplete || status == types.TodoStatusIncomplete
}