package mocks

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/mock"
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

func (m *MockTripStore) CreateTrip(ctx context.Context, trip types.Trip) (string, error) {
	args := m.Called(ctx, trip)
	return args.Get(0).(string), args.Error(1)
}

func (m *MockTripStore) GetTrip(ctx context.Context, id string) (*types.Trip, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Trip), args.Error(1)
}

func (m *MockTripStore) UpdateTrip(ctx context.Context, id string, update types.TripUpdate) error {
	args := m.Called(ctx, id, update)
	return args.Error(0)
}

func (m *MockTripStore) SoftDeleteTrip(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTripStore) ListUserTrips(ctx context.Context, userid string) ([]*types.Trip, error) {
	args := m.Called(ctx, userid)
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

func (m *MockTripStore) AddMember(ctx context.Context, membership *types.TripMembership) error {
    args := m.Called(ctx, membership)
    return args.Error(0)
}

func (m *MockTripStore) UpdateMemberRole(ctx context.Context, tripID string, userID string, role types.MemberRole) error {
    args := m.Called(ctx, tripID, userID, role)
    return args.Error(0)
}

func (m *MockTripStore) RemoveMember(ctx context.Context, tripID string, userID string) error {
    args := m.Called(ctx, tripID, userID)
    return args.Error(0)
}

func (m *MockTripStore) GetTripMembers(ctx context.Context, tripID string) ([]types.TripMembership, error) {
    args := m.Called(ctx, tripID)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).([]types.TripMembership), args.Error(1)
}

func (m *MockTripStore) GetUserRole(ctx context.Context, tripID string, userID string) (types.MemberRole, error) {
    args := m.Called(ctx, tripID, userID)
    return args.Get(0).(types.MemberRole), args.Error(1)
}