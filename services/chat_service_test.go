package services_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/events"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	chatservice "github.com/NomadCrew/nomad-crew-backend/models/chat/service"
	"github.com/NomadCrew/nomad-crew-backend/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Initialize logger for tests
func init() {
	logger.InitLogger() // Removed "test" argument
}

// MockChatStore is a mock implementation of store.ChatStore
type MockChatStore struct {
	mock.Mock
}

// AddReaction implements the ChatStore interface
func (m *MockChatStore) AddReaction(ctx context.Context, messageID, userID, reaction string) error {
	args := m.Called(ctx, messageID, userID, reaction)
	return args.Error(0)
}

func (m *MockChatStore) CreateChatGroup(ctx context.Context, group types.ChatGroup) (string, error) {
	args := m.Called(ctx, group)
	return args.String(0), args.Error(1)
}

func (m *MockChatStore) AddChatGroupMember(ctx context.Context, groupID string, userID string) error {
	args := m.Called(ctx, groupID, userID)
	return args.Error(0)
}

func (m *MockChatStore) CreateChatMessage(ctx context.Context, message types.ChatMessage) (string, error) {
	args := m.Called(ctx, message)
	return args.String(0), args.Error(1)
}

func (m *MockChatStore) GetChatMessages(ctx context.Context, tripID string, params types.PaginationParams) ([]types.ChatMessage, int, error) {
	args := m.Called(ctx, tripID, params)
	var messages []types.ChatMessage
	if ret0 := args.Get(0); ret0 != nil {
		messages = ret0.([]types.ChatMessage)
	}
	return messages, args.Int(1), args.Error(2)
}

func (m *MockChatStore) UpdateChatMessage(ctx context.Context, messageID string, newContent string) error {
	args := m.Called(ctx, messageID, newContent)
	return args.Error(0)
}

func (m *MockChatStore) DeleteChatMessage(ctx context.Context, messageID string) error {
	args := m.Called(ctx, messageID)
	return args.Error(0)
}

func (m *MockChatStore) GetChatMessageByID(ctx context.Context, messageID string) (*types.ChatMessage, error) {
	args := m.Called(ctx, messageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	msg, ok := args.Get(0).(*types.ChatMessage)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("Mock GetChatMessageByID returned unexpected type: %T", args.Get(0)))
	}
	return msg, args.Error(1)
}

// Add missing ListChatMessageReactions
func (m *MockChatStore) ListChatMessageReactions(ctx context.Context, messageID string) ([]types.ChatMessageReaction, error) {
	args := m.Called(ctx, messageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	reactions, ok := args.Get(0).([]types.ChatMessageReaction)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("Mock ListChatMessageReactions returned unexpected type: %T", args.Get(0)))
	}
	return reactions, args.Error(1)
}

// Add missing GetChatGroup
func (m *MockChatStore) GetChatGroup(ctx context.Context, groupID string) (*types.ChatGroup, error) {
	args := m.Called(ctx, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	group, ok := args.Get(0).(*types.ChatGroup)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("Mock GetChatGroup returned unexpected type: %T", args.Get(0)))
	}
	return group, args.Error(1)
}

// Add missing UpdateChatGroup
func (m *MockChatStore) UpdateChatGroup(ctx context.Context, groupID string, update types.ChatGroupUpdateRequest) error {
	args := m.Called(ctx, groupID, update)
	return args.Error(0)
}

// Add missing DeleteChatGroup
func (m *MockChatStore) DeleteChatGroup(ctx context.Context, groupID string) error {
	args := m.Called(ctx, groupID)
	return args.Error(0)
}

// Add missing ListChatGroupsByTrip
func (m *MockChatStore) ListChatGroupsByTrip(ctx context.Context, tripID string, limit, offset int) (*types.ChatGroupPaginatedResponse, error) {
	args := m.Called(ctx, tripID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	resp, ok := args.Get(0).(*types.ChatGroupPaginatedResponse)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("Mock ListChatGroupsByTrip returned unexpected type: %T", args.Get(0)))
	}
	return resp, args.Error(1)
}

// Add missing GetChatMessage
func (m *MockChatStore) GetChatMessage(ctx context.Context, messageID string) (*types.ChatMessage, error) {
	args := m.Called(ctx, messageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	msg, ok := args.Get(0).(*types.ChatMessage)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("Mock GetChatMessage returned unexpected type: %T", args.Get(0)))
	}
	return msg, args.Error(1)
}

// ListChatMessages implements the ChatStore interface
func (m *MockChatStore) ListChatMessages(ctx context.Context, groupID string, params types.PaginationParams) ([]types.ChatMessage, int, error) {
	args := m.Called(ctx, groupID, params)
	var messages []types.ChatMessage
	if ret0 := args.Get(0); ret0 != nil {
		messages = ret0.([]types.ChatMessage)
	}
	return messages, args.Int(1), args.Error(2)
}

// Add missing ListTripMessages
func (m *MockChatStore) ListTripMessages(ctx context.Context, tripID string, limit, offset int) ([]types.ChatMessage, int, error) {
	args := m.Called(ctx, tripID, limit, offset)
	var messages []types.ChatMessage
	if ret0 := args.Get(0); ret0 != nil {
		messages = ret0.([]types.ChatMessage)
	}
	return messages, args.Int(1), args.Error(2)
}

// Add missing RemoveChatGroupMember
func (m *MockChatStore) RemoveChatGroupMember(ctx context.Context, groupID, userID string) error {
	args := m.Called(ctx, groupID, userID)
	return args.Error(0)
}

// Add missing ListChatGroupMembers
func (m *MockChatStore) ListChatGroupMembers(ctx context.Context, groupID string) ([]types.UserResponse, error) {
	args := m.Called(ctx, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	members, ok := args.Get(0).([]types.UserResponse)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("Mock ListChatGroupMembers returned unexpected type: %T", args.Get(0)))
	}
	return members, args.Error(1)
}

func (m *MockChatStore) UpdateLastReadMessage(ctx context.Context, groupID, userID, messageID string) error {
	args := m.Called(ctx, groupID, userID, messageID)
	return args.Error(0)
}

func (m *MockChatStore) GetUserInfo(ctx context.Context, userID string) (*types.UserResponse, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	user, ok := args.Get(0).(*types.UserResponse)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("Mock GetUserInfo returned unexpected type: %T", args.Get(0)))
	}
	return user, args.Error(1)
}

// GetUserByID implements the ChatStore interface
func (m *MockChatStore) GetUserByID(ctx context.Context, userID string) (*types.SupabaseUser, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	user, ok := args.Get(0).(*types.SupabaseUser)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("Mock GetUserByID returned unexpected type: %T", args.Get(0)))
	}
	return user, args.Error(1)
}

// RemoveReaction implements the ChatStore interface
func (m *MockChatStore) RemoveReaction(ctx context.Context, messageID, userID, reaction string) error {
	args := m.Called(ctx, messageID, userID, reaction)
	return args.Error(0)
}

// MockTripStore is a mock implementation of store.TripStore
type MockTripStore struct {
	mock.Mock
}

// AddMember implements the missing method for TripStore interface
func (m *MockTripStore) AddMember(ctx context.Context, membership *types.TripMembership) error {
	args := m.Called(ctx, membership)
	return args.Error(0)
}

// Add missing GetPool
func (m *MockTripStore) GetPool() *pgxpool.Pool {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*pgxpool.Pool)
}

// Add missing CreateTrip
func (m *MockTripStore) CreateTrip(ctx context.Context, trip types.Trip) (string, error) {
	args := m.Called(ctx, trip)
	return args.String(0), args.Error(1)
}

