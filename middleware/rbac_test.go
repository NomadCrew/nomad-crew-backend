package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

type MockTripModel struct {
	mock.Mock
}

// Mock implementation of TripModelInterface methods
func (m *MockTripModel) GetUserRole(ctx context.Context, tripID string, userID string) (types.MemberRole, error) {
	args := m.Called(ctx, tripID, userID)
	return args.Get(0).(types.MemberRole), args.Error(1)
}

func (m *MockTripModel) CreateTrip(ctx context.Context, trip *types.Trip) (*types.Trip, error) {
	args := m.Called(ctx, trip)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Trip), args.Error(1)
}

func (m *MockTripModel) GetTripByID(ctx context.Context, id string, userID string) (*types.Trip, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Trip), args.Error(1)
}

func (m *MockTripModel) UpdateTrip(ctx context.Context, id string, userID string, update *types.TripUpdate) (*types.Trip, error) {
	args := m.Called(ctx, id, userID, update)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Trip), args.Error(1)
}

func (m *MockTripModel) DeleteTrip(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTripModel) ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*types.Trip), args.Error(1)
}

func (m *MockTripModel) SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error) {
	args := m.Called(ctx, criteria)
	return args.Get(0).([]*types.Trip), args.Error(1)
}

func (m *MockTripModel) AddMember(ctx context.Context, membership *types.TripMembership) error {
	args := m.Called(ctx, membership)
	return args.Error(0)
}

func (m *MockTripModel) UpdateMemberRole(ctx context.Context, tripID, userID string, role types.MemberRole) (*interfaces.CommandResult, error) {
	args := m.Called(ctx, tripID, userID, role)
	return args.Get(0).(*interfaces.CommandResult), args.Error(1)
}

func (m *MockTripModel) RemoveMember(ctx context.Context, tripID, userID string) error {
	args := m.Called(ctx, tripID, userID)
	return args.Error(0)
}

func (m *MockTripModel) CreateInvitation(ctx context.Context, invitation *types.TripInvitation) error {
	args := m.Called(ctx, invitation)
	return args.Error(0)
}

func (m *MockTripModel) GetInvitation(ctx context.Context, invitationID string) (*types.TripInvitation, error) {
	args := m.Called(ctx, invitationID)
	return args.Get(0).(*types.TripInvitation), args.Error(1)
}

func (m *MockTripModel) UpdateInvitationStatus(ctx context.Context, invitationID string, status types.InvitationStatus) error {
	args := m.Called(ctx, invitationID, status)
	return args.Error(0)
}

func (m *MockTripModel) LookupUserByEmail(ctx context.Context, email string) (*types.SupabaseUser, error) {
	args := m.Called(ctx, email)
	return args.Get(0).(*types.SupabaseUser), args.Error(1)
}

func (m *MockTripModel) GetTripWithMembers(ctx context.Context, tripID string, userID string) (*types.TripWithMembers, error) {
	args := m.Called(ctx, tripID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.TripWithMembers), args.Error(1)
}

// GetCommandContext returns the command context for testing
func (m *MockTripModel) GetCommandContext() *interfaces.CommandContext {
	args := m.Called()
	if args.Get(0) == nil {
		return &interfaces.CommandContext{
			RequestData: &sync.Map{},
		}
	}
	return args.Get(0).(*interfaces.CommandContext)
}

// UpdateTripStatus mock implementation
func (m *MockTripModel) UpdateTripStatus(ctx context.Context, tripID string, newStatus types.TripStatus) error {
	args := m.Called(ctx, tripID, newStatus)
	return args.Error(0)
}

// GetTripMembers mock implementation
func (m *MockTripModel) GetTripMembers(ctx context.Context, tripID string) ([]types.TripMembership, error) {
	args := m.Called(ctx, tripID)
	return args.Get(0).([]types.TripMembership), args.Error(1)
}

// InviteMember mock implementation
func (m *MockTripModel) InviteMember(ctx context.Context, invitation *types.TripInvitation) error {
	args := m.Called(ctx, invitation)
	return args.Error(0)
}

