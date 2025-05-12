package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/store" // Import internal store for Transaction
	tripservice "github.com/NomadCrew/nomad-crew-backend/models/trip/service"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// Define context key locally to avoid import cycles
const testUserIDKey = contextKey("userID")

type contextKey string

// MockTransaction implements store.Transaction
type MockTransaction struct {
	mock.Mock
}

func (m *MockTransaction) Commit() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockTransaction) Rollback() error {
	args := m.Called()
	return args.Error(0)
}

// MockTripStore is a mock implementation of store.TripStore
type MockTripStore struct {
	mock.Mock
}

// Implement store.TripStore methods for MockTripStore
func (m *MockTripStore) CreateTrip(ctx context.Context, trip types.Trip) (string, error) {
	args := m.Called(ctx, trip)
	return args.String(0), args.Error(1)
}

func (m *MockTripStore) GetTrip(ctx context.Context, id string) (*types.Trip, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Trip), args.Error(1)
}

func (m *MockTripStore) UpdateTrip(ctx context.Context, id string, updateData types.TripUpdate) (*types.Trip, error) {
	args := m.Called(ctx, id, updateData)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Trip), args.Error(1)
}

func (m *MockTripStore) SoftDeleteTrip(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTripStore) ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error) {
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

func (m *MockTripStore) AddMember(ctx context.Context, membership *types.TripMembership) error {
	args := m.Called(ctx, membership)
	return args.Error(0)
}

func (m *MockTripStore) GetUserRole(ctx context.Context, tripID string, userID string) (types.MemberRole, error) {
	args := m.Called(ctx, tripID, userID)
	roleArg := args.Get(0)
	if roleArg == nil {
		if args.Error(1) != nil {
			return types.MemberRole(""), args.Error(1)
		}
		return types.MemberRole(""), fmt.Errorf("mock GetUserRole returned nil role without error")
	}
	return roleArg.(types.MemberRole), args.Error(1)
}

func (m *MockTripStore) GetTripMembers(ctx context.Context, tripID string) ([]types.TripMembership, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.TripMembership), args.Error(1)
}

// Assuming GetPool might be needed by internal implementation details, adding a basic mock
func (m *MockTripStore) GetPool() *pgxpool.Pool { // Import "github.com/jackc/pgx/v4/pgxpool"
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*pgxpool.Pool)
}

func (m *MockTripStore) UpdateMemberRole(ctx context.Context, tripID string, userID string, role types.MemberRole) error {
	args := m.Called(ctx, tripID, userID, role)
	return args.Error(0)
}

func (m *MockTripStore) RemoveMember(ctx context.Context, tripID string, userID string) error {
	args := m.Called(ctx, tripID, userID)
	return args.Error(0)
}

func (m *MockTripStore) LookupUserByEmail(ctx context.Context, email string) (*types.SupabaseUser, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SupabaseUser), args.Error(1)
}

func (m *MockTripStore) CreateInvitation(ctx context.Context, invitation *types.TripInvitation) error {
	args := m.Called(ctx, invitation)
	return args.Error(0)
}

func (m *MockTripStore) GetInvitation(ctx context.Context, invitationID string) (*types.TripInvitation, error) {
	args := m.Called(ctx, invitationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.TripInvitation), args.Error(1)
}

func (m *MockTripStore) GetInvitationsByTripID(ctx context.Context, tripID string) ([]*types.TripInvitation, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.TripInvitation), args.Error(1)
}

func (m *MockTripStore) UpdateInvitationStatus(ctx context.Context, invitationID string, status types.InvitationStatus) error {
	args := m.Called(ctx, invitationID, status)
	return args.Error(0)
}

// Corrected transaction methods
func (m *MockTripStore) BeginTx(ctx context.Context) (store.Transaction, error) {
	args := m.Called(ctx)
	// Return a mock transaction and the error
	mockTx := args.Get(0)
	if mockTx == nil {
		// If no specific mock transaction is provided, return a default one or nil
		// depending on what the error is. If error is nil, should likely return
		// a valid mock transaction.
		if args.Error(1) == nil {
			return new(MockTransaction), nil // Return a new mock transaction if no error
		}
		return nil, args.Error(1)
	}
	return mockTx.(store.Transaction), args.Error(1)
}