func (m *MockTripStore) GetTrip(ctx context.Context, tripID string) (*types.Trip, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	trip, ok := args.Get(0).(*types.Trip)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("Mock GetTrip returned unexpected type: %T", args.Get(0)))
	}
	return trip, args.Error(1)
}

// Add missing UpdateTrip
func (m *MockTripStore) UpdateTrip(ctx context.Context, id string, update types.TripUpdate) (*types.Trip, error) {
	args := m.Called(ctx, id, update)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	trip, ok := args.Get(0).(*types.Trip)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("Mock UpdateTrip returned unexpected type: %T", args.Get(0)))
	}
	return trip, args.Error(1)
}

// Add missing SoftDeleteTrip
func (m *MockTripStore) SoftDeleteTrip(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// Add missing ListUserTrips
func (m *MockTripStore) ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	trips, ok := args.Get(0).([]*types.Trip)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("Mock ListUserTrips returned unexpected type: %T", args.Get(0)))
	}
	return trips, args.Error(1)
}

// Add missing SearchTrips
func (m *MockTripStore) SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error) {
	args := m.Called(ctx, criteria)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	trips, ok := args.Get(0).([]*types.Trip)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("Mock SearchTrips returned unexpected type: %T", args.Get(0)))
	}
	return trips, args.Error(1)
}

// Add missing UpdateMemberRole
func (m *MockTripStore) UpdateMemberRole(ctx context.Context, tripID string, userID string, role types.MemberRole) error {
	args := m.Called(ctx, tripID, userID, role)
	return args.Error(0)
}

// Add missing RemoveMember
func (m *MockTripStore) RemoveMember(ctx context.Context, tripID string, userID string) error {
	args := m.Called(ctx, tripID, userID)
	return args.Error(0)
}

// Add missing LookupUserByEmail
func (m *MockTripStore) LookupUserByEmail(ctx context.Context, email string) (*types.SupabaseUser, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	user, ok := args.Get(0).(*types.SupabaseUser)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("Mock LookupUserByEmail returned unexpected type: %T", args.Get(0)))
	}
	return user, args.Error(1)
}

// Add missing CreateInvitation
func (m *MockTripStore) CreateInvitation(ctx context.Context, invitation *types.TripInvitation) error {
	args := m.Called(ctx, invitation)
	return args.Error(0)
}

// Add missing GetInvitation
func (m *MockTripStore) GetInvitation(ctx context.Context, invitationID string) (*types.TripInvitation, error) {
	args := m.Called(ctx, invitationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	inv, ok := args.Get(0).(*types.TripInvitation)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("Mock GetInvitation returned unexpected type: %T", args.Get(0)))
	}
	return inv, args.Error(1)
}

// Add missing GetInvitationsByTripID
func (m *MockTripStore) GetInvitationsByTripID(ctx context.Context, tripID string) ([]*types.TripInvitation, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	invs, ok := args.Get(0).([]*types.TripInvitation)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("Mock GetInvitationsByTripID returned unexpected type: %T", args.Get(0)))
	}
	return invs, args.Error(1)
}

// Add missing UpdateInvitationStatus
func (m *MockTripStore) UpdateInvitationStatus(ctx context.Context, invitationID string, status types.InvitationStatus) error {
	args := m.Called(ctx, invitationID, status)
	return args.Error(0)
}

// Add missing BeginTx, Commit, Rollback - Note: Mocking transactions can be complex
func (m *MockTripStore) BeginTx(ctx context.Context) (store.Transaction, error) {
	args := m.Called(ctx)
	// Returning nil mocks for simplicity, adjust if detailed transaction testing is needed
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(store.Transaction), args.Error(1)
}

func (m *MockTripStore) Commit() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockTripStore) Rollback() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockTripStore) GetUserRole(ctx context.Context, tripID string, userID string) (types.MemberRole, error) {
	args := m.Called(ctx, tripID, userID)
	roleStr, ok := args.Get(0).(types.MemberRole)
	if !ok {
		roleStr = types.MemberRole(args.String(0))
	}
	return roleStr, args.Error(1)
}

// Changed return type from []types.TripMember to []types.TripMembership
func (m *MockTripStore) GetTripMembers(ctx context.Context, tripID string) ([]types.TripMembership, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	// Changed type assertion to []types.TripMembership
	members, ok := args.Get(0).([]types.TripMembership)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("Mock GetTripMembers returned unexpected type: %T", args.Get(0)))
	}
	return members, args.Error(1)
}

// MockEventPublisher implements events.Service
type MockEventPublisher struct {
	events.Service
	mock.Mock
}

func NewMockEventPublisher() *MockEventPublisher {
	return &MockEventPublisher{
		Service: *events.NewService(),
	}
}

func (m *MockEventPublisher) RegisterHandler(name string, handler types.EventHandler) error {
	args := m.Called(name, handler)
	return args.Error(0)
}

func (m *MockEventPublisher) UnregisterHandler(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *MockEventPublisher) Publish(ctx context.Context, tripID string, event types.Event) error {
	args := m.Called(ctx, tripID, event)
	return args.Error(0)
}

func (m *MockEventPublisher) Subscribe(ctx context.Context, tripID string, userID string, filters ...types.EventType) (<-chan types.Event, error) {
	args := m.Called(ctx, tripID, userID, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	ch, ok := args.Get(0).(<-chan types.Event)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("Mock Subscribe returned unexpected type: %T", args.Get(0)))
	}
	return ch, args.Error(1)
}

func (m *MockEventPublisher) Unsubscribe(ctx context.Context, tripID string, userID string) error {
	args := m.Called(ctx, tripID, userID)
	return args.Error(0)
}

// MockWebSocketConnection implements the services.WebSocketConnection interface for testing
type MockWebSocketConnection struct {
	mock.Mock
	// No need to embed *middleware.SafeConn anymore
	writeMu sync.Mutex // Keep mutex if needed for concurrent test scenarios
}

func (m *MockWebSocketConnection) WriteMessage(messageType int, data []byte) error {
	m.writeMu.Lock()
	defer m.writeMu.Unlock()
	args := m.Called(messageType, data)
	return args.Error(0)
}

func (m *MockWebSocketConnection) Close() error {
	args := m.Called()
	return args.Error(0)
}

// --- Test Suite Setup ---
type ChatServiceTestSuite struct {
	chatService   *chatservice.ChatService
	mockChatStore *MockChatStore
	mockTripStore *MockTripStore
	mockPublisher *MockEventPublisher
	ctx           context.Context
}

func setupTestSuite(t *testing.T) *ChatServiceTestSuite {
	mockChatStore := new(MockChatStore)
	mockTripStore := new(MockTripStore)
	mockPublisher := NewMockEventPublisher()
	chatService := chatservice.NewChatService(mockChatStore, mockTripStore, &mockPublisher.Service)

	return &ChatServiceTestSuite{
		chatService:   chatService,
		mockChatStore: mockChatStore,
		mockTripStore: mockTripStore,
		mockPublisher: mockPublisher,
		ctx:           context.Background(),
	}
}

// --- Test Cases ---

func TestNewChatService(t *testing.T) {
	ts := setupTestSuite(t)
	assert.NotNil(t, ts.chatService)
}

