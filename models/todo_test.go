package models

import (
	"context"
	"errors"
	"testing"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/models/trip"
	"github.com/NomadCrew/nomad-crew-backend/tests/mocks"
	"github.com/NomadCrew/nomad-crew-backend/types"
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

// TripModelAdapter adapts TripModelFacade to trip.TripModel for testing
type TripModelAdapter struct {
	facade *TripModelFacade
	store  *mocks.MockTripStore
}

func NewTripModelAdapter(facade *TripModelFacade) *trip.TripModel {
	// This is a hack for testing purposes only - we're creating a mock adapter
	// that will be used in tests but doesn't actually call the real implementation
	return &trip.TripModel{}
}

// MockTripModel is a mock implementation of trip.TripModel for testing
type MockTripModel struct {
	mock.Mock
}

// GetUserRole is a mock implementation of the GetUserRole method
func (m *MockTripModel) GetUserRole(ctx context.Context, tripID string, userID string) (types.MemberRole, error) {
	args := m.Called(ctx, tripID, userID)
	return args.Get(0).(types.MemberRole), args.Error(1)
}

func TestTodoModel_CreateTodo(t *testing.T) {
	mockStore := new(MockTodoStore)
	mockTripModel := new(MockTripModel)
	model := NewTodoModel(mockStore, mockTripModel)
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
		mockStore.On("CreateTodo", ctx, validTodo).Return(nil).Once()

		err := model.CreateTodo(ctx, validTodo)
		assert.NoError(t, err)
		mockStore.AssertExpectations(t)
		mockTripModel.AssertExpectations(t)
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
	mockTripModel := new(MockTripModel)
	model := NewTodoModel(mockStore, mockTripModel)
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
	mockStore.On("ListTodos", ctx, tripID, limit, offset).Return(todos, totalCount, nil)

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
		model := NewTodoModel(mockStore, mockTripModel)
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
		model := NewTodoModel(mockStore, mockTripModel)
		ctx := context.Background()

		tripID := "test-trip-id"
		userID := "test-user-id"
		limit := 10
		offset := 0

		mockTripModel.On("GetUserRole", ctx, tripID, userID).Return(types.MemberRoleMember, nil)
		mockStore.On("ListTodos", ctx, tripID, limit, offset).Return(nil, 0, errors.New("database error"))

		response, err := model.ListTripTodos(ctx, tripID, userID, limit, offset)
		require.Error(t, err)
		assert.Nil(t, response)

		mockStore.AssertExpectations(t)
		mockTripModel.AssertExpectations(t)
	})
}