// FindInvitationByTripAndEmail mock implementation
func (m *MockTripModel) FindInvitationByTripAndEmail(ctx context.Context, tripID, email string) (*types.TripInvitation, error) {
	args := m.Called(ctx, tripID, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.TripInvitation), args.Error(1)
}

// GetTrip mock implementation
func (m *MockTripModel) GetTrip(ctx context.Context, tripID string) (*types.Trip, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Trip), args.Error(1)
}

func TestRequireRole(t *testing.T) {
	// Use test logger configuration
	logger.IsTest = true
	logger.InitLogger()
	defer logger.Close()

	// Mocking dependencies
	mockTripModel := &MockTripModel{}
	gin.SetMode(gin.TestMode)

	t.Run("Owner can access owner resources", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest(http.MethodGet, "/v1/trips/trip-123", nil)
		c.Request = req

		c.Set("user_id", "user-123")
		c.Params = append(c.Params, gin.Param{Key: "id", Value: "trip-123"})

		// Mock behavior
		mockTripModel.On("GetUserRole", mock.Anything, "trip-123", "user-123").Return(types.MemberRoleOwner, nil).Once()

		middleware := RequireRole(mockTripModel, types.MemberRoleOwner)
		middleware(c)

		assert.Equal(t, http.StatusOK, w.Code)
		mockTripModel.AssertExpectations(t)
	})

	t.Run("Member cannot access owner resources", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest(http.MethodGet, "/v1/trips/trip-123", nil)
		c.Request = req

		c.Set("user_id", "user-123")
		c.Params = append(c.Params, gin.Param{Key: "id", Value: "trip-123"})

		mockTripModel.On("GetUserRole", mock.Anything, "trip-123", "user-123").Return(types.MemberRoleMember, nil).Once()

		middleware := RequireRole(mockTripModel, types.MemberRoleOwner)
		middleware(c)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "User does not have access to this resource")
		mockTripModel.AssertExpectations(t)
	})

	t.Run("Missing trip ID", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest(http.MethodGet, "/v1/trips", nil)
		c.Request = req

		c.Set("user_id", "user-123")

		middleware := RequireRole(mockTripModel, types.MemberRoleOwner)
		middleware(c)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "User ID or Trip ID missing in request")
	})

	t.Run("Missing user ID", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest(http.MethodGet, "/v1/trips/trip-123", nil)
		c.Request = req

		c.Params = append(c.Params, gin.Param{Key: "id", Value: "trip-123"})

		middleware := RequireRole(mockTripModel, types.MemberRoleOwner)
		middleware(c)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "User ID or Trip ID missing in request")
	})
}

// =============================================================================
// RequirePermission Tests - Permission Matrix Based Authorization
// =============================================================================