func (m *MockTripStore) Commit() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockTripStore) Rollback() error {
	args := m.Called()
	return args.Error(0)
}

// --- Mock Event Publisher ---
type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) Publish(ctx context.Context, tripID string, event types.Event) error {
	args := m.Called(ctx, tripID, event)
	return args.Error(0)
}

// Add missing PublishBatch method
func (m *MockEventPublisher) PublishBatch(ctx context.Context, tripID string, events []types.Event) error {
	args := m.Called(ctx, tripID, events)
	return args.Error(0)
}

// Add missing Subscribe/Unsubscribe methods (assuming they are part of the interface)
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

// --- Mock Weather Service ---
type MockWeatherService struct {
	mock.Mock
}

func (m *MockWeatherService) TriggerImmediateUpdate(ctx context.Context, tripID string, destination types.Destination) error {
	args := m.Called(ctx, tripID, destination)
	return args.Error(0)
}

// Corrected DecrementSubscribers signature
func (m *MockWeatherService) DecrementSubscribers(tripID string) {
	m.Called(tripID)
}

// Add missing methods from interface
func (m *MockWeatherService) StartWeatherUpdates(ctx context.Context, tripID string, destination types.Destination) {
	m.Called(ctx, tripID, destination)
}

func (m *MockWeatherService) IncrementSubscribers(tripID string, dest types.Destination) {
	m.Called(tripID, dest)
}

// Add mock for GetWeather - Already added earlier, this confirms it's present
func (m *MockWeatherService) GetWeather(ctx context.Context, tripID string) (*types.WeatherInfo, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.WeatherInfo), args.Error(1)
}

// --- Test Suite ---

type TripServiceTestSuite struct {
	suite.Suite
	mockStore          *MockTripStore
	mockEventPublisher *MockEventPublisher
	mockWeatherSvc     *MockWeatherService
	service            *tripservice.TripManagementService
	ctx                context.Context
	testUserID         string
	testTripID         string
}

// SetupTest runs before each test in the suite
func (suite *TripServiceTestSuite) SetupTest() {
	suite.mockStore = new(MockTripStore)
	suite.mockEventPublisher = new(MockEventPublisher)
	suite.mockWeatherSvc = new(MockWeatherService)

	// Initialize the service with mocks
	suite.service = tripservice.NewTripManagementService(
		suite.mockStore,
		suite.mockEventPublisher,
		suite.mockWeatherSvc,
	)

	suite.testUserID = uuid.New().String()
	suite.testTripID = uuid.New().String()
	// Add user ID to context using the local key
	suite.ctx = context.WithValue(context.Background(), testUserIDKey, suite.testUserID)
}

// TearDownTest runs after each test
func (suite *TripServiceTestSuite) TearDownTest() {
	// Optional: Verify mocks were called as expected
	// suite.mockStore.AssertExpectations(suite.T())
	// suite.mockEventPublisher.AssertExpectations(suite.T())
	// suite.mockWeatherSvc.AssertExpectations(suite.T())
}

// TestTripServiceTestSuite runs the entire test suite
func TestTripServiceTestSuite(t *testing.T) {
	suite.Run(t, new(TripServiceTestSuite))
}

// --- Example Test (Placeholder) ---

func (suite *TripServiceTestSuite) TestExample_Placeholder() {
	suite.T().Skip("Placeholder test, implement actual test cases")
	// Arrange
	// Mock calls etc.

	// Act
	// result, err := suite.service.SomeMethod(...)

	// Assert
	// assert.NoError(suite.T(), err)
	// assert.NotNil(suite.T(), result)
}

// --- Add tests for each method: CreateTrip, GetTrip, UpdateTrip, DeleteTrip, etc. ---