// Example for SendChatMessage:
func TestSendChatMessage_Success(t *testing.T) {
	ts := setupTestSuite(t)
	testUserID := "user1"
	testTripID := "trip1"
	testMessageContent := "Hello World"
	testMessageID := "msg1"
	// now := time.Now() // Timestamp is not part of ChatMessage struct

	message := types.ChatMessage{
		UserID:  testUserID,
		TripID:  testTripID,
		Content: testMessageContent,
		// Timestamp: now, // Removed Timestamp field
	}
	userResponse := types.UserResponse{
		ID:          testUserID,
		Email:       "test@example.com",
		Username:    "testuser",
		FirstName:   "Test",
		LastName:    "User",
		AvatarURL:   "https://example.com/avatar.jpg",
		DisplayName: "Test User",
	}

	ts.mockTripStore.On("GetUserRole", ts.ctx, testTripID, testUserID).Return(types.MemberRoleAdmin, nil)
	ts.mockChatStore.On("CreateChatMessage", ts.ctx, message).Return(testMessageID, nil)

	// Mock Publish - Expect event to be published (using types.Event)
	// Capture the event argument to assert its contents
	ts.mockPublisher.On("Publish", ts.ctx, testTripID, mock.AnythingOfType("types.Event")).Return(nil).Run(func(args mock.Arguments) {
		event := args.Get(2).(types.Event)
		assert.Equal(t, types.EventTypeChatMessageSent, event.Type) // Use types.EventTypeChatMessageSent
		assert.Equal(t, testTripID, event.TripID)

		// Need to unmarshal payload to check contents
		var payload types.ChatMessageEvent
		err := json.Unmarshal(event.Payload, &payload)
		assert.NoError(t, err)

		assert.Equal(t, testMessageID, payload.MessageID)
		// assert.Equal(t, testTripID, payload.TripID) // Already checked in BaseEvent
		assert.Equal(t, testMessageContent, payload.Content)

		// Assert user details in the payload
		assert.Equal(t, testUserID, payload.User.ID)
		assert.Equal(t, "Test User", payload.User.Username)

		// Assert timestamp (optional, allow for slight variations if needed)
		// assert.WithinDuration(t, time.Now(), payload.Timestamp, time.Second * 5) // Event timestamp should be recent
	})

	msgID, err := ts.chatService.SendChatMessage(ts.ctx, message, userResponse)

	assert.NoError(t, err)
	assert.Equal(t, testMessageID, msgID)
	ts.mockTripStore.AssertExpectations(t)
	ts.mockChatStore.AssertExpectations(t)
	ts.mockPublisher.AssertExpectations(t)
}

func TestSendChatMessage_Success_FetchUserInfo(t *testing.T) {
	ts := setupTestSuite(t)
	testUserID := "user123"
	testTripID := "trip1"
	testMessageContent := "Hello World Fetch"
	testMessageID := "msg2"

	message := types.ChatMessage{
		UserID:  testUserID,
		TripID:  testTripID,
		Content: testMessageContent,
	}

	fetchedUser := &types.UserResponse{
		ID:          testUserID,
		Username:    "Fetched User",
		AvatarURL:   "https://example.com/avatar.jpg",
		DisplayName: "Fetched User",
	}

	ts.mockTripStore.On("GetUserRole", ts.ctx, testTripID, testUserID).Return(types.MemberRoleAdmin, nil)
	ts.mockChatStore.On("CreateChatMessage", ts.ctx, message).Return(testMessageID, nil)
	ts.mockChatStore.On("GetUserByID", ts.ctx, testUserID).Return(fetchedUser, nil)

	ts.mockPublisher.On("Publish", ts.ctx, testTripID, mock.AnythingOfType("types.Event")).Return(nil).Run(func(args mock.Arguments) {
		event := args.Get(2).(types.Event)
		assert.Equal(t, types.EventTypeChatMessageSent, event.Type)

		var payload types.ChatMessageEvent
		err := json.Unmarshal(event.Payload, &payload)
		assert.NoError(t, err)

		assert.Equal(t, testMessageID, payload.MessageID)
		assert.Equal(t, fetchedUser.ID, payload.User.ID)
		assert.Equal(t, fetchedUser.Username, payload.User.Username)
		assert.Equal(t, fetchedUser.AvatarURL, payload.User.AvatarURL)
	})

	msgID, err := ts.chatService.SendChatMessage(ts.ctx, message, types.UserResponse{})

	assert.NoError(t, err)
	assert.Equal(t, testMessageID, msgID)
	ts.mockTripStore.AssertExpectations(t)
	ts.mockChatStore.AssertExpectations(t)
	ts.mockPublisher.AssertExpectations(t)
}

func TestSendChatMessage_UserNotMember(t *testing.T) {
	ts := setupTestSuite(t)
	testUserID := "user1"
	testTripID := "trip1"
	testMessageContent := "Hello World"

	message := types.ChatMessage{
		UserID:  testUserID,
		TripID:  testTripID,
		Content: testMessageContent,
	}
	user := types.UserResponse{
		ID:       testUserID,
		Username: "Test User",
	}

	ts.mockTripStore.On("GetUserRole", ts.ctx, testTripID, testUserID).Return(types.MemberRoleNone, nil)

	msgID, err := ts.chatService.SendChatMessage(ts.ctx, message, user)

	assert.Error(t, err)
	assert.Empty(t, msgID)
	appErr, ok := err.(*errors.AppError)
	assert.True(t, ok)
	assert.Equal(t, errors.ForbiddenError, appErr.Type)

	ts.mockTripStore.AssertExpectations(t)
	ts.mockChatStore.AssertNotCalled(t, "CreateChatMessage", mock.Anything, mock.Anything)
	// Publisher mock takes (ctx, tripID, event)
	ts.mockPublisher.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything)
}

func TestSendChatMessage_CreateMessageError(t *testing.T) {
	ts := setupTestSuite(t)
	testUserID := "user1"
	testTripID := "trip1"
	testMessageContent := "Fail Create"
	// now := time.Now() // Timestamp not part of ChatMessage

	message := types.ChatMessage{
		UserID:  testUserID,
		TripID:  testTripID,
		Content: testMessageContent,
		// Timestamp: now, // Removed timestamp
	}
	user := types.UserResponse{ID: testUserID}
	dbError := errors.NewDatabaseError(fmt.Errorf("failed to insert"))

	ts.mockTripStore.On("GetUserRole", ts.ctx, testTripID, testUserID).Return(types.MemberRoleMember, nil)
	ts.mockChatStore.On("CreateChatMessage", ts.ctx, message).Return("", dbError)

	msgID, err := ts.chatService.SendChatMessage(ts.ctx, message, user)

	assert.Error(t, err)
	assert.Empty(t, msgID)
	appErr, ok := err.(*errors.AppError)
	assert.True(t, ok)
	assert.Equal(t, errors.DatabaseError, appErr.Type)

	ts.mockTripStore.AssertExpectations(t)
	ts.mockChatStore.AssertExpectations(t)
	// Publisher mock takes (ctx, tripID, event)
	ts.mockPublisher.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything)
}