func TestRequirePermission_TripResource(t *testing.T) {
	logger.IsTest = true
	logger.InitLogger()
	defer logger.Close()

	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		action         types.Action
		resource       types.Resource
		userRole       types.MemberRole
		expectedStatus int
		expectAllowed  bool
	}{
		// Trip Read - Any member can read
		{
			name:           "OWNER can read trip",
			action:         types.ActionRead,
			resource:       types.ResourceTrip,
			userRole:       types.MemberRoleOwner,
			expectedStatus: http.StatusOK,
			expectAllowed:  true,
		},
		{
			name:           "ADMIN can read trip",
			action:         types.ActionRead,
			resource:       types.ResourceTrip,
			userRole:       types.MemberRoleAdmin,
			expectedStatus: http.StatusOK,
			expectAllowed:  true,
		},
		{
			name:           "MEMBER can read trip",
			action:         types.ActionRead,
			resource:       types.ResourceTrip,
			userRole:       types.MemberRoleMember,
			expectedStatus: http.StatusOK,
			expectAllowed:  true,
		},
		// Trip Update - ADMIN+ can update
		{
			name:           "OWNER can update trip",
			action:         types.ActionUpdate,
			resource:       types.ResourceTrip,
			userRole:       types.MemberRoleOwner,
			expectedStatus: http.StatusOK,
			expectAllowed:  true,
		},
		{
			name:           "ADMIN can update trip",
			action:         types.ActionUpdate,
			resource:       types.ResourceTrip,
			userRole:       types.MemberRoleAdmin,
			expectedStatus: http.StatusOK,
			expectAllowed:  true,
		},
		{
			name:           "MEMBER cannot update trip",
			action:         types.ActionUpdate,
			resource:       types.ResourceTrip,
			userRole:       types.MemberRoleMember,
			expectedStatus: http.StatusForbidden,
			expectAllowed:  false,
		},
		// Trip Delete - Only OWNER can delete
		{
			name:           "OWNER can delete trip",
			action:         types.ActionDelete,
			resource:       types.ResourceTrip,
			userRole:       types.MemberRoleOwner,
			expectedStatus: http.StatusOK,
			expectAllowed:  true,
		},
		{
			name:           "ADMIN cannot delete trip",
			action:         types.ActionDelete,
			resource:       types.ResourceTrip,
			userRole:       types.MemberRoleAdmin,
			expectedStatus: http.StatusForbidden,
			expectAllowed:  false,
		},
		{
			name:           "MEMBER cannot delete trip",
			action:         types.ActionDelete,
			resource:       types.ResourceTrip,
			userRole:       types.MemberRoleMember,
			expectedStatus: http.StatusForbidden,
			expectAllowed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTripModel := &MockTripModel{}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			req, _ := http.NewRequest(http.MethodGet, "/v1/trips/trip-123", nil)
			c.Request = req
			c.Set("user_id", "user-123")
			c.Params = append(c.Params, gin.Param{Key: "id", Value: "trip-123"})

			mockTripModel.On("GetUserRole", mock.Anything, "trip-123", "user-123").Return(tt.userRole, nil).Once()

			middleware := RequirePermission(mockTripModel, tt.action, tt.resource, nil)
			middleware(c)

			assert.Equal(t, tt.expectedStatus, w.Code, "Expected status %d, got %d", tt.expectedStatus, w.Code)
			mockTripModel.AssertExpectations(t)

			if tt.expectAllowed {
				// Verify role was stored in context
				role, exists := c.Get(ContextKeyUserRole)
				assert.True(t, exists, "Role should be stored in context")
				assert.Equal(t, tt.userRole, role.(types.MemberRole))
			}
		})
	}
}

func TestRequirePermission_MemberResource(t *testing.T) {
	logger.IsTest = true
	logger.InitLogger()
	defer logger.Close()

	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		action         types.Action
		userRole       types.MemberRole
		expectedStatus int
	}{
		// Member Read - Any member can read member list
		{
			name:           "OWNER can read members",
			action:         types.ActionRead,
			userRole:       types.MemberRoleOwner,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "ADMIN can read members",
			action:         types.ActionRead,
			userRole:       types.MemberRoleAdmin,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "MEMBER can read members",
			action:         types.ActionRead,
			userRole:       types.MemberRoleMember,
			expectedStatus: http.StatusOK,
		},
		// Member Remove - Only OWNER can remove members
		{
			name:           "OWNER can remove members",
			action:         types.ActionRemove,
			userRole:       types.MemberRoleOwner,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "ADMIN cannot remove members",
			action:         types.ActionRemove,
			userRole:       types.MemberRoleAdmin,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "MEMBER cannot remove members",
			action:         types.ActionRemove,
			userRole:       types.MemberRoleMember,
			expectedStatus: http.StatusForbidden,
		},
		// Member ChangeRole - ADMIN+ can change roles
		{
			name:           "OWNER can change roles",
			action:         types.ActionChangeRole,
			userRole:       types.MemberRoleOwner,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "ADMIN can change roles",
			action:         types.ActionChangeRole,
			userRole:       types.MemberRoleAdmin,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "MEMBER cannot change roles",
			action:         types.ActionChangeRole,
			userRole:       types.MemberRoleMember,
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTripModel := &MockTripModel{}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			req, _ := http.NewRequest(http.MethodGet, "/v1/trips/trip-123/members", nil)
			c.Request = req
			c.Set("user_id", "user-123")
			c.Params = append(c.Params, gin.Param{Key: "id", Value: "trip-123"})

			mockTripModel.On("GetUserRole", mock.Anything, "trip-123", "user-123").Return(tt.userRole, nil).Once()

			middleware := RequirePermission(mockTripModel, tt.action, types.ResourceMember, nil)
			middleware(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockTripModel.AssertExpectations(t)
		})
	}
}

