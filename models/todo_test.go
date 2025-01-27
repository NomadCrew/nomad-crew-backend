package models

import (
	"context"
	"testing"
	"errors"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/NomadCrew/nomad-crew-backend/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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

func (m *MockTodoStore) ListTodos(ctx context.Context, tripID string, limit int, offset int) ([]*types.Todo, int, error) {
	args := m.Called(ctx, tripID, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*types.Todo), args.Int(1), args.Error(2)
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
	mockTripStore := new(mocks.MockTripStore)
	tripModel := NewTripModel(mockTripStore)
	model := NewTodoModel(mockStore, tripModel)
	ctx := context.Background()

	validTodo := &types.Todo{
		TripID:    "trip-123",
		Text:      "Buy supplies",
		CreatedBy: "user-456",
		Status:    types.TodoStatusIncomplete,
	}

	t.Run("successful creation", func(t *testing.T) {
		mockTripStore.On("GetUserRole", ctx, validTodo.TripID, validTodo.CreatedBy).Return(types.MemberRoleMember, nil).Once()
		mockStore.On("CreateTodo", ctx, validTodo).Return(nil).Once()
		
		err := model.CreateTodo(ctx, validTodo)
		assert.NoError(t, err)
		mockStore.AssertExpectations(t)
		mockTripStore.AssertExpectations(t)
	})

	t.Run("validation error", func(t *testing.T) {
		invalidTodo := &types.Todo{
			TripID:    "", // Missing required field
			Text:      "Invalid todo",
			CreatedBy: "user-456",
		}
		err := model.CreateTodo(ctx, invalidTodo)
		assert.Error(t, err)
		assert.IsType(t, &apperrors.AppError{}, err)
	})
}

func TestTodoModel_ListTripTodos(t *testing.T) {
	mockStore := new(MockTodoStore)
	mockTripStore := new(mocks.MockTripStore)
	tripModel := NewTripModel(mockTripStore)
	model := NewTodoModel(mockStore, tripModel)
	ctx := context.Background()

	tripID := "test-trip-id"
	userID := "test-user-id"
	limit := 10
	offset := 0

	todos := []*types.Todo{
		{
			ID:        "todo-1",
			TripID:    tripID,
			Text:      "Test Todo 1",
			CreatedBy: userID,
		},
		{
			ID:        "todo-2",
			TripID:    tripID,
			Text:      "Test Todo 2",
			CreatedBy: userID,
		},
	}
	totalCount := 2

	// Mock trip membership verification
	mockTripStore.On("GetUserRole", ctx, tripID, userID).Return(types.MemberRoleMember, nil)

	// Mock todo listing with pagination
	mockStore.On("ListTodos", ctx, tripID, limit, offset).Return(todos, totalCount, nil)

	response, err := model.ListTripTodos(ctx, tripID, userID, limit, offset)
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, todos, response.Data)
	assert.Equal(t, totalCount, response.Pagination.Total)
	assert.Equal(t, limit, response.Pagination.Limit)
	assert.Equal(t, offset, response.Pagination.Offset)

	mockStore.AssertExpectations(t)
	mockTripStore.AssertExpectations(t)
}

func TestTodoModel_ListTripTodos_Error(t *testing.T) {
    t.Run("unauthorized access", func(t *testing.T) {
        mockStore := new(MockTodoStore)
        mockTripStore := new(mocks.MockTripStore)
        tripModel := NewTripModel(mockTripStore)
        model := NewTodoModel(mockStore, tripModel)
        ctx := context.Background()

        tripID := "test-trip-id"
        userID := "test-user-id"
        limit := 10
        offset := 0

        mockTripStore.On("GetUserRole", ctx, tripID, userID).Return(types.MemberRole(""), errors.New("unauthorized"))

        response, err := model.ListTripTodos(ctx, tripID, userID, limit, offset)
        require.Error(t, err)
        assert.Nil(t, response)

        mockTripStore.AssertExpectations(t)
    })

    t.Run("database error", func(t *testing.T) {
        mockStore := new(MockTodoStore)
        mockTripStore := new(mocks.MockTripStore)
        tripModel := NewTripModel(mockTripStore)
        model := NewTodoModel(mockStore, tripModel)
        ctx := context.Background()

        tripID := "test-trip-id"
        userID := "test-user-id"
        limit := 10
        offset := 0

        mockTripStore.On("GetUserRole", ctx, tripID, userID).Return(types.MemberRoleMember, nil)
        mockStore.On("ListTodos", ctx, tripID, limit, offset).Return(nil, 0, errors.New("database error"))

        response, err := model.ListTripTodos(ctx, tripID, userID, limit, offset)
        require.Error(t, err)
        assert.Nil(t, response)

        mockStore.AssertExpectations(t)
        mockTripStore.AssertExpectations(t)
    })
}