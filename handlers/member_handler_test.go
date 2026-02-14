package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	istore "github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockMemberUserStore implements istore.UserStore for member handler tests.
// Named differently from MockUserStore in mocks_test.go to avoid redeclare.
type MockMemberUserStore struct {
	mock.Mock
}

func (m *MockMemberUserStore) GetUserByID(ctx context.Context, userID string) (*types.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}

func (m *MockMemberUserStore) GetUserByEmail(ctx context.Context, email string) (*types.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}

func (m *MockMemberUserStore) GetUserBySupabaseID(ctx context.Context, supabaseID string) (*types.User, error) {
	args := m.Called(ctx, supabaseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}

func (m *MockMemberUserStore) CreateUser(ctx context.Context, user *types.User) (string, error) {
	args := m.Called(ctx, user)
	return args.String(0), args.Error(1)
}

func (m *MockMemberUserStore) UpdateUser(ctx context.Context, userID string, updates map[string]interface{}) (*types.User, error) {
	args := m.Called(ctx, userID, updates)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}

func (m *MockMemberUserStore) GetUserByUsername(ctx context.Context, username string) (*types.User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}

func (m *MockMemberUserStore) ListUsers(ctx context.Context, offset, limit int) ([]*types.User, int, error) {
	args := m.Called(ctx, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*types.User), args.Int(1), args.Error(2)
}

func (m *MockMemberUserStore) SyncUserFromSupabase(ctx context.Context, supabaseID string) (*types.User, error) {
	args := m.Called(ctx, supabaseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}

func (m *MockMemberUserStore) GetSupabaseUser(ctx context.Context, userID string) (*types.SupabaseUser, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SupabaseUser), args.Error(1)
}

func (m *MockMemberUserStore) ConvertToUserResponse(user *types.User) (types.UserResponse, error) {
	args := m.Called(user)
	return args.Get(0).(types.UserResponse), args.Error(1)
}

func (m *MockMemberUserStore) GetUserProfile(ctx context.Context, userID string) (*types.UserProfile, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.UserProfile), args.Error(1)
}

func (m *MockMemberUserStore) GetUserProfiles(ctx context.Context, userIDs []string) (map[string]*types.UserProfile, error) {
	args := m.Called(ctx, userIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]*types.UserProfile), args.Error(1)
}

func (m *MockMemberUserStore) UpdateLastSeen(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockMemberUserStore) SetOnlineStatus(ctx context.Context, userID string, isOnline bool) error {
	args := m.Called(ctx, userID, isOnline)
	return args.Error(0)
}

func (m *MockMemberUserStore) UpdateUserPreferences(ctx context.Context, userID string, preferences map[string]interface{}) error {
	args := m.Called(ctx, userID, preferences)
	return args.Error(0)
}

func (m *MockMemberUserStore) BeginTx(ctx context.Context) (types.DatabaseTransaction, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(types.DatabaseTransaction), args.Error(1)
}

func (m *MockMemberUserStore) GetUserByContactEmail(ctx context.Context, contactEmail string) (*types.User, error) {
	args := m.Called(ctx, contactEmail)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}

func (m *MockMemberUserStore) SearchUsers(ctx context.Context, query string, limit int) ([]*types.UserSearchResult, error) {
	args := m.Called(ctx, query, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.UserSearchResult), args.Error(1)
}

func (m *MockMemberUserStore) UpdateContactEmail(ctx context.Context, userID string, email string) error {
	args := m.Called(ctx, userID, email)
	return args.Error(0)
}

// Compile-time interface check
var _ istore.UserStore = (*MockMemberUserStore)(nil)

// dummyCommandResult is a valid CommandResult used in UpdateMemberRole success test cases.
var dummyCommandResult = interfaces.CommandResult{
	Success: true,
	Data: &types.TripMembership{
		TripID: "trip-123",
		UserID: "user-456",
		Role:   types.MemberRoleAdmin,
	},
}

// setupMemberHandler creates a MemberHandler with mock dependencies for testing.
func setupMemberHandler() (*MemberHandler, *MockTripModel, *MockMemberUserStore, *MockEventPublisher) {
	mockTripModel := new(MockTripModel)
	mockUserStore := new(MockMemberUserStore)
	mockEventPublisher := new(MockEventPublisher)

	handler := NewMemberHandler(mockTripModel, mockUserStore, mockEventPublisher)
	return handler, mockTripModel, mockUserStore, mockEventPublisher
}