func TestRequirePermission_InvitationResource(t *testing.T) {
	logger.IsTest = true
	logger.InitLogger()
	defer logger.Close()

	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		action         types.Action
		userRole       types.MemberRole
		expectedStatus int
	}{
		// Invitation Create - ADMIN+ can create invitations
		{
			name:           "OWNER can create invitations",
			action:         types.ActionCreate,
			userRole:       types.MemberRoleOwner,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "ADMIN can create invitations",
			action:         types.ActionCreate,
			userRole:       types.MemberRoleAdmin,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "MEMBER cannot create invitations",
			action:         types.ActionCreate,
			userRole:       types.MemberRoleMember,
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTripModel := &MockTripModel{}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			req, _ := http.NewRequest(http.MethodPost, "/v1/trips/trip-123/invitations", nil)
			c.Request = req
			c.Set("user_id", "user-123")
			c.Params = append(c.Params, gin.Param{Key: "id", Value: "trip-123"})

			mockTripModel.On("GetUserRole", mock.Anything, "trip-123", "user-123").Return(tt.userRole, nil).Once()

			middleware := RequirePermission(mockTripModel, tt.action, types.ResourceInvitation, nil)
			middleware(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockTripModel.AssertExpectations(t)
		})
	}
}

func TestRequirePermission_TodoResource_WithOwnership(t *testing.T) {
	logger.IsTest = true
	logger.InitLogger()
	defer logger.Close()

	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		action         types.Action
		userRole       types.MemberRole
		isOwner        bool
		expectedStatus int
	}{
		// Todo Update - ADMIN+ can update any, MEMBER can update own
		{
			name:           "OWNER can update any todo",
			action:         types.ActionUpdate,
			userRole:       types.MemberRoleOwner,
			isOwner:        false,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "ADMIN can update any todo",
			action:         types.ActionUpdate,
			userRole:       types.MemberRoleAdmin,
			isOwner:        false,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "MEMBER can update own todo",
			action:         types.ActionUpdate,
			userRole:       types.MemberRoleMember,
			isOwner:        true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "MEMBER cannot update others todo",
			action:         types.ActionUpdate,
			userRole:       types.MemberRoleMember,
			isOwner:        false,
			expectedStatus: http.StatusForbidden,
		},
		// Todo Delete - ADMIN+ can delete any, MEMBER can delete own
		{
			name:           "OWNER can delete any todo",
			action:         types.ActionDelete,
			userRole:       types.MemberRoleOwner,
			isOwner:        false,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "ADMIN can delete any todo",
			action:         types.ActionDelete,
			userRole:       types.MemberRoleAdmin,
			isOwner:        false,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "MEMBER can delete own todo",
			action:         types.ActionDelete,
			userRole:       types.MemberRoleMember,
			isOwner:        true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "MEMBER cannot delete others todo",
			action:         types.ActionDelete,
			userRole:       types.MemberRoleMember,
			isOwner:        false,
			expectedStatus: http.StatusForbidden,
		},
		// Todo Create - Any member can create
		{
			name:           "MEMBER can create todo",
			action:         types.ActionCreate,
			userRole:       types.MemberRoleMember,
			isOwner:        false,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTripModel := &MockTripModel{}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			req, _ := http.NewRequest(http.MethodPut, "/v1/trips/trip-123/todos/todo-456", nil)
			c.Request = req
			c.Set("user_id", "user-123")
			c.Params = append(c.Params, gin.Param{Key: "id", Value: "trip-123"})
			c.Params = append(c.Params, gin.Param{Key: "todoID", Value: "todo-456"})

			mockTripModel.On("GetUserRole", mock.Anything, "trip-123", "user-123").Return(tt.userRole, nil).Once()

			// Owner ID extractor - simulates fetching todo owner from context
			var ownerExtractor OwnerIDExtractor
			if tt.isOwner {
				ownerExtractor = func(c *gin.Context) string {
					return "user-123" // Same as user_id
				}
			} else {
				ownerExtractor = func(c *gin.Context) string {
					return "other-user-456" // Different from user_id
				}
			}

			middleware := RequirePermission(mockTripModel, tt.action, types.ResourceTodo, ownerExtractor)
			middleware(c)

			assert.Equal(t, tt.expectedStatus, w.Code, "Test: %s", tt.name)
			mockTripModel.AssertExpectations(t)
		})
	}
}