// --- GetChatMessages Tests ---
func TestGetChatMessages_Success(t *testing.T) {
	ts := setupTestSuite(t)
	testTripID := "trip1"
	testUserID := "user1"
	params := types.PaginationParams{Limit: 10, Offset: 0}
	expectedMessages := []types.ChatMessage{
		{ID: "msg1", TripID: testTripID, UserID: "userA", Content: "Msg 1"},
		{ID: "msg2", TripID: testTripID, UserID: "userB", Content: "Msg 2"},
	}
	totalCount := 50

	ts.mockTripStore.On("GetUserRole", ts.ctx, testTripID, testUserID).Return(types.MemberRoleMember, nil)
	// Use ListTripMessages instead of GetChatMessages based on service implementation
	ts.mockChatStore.On("ListTripMessages", ts.ctx, testTripID, params.Limit, params.Offset).Return(expectedMessages, totalCount, nil)

	paginatedResponse, err := ts.chatService.GetChatMessages(ts.ctx, testTripID, testUserID, params)

	assert.NoError(t, err)
	assert.NotNil(t, paginatedResponse)
	// The service layer GetChatMessages now returns []ChatMessageWithUser
	// Need to adjust mock and assertion if that's intended, or fix service layer.
	// Assuming service returns PaginatedResponse with Data: []types.ChatMessage for now
	// based on GetChatMessages function signature in chat_service.go
	// Let's adapt the mock return and assertion for now.
	responseData, ok := paginatedResponse.Data.([]types.ChatMessage)
	assert.True(t, ok, "Response data is not []types.ChatMessage")
	assert.Equal(t, expectedMessages, responseData)
	assert.Equal(t, totalCount, paginatedResponse.Pagination.Total)
	assert.Equal(t, params.Limit, paginatedResponse.Pagination.Limit)
	assert.Equal(t, params.Offset, paginatedResponse.Pagination.Offset)

	ts.mockTripStore.AssertExpectations(t)
	ts.mockChatStore.AssertExpectations(t)
}