func (suite *TripServiceTestSuite) TestCreateTrip_Success() {
	// Arrange
	now := time.Now()
	destination := types.Destination{
		Address: "123 Test St",
		PlaceID: "ChIJtestplaceid",
		Coordinates: &struct {
			Lat float64 `json:"lat"`
			Lng float64 `json:"lng"`
		}{Lat: 40.7128, Lng: -74.0060},
	}
	tripInput := &types.Trip{
		Name:        "Test Trip",
		Description: "A trip for testing",
		Destination: destination,
		StartDate:   now,
		EndDate:     now.AddDate(0, 0, 7),
		Status:      types.TripStatusPlanning,
	}
	expectedTripID := suite.testTripID
	expectedMembership := &types.TripMembership{
		TripID: expectedTripID,
		UserID: suite.testUserID,
		Role:   types.MemberRoleOwner,
	}

	// --- Mock Expectations ---
	suite.mockStore.On("CreateTrip", mock.Anything, mock.MatchedBy(func(t types.Trip) bool {
		return t.Name == tripInput.Name && t.CreatedBy == suite.testUserID
	})).Return(expectedTripID, nil).Once()

	suite.mockStore.On("AddMember", suite.ctx, expectedMembership).Return(nil).Once()

	suite.mockEventPublisher.On(
		"Publish",
		mock.Anything, // ctx
		expectedTripID,
		mock.MatchedBy(func(e types.Event) bool {
			return e.Type == tripservice.EventTypeTripCreated &&
				e.UserID == suite.testUserID &&
				e.Metadata.Source == "trip-management-service"
		}),
	).Return(nil).Maybe()

	suite.mockWeatherSvc.On(
		"TriggerImmediateUpdate",
		mock.Anything, // Use mock.Anything for context to avoid matching issues
		expectedTripID,
		mock.AnythingOfType("types.Destination"),
	).Return(nil).Once()

	// --- Debug Logging ---
	// Removed debug logging

	// Act
	createdTrip, err := suite.service.CreateTrip(suite.ctx, tripInput)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), createdTrip)
	assert.Equal(suite.T(), expectedTripID, createdTrip.ID)
	assert.Equal(suite.T(), suite.testUserID, createdTrip.CreatedBy)
	assert.Equal(suite.T(), tripInput.Name, createdTrip.Name)

	// Verify mocks
	suite.mockStore.AssertExpectations(suite.T())
	suite.mockEventPublisher.AssertExpectations(suite.T())
	suite.mockWeatherSvc.AssertExpectations(suite.T())
}

// Add more tests for CreateTrip failure cases (store error, add member error)

// Add tests for GetTrip (success, not found, permission denied)
func (suite *TripServiceTestSuite) TestGetTrip_Success() {
	// Arrange
	expectedTrip := createMockTrip(suite.testTripID, uuid.NewString()) // Use helper

	// Corrected type: MemberRole
	suite.mockStore.On("GetUserRole", suite.ctx, suite.testTripID, suite.testUserID).Return(types.MemberRoleMember, nil).Once()
	suite.mockStore.On("GetTrip", suite.ctx, suite.testTripID).Return(expectedTrip, nil).Once()

	// Act
	trip, err := suite.service.GetTrip(suite.ctx, suite.testTripID, suite.testUserID)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), trip)
	assert.Equal(suite.T(), expectedTrip, trip)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *TripServiceTestSuite) TestGetTrip_PermissionDenied() {
	// Arrange
	notFoundErr := apperrors.NotFound("Membership", fmt.Sprintf("user %s in trip %s", suite.testUserID, suite.testTripID))
	// Corrected type: MemberRole
	suite.mockStore.On("GetUserRole", suite.ctx, suite.testTripID, suite.testUserID).Return(types.MemberRole(""), notFoundErr).Once()

	// Act
	trip, err := suite.service.GetTrip(suite.ctx, suite.testTripID, suite.testUserID)

	// Assert
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), trip)
	appErr, ok := err.(*apperrors.AppError)
	assert.True(suite.T(), ok, "Error should be an AppError")
	assert.Equal(suite.T(), apperrors.AuthorizationError, appErr.Type)
	suite.mockStore.AssertExpectations(suite.T())
	suite.mockStore.AssertNotCalled(suite.T(), "GetTrip", mock.Anything, mock.Anything)
}

