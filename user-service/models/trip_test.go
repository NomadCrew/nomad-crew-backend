package models

import (
    "context"
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/jackc/pgx/v4/pgxpool"
    "github.com/NomadCrew/nomad-crew-backend/user-service/types"
    "github.com/NomadCrew/nomad-crew-backend/user-service/errors"
    "github.com/NomadCrew/nomad-crew-backend/user-service/internal/store"
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

func (m *MockTripStore) UpdateTrip(ctx context.Context, tripID int64, update types.TripUpdate) error {
    args := m.Called(ctx, tripID, update)
    return args.Error(0)
}

func (m *MockTripStore) SoftDeleteTrip(ctx context.Context, tripID int64) error {
    args := m.Called(ctx, tripID)
    return args.Error(0)
}

// Verify interface compliance at compile time
var _ store.TripStore = (*MockTripStore)(nil)

func TestTripModel_CreateTrip(t *testing.T) {
    mockStore := new(MockTripStore)
    mockStore.On("GetPool").Return(nil)
    tripModel := NewTripModel(mockStore)
    ctx := context.Background()

    validTrip := &types.Trip{
        Name:        "Test Trip",
        Description: "Test Description",
        Destination: "Test Destination",
        StartDate:   time.Now().Add(24 * time.Hour),
        EndDate:     time.Now().Add(48 * time.Hour),
        CreatedBy:   1,
    }

    t.Run("successful creation", func(t *testing.T) {
        mockStore.On("CreateTrip", ctx, *validTrip).Return(int64(1), nil).Once()
        err := tripModel.CreateTrip(ctx, validTrip)
        assert.NoError(t, err)
        assert.Equal(t, int64(1), validTrip.ID)
        mockStore.AssertExpectations(t)
    })

    t.Run("validation error - missing name", func(t *testing.T) {
        invalidTrip := *validTrip
        invalidTrip.Name = ""
        err := tripModel.CreateTrip(ctx, &invalidTrip)
        assert.Error(t, err)
        assert.Equal(t, errors.ValidationError, err.(*errors.AppError).Type)
    })

    t.Run("validation error - end date before start date", func(t *testing.T) {
        invalidTrip := *validTrip
        invalidTrip.StartDate = time.Now().Add(48 * time.Hour)
        invalidTrip.EndDate = time.Now().Add(24 * time.Hour)
        err := tripModel.CreateTrip(ctx, &invalidTrip)
        assert.Error(t, err)
        assert.Equal(t, errors.ValidationError, err.(*errors.AppError).Type)
    })

    t.Run("store error", func(t *testing.T) {
        mockStore.On("CreateTrip", ctx, *validTrip).Return(int64(0), errors.NewDatabaseError(assert.AnError)).Once()
        err := tripModel.CreateTrip(ctx, validTrip)
        assert.Error(t, err)
        assert.Equal(t, errors.DatabaseError, err.(*errors.AppError).Type)
        mockStore.AssertExpectations(t)
    })
}

func TestTripModel_GetTripByID(t *testing.T) {
    mockStore := new(MockTripStore)
    mockStore.On("GetPool").Return(nil)
    tripModel := NewTripModel(mockStore)
    ctx := context.Background()

    expectedTrip := &types.Trip{
        ID:          1,
        Name:        "Test Trip",
        Description: "Test Description",
        Destination: "Test Destination",
        StartDate:   time.Now().Add(24 * time.Hour),
        EndDate:     time.Now().Add(48 * time.Hour),
        CreatedBy:   1,
    }

    t.Run("successful retrieval", func(t *testing.T) {
        mockStore.On("GetTrip", ctx, int64(1)).Return(expectedTrip, nil).Once()
        trip, err := tripModel.GetTripByID(ctx, 1)
        assert.NoError(t, err)
        assert.Equal(t, expectedTrip, trip)
        mockStore.AssertExpectations(t)
    })

    t.Run("not found", func(t *testing.T) {
        mockStore.On("GetTrip", ctx, int64(999)).Return(nil, errors.NotFound("Trip", 999)).Once()
        trip, err := tripModel.GetTripByID(ctx, 999)
        assert.Error(t, err)
        assert.Nil(t, trip)
        assert.Equal(t, errors.NotFoundError, err.(*errors.AppError).Type)
        mockStore.AssertExpectations(t)
    })
}

func TestTripModel_UpdateTrip(t *testing.T) {
    mockStore := new(MockTripStore)
    mockStore.On("GetPool").Return(nil)
    tripModel := NewTripModel(mockStore)
    ctx := context.Background()

    update := &types.TripUpdate{
        Name:        "Updated Trip",
        Description: "Updated Description",
        Destination: "Updated Destination",
        StartDate:   time.Now().Add(24 * time.Hour),
        EndDate:     time.Now().Add(48 * time.Hour),
    }

    t.Run("successful update", func(t *testing.T) {
        mockStore.On("UpdateTrip", ctx, int64(1), *update).Return(nil).Once()
        err := tripModel.UpdateTrip(ctx, 1, update)
        assert.NoError(t, err)
        mockStore.AssertExpectations(t)
    })

    t.Run("validation error - invalid dates", func(t *testing.T) {
        invalidUpdate := *update
        invalidUpdate.StartDate = time.Now().Add(48 * time.Hour)
        invalidUpdate.EndDate = time.Now().Add(24 * time.Hour)
        err := tripModel.UpdateTrip(ctx, 1, &invalidUpdate)
        assert.Error(t, err)
        assert.Equal(t, errors.ValidationError, err.(*errors.AppError).Type)
    })

    t.Run("store error", func(t *testing.T) {
        mockStore.On("UpdateTrip", ctx, int64(1), *update).Return(errors.NewDatabaseError(assert.AnError)).Once()
        err := tripModel.UpdateTrip(ctx, 1, update)
        assert.Error(t, err)
        assert.Equal(t, errors.DatabaseError, err.(*errors.AppError).Type)
        mockStore.AssertExpectations(t)
    })
}

func TestTripModel_DeleteTrip(t *testing.T) {
    mockStore := new(MockTripStore)
    mockStore.On("GetPool").Return(nil)
    tripModel := NewTripModel(mockStore)
    ctx := context.Background()

    t.Run("successful deletion", func(t *testing.T) {
        mockStore.On("SoftDeleteTrip", ctx, int64(1)).Return(nil).Once()
        err := tripModel.DeleteTrip(ctx, 1)
        assert.NoError(t, err)
        mockStore.AssertExpectations(t)
    })

    t.Run("store error", func(t *testing.T) {
        mockStore.On("SoftDeleteTrip", ctx, int64(1)).Return(errors.NewDatabaseError(assert.AnError)).Once()
        err := tripModel.DeleteTrip(ctx, 1)
        assert.Error(t, err)
        assert.Equal(t, errors.DatabaseError, err.(*errors.AppError).Type)
        mockStore.AssertExpectations(t)
    })
}

func TestTripModel_SearchTrips(t *testing.T) {
    mockStore := new(MockTripStore)
    mockStore.On("GetPool").Return(nil)
    tripModel := NewTripModel(mockStore)
    ctx := context.Background()

    criteria := types.TripSearchCriteria{
        Destination:   "Test City",
        StartDateFrom: time.Now(),
        StartDateTo:   time.Now().Add(72 * time.Hour),
    }

    expectedTrips := []*types.Trip{
        {
            ID:          1,
            Name:        "Trip 1",
            Destination: "Test City",
            StartDate:   time.Now().Add(24 * time.Hour),
            EndDate:     time.Now().Add(48 * time.Hour),
        },
        {
            ID:          2,
            Name:        "Trip 2",
            Destination: "Test City",
            StartDate:   time.Now().Add(36 * time.Hour),
            EndDate:     time.Now().Add(60 * time.Hour),
        },
    }

    t.Run("successful search", func(t *testing.T) {
        mockStore.On("GetPool").Return(nil).Once()
        mockStore.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(expectedTrips, nil).Once()

        trips, err := tripModel.SearchTrips(ctx, criteria)
        assert.NoError(t, err)
        assert.Equal(t, expectedTrips, trips)
        mockStore.AssertExpectations(t)
    })

    t.Run("store error", func(t *testing.T) {
        mockStore.On("GetPool").Return(nil).Once()
        mockStore.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).
            Return(nil, errors.NewDatabaseError(assert.AnError)).Once()

        trips, err := tripModel.SearchTrips(ctx, criteria)
        assert.Error(t, err)
        assert.Nil(t, trips)
        assert.Equal(t, errors.DatabaseError, err.(*errors.AppError).Type)
        mockStore.AssertExpectations(t)
    })
}