func TestGetChatMessages_UserNotMember(t *testing.T) {
	ts := setupTestSuite(t)
	testTripID := "trip1"
	testUserID := "user1"
	params := types.PaginationParams{Limit: 10, Offset: 0}

	ts.mockTripStore.On("GetUserRole", ts.ctx, testTripID, testUserID).Return(types.MemberRoleNone, nil)

	paginatedResponse, err := ts.chatService.GetChatMessages(ts.ctx, testTripID, testUserID, params)

	assert.Error(t, err)
	assert.Nil(t, paginatedResponse)
	appErr, ok := err.(*errors.AppError)
	assert.True(t, ok)
	assert.Equal(t, errors.ForbiddenError, appErr.Type)

	ts.mockTripStore.AssertExpectations(t)
	ts.mockChatStore.AssertNotCalled(t, "ListTripMessages", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestGetChatMessages_StoreError(t *testing.T) {
	ts := setupTestSuite(t)
	testTripID := "trip1"
	testUserID := "user1"
	params := types.PaginationParams{Limit: 10, Offset: 0}
	dbError := errors.NewDatabaseError(fmt.Errorf("db query failed"))

	ts.mockTripStore.On("GetUserRole", ts.ctx, testTripID, testUserID).Return(types.MemberRoleMember, nil)
	ts.mockChatStore.On("ListTripMessages", ts.ctx, testTripID, params.Limit, params.Offset).Return(nil, 0, dbError)

	paginatedResponse, err := ts.chatService.GetChatMessages(ts.ctx, testTripID, testUserID, params)

	assert.Error(t, err)
	assert.Nil(t, paginatedResponse)
	appErr, ok := err.(*errors.AppError)
	assert.True(t, ok)
	assert.Equal(t, errors.DatabaseError, appErr.Type)

	ts.mockTripStore.AssertExpectations(t)
	ts.mockChatStore.AssertExpectations(t)
}

// --- GetUserInfo Tests ---
func TestGetUserInfo_Success(t *testing.T) {
	ts := setupTestSuite(t)
	testUserID := "user123"

	expectedUser := &types.SupabaseUser{
		ID:    testUserID,
		Email: "test@example.com",
		UserMetadata: types.UserMetadata{
			Username:       "testuser",
			FirstName:      "Test",
			LastName:       "User",
			ProfilePicture: "https://example.com/avatar.jpg",
		},
	}

	ts.mockChatStore.On("GetUserByID", ts.ctx, testUserID).Return(expectedUser, nil)

	user, err := ts.chatService.GetUserByID(ts.ctx, testUserID)

	assert.NoError(t, err)
	assert.Equal(t, expectedUser, user)
	ts.mockChatStore.AssertExpectations(t)
}

func TestGetUserInfo_NotFound(t *testing.T) {
	ts := setupTestSuite(t)
	testUserID := "nonexistent"

	ts.mockChatStore.On("GetUserByID", ts.ctx, testUserID).Return(nil, errors.NotFound("User", testUserID))

	user, err := ts.chatService.GetUserByID(ts.ctx, testUserID)

	assert.Error(t, err)
	assert.Nil(t, user)
	appErr, ok := err.(*errors.AppError)
	assert.True(t, ok)
	assert.Equal(t, errors.NotFoundError, appErr.Type)
	ts.mockChatStore.AssertExpectations(t)
}

func TestGetUserInfo_DatabaseError(t *testing.T) {
	ts := setupTestSuite(t)
	testUserID := "user123"
	dbErr := fmt.Errorf("database connection error")

	ts.mockChatStore.On("GetUserByID", ts.ctx, testUserID).Return(nil, dbErr)

	user, err := ts.chatService.GetUserByID(ts.ctx, testUserID)

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Equal(t, dbErr, err)
	ts.mockChatStore.AssertExpectations(t)
}

// TODO: Add tests for CreateChatGroup, UpdateChatMessage, DeleteChatMessage, etc.
// Remember to mock SafeConn for WebSocket related tests if needed.

// --- CreateChatGroup Tests ---

func TestCreateChatGroup_Success(t *testing.T) {
	ts := setupTestSuite(t)
	testTripID := "trip1"
	testUserID := "user1"
	testGroupID := "group1"
	testGroupName := "Test Group"

	group := types.ChatGroup{
		TripID:    testTripID,
		Name:      testGroupName,
		CreatedBy: testUserID,
	}

	// Mock GetTrip - Success
	testTrip := &types.Trip{ID: testTripID, Name: "Test Trip"}
	ts.mockTripStore.On("GetTrip", ts.ctx, testTripID).Return(testTrip, nil)

	// Mock GetUserRole - User is member
	ts.mockTripStore.On("GetUserRole", ts.ctx, testTripID, testUserID).Return(types.MemberRoleAdmin, nil)

	// Mock CreateChatGroup - Success
	ts.mockChatStore.On("CreateChatGroup", ts.ctx, group).Return(testGroupID, nil)

	// Mock GetTripMembers - Success (Return some members)
	testMembers := []types.TripMembership{
		{UserID: "user1", Role: types.MemberRoleAdmin},
		{UserID: "user2", Role: types.MemberRoleMember},
	}
	ts.mockTripStore.On("GetTripMembers", ts.ctx, testTripID).Return(testMembers, nil)

	// Mock AddChatGroupMember - Success (Called for each member)
	ts.mockChatStore.On("AddChatGroupMember", ts.ctx, testGroupID, "user1").Return(nil)
	ts.mockChatStore.On("AddChatGroupMember", ts.ctx, testGroupID, "user2").Return(nil)

	// Call the method
	groupID, err := ts.chatService.CreateChatGroup(ts.ctx, group)

	// Assert results
	assert.NoError(t, err)
	assert.Equal(t, testGroupID, groupID)

	// Verify mocks
	ts.mockTripStore.AssertExpectations(t)
	ts.mockChatStore.AssertExpectations(t)
}

func TestCreateChatGroup_TripNotFound(t *testing.T) {
	ts := setupTestSuite(t)
	testTripID := "nonexistent_trip"
	testUserID := "user1"
	group := types.ChatGroup{TripID: testTripID, CreatedBy: testUserID}
	notFoundErr := errors.NotFound("trip", testTripID)

	// Mock GetTrip - Not Found
	ts.mockTripStore.On("GetTrip", ts.ctx, testTripID).Return(nil, notFoundErr)

	groupID, err := ts.chatService.CreateChatGroup(ts.ctx, group)

	assert.Error(t, err)
	assert.Empty(t, groupID)
	appErr, ok := err.(*errors.AppError)
	assert.True(t, ok)
	assert.Equal(t, errors.NotFoundError, appErr.Type)

	ts.mockTripStore.AssertExpectations(t)
	ts.mockChatStore.AssertNotCalled(t, "CreateChatGroup", mock.Anything, mock.Anything)
	ts.mockTripStore.AssertNotCalled(t, "GetUserRole", mock.Anything, mock.Anything, mock.Anything)
}

func TestCreateChatGroup_UserNotMember(t *testing.T) {
	ts := setupTestSuite(t)
	testTripID := "trip1"
	testUserID := "user_not_member"
	group := types.ChatGroup{TripID: testTripID, CreatedBy: testUserID}

	testTrip := &types.Trip{ID: testTripID}
	ts.mockTripStore.On("GetTrip", ts.ctx, testTripID).Return(testTrip, nil)
	// Mock GetUserRole - User not member
	ts.mockTripStore.On("GetUserRole", ts.ctx, testTripID, testUserID).Return(types.MemberRoleNone, nil)

	groupID, err := ts.chatService.CreateChatGroup(ts.ctx, group)

	assert.Error(t, err)
	assert.Empty(t, groupID)
	appErr, ok := err.(*errors.AppError)
	assert.True(t, ok)
	assert.Equal(t, errors.ForbiddenError, appErr.Type)

	ts.mockTripStore.AssertExpectations(t)
	ts.mockChatStore.AssertNotCalled(t, "CreateChatGroup", mock.Anything, mock.Anything)
	ts.mockTripStore.AssertNotCalled(t, "GetTripMembers", mock.Anything, mock.Anything)
}

func TestCreateChatGroup_CreateGroupError(t *testing.T) {
	ts := setupTestSuite(t)
	testTripID := "trip1"
	testUserID := "user1"
	group := types.ChatGroup{TripID: testTripID, CreatedBy: testUserID, Name: "Fail Group"}
	dbError := errors.NewDatabaseError(fmt.Errorf("create failed"))

	testTrip := &types.Trip{ID: testTripID}
	ts.mockTripStore.On("GetTrip", ts.ctx, testTripID).Return(testTrip, nil)
	ts.mockTripStore.On("GetUserRole", ts.ctx, testTripID, testUserID).Return(types.MemberRoleAdmin, nil)
	// Mock CreateChatGroup - Failure
	ts.mockChatStore.On("CreateChatGroup", ts.ctx, group).Return("", dbError)

	groupID, err := ts.chatService.CreateChatGroup(ts.ctx, group)

	assert.Error(t, err)
	assert.Empty(t, groupID)
	appErr, ok := err.(*errors.AppError)
	assert.True(t, ok)
	assert.Equal(t, errors.DatabaseError, appErr.Type)

	ts.mockTripStore.AssertExpectations(t)
	ts.mockChatStore.AssertExpectations(t)
	ts.mockTripStore.AssertNotCalled(t, "GetTripMembers", mock.Anything, mock.Anything)
	ts.mockChatStore.AssertNotCalled(t, "AddChatGroupMember", mock.Anything, mock.Anything, mock.Anything)
}

// --- UpdateChatMessage Tests ---

func TestUpdateChatMessage_Success(t *testing.T) {
	ts := setupTestSuite(t)
	testMessageID := "msg1"
	testUserID := "user1" // Assume this user is the author
	newContent := "Updated content"
	testTripID := "trip1"

	originalMessage := &types.ChatMessage{
		ID:      testMessageID,
		UserID:  testUserID,
		TripID:  testTripID,
		Content: "Original content",
	}

	// Mock GetChatMessage to return the original message (for ownership check)
	ts.mockChatStore.On("GetChatMessage", ts.ctx, testMessageID).Return(originalMessage, nil)
	// Mock UpdateChatMessage store call
	ts.mockChatStore.On("UpdateChatMessage", ts.ctx, testMessageID, newContent).Return(nil)

	// Mock Publish event (EventTypeChatMessageEdited)
	ts.mockPublisher.On("Publish", ts.ctx, testTripID, mock.AnythingOfType("types.Event")).Return(nil).Run(func(args mock.Arguments) {
		event := args.Get(2).(types.Event)
		assert.Equal(t, types.EventTypeChatMessageEdited, event.Type)

		var payload types.ChatMessageEvent // Use ChatMessageEvent for edited messages too
		err := json.Unmarshal(event.Payload, &payload)
		assert.NoError(t, err)

		assert.Equal(t, testMessageID, payload.MessageID)
		assert.Equal(t, newContent, payload.Content)
		assert.Equal(t, testUserID, payload.User.ID) // User who edited
	})

	// Call the service method
	err := ts.chatService.UpdateChatMessage(ts.ctx, testMessageID, testUserID, newContent)

	// Assert results
	assert.NoError(t, err)

	// Verify mocks
	ts.mockChatStore.AssertExpectations(t)
	ts.mockPublisher.AssertExpectations(t)
}

func TestUpdateChatMessage_NotFound(t *testing.T) {
	ts := setupTestSuite(t)
	testMessageID := "notfound_msg"
	testUserID := "user1"
	newContent := "Updated content"
	notFoundErr := errors.NotFound("chat message", testMessageID)

	// Mock GetChatMessage to return NotFound error
	ts.mockChatStore.On("GetChatMessage", ts.ctx, testMessageID).Return(nil, notFoundErr)

	err := ts.chatService.UpdateChatMessage(ts.ctx, testMessageID, testUserID, newContent)

	assert.Error(t, err)
	appErr, ok := err.(*errors.AppError)
	assert.True(t, ok)
	assert.Equal(t, errors.NotFoundError, appErr.Type)

	ts.mockChatStore.AssertExpectations(t)
	ts.mockChatStore.AssertNotCalled(t, "UpdateChatMessage", mock.Anything, mock.Anything, mock.Anything)
	ts.mockPublisher.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything)
}

func TestUpdateChatMessage_Forbidden(t *testing.T) {
	ts := setupTestSuite(t)
	testMessageID := "msg1"
	authorUserID := "user_author"
	updaterUserID := "user_updater" // Different user trying to update
	newContent := "Updated content"
	testTripID := "trip1"

	originalMessage := &types.ChatMessage{
		ID:     testMessageID,
		UserID: authorUserID, // Original author
		TripID: testTripID,
	}

	ts.mockChatStore.On("GetChatMessage", ts.ctx, testMessageID).Return(originalMessage, nil)

	err := ts.chatService.UpdateChatMessage(ts.ctx, testMessageID, updaterUserID, newContent)

	assert.Error(t, err)
	appErr, ok := err.(*errors.AppError)
	assert.True(t, ok)
	assert.Equal(t, errors.ForbiddenError, appErr.Type)

	ts.mockChatStore.AssertExpectations(t)
	ts.mockChatStore.AssertNotCalled(t, "UpdateChatMessage", mock.Anything, mock.Anything, mock.Anything)
	ts.mockPublisher.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything)
}