// Add tests for UpdateTrip (success, permission denied, not found, store errors)
// Add tests for DeleteTrip (success, not found, store errors)
// Add tests for ListUserTrips
// Add tests for SearchTrips
// Add tests for UpdateTripStatus (success, invalid transition, not found)
// Add tests for GetTripWithMembers (success, not found)
// Add tests for TriggerWeatherUpdate (success, skipped, not found)
// Add tests for GetWeatherForTrip (success, skipped, not found, not implemented yet)

// --- GetTripWithMembers Tests ---

func (suite *TripServiceTestSuite) TestGetTripWithMembers_Success() {
	// Arrange
	existingTrip := createMockTrip(suite.testTripID, uuid.NewString())
	expectedMembers := []types.TripMembership{
		{ID: uuid.NewString(), TripID: suite.testTripID, UserID: suite.testUserID, Role: types.MemberRoleMember, Status: types.MembershipStatusActive},
		{ID: uuid.NewString(), TripID: suite.testTripID, UserID: existingTrip.CreatedBy, Role: types.MemberRoleOwner, Status: types.MembershipStatusActive},
	}
	// Convert to slice of pointers for the expected result struct
	expectedMemberPtrs := make([]*types.TripMembership, len(expectedMembers))
	for i := range expectedMembers {
		expectedMemberPtrs[i] = &expectedMembers[i]
	}

	// Mock GetUserRole (for permission check in underlying GetTrip)
	suite.mockStore.On("GetUserRole", suite.ctx, suite.testTripID, suite.testUserID).Return(types.MemberRoleMember, nil).Once()
	// Mock GetTrip (for permission check and base trip data)
	suite.mockStore.On("GetTrip", suite.ctx, suite.testTripID).Return(existingTrip, nil).Once()
	// Mock GetTripMembers
	suite.mockStore.On("GetTripMembers", suite.ctx, suite.testTripID).Return(expectedMembers, nil).Once()

	// Act
	tripWithMembers, err := suite.service.GetTripWithMembers(suite.ctx, suite.testTripID, suite.testUserID)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), tripWithMembers)
	assert.Equal(suite.T(), *existingTrip, tripWithMembers.Trip)
	assert.Equal(suite.T(), expectedMemberPtrs, tripWithMembers.Members)
	assert.Len(suite.T(), tripWithMembers.Members, 2)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *TripServiceTestSuite) TestGetTripWithMembers_PermissionDenied() {
	// Arrange
	notFoundErr := apperrors.NotFound("Membership", fmt.Sprintf("user %s in trip %s", suite.testUserID, suite.testTripID))

	// Mock GetUserRole failure (from underlying GetTrip call)
	suite.mockStore.On("GetUserRole", suite.ctx, suite.testTripID, suite.testUserID).Return(types.MemberRole(""), notFoundErr).Once()

	// Act
	tripWithMembers, err := suite.service.GetTripWithMembers(suite.ctx, suite.testTripID, suite.testUserID)

	// Assert
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), tripWithMembers)
	appErr, ok := err.(*apperrors.AppError)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), apperrors.AuthorizationError, appErr.Type)
	suite.mockStore.AssertExpectations(suite.T())
	// Ensure GetTrip (the second one) and GetTripMembers were not called
	suite.mockStore.AssertNotCalled(suite.T(), "GetTrip", mock.Anything, mock.Anything)
	suite.mockStore.AssertNotCalled(suite.T(), "GetTripMembers", mock.Anything, mock.Anything)
}

