package models

import (
	"context"
	"testing"
	"time"

	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/services"
	"github.com/NomadCrew/nomad-crew-backend/tests/mocks"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

type MockTripStore struct {
	mock.Mock
}

const (
	testTripID = "trip-123"
	testUserID = "user-456"
)

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

func (m *MockTripStore) UpdateTrip(ctx context.Context, id string, update types.TripUpdate) (*types.Trip, error) {
	args := m.Called(ctx, id, update)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Trip), args.Error(1)
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

// Verify interface compliance
var _ store.TripStore = (*mocks.MockTripStore)(nil)

type MockWeatherService struct {
	mock.Mock
}

func (m *MockWeatherService) StartWeatherUpdates(ctx context.Context, tripID string, destination types.Destination) {
	m.Called(ctx, tripID, destination)
}

func (m *MockWeatherService) IncrementSubscribers(tripID string, dest types.Destination) {
	m.Called(tripID, dest)
}

func (m *MockWeatherService) DecrementSubscribers(tripID string) {
	m.Called(tripID)
}

func (m *MockWeatherService) TriggerImmediateUpdate(ctx context.Context, tripID string, destination types.Destination) {
	m.Called(ctx, tripID, destination)
}

func TestTripModel_CreateTrip(t *testing.T) {
	mockStore := new(mocks.MockTripStore)

	// Initialize WeatherService with mock event publisher
	weatherService := services.NewWeatherService(nil)
	mockEventPublisher := new(mocks.MockEventPublisher)
	tripModel := NewTripModel(mockStore, weatherService, mockEventPublisher)

	ctx := context.Background()

	validTrip := &types.Trip{
		Name:        "Test Trip",
		Description: "Test Description",
		Destination: types.Destination{
			Address: "Test Destination",
		},
		StartDate: time.Now().Add(24 * time.Hour),
		EndDate:   time.Now().Add(48 * time.Hour),
		CreatedBy: testUserID,
		Status:    types.TripStatusPlanning,
	}

	t.Run("successful creation", func(t *testing.T) {
		mockStore.On("CreateTrip", ctx, *validTrip).Return(testTripID, nil).Once()

		// Add this expectation for the event publisher
		mockEventPublisher.On("Publish", mock.Anything, testTripID, mock.AnythingOfType("types.Event")).
			Return(nil).
			Once()

		err := tripModel.CreateTrip(ctx, validTrip)
		assert.NoError(t, err)

		// Verify all expectations including the event publisher
		mockStore.AssertExpectations(t)
		mockEventPublisher.AssertExpectations(t)
	})

	t.Run("validation error - missing name", func(t *testing.T) {
		invalidTrip := *validTrip
		invalidTrip.Name = ""
		err := tripModel.CreateTrip(ctx, &invalidTrip)
		assert.Error(t, err)
		assert.IsType(t, &errors.AppError{}, err)
		assert.Equal(t, errors.ValidationError, err.(*errors.AppError).Type)
	})

	t.Run("store error", func(t *testing.T) {
		mockStore.On("CreateTrip", ctx, *validTrip).Return("", errors.NewDatabaseError(assert.AnError)).Once()
		err := tripModel.CreateTrip(ctx, validTrip)
		assert.Error(t, err)
		assert.IsType(t, &errors.AppError{}, err)
		assert.Equal(t, errors.DatabaseError, err.(*errors.AppError).Type)
		mockStore.AssertExpectations(t)
	})
}

func TestTripModel_GetTripByID(t *testing.T) {
	mockStore := new(mocks.MockTripStore)
	tripModel := NewTripModel(mockStore, nil, nil)
	ctx := context.Background()

	expectedTrip := &types.Trip{
		ID:          testTripID,
		Name:        "Test Trip",
		Description: "Test Description",
		Destination: types.Destination{
			Address: "Test Destination",
		},
		StartDate: time.Now().Add(24 * time.Hour),
		EndDate:   time.Now().Add(48 * time.Hour),
		CreatedBy: testUserID,
	}

	t.Run("successful retrieval", func(t *testing.T) {
		mockStore.On("GetTrip", ctx, testTripID).Return(expectedTrip, nil).Once()
		trip, err := tripModel.GetTripByID(ctx, testTripID)
		assert.NoError(t, err)
		assert.Equal(t, expectedTrip, trip)
		mockStore.AssertExpectations(t)
	})

	t.Run("not found", func(t *testing.T) {
		nonExistentID := "non-existent-id"
		mockStore.On("GetTrip", ctx, nonExistentID).Return(nil, &TripError{Code: ErrTripNotFound}).Once()
		trip, err := tripModel.GetTripByID(ctx, nonExistentID)
		assert.Error(t, err)
		assert.Nil(t, trip)
		assert.Equal(t, ErrTripNotFound, err.(*TripError).Code)
		mockStore.AssertExpectations(t)
	})
}

func TestTripModel_UpdateTrip(t *testing.T) {
	mockStore := new(mocks.MockTripStore)
	mockEventPublisher := new(mocks.MockEventPublisher)
	tripModel := NewTripModel(mockStore, nil, mockEventPublisher)
	ctx := context.Background()

	existingTrip := &types.Trip{
		ID:          testTripID,
		Name:        "Test Trip",
		Description: "Test Description",
		Destination: types.Destination{
			Address: "Test Destination",
		},
		StartDate: time.Now().Add(24 * time.Hour),
		EndDate:   time.Now().Add(48 * time.Hour),
		CreatedBy: testUserID,
	}

	update := &types.TripUpdate{
		Name:        stringPtr("Updated Trip"),
		Description: stringPtr("Updated Description"),
		Destination: &types.Destination{
			Address: "Updated Destination",
		},
		StartDate: timePtr(time.Now().Add(24 * time.Hour)),
		EndDate:   timePtr(time.Now().Add(48 * time.Hour)),
	}

	t.Run("successful update", func(t *testing.T) {
		mockStore.On("GetTrip", ctx, testTripID).Return(existingTrip, nil).Once()
		mockStore.On("UpdateTrip", ctx, testTripID, mock.MatchedBy(func(update types.TripUpdate) bool {
			return *update.Name == "Updated Trip" &&
				*update.Description == "Updated Description" &&
				update.Destination.Address == "Updated Destination"
		})).Return(&types.Trip{
			ID:          testTripID,
			Name:        "Updated Trip",
			Description: "Updated Description",
			Destination: types.Destination{Address: "Updated Destination"},
		}, nil).Once()

		// Add event publishing expectation
		mockEventPublisher.On("Publish", mock.Anything, testTripID, mock.MatchedBy(func(event types.Event) bool {
			return event.Type == types.EventTypeTripUpdated
		})).Return(nil).Once()

		err := tripModel.UpdateTrip(ctx, testTripID, update)
		assert.NoError(t, err)
		mockStore.AssertExpectations(t)
		mockEventPublisher.AssertExpectations(t)
	})

	t.Run("not found", func(t *testing.T) {
		nonExistentID := "non-existent-id"
		mockStore.On("GetTrip", ctx, nonExistentID).Return(nil, &TripError{Code: ErrTripNotFound}).Once()
		err := tripModel.UpdateTrip(ctx, nonExistentID, update)
		assert.Error(t, err)

		tripErr, ok := err.(*TripError)
		assert.True(t, ok)
		assert.Equal(t, ErrTripNotFound, tripErr.Code)
		mockStore.AssertExpectations(t)
	})

	t.Run("validation error - invalid dates", func(t *testing.T) {
		invalidUpdate := types.TripUpdate{
			Destination: &types.Destination{Address: "Invalid"},
			StartDate:   timePtr(time.Now().Add(48 * time.Hour)),
			EndDate:     timePtr(time.Now().Add(24 * time.Hour)),
		}
		err := tripModel.UpdateTrip(ctx, testTripID, &invalidUpdate)
		assert.Error(t, err)
		assert.Equal(t, errors.ValidationError, err.(*errors.AppError).Type)
	})
}

func TestTripModel_DeleteTrip(t *testing.T) {
	mockStore := new(mocks.MockTripStore)
	tripModel := NewTripModel(mockStore, nil, nil)
	ctx := context.Background()

	existingTrip := &types.Trip{
		ID:          testTripID,
		Name:        "Test Trip",
		Description: "Test Description",
		Destination: types.Destination{
			Address: "Test Destination",
		},
		StartDate: time.Now().Add(24 * time.Hour),
		EndDate:   time.Now().Add(48 * time.Hour),
		CreatedBy: testUserID,
	}

	t.Run("successful deletion", func(t *testing.T) {
		mockStore.On("GetTrip", ctx, testTripID).Return(existingTrip, nil).Once()
		mockStore.On("SoftDeleteTrip", ctx, testTripID).Return(nil).Once()
		err := tripModel.DeleteTrip(ctx, testTripID)
		assert.NoError(t, err)
		mockStore.AssertExpectations(t)
	})

	t.Run("not found", func(t *testing.T) {
		nonExistentID := "non-existent-id"
		mockStore.On("GetTrip", ctx, nonExistentID).Return(nil, assert.AnError).Once()
		err := tripModel.DeleteTrip(ctx, nonExistentID)
		assert.Error(t, err)
		assert.Equal(t, errors.NotFoundError, err.(*errors.AppError).Type)
		mockStore.AssertExpectations(t)
	})
}

// Add this after the existing test cases in models/trip_test.go

func TestTripModel_UpdateTripStatus(t *testing.T) {
	mockStore := new(mocks.MockTripStore)
	mockWeather := new(MockWeatherService)
	tripModel := NewTripModel(mockStore, mockWeather, nil)
	ctx := context.Background()

	baseTrip := &types.Trip{
		ID:          testTripID,
		Name:        "Test Trip",
		Description: "Test Description",
		Destination: types.Destination{
			Address: "Test Location",
		},
		StartDate: time.Now().Add(24 * time.Hour),
		EndDate:   time.Now().Add(48 * time.Hour),
		CreatedBy: testUserID,
		Status:    types.TripStatusPlanning,
	}

	t.Run("valid transition - planning to active", func(t *testing.T) {
		mockStore.On("GetTrip", ctx, testTripID).Return(baseTrip, nil).Once()
		mockStore.On("UpdateTrip", ctx, testTripID, mock.MatchedBy(func(update types.TripUpdate) bool {
			return update.Status == types.TripStatusActive
		})).Return(nil).Once()

		mockWeather.On("StartWeatherUpdates", ctx, testTripID, baseTrip.Destination).Once()

		err := tripModel.UpdateTripStatus(ctx, testTripID, types.TripStatusActive)
		assert.NoError(t, err)
		mockStore.AssertExpectations(t)
		mockWeather.AssertExpectations(t)
	})

	t.Run("invalid transition - completed to active", func(t *testing.T) {
		completedTrip := *baseTrip
		completedTrip.Status = types.TripStatusCompleted

		mockStore.On("GetTrip", ctx, testTripID).Return(&completedTrip, nil).Once()

		err := tripModel.UpdateTripStatus(ctx, testTripID, types.TripStatusActive)
		assert.Error(t, err)
		assert.Equal(t, errors.ValidationError, err.(*errors.AppError).Type)
		mockStore.AssertExpectations(t)
	})
}

func TestTripModel_ListUserTrips(t *testing.T) {
	mockStore := new(mocks.MockTripStore)
	tripModel := NewTripModel(mockStore, nil, nil)
	ctx := context.Background()

	trips := []*types.Trip{
		{
			ID:        testTripID,
			Name:      "Trip 1",
			CreatedBy: testUserID,
			Status:    types.TripStatusPlanning,
			StartDate: time.Now().Add(24 * time.Hour),
			EndDate:   time.Now().Add(48 * time.Hour),
		},
		{
			ID:        "trip-789",
			Name:      "Trip 2",
			CreatedBy: testUserID,
			Status:    types.TripStatusActive,
			StartDate: time.Now().Add(72 * time.Hour),
			EndDate:   time.Now().Add(96 * time.Hour),
		},
	}

	t.Run("successful list", func(t *testing.T) {
		mockStore.On("ListUserTrips", ctx, testUserID).Return(trips, nil).Once()

		result, err := tripModel.ListUserTrips(ctx, testUserID)
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, trips, result)
		mockStore.AssertExpectations(t)
	})

	t.Run("empty list", func(t *testing.T) {
		mockStore.On("ListUserTrips", ctx, testUserID).Return([]*types.Trip{}, nil).Once()

		result, err := tripModel.ListUserTrips(ctx, testUserID)
		assert.NoError(t, err)
		assert.Empty(t, result)
		mockStore.AssertExpectations(t)
	})
}