func TestUpdateChatMessage_StoreUpdateError(t *testing.T) {
	ts := setupTestSuite(t)
	testMessageID := "msg1"
	testUserID := "user1"
	newContent := "Updated content"
	testTripID := "trip1"
	dbError := errors.NewDatabaseError(fmt.Errorf("update failed"))

	originalMessage := &types.ChatMessage{ID: testMessageID, UserID: testUserID, TripID: testTripID}
	ts.mockChatStore.On("GetChatMessage", ts.ctx, testMessageID).Return(originalMessage, nil)
	// Mock UpdateChatMessage to return an error
	ts.mockChatStore.On("UpdateChatMessage", ts.ctx, testMessageID, newContent).Return(dbError)

	err := ts.chatService.UpdateChatMessage(ts.ctx, testMessageID, testUserID, newContent)

	assert.Error(t, err)
	appErr, ok := err.(*errors.AppError)
	assert.True(t, ok)
	assert.Equal(t, errors.DatabaseError, appErr.Type)

	ts.mockChatStore.AssertExpectations(t)
	ts.mockPublisher.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything)
}

// --- DeleteChatMessage Tests ---

func TestDeleteChatMessage_Success_Author(t *testing.T) {
	ts := setupTestSuite(t)
	testMessageID := "msg_to_delete"
	testUserID := "author1" // Author deleting their own message
	testTripID := "trip1"

	messageToDelete := &types.ChatMessage{ID: testMessageID, UserID: testUserID, TripID: testTripID}
	ts.mockChatStore.On("GetChatMessage", ts.ctx, testMessageID).Return(messageToDelete, nil)
	ts.mockChatStore.On("DeleteChatMessage", ts.ctx, testMessageID).Return(nil)

	// Mock Publish event (EventTypeChatMessageDeleted)
	ts.mockPublisher.On("Publish", ts.ctx, testTripID, mock.AnythingOfType("types.Event")).Return(nil).Run(func(args mock.Arguments) {
		event := args.Get(2).(types.Event)
		assert.Equal(t, types.EventTypeChatMessageDeleted, event.Type)

		var payload types.ChatMessageEvent // Payload might just contain IDs
		err := json.Unmarshal(event.Payload, &payload)
		assert.NoError(t, err)

		assert.Equal(t, testMessageID, payload.MessageID)
		assert.Equal(t, testUserID, payload.User.ID) // User who deleted
	})

	err := ts.chatService.DeleteChatMessage(ts.ctx, testMessageID, testUserID)

	assert.NoError(t, err)
	ts.mockChatStore.AssertExpectations(t)
	ts.mockPublisher.AssertExpectations(t)
}

func TestDeleteChatMessage_Success_Admin(t *testing.T) {
	ts := setupTestSuite(t)
	testMessageID := "msg_to_delete"
	authorUserID := "author1"
	adminUserID := "admin1" // Admin deleting someone else's message
	testTripID := "trip1"

	messageToDelete := &types.ChatMessage{ID: testMessageID, UserID: authorUserID, TripID: testTripID}
	ts.mockChatStore.On("GetChatMessage", ts.ctx, testMessageID).Return(messageToDelete, nil)
	// Mock GetUserRole for the admin check
	ts.mockTripStore.On("GetUserRole", ts.ctx, testTripID, adminUserID).Return(types.MemberRoleAdmin, nil)
	ts.mockChatStore.On("DeleteChatMessage", ts.ctx, testMessageID).Return(nil)

	ts.mockPublisher.On("Publish", ts.ctx, testTripID, mock.AnythingOfType("types.Event")).Return(nil).Run(func(args mock.Arguments) {
		event := args.Get(2).(types.Event)
		assert.Equal(t, types.EventTypeChatMessageDeleted, event.Type)
		var payload types.ChatMessageEvent
		err := json.Unmarshal(event.Payload, &payload)
		assert.NoError(t, err)
		assert.Equal(t, testMessageID, payload.MessageID)
		assert.Equal(t, adminUserID, payload.User.ID) // User performing deletion is admin
	})

	err := ts.chatService.DeleteChatMessage(ts.ctx, testMessageID, adminUserID)

	assert.NoError(t, err)
	ts.mockChatStore.AssertExpectations(t)
	ts.mockTripStore.AssertExpectations(t)
	ts.mockPublisher.AssertExpectations(t)
}

func TestDeleteChatMessage_NotFound(t *testing.T) {
	ts := setupTestSuite(t)
	testMessageID := "notfound_msg"
	testUserID := "user1"
	notFoundErr := errors.NotFound("chat message", testMessageID)

	ts.mockChatStore.On("GetChatMessage", ts.ctx, testMessageID).Return(nil, notFoundErr)

	err := ts.chatService.DeleteChatMessage(ts.ctx, testMessageID, testUserID)

	assert.Error(t, err)
	appErr, ok := err.(*errors.AppError)
	assert.True(t, ok)
	assert.Equal(t, errors.NotFoundError, appErr.Type)

	ts.mockChatStore.AssertExpectations(t)
	ts.mockTripStore.AssertNotCalled(t, "GetUserRole", mock.Anything, mock.Anything, mock.Anything)
	ts.mockChatStore.AssertNotCalled(t, "DeleteChatMessage", mock.Anything, mock.Anything)
	ts.mockPublisher.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything)
}

func TestDeleteChatMessage_Forbidden(t *testing.T) {
	ts := setupTestSuite(t)
	testMessageID := "msg1"
	authorUserID := "author1"
	deleterUserID := "other_user" // Non-author, non-admin
	testTripID := "trip1"

	messageToDelete := &types.ChatMessage{ID: testMessageID, UserID: authorUserID, TripID: testTripID}
	ts.mockChatStore.On("GetChatMessage", ts.ctx, testMessageID).Return(messageToDelete, nil)
	// Mock GetUserRole - user is just a member
	ts.mockTripStore.On("GetUserRole", ts.ctx, testTripID, deleterUserID).Return(types.MemberRoleMember, nil)

	err := ts.chatService.DeleteChatMessage(ts.ctx, testMessageID, deleterUserID)

	assert.Error(t, err)
	appErr, ok := err.(*errors.AppError)
	assert.True(t, ok)
	assert.Equal(t, errors.ForbiddenError, appErr.Type)

	ts.mockChatStore.AssertExpectations(t)
	ts.mockTripStore.AssertExpectations(t)
	ts.mockChatStore.AssertNotCalled(t, "DeleteChatMessage", mock.Anything, mock.Anything)
	ts.mockPublisher.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything)
}

// --- AddReaction Tests ---

func TestAddReaction_Success(t *testing.T) {
	ts := setupTestSuite(t)
	testMessageID := "msg1"
	testUserID := "user1"
	testReaction := "👍"
	testTripID := "trip1"

	message := &types.ChatMessage{ID: testMessageID, TripID: testTripID} // Need TripID for event
	ts.mockChatStore.On("GetChatMessage", ts.ctx, testMessageID).Return(message, nil)
	ts.mockTripStore.On("GetUserRole", ts.ctx, testTripID, testUserID).Return(types.MemberRoleMember, nil)
	ts.mockChatStore.On("AddChatMessageReaction", ts.ctx, testMessageID, testUserID, testReaction).Return(nil)

	ts.mockPublisher.On("Publish", ts.ctx, testTripID, mock.AnythingOfType("types.Event")).Return(nil).Run(func(args mock.Arguments) {
		event := args.Get(2).(types.Event)
		assert.Equal(t, types.EventTypeChatReactionAdded, event.Type)
		var payload types.ChatReactionEvent
		err := json.Unmarshal(event.Payload, &payload)
		assert.NoError(t, err)
		assert.Equal(t, testMessageID, payload.MessageID)
		assert.Equal(t, testUserID, payload.User.ID)
		assert.Equal(t, testReaction, payload.Reaction)
	})

	err := ts.chatService.AddReaction(ts.ctx, testMessageID, testUserID, testReaction)

	assert.NoError(t, err)
	ts.mockChatStore.AssertExpectations(t)
	ts.mockTripStore.AssertExpectations(t)
	ts.mockPublisher.AssertExpectations(t)
}

