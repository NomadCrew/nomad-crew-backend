package service_test

import (
	"context"
	"testing"

	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	tripservice "github.com/NomadCrew/nomad-crew-backend/models/trip/service"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	// Add other necessary imports like config, interfaces etc. as needed
)

// contextKey is used to avoid key collisions in context values
type testContextKey string

// Define constants for context keys
const testUserIDKey testContextKey = "userID"

// --- Mock Services --- //
// Define mocks by embedding mock.Mock into structs with the same names
// as the services they mock. This avoids needing separate interfaces just for testing.

type MockTripManagementService struct {
	mock.Mock
	// We need to add ALL methods from the actual TripManagementService signature
}

// Implement methods for MockTripManagementService
func (m *MockTripManagementService) CreateTrip(ctx context.Context, trip *types.Trip) (*types.Trip, error) {
	args := m.Called(ctx, trip)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Trip), args.Error(1)
}
func (m *MockTripManagementService) GetTrip(ctx context.Context, id string, userID string) (*types.Trip, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Trip), args.Error(1)
}
func (m *MockTripManagementService) UpdateTrip(ctx context.Context, id string, userID string, updateData types.TripUpdate) (*types.Trip, error) {
	args := m.Called(ctx, id, userID, updateData)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Trip), args.Error(1)
}
func (m *MockTripManagementService) DeleteTrip(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockTripManagementService) ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Trip), args.Error(1)
}
func (m *MockTripManagementService) SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error) {
	args := m.Called(ctx, criteria)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Trip), args.Error(1)
}
func (m *MockTripManagementService) UpdateTripStatus(ctx context.Context, tripID, userID string, newStatus types.TripStatus) error {
	args := m.Called(ctx, tripID, userID, newStatus)
	return args.Error(0)
}
func (m *MockTripManagementService) GetTripWithMembers(ctx context.Context, tripID string, userID string) (*types.TripWithMembers, error) {
	args := m.Called(ctx, tripID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.TripWithMembers), args.Error(1)
}
func (m *MockTripManagementService) TriggerWeatherUpdate(ctx context.Context, tripID string) error {
	args := m.Called(ctx, tripID)
	return args.Error(0)
}
func (m *MockTripManagementService) GetWeatherForTrip(ctx context.Context, tripID string) (*types.WeatherInfo, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.WeatherInfo), args.Error(1)
}

// Add internal methods if coordinator calls them (e.g., getTripInternal - but it seems it doesn't)

type MockTripMemberService struct {
	mock.Mock
}

// Implement methods for MockTripMemberService
func (m *MockTripMemberService) GetUserRole(ctx context.Context, tripID, userID string) (types.MemberRole, error) {
	args := m.Called(ctx, tripID, userID)
	return args.Get(0).(types.MemberRole), args.Error(1)
}
func (m *MockTripMemberService) AddMember(ctx context.Context, membership *types.TripMembership) error {
	args := m.Called(ctx, membership)
	return args.Error(0)
}

// Correct return type for UpdateMemberRole to match interface
func (m *MockTripMemberService) UpdateMemberRole(ctx context.Context, tripID, userID string, role types.MemberRole) (*interfaces.CommandResult, error) {
	args := m.Called(ctx, tripID, userID, role)
	// Handle nil case for pointer return
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.CommandResult), args.Error(1)
}
func (m *MockTripMemberService) RemoveMember(ctx context.Context, tripID, userID string) error {
	args := m.Called(ctx, tripID, userID)
	return args.Error(0)
}
func (m *MockTripMemberService) GetTripMembers(ctx context.Context, tripID string) ([]types.TripMembership, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.TripMembership), args.Error(1)
}

type MockInvitationService struct {
	mock.Mock
}

// Implement methods for MockInvitationService
func (m *MockInvitationService) CreateInvitation(ctx context.Context, invitation *types.TripInvitation) error {
	args := m.Called(ctx, invitation)
	return args.Error(0)
}
func (m *MockInvitationService) GetInvitation(ctx context.Context, invitationID string) (*types.TripInvitation, error) {
	args := m.Called(ctx, invitationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.TripInvitation), args.Error(1)
}
func (m *MockInvitationService) UpdateInvitationStatus(ctx context.Context, invitationID string, status types.InvitationStatus) error {
	args := m.Called(ctx, invitationID, status)
	return args.Error(0)
}
func (m *MockInvitationService) LookupUserByEmail(ctx context.Context, email string) (*types.SupabaseUser, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SupabaseUser), args.Error(1)
}
func (m *MockInvitationService) FindInvitationByTripAndEmail(ctx context.Context, tripID, email string) (*types.TripInvitation, error) {
	args := m.Called(ctx, tripID, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.TripInvitation), args.Error(1)
}