func TestTripModel_SearchTrips(t *testing.T) {
	mockStore := new(mocks.MockTripStore)
	tripModel := NewTripModel(mockStore, nil, nil)
	ctx := context.Background()

	searchResults := []*types.Trip{
		{
			ID:   testTripID,
			Name: "Paris Trip",
			Destination: types.Destination{
				Address: "Paris",
			},
			CreatedBy: testUserID,
			StartDate: time.Now().Add(24 * time.Hour),
			EndDate:   time.Now().Add(48 * time.Hour),
		},
	}

	t.Run("search by destination", func(t *testing.T) {
		criteria := types.TripSearchCriteria{
			Destination: "Paris",
		}

		mockStore.On("SearchTrips", ctx, criteria).Return(searchResults, nil).Once()

		result, err := tripModel.SearchTrips(ctx, criteria)
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "Paris", result[0].Destination.Address)
		mockStore.AssertExpectations(t)
	})

	t.Run("search by date range", func(t *testing.T) {
		startDate := time.Now()
		endDate := time.Now().Add(72 * time.Hour)

		criteria := types.TripSearchCriteria{
			StartDateFrom: startDate,
			StartDateTo:   endDate,
		}

		mockStore.On("SearchTrips", ctx, criteria).Return(searchResults, nil).Once()

		result, err := tripModel.SearchTrips(ctx, criteria)
		assert.NoError(t, err)
		assert.NotEmpty(t, result)
		mockStore.AssertExpectations(t)
	})

	t.Run("no results", func(t *testing.T) {
		criteria := types.TripSearchCriteria{
			Destination: "NonExistentPlace",
		}

		mockStore.On("SearchTrips", ctx, criteria).Return([]*types.Trip{}, nil).Once()

		result, err := tripModel.SearchTrips(ctx, criteria)
		assert.NoError(t, err)
		assert.Empty(t, result)
		mockStore.AssertExpectations(t)
	})
}