// TestAddMemberHandler tests the AddMember HTTP handler.
func TestAddMemberHandler(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		setupMocks     func(*MockTripModel)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "Success - Add member with MEMBER role",
			requestBody: AddMemberRequest{
				UserID: "user-456",
				Role:   types.MemberRoleMember,
			},
			setupMocks: func(m *MockTripModel) {
				m.On("AddMember", mock.Anything, mock.MatchedBy(func(ms *types.TripMembership) bool {
					return ms.TripID == "trip-123" && ms.UserID == "user-456" && ms.Role == types.MemberRoleMember
				})).Return(nil)
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp types.TripMembership
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.Equal(t, "trip-123", resp.TripID)
				assert.Equal(t, "user-456", resp.UserID)
				assert.Equal(t, types.MemberRoleMember, resp.Role)
			},
		},
		{
			name:           "Error - Invalid JSON payload",
			requestBody:    []byte(`{invalid json}`),
			setupMocks:     func(m *MockTripModel) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Success - Role normalization lowercase admin to ADMIN",
			requestBody: AddMemberRequest{
				UserID: "user-789",
				Role:   types.MemberRole("admin"),
			},
			setupMocks: func(m *MockTripModel) {
				m.On("AddMember", mock.Anything, mock.MatchedBy(func(ms *types.TripMembership) bool {
					return ms.Role == types.MemberRoleAdmin
				})).Return(nil)
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp types.TripMembership
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.Equal(t, types.MemberRoleAdmin, resp.Role)
			},
		},
		{
			name: "Error - Invalid role",
			requestBody: AddMemberRequest{
				UserID: "user-456",
				Role:   types.MemberRole("SUPERUSER"),
			},
			setupMocks:     func(m *MockTripModel) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Error - User already a member (conflict)",
			requestBody: AddMemberRequest{
				UserID: "user-456",
				Role:   types.MemberRoleMember,
			},
			setupMocks: func(m *MockTripModel) {
				m.On("AddMember", mock.Anything, mock.Anything).Return(
					apperrors.NewConflictError("already_member", "User is already a member of this trip"),
				)
			},
			expectedStatus: http.StatusConflict,
		},
		{
			name: "Error - Model returns internal error",
			requestBody: AddMemberRequest{
				UserID: "user-456",
				Role:   types.MemberRoleMember,
			},
			setupMocks: func(m *MockTripModel) {
				m.On("AddMember", mock.Anything, mock.Anything).Return(
					apperrors.InternalServerError("database error"),
				)
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockTripModel, _, _ := setupMemberHandler()
			tt.setupMocks(mockTripModel)

			var bodyBytes []byte
			switch b := tt.requestBody.(type) {
			case []byte:
				bodyBytes = b
			default:
				bodyBytes, _ = json.Marshal(tt.requestBody)
			}

			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.Use(middleware.ErrorHandler())
			r.POST("/trips/:id/members", handler.AddMemberHandler)

			req, _ := http.NewRequest("POST", "/trips/trip-123/members", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}

			mockTripModel.AssertExpectations(t)
		})
	}
}