type MockTripChatService struct {
	mock.Mock
}

// Implement methods for MockTripChatService
func (m *MockTripChatService) ListMessages(ctx context.Context, tripID string, userID string, limit int, before string) ([]*types.ChatMessage, error) {
	args := m.Called(ctx, tripID, userID, limit, before)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.ChatMessage), args.Error(1)
}
func (m *MockTripChatService) UpdateLastReadMessage(ctx context.Context, tripID string, userID string, messageID string) error {
	args := m.Called(ctx, tripID, userID, messageID)
	return args.Error(0)
}

// --- Test Suite --- //

type CoordinatorTestSuite struct {
	suite.Suite
	coordinator *tripservice.TripModelCoordinator
	// Use interface types for mocks to match coordinator fields
	mockTripSvc   *MockTripManagementService
	mockMemberSvc *MockTripMemberService
	mockInviteSvc *MockInvitationService
	mockChatSvc   *MockTripChatService
	ctx           context.Context
	testUserID    string
	testTripID    string
}

// SetupTest runs before each test in the suite
func (suite *CoordinatorTestSuite) SetupTest() {
	suite.mockTripSvc = new(MockTripManagementService)
	suite.mockMemberSvc = new(MockTripMemberService)
	suite.mockInviteSvc = new(MockInvitationService)
	suite.mockChatSvc = new(MockTripChatService)

	// Manually construct the coordinator, assigning mocks to interface fields.
	suite.coordinator = &tripservice.TripModelCoordinator{
		TripService:       suite.mockTripSvc,
		MemberService:     suite.mockMemberSvc,
		InvitationService: suite.mockInviteSvc,
		ChatService:       suite.mockChatSvc,
	}

	// Basic context and test IDs
	suite.testUserID = uuid.New().String()
	suite.testTripID = uuid.New().String()

	// Use typed context key to prevent collisions
	suite.ctx = context.WithValue(context.Background(), testUserIDKey, suite.testUserID)
}

func TestCoordinatorTestSuite(t *testing.T) {
	suite.Run(t, new(CoordinatorTestSuite))
}

// --- Test Cases --- //

func (suite *CoordinatorTestSuite) TestCreateTrip_Delegates() {
	// Arrange
	tripInput := &types.Trip{Name: "Test"}
	expectedTrip := &types.Trip{ID: suite.testTripID, Name: "Test"}
	// Expect call on the *mock* service
	suite.mockTripSvc.On("CreateTrip", suite.ctx, tripInput).Return(expectedTrip, nil).Once()

	// Act
	// Call the *coordinator* method
	result, err := suite.coordinator.CreateTrip(suite.ctx, tripInput)

	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedTrip, result)
	// Verify the mock was called as expected
	suite.mockTripSvc.AssertExpectations(suite.T())
}

// Add similar delegate tests for other methods...
func (suite *CoordinatorTestSuite) TestAddMember_Delegates() {
	// Arrange
	membership := &types.TripMembership{UserID: suite.testUserID, TripID: suite.testTripID}
	suite.mockMemberSvc.On("AddMember", suite.ctx, membership).Return(nil).Once()

	// Act
	err := suite.coordinator.AddMember(suite.ctx, membership)

	// Assert
	assert.NoError(suite.T(), err)
	suite.mockMemberSvc.AssertExpectations(suite.T())
}

func (suite *CoordinatorTestSuite) TestListMessages_Delegates() {
	// Arrange
	limit := 50
	before := "some_cursor"
	expectedMessages := []*types.ChatMessage{{ID: "msg1"}}
	// Expect call on the *mock* service, including the userID extracted from context
	suite.mockChatSvc.On("ListMessages", suite.ctx, suite.testTripID, suite.testUserID, limit, before).Return(expectedMessages, nil).Once()

	// Act
	// Call the *coordinator* method
	result, err := suite.coordinator.ListMessages(suite.ctx, suite.testTripID, limit, before)

	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedMessages, result)
	suite.mockChatSvc.AssertExpectations(suite.T())
}

// Test methods that extract userID from context
func (suite *CoordinatorTestSuite) TestUpdateTripStatus_Delegates() {
	// Arrange
	newStatus := types.TripStatusActive
	suite.mockTripSvc.On("UpdateTripStatus", suite.ctx, suite.testTripID, suite.testUserID, newStatus).Return(nil).Once()

	// Act
	err := suite.coordinator.UpdateTripStatus(suite.ctx, suite.testTripID, newStatus)

	// Assert
	assert.NoError(suite.T(), err)
	suite.mockTripSvc.AssertExpectations(suite.T())
}