func TestRequirePermission_LocationResource_OwnerOnly(t *testing.T) {
	logger.IsTest = true
	logger.InitLogger()
	defer logger.Close()

	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		action         types.Action
		userRole       types.MemberRole
		isOwner        bool
		expectedStatus int
	}{
		// Location Update - Only owner can update their own location
		{
			name:           "OWNER can update own location",
			action:         types.ActionUpdate,
			userRole:       types.MemberRoleOwner,
			isOwner:        true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "ADMIN cannot update others location",
			action:         types.ActionUpdate,
			userRole:       types.MemberRoleAdmin,
			isOwner:        false,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "MEMBER can update own location",
			action:         types.ActionUpdate,
			userRole:       types.MemberRoleMember,
			isOwner:        true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "MEMBER cannot update others location",
			action:         types.ActionUpdate,
			userRole:       types.MemberRoleMember,
			isOwner:        false,
			expectedStatus: http.StatusForbidden,
		},
		// Location Read - Any member can read
		{
			name:           "MEMBER can read locations",
			action:         types.ActionRead,
			userRole:       types.MemberRoleMember,
			isOwner:        false,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTripModel := &MockTripModel{}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			req, _ := http.NewRequest(http.MethodPut, "/v1/trips/trip-123/locations", nil)
			c.Request = req
			c.Set("user_id", "user-123")
			c.Params = append(c.Params, gin.Param{Key: "id", Value: "trip-123"})

			mockTripModel.On("GetUserRole", mock.Anything, "trip-123", "user-123").Return(tt.userRole, nil).Once()

			// Owner ID extractor for location - returns current user ID
			var ownerExtractor OwnerIDExtractor
			if tt.isOwner {
				ownerExtractor = func(c *gin.Context) string {
					return c.GetString("user_id") // Same as current user
				}
			} else {
				ownerExtractor = func(c *gin.Context) string {
					return "other-user-456" // Different user
				}
			}

			middleware := RequirePermission(mockTripModel, tt.action, types.ResourceLocation, ownerExtractor)
			middleware(c)

			assert.Equal(t, tt.expectedStatus, w.Code, "Test: %s", tt.name)
			mockTripModel.AssertExpectations(t)
		})
	}
}

