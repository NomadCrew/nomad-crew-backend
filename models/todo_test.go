package models

import (
	"context"
	"testing"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockTodoStore struct {
	mock.Mock
}

func (m *MockTodoStore) CreateTodo(ctx context.Context, todo *types.Todo) error {
	args := m.Called(ctx, todo)
	return args.Error(0)
}

func (m *MockTodoStore) UpdateTodo(ctx context.Context, id string, update *types.TodoUpdate) error {
	args := m.Called(ctx, id, update)
	return args.Error(0)
}

func (m *MockTodoStore) ListTodos(ctx context.Context, tripID string) ([]*types.Todo, error) {
	args := m.Called(ctx, tripID)
	return args.Get(0).([]*types.Todo), args.Error(1)
}

func (m *MockTodoStore) DeleteTodo(ctx context.Context, id string, userID string) error {
	args := m.Called(ctx, id, userID)
	return args.Error(0)
}

func (m *MockTodoStore) GetTodo(ctx context.Context, id string) (*types.Todo, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Todo), args.Error(1)
}

func TestTodoModel_CreateTodo(t *testing.T) {
	mockStore := new(MockTodoStore)
	model := NewTodoModel(mockStore)
	ctx := context.Background()

	validTodo := &types.Todo{
		TripID:    "trip-123",
		Text:      "Buy supplies",
		CreatedBy: "user-456",
		Status:    types.TodoStatusIncomplete,
	}

	t.Run("successful creation", func(t *testing.T) {
		mockStore.On("CreateTodo", ctx, validTodo).Return(nil).Once()
		err := model.CreateTodo(ctx, validTodo)
		assert.NoError(t, err)
		mockStore.AssertExpectations(t)
	})

	t.Run("validation error", func(t *testing.T) {
		invalidTodo := &types.Todo{
			TripID:    "", // Missing required field
			Text:      "Invalid todo",
			CreatedBy: "user-456",
		}
		err := model.CreateTodo(ctx, invalidTodo)
		assert.Error(t, err)
		assert.IsType(t, &errors.AppError{}, err)
	})
}

func TestTodoModel_UpdateTodo(t *testing.T) {
	mockStore := new(MockTodoStore)
	model := NewTodoModel(mockStore)
	ctx := context.Background()

	t.Run("valid status update", func(t *testing.T) {
		update := &types.TodoUpdate{
			Status: types.TodoStatusComplete.Ptr(),
		}
		mockStore.On("UpdateTodo", ctx, "todo-123", update).Return(nil).Once()
		err := model.UpdateTodo(ctx, "todo-123", "user-456", update)
		assert.NoError(t, err)
		mockStore.AssertExpectations(t)
	})

	t.Run("invalid status update", func(t *testing.T) {
		invalidStatus := types.TodoStatus("INVALID")
		update := &types.TodoUpdate{
			Status: invalidStatus.Ptr(),
		}
		err := model.UpdateTodo(ctx, "todo-123", "user-456", update)
		assert.Error(t, err)
		assert.IsType(t, &errors.AppError{}, err)
	})
} 