package models

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/internal/utils"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

// TripModelInterface defines the interface for trip model methods used by TodoModel
type TripModelInterface interface {
	GetUserRole(ctx context.Context, tripID, userID string) (types.MemberRole, error)
}

// EventPublisherInterface defines the interface for publishing events
type EventPublisherInterface interface {
	Publish(ctx context.Context, tripID string, event types.Event) error
	PublishBatch(ctx context.Context, tripID string, events []types.Event) error
	Subscribe(ctx context.Context, tripID string, userID string, filters ...types.EventType) (<-chan types.Event, error)
	Unsubscribe(ctx context.Context, tripID string, userID string) error
}

type TodoModel struct {
	store          store.TodoStore
	tripModel      TripModelInterface
	eventPublisher EventPublisherInterface
}

func NewTodoModel(store store.TodoStore, tripModel TripModelInterface, eventPublisher EventPublisherInterface) *TodoModel {
	return &TodoModel{
		store:          store,
		tripModel:      tripModel,
		eventPublisher: eventPublisher,
	}
}

// CreateTodo inserts a new todo item.
// It validates the input, verifies trip membership, and uses the store to create the todo.
// Returns the ID of the created todo and any error encountered.
func (tm *TodoModel) CreateTodo(ctx context.Context, todo *types.Todo) (string, error) {
	log := logger.GetLogger()

	if err := validateTodo(todo); err != nil {
		return "", err
	}

	// Verify trip exists and user is a member
	if err := tm.verifyTripMembership(ctx, todo.TripID, todo.CreatedBy); err != nil {
		return "", err
	}

	// Correctly handle the returned ID and error
	newTodoID, err := tm.store.CreateTodo(ctx, todo)
	if err != nil {
		log.Errorw("Failed to create todo",
			"tripId", todo.TripID,
			"error", err,
		)
		return "", errors.NewDatabaseError(err)
	}
	// Assign the ID back to the input object for the caller
	todo.ID = newTodoID

	return newTodoID, nil
}

// CreateTodoWithEvent creates a todo and publishes a creation event
func (tm *TodoModel) CreateTodoWithEvent(ctx context.Context, todo *types.Todo) (string, error) {
	log := logger.GetLogger()

	// Create the todo
	todoID, err := tm.CreateTodo(ctx, todo)
	if err != nil {
		return "", err
	}

	// Ensure ID is set
	if todo.ID == "" {
		todo.ID = todoID
	}

	// Convert todo to payload map
	payload, err := tm.convertTodoToPayload(todo)
	if err != nil {
		log.Warnw("Failed to convert todo to payload", "error", err, "todoID", todo.ID)
		// Continue even if we couldn't convert the payload - todo was created successfully
		return todoID, nil
	}

	// Publish event
	if err := tm.publishTodoEvent(ctx, types.EventTypeTodoCreated, todo.TripID, todo.CreatedBy, payload); err != nil {
		log.Warnw("Failed to publish todo created event", "error", err, "todoID", todo.ID)
		// Continue even if we couldn't publish the event - todo was created successfully
	}

	return todoID, nil
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

	// Correctly handle the returned Todo and error
	updatedTodo, err := tm.store.UpdateTodo(ctx, id, update)
	if err != nil {
		log.Errorw("Failed to update todo",
			"todoId", id,
			"error", err,
		)
		return err // Propagate the error (might be AppError already)
	}

	// Use the updatedTodo if needed
	_ = updatedTodo

	return nil
}

// UpdateTodoWithEvent updates a todo and publishes an update event
func (tm *TodoModel) UpdateTodoWithEvent(ctx context.Context, id string, userID string, update *types.TodoUpdate) (*types.Todo, error) {
	log := logger.GetLogger()

	// Update the todo
	if err := tm.UpdateTodo(ctx, id, userID, update); err != nil {
		return nil, err
	}

	// Get the updated todo
	todo, err := tm.GetTodo(ctx, id)
	if err != nil {
		log.Warnw("Failed to retrieve updated todo for event", "error", err, "todoID", id)
		return nil, err
	}

	// Convert todo to payload map
	payload, err := tm.convertTodoToPayload(todo)
	if err != nil {
		log.Warnw("Failed to convert todo to payload", "error", err, "todoID", id)
		// Continue even if we couldn't convert the payload - todo was updated successfully
		return todo, nil
	}

	// Publish event
	if err := tm.publishTodoEvent(ctx, types.EventTypeTodoUpdated, todo.TripID, userID, payload); err != nil {
		log.Warnw("Failed to publish todo updated event", "error", err, "todoID", id)
		// Continue even if we couldn't publish the event - todo was updated successfully
	}

	return todo, nil
}

