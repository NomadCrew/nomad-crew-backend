package mocks

import (
    "context"
    "github.com/jackc/pgx/v4/pgxpool"
    "github.com/stretchr/testify/mock"
    "github.com/NomadCrew/nomad-crew-backend/types"
)

type MockTripStore struct {
    mock.Mock
}

func (m *MockTripStore) GetPool() *pgxpool.Pool {
    args := m.Called()
    if args.Get(0) == nil {
        return nil
    }
    return args.Get(0).(*pgxpool.Pool)
}

func (m *MockTripStore) CreateTrip(ctx context.Context, trip types.Trip) (int64, error) {
    args := m.Called(ctx, trip)
    return args.Get(0).(int64), args.Error(1)
}

func (m *MockTripStore) GetTrip(ctx context.Context, id int64) (*types.Trip, error) {
    args := m.Called(ctx, id)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*types.Trip), args.Error(1)
}

func (m *MockTripStore) UpdateTrip(ctx context.Context, id int64, update types.TripUpdate) error {
    args := m.Called(ctx, id, update)
    return args.Error(0)
}

func (m *MockTripStore) SoftDeleteTrip(ctx context.Context, id int64) error {
    args := m.Called(ctx, id)
    return args.Error(0)
}

func (m *MockTripStore) ListUserTrips(ctx context.Context, userID int64) ([]*types.Trip, error) {
    args := m.Called(ctx, userID)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).([]*types.Trip), args.Error(1)
}

func (m *MockTripStore) SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error) {
    args := m.Called(ctx, criteria)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).([]*types.Trip), args.Error(1)
}