func (suite *CoordinatorTestSuite) TestUpdateLastReadMessage_Delegates() {
	// Arrange
	messageID := uuid.NewString()
	suite.mockChatSvc.On("UpdateLastReadMessage", suite.ctx, suite.testTripID, suite.testUserID, messageID).Return(nil).Once()

	// Act
	err := suite.coordinator.UpdateLastReadMessage(suite.ctx, suite.testTripID, messageID)

	// Assert
	assert.NoError(suite.T(), err)
	suite.mockChatSvc.AssertExpectations(suite.T())
}

func (suite *CoordinatorTestSuite) TestGetTripByID_Delegates() {
	// Arrange
	expectedTrip := &types.Trip{ID: suite.testTripID}
	suite.mockTripSvc.On("GetTrip", suite.ctx, suite.testTripID, suite.testUserID).Return(expectedTrip, nil).Once()
	// Act
	result, err := suite.coordinator.GetTripByID(suite.ctx, suite.testTripID, suite.testUserID)
	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedTrip, result)
	suite.mockTripSvc.AssertExpectations(suite.T())
}

func (suite *CoordinatorTestSuite) TestUpdateTrip_Delegates() {
	// Arrange
	updateData := &types.TripUpdate{ /* ... */ }
	expectedTrip := &types.Trip{ID: suite.testTripID}
	suite.mockTripSvc.On("UpdateTrip", suite.ctx, suite.testTripID, suite.testUserID, *updateData).Return(expectedTrip, nil).Once()
	// Act
	result, err := suite.coordinator.UpdateTrip(suite.ctx, suite.testTripID, suite.testUserID, updateData)
	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedTrip, result)
	suite.mockTripSvc.AssertExpectations(suite.T())
}

func (suite *CoordinatorTestSuite) TestDeleteTrip_Delegates() {
	// Arrange
	suite.mockTripSvc.On("DeleteTrip", suite.ctx, suite.testTripID).Return(nil).Once()
	// Act
	err := suite.coordinator.DeleteTrip(suite.ctx, suite.testTripID)
	// Assert
	assert.NoError(suite.T(), err)
	suite.mockTripSvc.AssertExpectations(suite.T())
}

func (suite *CoordinatorTestSuite) TestListUserTrips_Delegates() {
	// Arrange
	expectedTrips := []*types.Trip{{ID: suite.testTripID}}
	suite.mockTripSvc.On("ListUserTrips", suite.ctx, suite.testUserID).Return(expectedTrips, nil).Once()
	// Act
	result, err := suite.coordinator.ListUserTrips(suite.ctx, suite.testUserID)
	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedTrips, result)
	suite.mockTripSvc.AssertExpectations(suite.T())
}

func (suite *CoordinatorTestSuite) TestSearchTrips_Delegates() {
	// Arrange
	criteria := types.TripSearchCriteria{ /* ... */ }
	expectedTrips := []*types.Trip{{ID: suite.testTripID}}
	suite.mockTripSvc.On("SearchTrips", suite.ctx, criteria).Return(expectedTrips, nil).Once()
	// Act
	result, err := suite.coordinator.SearchTrips(suite.ctx, criteria)
	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedTrips, result)
	suite.mockTripSvc.AssertExpectations(suite.T())
}

func (suite *CoordinatorTestSuite) TestGetUserRole_Delegates() {
	// Arrange
	expectedRole := types.MemberRoleOwner
	suite.mockMemberSvc.On("GetUserRole", suite.ctx, suite.testTripID, suite.testUserID).Return(expectedRole, nil).Once()
	// Act
	result, err := suite.coordinator.GetUserRole(suite.ctx, suite.testTripID, suite.testUserID)
	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedRole, result)
	suite.mockMemberSvc.AssertExpectations(suite.T())
}

func (suite *CoordinatorTestSuite) TestUpdateMemberRole_Delegates() {
	// Arrange
	newRole := types.MemberRoleMember
	expectedResult := &interfaces.CommandResult{ /* ... */ } // Assuming CommandResult is needed
	suite.mockMemberSvc.On("UpdateMemberRole", suite.ctx, suite.testTripID, suite.testUserID, newRole).Return(expectedResult, nil).Once()
	// Act
	result, err := suite.coordinator.UpdateMemberRole(suite.ctx, suite.testTripID, suite.testUserID, newRole)
	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedResult, result)
	suite.mockMemberSvc.AssertExpectations(suite.T())
}

func (suite *CoordinatorTestSuite) TestRemoveMember_Delegates() {
	// Arrange
	memberToRemoveID := uuid.NewString()
	suite.mockMemberSvc.On("RemoveMember", suite.ctx, suite.testTripID, memberToRemoveID).Return(nil).Once()
	// Act
	err := suite.coordinator.RemoveMember(suite.ctx, suite.testTripID, memberToRemoveID)
	// Assert
	assert.NoError(suite.T(), err)
	suite.mockMemberSvc.AssertExpectations(suite.T())
}