func (tm *TodoModel) DeleteTodo(ctx context.Context, id string, userID string) error {
	// First, check if the user is the creator before allowing deletion
	todo, err := tm.store.GetTodo(ctx, id)
	if err != nil {
		// Handle not found or other errors from GetTodo
		return err
	}

	if todo.CreatedBy != userID {
		return errors.ValidationFailed(
			"unauthorized",
			"only creator can delete todo",
		)
	}

	// Call DeleteTodo with the correct signature (ctx, id)
	return tm.store.DeleteTodo(ctx, id)
}

// DeleteTodoWithEvent deletes a todo and publishes a deletion event
func (tm *TodoModel) DeleteTodoWithEvent(ctx context.Context, id string, userID string) error {
	log := logger.GetLogger()

	// Get the todo before deletion for the event
	todo, err := tm.GetTodo(ctx, id)
	if err != nil {
		return err
	}

	// Store trip ID for later use in event
	tripID := todo.TripID

	// Delete the todo
	if err := tm.DeleteTodo(ctx, id, userID); err != nil {
		return err
	}

	// Publish event with minimal payload
	payload := map[string]interface{}{"id": id}
	if err := tm.publishTodoEvent(ctx, types.EventTypeTodoDeleted, tripID, userID, payload); err != nil {
		log.Warnw("Failed to publish todo deleted event", "error", err, "todoID", id)
		// Continue even if we couldn't publish the event - todo was deleted successfully
	}

	return nil
}

func (tm *TodoModel) ListTripTodos(ctx context.Context, tripID string, userID string, limit int, offset int) (*types.PaginatedResponse, error) {
	// First verify trip access
	if err := tm.verifyTripMembership(ctx, tripID, userID); err != nil {
		return nil, err
	}

	// Call ListTodos with the correct signature (ctx, tripID)
	todos, err := tm.store.ListTodos(ctx, tripID)
	if err != nil {
		return nil, err
	}

	// Manual pagination after fetching all todos
	total := len(todos)
	start := offset
	end := offset + limit

	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	pagedTodos := []*types.Todo{}
	if start < end {
		pagedTodos = todos[start:end]
	}

	return &types.PaginatedResponse{
		Items: pagedTodos, // Return the manually paginated slice
		Pagination: &types.PageInfo{
			Page:       (offset / limit) + 1,
			PerPage:    limit,
			Total:      int64(total),
			TotalPages: (total + limit - 1) / limit,
			HasMore:    end < total,
		},
	}, nil
}

// ValidateAndNormalizeListParams validates and normalizes list parameters
func (tm *TodoModel) ValidateAndNormalizeListParams(params *types.ListTodosParams) {
	// Validate and adjust limits
	if params.Limit <= 0 || params.Limit > 100 {
		params.Limit = 20
	}
	if params.Offset < 0 {
		params.Offset = 0
	}
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

// Event handling helpers

// convertTodoToPayload converts a todo to a map for event payload
func (tm *TodoModel) convertTodoToPayload(todo *types.Todo) (map[string]interface{}, error) {
	var payloadMap map[string]interface{}
	todoJSON, err := json.Marshal(todo)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(todoJSON, &payloadMap); err != nil {
		return nil, err
	}

	return payloadMap, nil
}

// publishTodoEvent publishes a todo-related event
func (tm *TodoModel) publishTodoEvent(ctx context.Context, eventType types.EventType, tripID string, userID string, payload map[string]interface{}) error {
	// Convert payload to JSON
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create event
	event := types.Event{
		BaseEvent: types.BaseEvent{
			ID:        utils.GenerateEventID(),
			Type:      eventType,
			TripID:    tripID,
			UserID:    userID,
			Timestamp: time.Now(),
			Version:   1,
		},
		Metadata: types.EventMetadata{
			Source: "todo_model",
		},
		Payload: payloadJSON,
	}

	// Publish event directly using the new interface
	return tm.eventPublisher.Publish(ctx, tripID, event)
}
