package models

import (
	"context"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

// MockUserStore implements store.UserStore interface
type MockUserStore struct {
	mock.Mock
}

func (m *MockUserStore) GetPool() *pgxpool.Pool {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*pgxpool.Pool)
}

func (m *MockUserStore) SaveUser(ctx context.Context, user *types.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserStore) GetUserByID(ctx context.Context, id string) (*types.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}

func (m *MockUserStore) UpdateUser(ctx context.Context, user *types.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserStore) DeleteUser(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserStore) AuthenticateUser(ctx context.Context, email string) (*types.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}

// Verify interface compliance at compile time
var _ store.UserStore = (*MockUserStore)(nil)

func TestUserModel_CreateUser(t *testing.T) {
	mockStore := new(MockUserStore)
	userModel := NewUserModel(mockStore)
	ctx := context.Background()

	user := &types.User{
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: "hashedpassword",
		FirstName:    "Test",
		LastName:     "User",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	t.Run("successful creation", func(t *testing.T) {
		mockStore.On("SaveUser", ctx, user).Return(nil).Once()
		err := userModel.CreateUser(ctx, user)
		assert.NoError(t, err)
		mockStore.AssertExpectations(t)
	})

	t.Run("store error", func(t *testing.T) {
		mockStore.On("SaveUser", ctx, user).Return(errors.NewDatabaseError(assert.AnError)).Once()
		err := userModel.CreateUser(ctx, user)
		assert.Error(t, err)
		mockStore.AssertExpectations(t)
	})
}

func TestUserModel_GetUserByID(t *testing.T) {
	mockStore := new(MockUserStore)
	userModel := NewUserModel(mockStore)
	ctx := context.Background()

	expectedUser := &types.User{
		ID:       1,
		Username: "testuser",
		Email:    "test@example.com",
	}

	t.Run("successful retrieval", func(t *testing.T) {
		mockStore.On("GetUserByID", ctx, string(1)).Return(expectedUser, nil).Once()
		user, err := userModel.GetUserByID(ctx, 1)
		assert.NoError(t, err)
		assert.Equal(t, expectedUser, user)
		mockStore.AssertExpectations(t)
	})

	t.Run("user not found", func(t *testing.T) {
		mockStore.On("GetUserByID", ctx, string(999)).Return(nil, errors.NotFound("User", 999)).Once()
		user, err := userModel.GetUserByID(ctx, 999)
		assert.Error(t, err)
		assert.Nil(t, user)
		mockStore.AssertExpectations(t)
	})
}

func TestUserModel_AuthenticateUser(t *testing.T) {
	mockStore := new(MockUserStore)
	userModel := NewUserModel(mockStore)
	ctx := context.Background()

	password := "testpassword"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	user := &types.User{
		ID:           1,
		Email:        "test@example.com",
		PasswordHash: string(hashedPassword),
	}

	t.Run("successful authentication", func(t *testing.T) {
		mockStore.On("AuthenticateUser", ctx, user.Email).Return(user, nil).Once()
		authenticatedUser, err := userModel.AuthenticateUser(ctx, user.Email, password)
		assert.NoError(t, err)
		assert.Equal(t, user.ID, authenticatedUser.ID)
		mockStore.AssertExpectations(t)
	})

	t.Run("invalid password", func(t *testing.T) {
		mockStore.On("AuthenticateUser", ctx, user.Email).Return(user, nil).Once()
		_, err := userModel.AuthenticateUser(ctx, user.Email, "wrongpassword")
		assert.Error(t, err)
		mockStore.AssertExpectations(t)
	})

	t.Run("user not found", func(t *testing.T) {
		mockStore.On("AuthenticateUser", ctx, "nonexistent@example.com").Return(nil, errors.NotFound("User", 0)).Once()
		_, err := userModel.AuthenticateUser(ctx, "nonexistent@example.com", password)
		assert.Error(t, err)
		mockStore.AssertExpectations(t)
	})
}

func TestGenerateJWT(t *testing.T) {
	user := &types.User{
		ID:       1,
		Username: "testuser",
		Email:    "test@example.com",
	}

	t.Run("successful token generation", func(t *testing.T) {
		t.Setenv("JWT_SECRET_KEY", "test-secret")
		token, err := GenerateJWT(user)
		assert.NoError(t, err)
		assert.NotEmpty(t, token)
	})

	t.Run("missing secret key", func(t *testing.T) {
		t.Setenv("JWT_SECRET_KEY", "")
		token, err := GenerateJWT(user)
		assert.Error(t, err)
		assert.Empty(t, token)
		assert.Equal(t, errors.ServerError, err.(*errors.AppError).Type)
	})
}