func (suite *TripServiceTestSuite) TestGetTripWithMembers_NotFound_Trip() {
	// Arrange
	notFoundErr := apperrors.NotFound("Trip", suite.testTripID)

	// Mock GetUserRole success, but GetTrip fails
	suite.mockStore.On("GetUserRole", suite.ctx, suite.testTripID, suite.testUserID).Return(types.MemberRoleMember, nil).Once()
	suite.mockStore.On("GetTrip", suite.ctx, suite.testTripID).Return(nil, notFoundErr).Once()

	// Act
	tripWithMembers, err := suite.service.GetTripWithMembers(suite.ctx, suite.testTripID, suite.testUserID)

	// Assert
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), tripWithMembers)
	appErr, ok := err.(*apperrors.AppError)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), apperrors.NotFoundError, appErr.Type)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *TripServiceTestSuite) TestGetTripWithMembers_StoreError_Members() {
	// Arrange
	existingTrip := createMockTrip(suite.testTripID, uuid.NewString())
	dbError := fmt.Errorf("database error fetching members")

	// Mock initial permission checks and GetTrip success
	suite.mockStore.On("GetUserRole", suite.ctx, suite.testTripID, suite.testUserID).Return(types.MemberRoleMember, nil).Once()
	suite.mockStore.On("GetTrip", suite.ctx, suite.testTripID).Return(existingTrip, nil).Once()
	// Mock GetTripMembers failure
	suite.mockStore.On("GetTripMembers", suite.ctx, suite.testTripID).Return(nil, dbError).Once()

	// Act
	tripWithMembers, err := suite.service.GetTripWithMembers(suite.ctx, suite.testTripID, suite.testUserID)

	// Assert
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), dbError, err) // Service returns the raw error here
	assert.Nil(suite.T(), tripWithMembers)
	suite.mockStore.AssertExpectations(suite.T())
}

// --- Weather Method Tests ---

func (suite *TripServiceTestSuite) TestTriggerWeatherUpdate_Success() {
	// Arrange
	// Create a trip that meets the criteria for shouldUpdateWeather
	tripToUpdate := createMockTrip(suite.testTripID, uuid.NewString())
	tripToUpdate.Status = types.TripStatusActive
	tripToUpdate.StartDate = time.Now().AddDate(0, 0, 1)
	tripToUpdate.EndDate = time.Now().AddDate(0, 0, 8)
	// Ensure Destination is valid (using helper defaults which should be valid)

	// Mock internal GetTrip
	suite.mockStore.On("GetTrip", suite.ctx, suite.testTripID).Return(tripToUpdate, nil).Once()
	// Expect weather service call with a nil error return
	suite.mockWeatherSvc.On("TriggerImmediateUpdate", suite.ctx, suite.testTripID, tripToUpdate.Destination).Return(nil).Once()

	// Act
	err := suite.service.TriggerWeatherUpdate(suite.ctx, suite.testTripID)

	// Assert
	assert.NoError(suite.T(), err)
	suite.mockStore.AssertExpectations(suite.T())
	suite.mockWeatherSvc.AssertExpectations(suite.T())
}

func (suite *TripServiceTestSuite) TestTriggerWeatherUpdate_Skipped() {
	// Arrange
	// Create a trip that does NOT meet the criteria (e.g., completed status)
	tripToUpdate := createMockTrip(suite.testTripID, uuid.NewString())
	tripToUpdate.Status = types.TripStatusCompleted

	// Mock internal GetTrip
	suite.mockStore.On("GetTrip", suite.ctx, suite.testTripID).Return(tripToUpdate, nil).Once()

	// Act
	err := suite.service.TriggerWeatherUpdate(suite.ctx, suite.testTripID)

	// Assert
	assert.Error(suite.T(), err)
	appErr, ok := err.(*apperrors.AppError)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), apperrors.ValidationError, appErr.Type)
	suite.mockStore.AssertExpectations(suite.T())
	suite.mockWeatherSvc.AssertNotCalled(suite.T(), "TriggerImmediateUpdate", mock.Anything, mock.Anything, mock.Anything)
}