func (suite *CoordinatorTestSuite) TestGetTripMembers_Delegates() {
	// Arrange
	expectedMembers := []types.TripMembership{{UserID: suite.testUserID}}
	suite.mockMemberSvc.On("GetTripMembers", suite.ctx, suite.testTripID).Return(expectedMembers, nil).Once()
	// Act
	result, err := suite.coordinator.GetTripMembers(suite.ctx, suite.testTripID)
	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedMembers, result)
	suite.mockMemberSvc.AssertExpectations(suite.T())
}

func (suite *CoordinatorTestSuite) TestCreateInvitation_Delegates() {
	// Arrange
	invitation := &types.TripInvitation{ /* ... */ }
	suite.mockInviteSvc.On("CreateInvitation", suite.ctx, invitation).Return(nil).Once()
	// Act
	err := suite.coordinator.CreateInvitation(suite.ctx, invitation)
	// Assert
	assert.NoError(suite.T(), err)
	suite.mockInviteSvc.AssertExpectations(suite.T())
}

func (suite *CoordinatorTestSuite) TestGetInvitation_Delegates() {
	// Arrange
	invID := uuid.NewString()
	expectedInv := &types.TripInvitation{ID: invID}
	suite.mockInviteSvc.On("GetInvitation", suite.ctx, invID).Return(expectedInv, nil).Once()
	// Act
	result, err := suite.coordinator.GetInvitation(suite.ctx, invID)
	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedInv, result)
	suite.mockInviteSvc.AssertExpectations(suite.T())
}

func (suite *CoordinatorTestSuite) TestUpdateInvitationStatus_Delegates() {
	// Arrange
	invID := uuid.NewString()
	newStatus := types.InvitationStatusAccepted
	suite.mockInviteSvc.On("UpdateInvitationStatus", suite.ctx, invID, newStatus).Return(nil).Once()
	// Act
	err := suite.coordinator.UpdateInvitationStatus(suite.ctx, invID, newStatus)
	// Assert
	assert.NoError(suite.T(), err)
	suite.mockInviteSvc.AssertExpectations(suite.T())
}

func (suite *CoordinatorTestSuite) TestLookupUserByEmail_Delegates() {
	// Arrange
	email := "test@example.com"
	expectedUser := &types.SupabaseUser{ /* ... */ }
	suite.mockInviteSvc.On("LookupUserByEmail", suite.ctx, email).Return(expectedUser, nil).Once()
	// Act
	result, err := suite.coordinator.LookupUserByEmail(suite.ctx, email)
	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedUser, result)
	suite.mockInviteSvc.AssertExpectations(suite.T())
}

func (suite *CoordinatorTestSuite) TestFindInvitationByTripAndEmail_Delegates() {
	// Arrange
	email := "test@example.com"
	expectedInv := &types.TripInvitation{ /* ... */ }
	suite.mockInviteSvc.On("FindInvitationByTripAndEmail", suite.ctx, suite.testTripID, email).Return(expectedInv, nil).Once()
	// Act
	result, err := suite.coordinator.FindInvitationByTripAndEmail(suite.ctx, suite.testTripID, email)
	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedInv, result)
	suite.mockInviteSvc.AssertExpectations(suite.T())
}

func (suite *CoordinatorTestSuite) TestInviteMember_Delegates() {
	// Arrange (Same as CreateInvitation)
	invitation := &types.TripInvitation{ /* ... */ }
	suite.mockInviteSvc.On("CreateInvitation", suite.ctx, invitation).Return(nil).Once()
	// Act
	err := suite.coordinator.InviteMember(suite.ctx, invitation)
	// Assert
	assert.NoError(suite.T(), err)
	suite.mockInviteSvc.AssertExpectations(suite.T())
}

func (suite *CoordinatorTestSuite) TestGetTripWithMembers_Delegates() {
	// Arrange
	expectedResult := &types.TripWithMembers{ /* ... */ }
	suite.mockTripSvc.On("GetTripWithMembers", suite.ctx, suite.testTripID, suite.testUserID).Return(expectedResult, nil).Once()
	// Act
	result, err := suite.coordinator.GetTripWithMembers(suite.ctx, suite.testTripID, suite.testUserID)
	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedResult, result)
	suite.mockTripSvc.AssertExpectations(suite.T())
}

// Tests for GetCommandContext, GetTripStore, GetChatStore are likely trivial
// func (suite *CoordinatorTestSuite) TestGetStores_ReturnsValue() { ... }