func TestTripModel_CreateTrip_Validation(t *testing.T) {
	mockStore := new(mocks.MockTripStore)
	tripModel := NewTripModel(mockStore, nil, nil)
	ctx := context.Background()
	now := time.Now()

	tests := []struct {
		name        string
		trip        *types.Trip
		expectError string
	}{
		{
			name: "empty name",
			trip: &types.Trip{
				Description: "Test Description",
				Destination: types.Destination{
					Address: "Paris",
				},
				StartDate: now.Add(24 * time.Hour),
				EndDate:   now.Add(48 * time.Hour),
				CreatedBy: testUserID,
			},
			expectError: "trip name is required",
		},
		{
			name: "empty destination",
			trip: &types.Trip{
				Name:        "Test Trip",
				Description: "Test Description",
				StartDate:   now.Add(24 * time.Hour),
				EndDate:     now.Add(48 * time.Hour),
				CreatedBy:   testUserID,
			},
			expectError: "trip destination is required",
		},
		{
			name: "end date before start date",
			trip: &types.Trip{
				Name:        "Test Trip",
				Description: "Test Description",
				Destination: types.Destination{
					Address: "Paris",
				},
				StartDate: now.Add(48 * time.Hour),
				EndDate:   now.Add(24 * time.Hour),
				CreatedBy: testUserID,
			},
			expectError: "trip end date cannot be before start date",
		},
		{
			name: "missing creator ID",
			trip: &types.Trip{
				Name:        "Test Trip",
				Description: "Test Description",
				Destination: types.Destination{
					Address: "Paris",
				},
				StartDate: now.Add(24 * time.Hour),
				EndDate:   now.Add(48 * time.Hour),
			},
			expectError: "trip creator ID is required",
		},
		{
			name: "invalid status",
			trip: &types.Trip{
				Name:        "Test Trip",
				Description: "Test Description",
				Destination: types.Destination{
					Address: "Paris",
				},
				StartDate: now.Add(24 * time.Hour),
				EndDate:   now.Add(48 * time.Hour),
				CreatedBy: testUserID,
				Status:    "INVALID_STATUS",
			},
			expectError: "invalid trip status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tripModel.CreateTrip(ctx, tt.trip)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

func TestTripModel_EdgeCases(t *testing.T) {
	mockStore := new(mocks.MockTripStore)
	tripModel := NewTripModel(mockStore, nil, nil)
	ctx := context.Background()
	now := time.Now()

	t.Run("trip spanning multiple years", func(t *testing.T) {
		longTrip := &types.Trip{
			ID:          testTripID,
			Name:        "World Tour",
			Description: "Year-long trip",
			Destination: types.Destination{
				Address: "Multiple",
			},
			StartDate: now,
			EndDate:   now.AddDate(1, 0, 0), // One year later
			CreatedBy: testUserID,
			Status:    types.TripStatusPlanning,
		}

		mockStore.On("CreateTrip", ctx, *longTrip).Return(testTripID, nil).Once()
		err := tripModel.CreateTrip(ctx, longTrip)
		assert.NoError(t, err)
	})

	t.Run("same day trip", func(t *testing.T) {
		sameDayTrip := &types.Trip{
			ID:          testTripID,
			Name:        "Day Trip",
			Description: "Single day trip",
			Destination: types.Destination{
				Address: "Nearby",
			},
			StartDate: now,
			EndDate:   now.Add(23 * time.Hour), // Same day
			CreatedBy: testUserID,
			Status:    types.TripStatusPlanning,
		}

		mockStore.On("CreateTrip", ctx, *sameDayTrip).Return(testTripID, nil).Once()
		err := tripModel.CreateTrip(ctx, sameDayTrip)
		assert.NoError(t, err)
	})

	t.Run("start date in past", func(t *testing.T) {
		pastTrip := &types.Trip{
			Name:        "Past Trip",
			Description: "Trip starting in past",
			Destination: types.Destination{
				Address: "Somewhere",
			},
			StartDate: now.AddDate(0, 0, -1), // Yesterday
			EndDate:   now.AddDate(0, 0, 5),
			CreatedBy: testUserID,
		}

		err := tripModel.CreateTrip(ctx, pastTrip)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "start date cannot be in the past"))
		mockStore.AssertNotCalled(t, "CreateTrip")
	})
}

func TestTripModel_StatusTransitionEdgeCases(t *testing.T) {
	mockStore := new(mocks.MockTripStore)
	tripModel := NewTripModel(mockStore, nil, nil)
	ctx := context.Background()
	now := time.Now()

	tests := []struct {
		name          string
		currentStatus types.TripStatus
		targetStatus  types.TripStatus
		tripStartDate time.Time
		tripEndDate   time.Time
		expectError   bool
		errorContains string
	}{
		{
			name:          "cannot complete future trip",
			currentStatus: types.TripStatusActive,
			targetStatus:  types.TripStatusCompleted,
			tripStartDate: now.Add(24 * time.Hour),
			tripEndDate:   now.Add(48 * time.Hour),
			expectError:   true,
			errorContains: "cannot complete a trip before its end date",
		},
		{
			name:          "cannot activate past trip",
			currentStatus: types.TripStatusPlanning,
			targetStatus:  types.TripStatusActive,
			tripStartDate: now.Add(-48 * time.Hour),
			tripEndDate:   now.Add(-24 * time.Hour),
			expectError:   true,
			errorContains: "cannot activate a trip that has already ended",
		},
		{
			name:          "cannot reactivate completed trip",
			currentStatus: types.TripStatusCompleted,
			targetStatus:  types.TripStatusActive,
			tripStartDate: now.Add(-48 * time.Hour),
			tripEndDate:   now.Add(-24 * time.Hour),
			expectError:   true,
			errorContains: "invalid status transition",
		},
		{
			name:          "cannot uncancel trip",
			currentStatus: types.TripStatusCancelled,
			targetStatus:  types.TripStatusPlanning,
			tripStartDate: now.Add(24 * time.Hour),
			tripEndDate:   now.Add(48 * time.Hour),
			expectError:   true,
			errorContains: "invalid status transition",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trip := &types.Trip{
				ID:        testTripID,
				Name:      "Test Trip",
				StartDate: tt.tripStartDate,
				EndDate:   tt.tripEndDate,
				Status:    tt.currentStatus,
				CreatedBy: testUserID,
			}

			mockStore.On("GetTrip", ctx, testTripID).Return(trip, nil).Once()

			err := tripModel.UpdateTripStatus(ctx, testTripID, tt.targetStatus)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}