// TestUpdateMemberRoleHandler tests the UpdateMemberRole HTTP handler.
func TestUpdateMemberRoleHandler(t *testing.T) {
	tests := []struct {
		name           string
		memberID       string
		requestBody    interface{}
		setupMocks     func(*MockTripModel)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:     "Success - Update role to ADMIN",
			memberID: "user-456",
			requestBody: UpdateMemberRoleRequest{
				Role: types.MemberRoleAdmin,
			},
			setupMocks: func(m *MockTripModel) {
				m.On("UpdateMemberRole", mock.Anything, "trip-123", "user-456", types.MemberRoleAdmin).
					Return(&dummyCommandResult, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp types.TripMembership
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.Equal(t, "trip-123", resp.TripID)
				assert.Equal(t, "user-456", resp.UserID)
				assert.Equal(t, types.MemberRoleAdmin, resp.Role)
			},
		},
		{
			name:     "Error - Member not found",
			memberID: "nonexistent",
			requestBody: UpdateMemberRoleRequest{
				Role: types.MemberRoleAdmin,
			},
			setupMocks: func(m *MockTripModel) {
				m.On("UpdateMemberRole", mock.Anything, "trip-123", "nonexistent", types.MemberRoleAdmin).
					Return(nil, apperrors.NotFound("Member", "nonexistent"))
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:     "Success - Role normalization lowercase member to MEMBER",
			memberID: "user-456",
			requestBody: UpdateMemberRoleRequest{
				Role: types.MemberRole("member"),
			},
			setupMocks: func(m *MockTripModel) {
				m.On("UpdateMemberRole", mock.Anything, "trip-123", "user-456", types.MemberRoleMember).
					Return(&dummyCommandResult, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp types.TripMembership
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.Equal(t, types.MemberRoleMember, resp.Role)
			},
		},
		{
			name:     "Error - Last owner protection",
			memberID: "owner-123",
			requestBody: UpdateMemberRoleRequest{
				Role: types.MemberRoleMember,
			},
			setupMocks: func(m *MockTripModel) {
				m.On("UpdateMemberRole", mock.Anything, "trip-123", "owner-123", types.MemberRoleMember).
					Return(nil, apperrors.Forbidden("last_owner", "Cannot change role of the last owner"))
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Error - Invalid JSON payload",
			memberID:       "user-456",
			requestBody:    []byte(`{invalid json}`),
			setupMocks:     func(m *MockTripModel) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:     "Error - Invalid role value",
			memberID: "user-456",
			requestBody: UpdateMemberRoleRequest{
				Role: types.MemberRole("SUPERUSER"),
			},
			setupMocks:     func(m *MockTripModel) {},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockTripModel, _, _ := setupMemberHandler()
			tt.setupMocks(mockTripModel)

			var bodyBytes []byte
			switch b := tt.requestBody.(type) {
			case []byte:
				bodyBytes = b
			default:
				bodyBytes, _ = json.Marshal(tt.requestBody)
			}

			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.Use(middleware.ErrorHandler())
			r.PUT("/trips/:id/members/:memberId/role", handler.UpdateMemberRoleHandler)

			path := "/trips/trip-123/members/" + tt.memberID + "/role"
			req, _ := http.NewRequest("PUT", path, bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}

			mockTripModel.AssertExpectations(t)
		})
	}
}

// TestRemoveMemberHandler tests the RemoveMember HTTP handler.
func TestRemoveMemberHandler(t *testing.T) {
	tests := []struct {
		name           string
		memberID       string
		setupMocks     func(*MockTripModel)
		expectedStatus int
	}{
		{
			name:     "Success - Remove member returns 204",
			memberID: "user-456",
			setupMocks: func(m *MockTripModel) {
				m.On("RemoveMember", mock.Anything, "trip-123", "user-456").Return(nil)
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:     "Error - Member not found",
			memberID: "nonexistent",
			setupMocks: func(m *MockTripModel) {
				m.On("RemoveMember", mock.Anything, "trip-123", "nonexistent").
					Return(apperrors.NotFound("Member", "nonexistent"))
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:     "Error - Last owner protection",
			memberID: "owner-123",
			setupMocks: func(m *MockTripModel) {
				m.On("RemoveMember", mock.Anything, "trip-123", "owner-123").
					Return(apperrors.Forbidden("last_owner", "Cannot remove the last owner of the trip"))
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:     "Error - Internal server error",
			memberID: "user-456",
			setupMocks: func(m *MockTripModel) {
				m.On("RemoveMember", mock.Anything, "trip-123", "user-456").
					Return(apperrors.InternalServerError("database error"))
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockTripModel, _, _ := setupMemberHandler()
			tt.setupMocks(mockTripModel)

			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.Use(middleware.ErrorHandler())
			r.DELETE("/trips/:id/members/:memberId", handler.RemoveMemberHandler)

			path := "/trips/trip-123/members/" + tt.memberID
			req, _ := http.NewRequest("DELETE", path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			statusCode := w.Result().StatusCode
			assert.Equal(t, tt.expectedStatus, statusCode)

			mockTripModel.AssertExpectations(t)
		})
	}
}

// TestGetTripMembersHandler tests the GetTripMembers HTTP handler.
func TestGetTripMembersHandler(t *testing.T) {
	tests := []struct {
		name           string
		setupMocks     func(*MockTripModel, *MockMemberUserStore)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "Success - Get members with batch profile fetch",
			setupMocks: func(mt *MockTripModel, mu *MockMemberUserStore) {
				mt.On("GetTripMembers", mock.Anything, "trip-123").Return([]types.TripMembership{
					{
						TripID: "trip-123",
						UserID: "user-1",
						Role:   types.MemberRoleOwner,
						Status: types.MembershipStatusActive,
					},
					{
						TripID: "trip-123",
						UserID: "user-2",
						Role:   types.MemberRoleMember,
						Status: types.MembershipStatusActive,
					},
				}, nil)

				mu.On("GetUserProfiles", mock.Anything, []string{"user-1", "user-2"}).Return(
					map[string]*types.UserProfile{
						"user-1": {
							ID:          "user-1",
							Username:    "alice",
							Email:       "alice@example.com",
							FirstName:   "Alice",
							LastName:    "Smith",
							DisplayName: "Alice Smith",
						},
						"user-2": {
							ID:          "user-2",
							Username:    "bob",
							Email:       "bob@example.com",
							FirstName:   "Bob",
							LastName:    "Jones",
							DisplayName: "Bob Jones",
						},
					}, nil,
				)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp []TripMemberResponse
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.Len(t, resp, 2)

				assert.Equal(t, "user-1", resp[0].Membership.UserID)
				assert.Equal(t, types.MemberRoleOwner, resp[0].Membership.Role)
				assert.Equal(t, "alice", resp[0].User.Username)
				assert.Equal(t, "Alice Smith", resp[0].User.DisplayName)

				assert.Equal(t, "user-2", resp[1].Membership.UserID)
				assert.Equal(t, types.MemberRoleMember, resp[1].Membership.Role)
				assert.Equal(t, "bob", resp[1].User.Username)
			},
		},
		{
			name: "Success - User profile not found graceful degradation",
			setupMocks: func(mt *MockTripModel, mu *MockMemberUserStore) {
				mt.On("GetTripMembers", mock.Anything, "trip-123").Return([]types.TripMembership{
					{TripID: "trip-123", UserID: "user-1", Role: types.MemberRoleOwner, Status: types.MembershipStatusActive},
					{TripID: "trip-123", UserID: "user-deleted", Role: types.MemberRoleMember, Status: types.MembershipStatusActive},
				}, nil)

				mu.On("GetUserProfiles", mock.Anything, []string{"user-1", "user-deleted"}).Return(
					map[string]*types.UserProfile{
						"user-1": {ID: "user-1", Username: "alice", Email: "alice@example.com"},
					}, nil,
				)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp []TripMemberResponse
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.Len(t, resp, 2)

				assert.Equal(t, "alice", resp[0].User.Username)
				assert.Equal(t, "user-deleted", resp[1].User.ID)
				assert.Equal(t, "User not found", resp[1].User.Username)
			},
		},
		{
			name: "Error - Trip not found",
			setupMocks: func(mt *MockTripModel, mu *MockMemberUserStore) {
				mt.On("GetTripMembers", mock.Anything, "trip-123").Return(
					nil, apperrors.NotFound("Trip", "trip-123"),
				)
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "Error - Profile batch fetch fails",
			setupMocks: func(mt *MockTripModel, mu *MockMemberUserStore) {
				mt.On("GetTripMembers", mock.Anything, "trip-123").Return([]types.TripMembership{
					{TripID: "trip-123", UserID: "user-1", Role: types.MemberRoleOwner},
				}, nil)

				mu.On("GetUserProfiles", mock.Anything, []string{"user-1"}).Return(
					nil, apperrors.InternalServerError("database error"),
				)
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "Success - Empty member list",
			setupMocks: func(mt *MockTripModel, mu *MockMemberUserStore) {
				mt.On("GetTripMembers", mock.Anything, "trip-123").Return([]types.TripMembership{}, nil)
				mu.On("GetUserProfiles", mock.Anything, []string{}).Return(
					map[string]*types.UserProfile{}, nil,
				)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp []TripMemberResponse
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.Empty(t, resp)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockTripModel, mockUserStore, _ := setupMemberHandler()
			tt.setupMocks(mockTripModel, mockUserStore)

			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.Use(middleware.ErrorHandler())
			r.GET("/trips/:id/members", handler.GetTripMembersHandler)

			req, _ := http.NewRequest("GET", "/trips/trip-123/members", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}

			mockTripModel.AssertExpectations(t)
			mockUserStore.AssertExpectations(t)
		})
	}
}

// TestHandleModelError_MemberContext tests handleModelError for member-specific error types.
func TestHandleModelError_MemberContext(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "Conflict error maps to 409",
			err:            apperrors.NewConflictError("already_member", "User is already a member"),
			expectedStatus: http.StatusConflict,
			expectedCode:   "CONFLICT",
		},
		{
			name:           "Forbidden error maps to 403",
			err:            apperrors.Forbidden("last_owner", "Cannot remove the last owner"),
			expectedStatus: http.StatusForbidden,
			expectedCode:   "AUTHORIZATION",
		},
		{
			name:           "Not found error maps to 404",
			err:            apperrors.NotFound("Member", "user-456"),
			expectedStatus: http.StatusNotFound,
			expectedCode:   "NOT_FOUND",
		},
		{
			name:           "Validation error maps to 400",
			err:            apperrors.ValidationFailed("invalid_role", "Role must be valid"),
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "VALIDATION",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupGinContext("GET", "/test", nil)
			handleModelError(c, tt.err)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var resp types.ErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, resp.Code)
		})
	}
}