func TestAddReaction_MessageNotFound(t *testing.T) {
	ts := setupTestSuite(t)
	testMessageID := "notfound_msg"
	testUserID := "user1"
	testReaction := "👍"
	notFoundErr := errors.NotFound("chat message", testMessageID)

	ts.mockChatStore.On("GetChatMessage", ts.ctx, testMessageID).Return(nil, notFoundErr)

	err := ts.chatService.AddReaction(ts.ctx, testMessageID, testUserID, testReaction)

	assert.Error(t, err)
	appErr, ok := err.(*errors.AppError)
	assert.True(t, ok)
	assert.Equal(t, errors.NotFoundError, appErr.Type)

	ts.mockChatStore.AssertExpectations(t)
	ts.mockTripStore.AssertNotCalled(t, "GetUserRole", mock.Anything, mock.Anything, mock.Anything)
	ts.mockChatStore.AssertNotCalled(t, "AddChatMessageReaction", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	ts.mockPublisher.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything)
}

func TestAddReaction_UserNotMember(t *testing.T) {
	ts := setupTestSuite(t)
	testMessageID := "msg1"
	testUserID := "user_not_member"
	testReaction := "👍"
	testTripID := "trip1"

	message := &types.ChatMessage{ID: testMessageID, TripID: testTripID}
	ts.mockChatStore.On("GetChatMessage", ts.ctx, testMessageID).Return(message, nil)
	ts.mockTripStore.On("GetUserRole", ts.ctx, testTripID, testUserID).Return(types.MemberRoleNone, nil)

	err := ts.chatService.AddReaction(ts.ctx, testMessageID, testUserID, testReaction)

	assert.Error(t, err)
	appErr, ok := err.(*errors.AppError)
	assert.True(t, ok)
	assert.Equal(t, errors.ForbiddenError, appErr.Type)

	ts.mockChatStore.AssertExpectations(t)
	ts.mockTripStore.AssertExpectations(t)
	ts.mockChatStore.AssertNotCalled(t, "AddChatMessageReaction", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	ts.mockPublisher.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything)
}

// --- RemoveReaction Tests ---

func TestRemoveReaction_Success(t *testing.T) {
	ts := setupTestSuite(t)
	testMessageID := "msg1"
	testUserID := "user1"
	testReaction := "👍"
	testTripID := "trip1"

	message := &types.ChatMessage{ID: testMessageID, TripID: testTripID} // Need TripID for event
	ts.mockChatStore.On("GetChatMessage", ts.ctx, testMessageID).Return(message, nil)
	ts.mockChatStore.On("RemoveChatMessageReaction", ts.ctx, testMessageID, testUserID, testReaction).Return(nil)

	ts.mockPublisher.On("Publish", ts.ctx, testTripID, mock.AnythingOfType("types.Event")).Return(nil).Run(func(args mock.Arguments) {
		event := args.Get(2).(types.Event)
		assert.Equal(t, types.EventTypeChatReactionRemoved, event.Type)
		var payload types.ChatReactionEvent
		err := json.Unmarshal(event.Payload, &payload)
		assert.NoError(t, err)
		assert.Equal(t, testMessageID, payload.MessageID)
		assert.Equal(t, testUserID, payload.User.ID)
		assert.Equal(t, testReaction, payload.Reaction)
	})

	err := ts.chatService.RemoveReaction(ts.ctx, testMessageID, testUserID, testReaction)

	assert.NoError(t, err)
	ts.mockChatStore.AssertExpectations(t)
	ts.mockPublisher.AssertExpectations(t)
}

func TestRemoveReaction_MessageNotFound(t *testing.T) {
	ts := setupTestSuite(t)
	testMessageID := "notfound_msg"
	testUserID := "user1"
	testReaction := "👍"
	notFoundErr := errors.NotFound("chat message", testMessageID)

	ts.mockChatStore.On("GetChatMessage", ts.ctx, testMessageID).Return(nil, notFoundErr)

	err := ts.chatService.RemoveReaction(ts.ctx, testMessageID, testUserID, testReaction)

	assert.Error(t, err)
	appErr, ok := err.(*errors.AppError)
	assert.True(t, ok)
	assert.Equal(t, errors.NotFoundError, appErr.Type)

	ts.mockChatStore.AssertExpectations(t)
	ts.mockChatStore.AssertNotCalled(t, "RemoveChatMessageReaction", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	ts.mockPublisher.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything)
}

// --- Connection Management Tests (Reinstated and updated) ---

func TestRegisterConnection(t *testing.T) {
	ts := setupTestSuite(t)
	testTripID := "trip1"
	testUserID := "user1"
	// Use the new MockWebSocketConnection
	mockConn := new(MockWebSocketConnection)
	welcomeMsg := types.WebSocketChatMessage{
		Type:    types.WebSocketMessageTypeInfo,
		Message: "Connected to chat",
	}
	welcomeJSON, _ := json.Marshal(welcomeMsg)

	// Expect WriteMessage to be called on the mock
	mockConn.On("WriteMessage", websocket.TextMessage, welcomeJSON).Return(nil)

	// Pass the mock implementing the interface
	ts.chatService.RegisterConnection(testTripID, testUserID, mockConn)

	// Verify mock interaction
	mockConn.AssertExpectations(t)

	// Clean up by unregistering
	// Expect Close() to be called on the mock during unregister
	mockConn.On("Close").Return(nil)
	ts.chatService.UnregisterConnection(testTripID, testUserID)
	mockConn.AssertExpectations(t) // Verify Close was called
}

func TestUnregisterConnection(t *testing.T) {
	ts := setupTestSuite(t)
	testTripID := "trip1"
	testUserID := "user1"
	// Use the new MockWebSocketConnection
	mockConn := new(MockWebSocketConnection)
	welcomeMsg := types.WebSocketChatMessage{Type: types.WebSocketMessageTypeInfo, Message: "Connected to chat"}
	welcomeJSON, _ := json.Marshal(welcomeMsg)

	// Register first
	mockConn.On("WriteMessage", websocket.TextMessage, welcomeJSON).Return(nil).Once()
	ts.chatService.RegisterConnection(testTripID, testUserID, mockConn)
	mockConn.AssertCalled(t, "WriteMessage", websocket.TextMessage, welcomeJSON)

	// Now unregister - expect Close to be called
	mockConn.On("Close").Return(nil).Once()
	ts.chatService.UnregisterConnection(testTripID, testUserID)
	mockConn.AssertCalled(t, "Close")

	// Verify broadcast doesn't try to write to the closed connection
	broadcastMsg := types.WebSocketChatMessage{Type: types.WebSocketMessageTypeChat, Content: "Test Broadcast"}
	ts.chatService.BroadcastMessage(ts.ctx, broadcastMsg, testTripID, "") // Should not panic

	// Assert WriteMessage was only called once (during registration)
	mockConn.AssertNumberOfCalls(t, "WriteMessage", 1)
	mockConn.AssertExpectations(t)
}

func TestUnregisterConnection_TripNotFound(t *testing.T) {
	ts := setupTestSuite(t)
	// Try unregistering from a trip that doesn't exist in the map
	ts.chatService.UnregisterConnection("non_existent_trip", "user1")
	// No error expected, should be a no-op
	assert.True(t, true) // Indicate test passed if no panic
}

