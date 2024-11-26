// user-service/models/trip_test.go
package models

import (
    "context"
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/jackc/pgx/v4/pgxpool"
    "github.com/NomadCrew/nomad-crew-backend/user-service/types"
)

type MockTripStore struct {
    mock.Mock
}

func (m *MockTripStore) GetPool() *pgxpool.Pool {
    args := m.Called()
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

func (m *MockTripStore) UpdateTrip(ctx context.Context, tripID int64, update types.TripUpdate) error {
    args := m.Called(ctx, tripID, update)
    return args.Error(0)
}

func (m *MockTripStore) SoftDeleteTrip(ctx context.Context, tripID int64) error {
    args := m.Called(ctx, tripID)
    return args.Error(0)
}

func TestTripModel_CreateTrip(t *testing.T) {
    now := time.Now()
    tests := []struct {
        name      string
        trip      *types.Trip
        mockSetup func(*MockTripStore)
        wantErr   bool
    }{
        {
            name: "successful creation",
            trip: &types.Trip{
                Name:        "Summer Trip",
                Description: "Beach vacation",
                StartDate:   now,
                EndDate:     now.Add(24 * time.Hour * 7),
                Destination: "Hawaii",
                CreatedBy:   1,
            },
            mockSetup: func(m *MockTripStore) {
                m.On("CreateTrip", mock.Anything, mock.AnythingOfType("types.Trip")).Return(int64(1), nil)
            },
            wantErr: false,
        },
        {
            name: "invalid dates",
            trip: &types.Trip{
                Name:        "Invalid Trip",
                StartDate:   now,
                EndDate:     now.Add(-24 * time.Hour), // End date before start date
                Destination: "Hawaii",
                CreatedBy:   1,
            },
            mockSetup: func(m *MockTripStore) {},
            wantErr:   true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockStore := new(MockTripStore)
            tt.mockSetup(mockStore)
            
            tripModel := NewTripModel(mockStore)
            err := tripModel.CreateTrip(context.Background(), tt.trip)

            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.NotZero(t, tt.trip.ID)
            }
            mockStore.AssertExpectations(t)
        })
    }
}

func TestTripModel_SearchTrips(t *testing.T) {
    now := time.Now()
    tests := []struct {
        name      string
        criteria  types.TripSearchCriteria
        mockSetup func(*MockTripStore)
        wantErr   bool
        wantLen   int
    }{
        {
            name: "search by destination",
            criteria: types.TripSearchCriteria{
                Destination: "Hawaii",
            },
            mockSetup: func(m *MockTripStore) {
                m.On("GetPool").Return(&pgxpool.Pool{})
            },
            wantErr: false,
            wantLen: 0,
        },
        {
            name: "search by date range",
            criteria: types.TripSearchCriteria{
                StartDateFrom: now,
                StartDateTo:   now.Add(24 * time.Hour * 30),
            },
            mockSetup: func(m *MockTripStore) {
                m.On("GetPool").Return(&pgxpool.Pool{})
            },
            wantErr: false,
            wantLen: 0,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockStore := new(MockTripStore)
            tt.mockSetup(mockStore)
            
            tripModel := NewTripModel(mockStore)
            trips, err := tripModel.SearchTrips(context.Background(), tt.criteria)

            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Len(t, trips, tt.wantLen)
            }
            mockStore.AssertExpectations(t)
        })
    }
}