func TestRequirePermission_ErrorCases(t *testing.T) {
	logger.IsTest = true
	logger.InitLogger()
	defer logger.Close()

	gin.SetMode(gin.TestMode)

	t.Run("Missing trip ID returns BadRequest", func(t *testing.T) {
		mockTripModel := &MockTripModel{}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest(http.MethodGet, "/v1/trips", nil)
		c.Request = req
		c.Set("user_id", "user-123")
		// No trip ID param

		middleware := RequirePermission(mockTripModel, types.ActionRead, types.ResourceTrip, nil)
		middleware(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Trip ID is required")
	})

	t.Run("Missing user ID returns Unauthorized", func(t *testing.T) {
		mockTripModel := &MockTripModel{}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest(http.MethodGet, "/v1/trips/trip-123", nil)
		c.Request = req
		// No user_id set
		c.Params = append(c.Params, gin.Param{Key: "id", Value: "trip-123"})

		middleware := RequirePermission(mockTripModel, types.ActionRead, types.ResourceTrip, nil)
		middleware(c)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Authentication required")
	})

	t.Run("Non-member returns Forbidden", func(t *testing.T) {
		mockTripModel := &MockTripModel{}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest(http.MethodGet, "/v1/trips/trip-123", nil)
		c.Request = req
		c.Set("user_id", "user-123")
		c.Params = append(c.Params, gin.Param{Key: "id", Value: "trip-123"})

		// Return error for non-member
		mockTripModel.On("GetUserRole", mock.Anything, "trip-123", "user-123").Return(types.MemberRole(""), assert.AnError).Once()

		middleware := RequirePermission(mockTripModel, types.ActionRead, types.ResourceTrip, nil)
		middleware(c)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "You are not a member of this trip")
		mockTripModel.AssertExpectations(t)
	})
}

// =============================================================================
// RequireTripMembership Tests - Lightweight Membership Check
// =============================================================================

func TestRequireTripMembership(t *testing.T) {
	logger.IsTest = true
	logger.InitLogger()
	defer logger.Close()

	gin.SetMode(gin.TestMode)

	t.Run("Member passes membership check", func(t *testing.T) {
		mockTripModel := &MockTripModel{}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest(http.MethodGet, "/v1/trips/trip-123/todos", nil)
		c.Request = req
		c.Set("user_id", "user-123")
		c.Params = append(c.Params, gin.Param{Key: "id", Value: "trip-123"})

		mockTripModel.On("GetUserRole", mock.Anything, "trip-123", "user-123").Return(types.MemberRoleMember, nil).Once()

		middleware := RequireTripMembership(mockTripModel)
		middleware(c)

		assert.Equal(t, http.StatusOK, w.Code)
		// Verify role is stored in context
		role, exists := c.Get(ContextKeyUserRole)
		assert.True(t, exists)
		assert.Equal(t, types.MemberRoleMember, role.(types.MemberRole))
		mockTripModel.AssertExpectations(t)
	})

	t.Run("Non-member fails membership check", func(t *testing.T) {
		mockTripModel := &MockTripModel{}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest(http.MethodGet, "/v1/trips/trip-123/todos", nil)
		c.Request = req
		c.Set("user_id", "user-123")
		c.Params = append(c.Params, gin.Param{Key: "id", Value: "trip-123"})

		mockTripModel.On("GetUserRole", mock.Anything, "trip-123", "user-123").Return(types.MemberRole(""), assert.AnError).Once()

		middleware := RequireTripMembership(mockTripModel)
		middleware(c)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "You are not a member of this trip")
		mockTripModel.AssertExpectations(t)
	})

	t.Run("Missing trip ID or user ID returns BadRequest", func(t *testing.T) {
		mockTripModel := &MockTripModel{}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest(http.MethodGet, "/v1/trips/todos", nil)
		c.Request = req
		// No user_id or trip ID

		middleware := RequireTripMembership(mockTripModel)
		middleware(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Trip ID and authentication required")
	})
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestGetUserRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Returns role when set", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ContextKeyUserRole, types.MemberRoleAdmin)

		role, exists := GetUserRole(c)

		assert.True(t, exists)
		assert.Equal(t, types.MemberRoleAdmin, role)
	})

	t.Run("Returns empty when not set", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		role, exists := GetUserRole(c)

		assert.False(t, exists)
		assert.Equal(t, types.MemberRole(""), role)
	})
}

func TestIsResourceOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Returns true when user is owner", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ContextKeyIsResourceOwner, true)

		assert.True(t, IsResourceOwner(c))
	})

	t.Run("Returns false when user is not owner", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ContextKeyIsResourceOwner, false)

		assert.False(t, IsResourceOwner(c))
	})

	t.Run("Returns false when not set", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		assert.False(t, IsResourceOwner(c))
	})
}