func TestUnregisterConnection_UserNotFoundInTrip(t *testing.T) {
	ts := setupTestSuite(t)
	testTripID := "trip1"
	registeredUserID := "user1"
	unregisteredUserID := "user2"
	// Use the new MockWebSocketConnection
	mockConnUser1 := new(MockWebSocketConnection)
	welcomeMsg := types.WebSocketChatMessage{Type: types.WebSocketMessageTypeInfo, Message: "Connected to chat"}
	welcomeJSON, _ := json.Marshal(welcomeMsg)

	// Register user1
	mockConnUser1.On("WriteMessage", websocket.TextMessage, welcomeJSON).Return(nil).Once()
	ts.chatService.RegisterConnection(testTripID, registeredUserID, mockConnUser1)
	mockConnUser1.AssertCalled(t, "WriteMessage", websocket.TextMessage, welcomeJSON)

	// Try unregistering user2 (who isn't registered in this trip) - should be no-op, no Close called
	ts.chatService.UnregisterConnection(testTripID, unregisteredUserID)
	mockConnUser1.AssertNotCalled(t, "Close") // Ensure user1's conn wasn't closed

	// Clean up user1 - expect Close to be called now
	mockConnUser1.On("Close").Return(nil).Once()
	ts.chatService.UnregisterConnection(testTripID, registeredUserID)
	mockConnUser1.AssertCalled(t, "Close")
	mockConnUser1.AssertExpectations(t)
}

// --- BroadcastMessage Tests ---

func TestBroadcastMessage_Success(t *testing.T) {
	ts := setupTestSuite(t)
	testTripID := "trip1"
	user1ID := "user1"
	user2ID := "user2"
	excludeUserID := ""

	mockConn1 := new(MockWebSocketConnection)
	mockConn2 := new(MockWebSocketConnection)

	// Register connections first
	mockConn1.On("WriteMessage", mock.AnythingOfType("int"), mock.AnythingOfType("[]uint8")).Return(nil).Once() // Welcome message
	ts.chatService.RegisterConnection(testTripID, user1ID, mockConn1)
	mockConn2.On("WriteMessage", mock.AnythingOfType("int"), mock.AnythingOfType("[]uint8")).Return(nil).Once() // Welcome message
	ts.chatService.RegisterConnection(testTripID, user2ID, mockConn2)

	// Prepare broadcast message
	broadcastPayload := types.WebSocketChatMessage{
		Type:    types.WebSocketMessageTypeChat,
		TripID:  testTripID,
		Content: "Broadcast test!",
	}
	broadcastJSON, _ := json.Marshal(broadcastPayload)

	// Expect WriteMessage to be called for both connections
	mockConn1.On("WriteMessage", websocket.TextMessage, broadcastJSON).Return(nil).Once()
	mockConn2.On("WriteMessage", websocket.TextMessage, broadcastJSON).Return(nil).Once()

	// Call BroadcastMessage
	ts.chatService.BroadcastMessage(ts.ctx, broadcastPayload, testTripID, excludeUserID)

	// Assert expectations
	mockConn1.AssertExpectations(t)
	mockConn2.AssertExpectations(t)

	// Clean up
	mockConn1.On("Close").Return(nil)
	mockConn2.On("Close").Return(nil)
	ts.chatService.UnregisterConnection(testTripID, user1ID)
	ts.chatService.UnregisterConnection(testTripID, user2ID)
}

func TestBroadcastMessage_ExcludeUser(t *testing.T) {
	ts := setupTestSuite(t)
	testTripID := "trip1"
	user1ID := "user1"
	user2ID := "user2"
	excludeUserID := user1ID // Exclude user1

	mockConn1 := new(MockWebSocketConnection)
	mockConn2 := new(MockWebSocketConnection)

	// Register connections
	mockConn1.On("WriteMessage", mock.AnythingOfType("int"), mock.AnythingOfType("[]uint8")).Return(nil).Once() // Welcome msg user1
	ts.chatService.RegisterConnection(testTripID, user1ID, mockConn1)
	mockConn2.On("WriteMessage", mock.AnythingOfType("int"), mock.AnythingOfType("[]uint8")).Return(nil).Once() // Welcome msg user2
	ts.chatService.RegisterConnection(testTripID, user2ID, mockConn2)

	// Prepare broadcast message
	broadcastPayload := types.WebSocketChatMessage{
		Type:    types.WebSocketMessageTypeChat,
		TripID:  testTripID,
		Content: "Broadcast excluding user1!",
	}
	broadcastJSON, _ := json.Marshal(broadcastPayload)

	// Expect WriteMessage only for user2 (user1 is excluded)
	mockConn2.On("WriteMessage", websocket.TextMessage, broadcastJSON).Return(nil).Once()

	// Call BroadcastMessage
	ts.chatService.BroadcastMessage(ts.ctx, broadcastPayload, testTripID, excludeUserID)

	// Assert expectations
	mockConn1.AssertNotCalled(t, "WriteMessage", websocket.TextMessage, broadcastJSON) // user1 should NOT receive broadcast
	mockConn2.AssertExpectations(t)                                                    // user2 SHOULD receive broadcast

	// Clean up
	mockConn1.On("Close").Return(nil)
	mockConn2.On("Close").Return(nil)
	ts.chatService.UnregisterConnection(testTripID, user1ID)
	ts.chatService.UnregisterConnection(testTripID, user2ID)
}

func TestBroadcastMessage_TripNotFound(t *testing.T) {
	ts := setupTestSuite(t)
	// Try broadcasting to a trip with no registered connections
	broadcastPayload := types.WebSocketChatMessage{Type: types.WebSocketMessageTypeChat, Content: "Nobody hears this"}
	ts.chatService.BroadcastMessage(ts.ctx, broadcastPayload, "non_existent_trip", "")
	// No mocks to assert, just ensure no error/panic occurs
	assert.True(t, true)
}

func TestBroadcastMessage_WriteError(t *testing.T) {
	ts := setupTestSuite(t)
	testTripID := "trip1"
	user1ID := "user1"
	user2ID := "user2_error"

	mockConn1 := new(MockWebSocketConnection)
	mockConn2 := new(MockWebSocketConnection)

	// Register connections
	mockConn1.On("WriteMessage", mock.Anything, mock.Anything).Return(nil) // Welcome + broadcast
	ts.chatService.RegisterConnection(testTripID, user1ID, mockConn1)
	mockConn2.On("WriteMessage", mock.Anything, mock.Anything).Return(nil).Once() // Welcome only
	ts.chatService.RegisterConnection(testTripID, user2ID, mockConn2)

	// Prepare broadcast message
	broadcastPayload := types.WebSocketChatMessage{Type: types.WebSocketMessageTypeChat, Content: "Broadcast with error"}
	broadcastJSON, _ := json.Marshal(broadcastPayload)

	// Expect WriteMessage for both, but user2 returns an error
	mockConn1.On("WriteMessage", websocket.TextMessage, broadcastJSON).Return(nil).Once()
	mockConn2.On("WriteMessage", websocket.TextMessage, broadcastJSON).Return(fmt.Errorf("simulated write error")).Once()

	// Call BroadcastMessage
	ts.chatService.BroadcastMessage(ts.ctx, broadcastPayload, testTripID, "")

	// Assert expectations - both should have been called
	mockConn1.AssertExpectations(t)
	mockConn2.AssertExpectations(t)

	// Clean up
	mockConn1.On("Close").Return(nil)
	mockConn2.On("Close").Return(nil)
	ts.chatService.UnregisterConnection(testTripID, user1ID)
	ts.chatService.UnregisterConnection(testTripID, user2ID)
}

// TODO: Add tests for HandleWebSocketMessage (If it exists/is implemented)
