package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func (m *MockTripModel) CreateTrip(ctx context.Context, trip *types.Trip) error {
	args := m.Called(ctx, trip)
	return args.Error(0)
}

func (m *MockTripModel) GetTripByID(ctx context.Context, id string) (*types.Trip, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*types.Trip), args.Error(1)
}

func (m *MockTripModel) UpdateTrip(ctx context.Context, id string, update *types.TripUpdate) error {
	args := m.Called(ctx, id, update)
	return args.Error(0)
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