func (suite *TripServiceTestSuite) TestTriggerWeatherUpdate_NotFound() {
	// Arrange
	notFoundErr := apperrors.NotFound("Trip", suite.testTripID)

	// Mock internal GetTrip failure
	suite.mockStore.On("GetTrip", suite.ctx, suite.testTripID).Return(nil, notFoundErr).Once()

	// Act
	err := suite.service.TriggerWeatherUpdate(suite.ctx, suite.testTripID)

	// Assert
	assert.Error(suite.T(), err)
	appErr, ok := err.(*apperrors.AppError)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), apperrors.NotFoundError, appErr.Type)
	suite.mockStore.AssertExpectations(suite.T())
	suite.mockWeatherSvc.AssertNotCalled(suite.T(), "TriggerImmediateUpdate", mock.Anything, mock.Anything, mock.Anything)
}

// --- GetWeatherForTrip Tests ---

func (suite *TripServiceTestSuite) TestGetWeatherForTrip_NotFound() {
	// Arrange
	notFoundErr := apperrors.NotFound("Trip", suite.testTripID)
	suite.mockStore.On("GetTrip", suite.ctx, suite.testTripID).Return(nil, notFoundErr).Once()

	// Act
	weatherInfo, err := suite.service.GetWeatherForTrip(suite.ctx, suite.testTripID)

	// Assert
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), weatherInfo)
	appErr, ok := err.(*apperrors.AppError)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), apperrors.NotFoundError, appErr.Type)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *TripServiceTestSuite) TestGetWeatherForTrip_Skipped() {
	// Arrange
	// Trip exists but doesn't meet criteria (e.g., no dates)
	trip := createMockTrip(suite.testTripID, uuid.NewString())
	trip.StartDate = time.Time{} // Zero date
	// Add coordinates to make destination valid initially - Use correct anonymous struct
	trip.Destination.Coordinates = &struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	}{Lat: 1, Lng: 1}
	suite.mockStore.On("GetTrip", suite.ctx, suite.testTripID).Return(trip, nil).Once()

	// Act
	weatherInfo, err := suite.service.GetWeatherForTrip(suite.ctx, suite.testTripID)

	// Assert
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), weatherInfo)
	appErr, ok := err.(*apperrors.AppError)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), apperrors.ValidationError, appErr.Type)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *TripServiceTestSuite) TestGetWeatherForTrip_Success() {
	// Arrange
	// Trip exists and meets criteria for shouldUpdateWeather
	trip := createMockTrip(suite.testTripID, uuid.NewString())
	trip.Status = types.TripStatusActive         // Valid status
	trip.StartDate = time.Now().AddDate(0, 0, 1) // Valid date
	trip.EndDate = time.Now().AddDate(0, 0, 8)   // Valid date
	// Destination is assumed valid from createMockTrip helper
	suite.mockStore.On("GetTrip", suite.ctx, suite.testTripID).Return(trip, nil).Once()

	// Mock the successful weather service call
	expectedWeather := &types.WeatherInfo{
		TripID:             suite.testTripID,
		Timestamp:          time.Now(),
		TemperatureCelsius: 25.5,
		Description:        "Sunny",
		// ... other relevant fields ...
	}
	suite.mockWeatherSvc.On("GetWeather", suite.ctx, suite.testTripID).Return(expectedWeather, nil).Once()

	// Act
	weatherInfo, err := suite.service.GetWeatherForTrip(suite.ctx, suite.testTripID)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), weatherInfo)
	assert.Equal(suite.T(), expectedWeather, weatherInfo)
	suite.mockStore.AssertExpectations(suite.T())
	suite.mockWeatherSvc.AssertExpectations(suite.T())
}

// Helper function to create a mock trip for tests
// Ensure Destination is populated if needed for weather checks
func createMockTrip(id, userID string) *types.Trip {
	now := time.Now()
	return &types.Trip{
		ID:          id,
		Name:        "Test Trip " + id,
		Description: "A description",
		// Add valid Coordinates to the default destination - Use correct anonymous struct
		Destination: types.Destination{
			Address: "Helper Addr",
			PlaceID: "helper-place-id",
			Coordinates: &struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			}{Lat: 40.7128, Lng: -74.0060},
		},
		StartDate: now,
		EndDate:   now.AddDate(0, 0, 10),
		Status:    types.TripStatusPlanning,
		CreatedBy: userID,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
