package models_test

import (
	"context"
	"errors"
	"testing"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/NomadCrew/nomad-crew-backend/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockTodoStore struct {
	mock.Mock
}

// Verify that MockTodoStore implements store.TodoStore
var _ store.TodoStore = (*MockTodoStore)(nil)

// BeginTx starts a new database transaction (mock)
func (m *MockTodoStore) BeginTx(ctx context.Context) (store.Transaction, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(store.Transaction), args.Error(1)
}

func (m *MockTodoStore) CreateTodo(ctx context.Context, todo *types.Todo) (string, error) {
	args := m.Called(ctx, todo)
	if len(args) == 1 {
		// Handle legacy mock calls that only return error
		return "", args.Error(0)
	}
	return args.String(0), args.Error(1)
}

func (m *MockTodoStore) UpdateTodo(ctx context.Context, id string, update *types.TodoUpdate) (*types.Todo, error) {
	args := m.Called(ctx, id, update)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Todo), args.Error(1)
}

func (m *MockTodoStore) ListTodos(ctx context.Context, tripID string) ([]*types.Todo, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Todo), args.Error(1)
}

func (m *MockTodoStore) DeleteTodo(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTodoStore) GetTodo(ctx context.Context, id string) (*types.Todo, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Todo), args.Error(1)
}

// MockTripModel is a mock implementation of models.TripModel for testing
type MockTripModel struct {
	mock.Mock
}

// GetUserRole is a mock implementation of the GetUserRole method
func (m *MockTripModel) GetUserRole(ctx context.Context, tripID string, userID string) (types.MemberRole, error) {
	args := m.Called(ctx, tripID, userID)
	return args.Get(0).(types.MemberRole), args.Error(1)
}

// MockEventPublisher is a mock implementation of models.EventPublisherInterface
type MockEventPublisher struct {
	mock.Mock
}

// Verify that MockEventPublisher implements models.EventPublisherInterface
var _ models.EventPublisherInterface = (*MockEventPublisher)(nil)

func (m *MockEventPublisher) Publish(ctx context.Context, tripID string, event types.Event) error {
	args := m.Called(ctx, tripID, event)
	return args.Error(0)
}

func (m *MockEventPublisher) PublishBatch(ctx context.Context, tripID string, events []types.Event) error {
	args := m.Called(ctx, tripID, events)
	return args.Error(0)
}

func (m *MockEventPublisher) Subscribe(ctx context.Context, tripID string, userID string, filters ...types.EventType) (<-chan types.Event, error) {
	args := m.Called(ctx, tripID, userID, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(<-chan types.Event), args.Error(1)
}

func (m *MockEventPublisher) Unsubscribe(ctx context.Context, tripID string, userID string) error {
	args := m.Called(ctx, tripID, userID)
	return args.Error(0)
}

func TestTodoModel_CreateTodo(t *testing.T) {
	mockStore := new(MockTodoStore)
	mockTripModel := new(MockTripModel)
	mockEventPublisher := new(MockEventPublisher)
	model := models.NewTodoModel(mockStore, mockTripModel, mockEventPublisher)
	ctx := context.Background()

	validTodo := &types.Todo{
		TripID:    "trip-123",
		Text:      "Buy supplies",
		CreatedBy: "user-456",
	}

	t.Run("successful creation", func(t *testing.T) {
		// Mock the trip membership verification
		mockTripModel.On("GetUserRole", ctx, validTodo.TripID, validTodo.CreatedBy).Return(types.MemberRoleMember, nil).Once()

		// Mock the todo creation
		mockStore.On("CreateTodo", ctx, validTodo).Return("todo-123", nil).Once()

		todoID, err := model.CreateTodo(ctx, validTodo)
		assert.NoError(t, err)
		assert.Equal(t, "todo-123", todoID)
		mockStore.AssertExpectations(t)
		mockTripModel.AssertExpectations(t)
	})

	t.Run("validation error", func(t *testing.T) {
		invalidTodo := &types.Todo{
			TripID:    "", // Missing required field
			Text:      "Invalid todo",
			CreatedBy: "user-456",
		}
		todoID, err := model.CreateTodo(ctx, invalidTodo)
		assert.Error(t, err)
		assert.Empty(t, todoID)
		assert.IsType(t, &apperrors.AppError{}, err)
	})
}

func TestTodoModel_ListTripTodos(t *testing.T) {
	mockStore := new(MockTodoStore)
	mockTripModel := new(MockTripModel)
	mockEventPublisher := new(MockEventPublisher)
	model := models.NewTodoModel(mockStore, mockTripModel, mockEventPublisher)
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
	mockTripModel.On("GetUserRole", ctx, tripID, userID).Return(types.MemberRoleMember, nil)

	// Mock todo listing with pagination
	mockStore.On("ListTodos", ctx, tripID).Return(todos, nil)

	response, err := model.ListTripTodos(ctx, tripID, userID, limit, offset)
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, todos, response.Data)
	assert.Equal(t, totalCount, response.Pagination.Total)
	assert.Equal(t, limit, response.Pagination.Limit)
	assert.Equal(t, offset, response.Pagination.Offset)

	mockStore.AssertExpectations(t)
	mockTripModel.AssertExpectations(t)
}

func TestTodoModel_ListTripTodos_Error(t *testing.T) {
	t.Run("unauthorized access", func(t *testing.T) {
		mockStore := new(MockTodoStore)
		mockTripModel := new(MockTripModel)
		mockEventPublisher := new(MockEventPublisher)
		model := models.NewTodoModel(mockStore, mockTripModel, mockEventPublisher)
		ctx := context.Background()

		tripID := "test-trip-id"
		userID := "test-user-id"
		limit := 10
		offset := 0

		mockTripModel.On("GetUserRole", ctx, tripID, userID).Return(types.MemberRole(""), errors.New("unauthorized"))

		response, err := model.ListTripTodos(ctx, tripID, userID, limit, offset)
		require.Error(t, err)
		assert.Nil(t, response)

		mockTripModel.AssertExpectations(t)
	})

	t.Run("database error", func(t *testing.T) {
		mockStore := new(MockTodoStore)
		mockTripModel := new(MockTripModel)
		mockEventPublisher := new(MockEventPublisher)
		model := models.NewTodoModel(mockStore, mockTripModel, mockEventPublisher)
		ctx := context.Background()

		tripID := "test-trip-id"
		userID := "test-user-id"
		limit := 10
		offset := 0

		mockTripModel.On("GetUserRole", ctx, tripID, userID).Return(types.MemberRoleMember, nil)
		mockStore.On("ListTodos", ctx, tripID).Return(nil, errors.New("database error"))

		response, err := model.ListTripTodos(ctx, tripID, userID, limit, offset)
		require.Error(t, err)
		assert.Nil(t, response)

		mockStore.AssertExpectations(t)
		mockTripModel.AssertExpectations(t)
	})
}
