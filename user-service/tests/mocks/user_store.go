// user-service/tests/mocks/user_store.go
package mocks

import (
	"context"
	"github.com/stretchr/testify/mock"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/NomadCrew/nomad-crew-backend/user-service/types"
)

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

func (m *MockUserStore) GetUserByID(ctx context.Context, id int64) (*types.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}

func (m *MockUserStore) SaveUser(ctx context.Context, user *types.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
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
