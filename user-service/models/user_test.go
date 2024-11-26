// user-service/models/user_test.go
package models

import (
    "context"
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/NomadCrew/nomad-crew-backend/user-service/errors"
    "github.com/NomadCrew/nomad-crew-backend/user-service/types"
)

// MockUserStore implements store.UserStore for testing
type MockUserStore struct {
    mock.Mock
}

func (m *MockUserStore) SaveUser(ctx context.Context, user *types.User) error {
    args := m.Called(ctx, user)
    return args.Error(0)
}

func (m *MockUserStore) GetUserByID(ctx context.Context, id int64) (*types.User, error) {
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

func (m *MockUserStore) DeleteUser(ctx context.Context, id int64) error {
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

func TestUserModel_CreateUser(t *testing.T) {
    tests := []struct {
        name      string
        user      *types.User
        mockSetup func(*MockUserStore)
        wantErr   bool
    }{
        {
            name: "successful creation",
            user: &types.User{
                Username:     "testuser",
                Email:        "test@example.com",
                PasswordHash: "hashed_password",
            },
            mockSetup: func(m *MockUserStore) {
                m.On("SaveUser", mock.Anything, mock.AnythingOfType("*types.User")).Return(nil)
            },
            wantErr: false,
        },
        {
            name: "database error",
            user: &types.User{
                Username:     "testuser",
                Email:        "test@example.com",
                PasswordHash: "hashed_password",
            },
            mockSetup: func(m *MockUserStore) {
                m.On("SaveUser", mock.Anything, mock.AnythingOfType("*types.User")).Return(
                    errors.NewDatabaseError(errors.New(errors.DatabaseError, "db error", "")))
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockStore := new(MockUserStore)
            tt.mockSetup(mockStore)
            
            userModel := NewUserModel(mockStore)
            err := userModel.CreateUser(context.Background(), tt.user)

            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
            mockStore.AssertExpectations(t)
        })
    }
}

func TestUserModel_AuthenticateUser(t *testing.T) {
    tests := []struct {
        name       string
        email      string
        password   string
        storedUser *types.User
        mockSetup  func(*MockUserStore)
        wantErr    bool
    }{
        {
            name:     "successful authentication",
            email:    "test@example.com",
            password: "password123",
            storedUser: &types.User{
                Email:        "test@example.com",
                PasswordHash: "$2a$10$eqBvXy1.CScHtS2YBHneZOqVQH/yhLoBbhb9Dc9P4mD/YHbwPxM/y", // hashed "password123"
            },
            mockSetup: func(m *MockUserStore) {
                m.On("AuthenticateUser", mock.Anything, "test@example.com").Return(
                    &types.User{
                        Email:        "test@example.com",
                        PasswordHash: "$2a$10$eqBvXy1.CScHtS2YBHneZOqVQH/yhLoBbhb9Dc9P4mD/YHbwPxM/y",
                    }, nil)
            },
            wantErr: false,
        },
        {
            name:     "user not found",
            email:    "notfound@example.com",
            password: "password123",
            mockSetup: func(m *MockUserStore) {
                m.On("AuthenticateUser", mock.Anything, "notfound@example.com").Return(
                    nil, errors.NotFound("User", "notfound@example.com"))
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockStore := new(MockUserStore)
            tt.mockSetup(mockStore)
            
            userModel := NewUserModel(mockStore)
            user, err := userModel.AuthenticateUser(context.Background(), tt.email, tt.password)

            if tt.wantErr {
                assert.Error(t, err)
                assert.Nil(t, user)
            } else {
                assert.NoError(t, err)
                assert.NotNil(t, user)
                assert.Equal(t, tt.storedUser.Email, user.Email)
            }
            mockStore.AssertExpectations(t)
        })
    }
}