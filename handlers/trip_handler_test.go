package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/pkg/pexels"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/supabase-community/supabase-go"
)

// Mock implementations
type MockTripModel struct {
	mock.Mock
}

func (m *MockTripModel) CreateTrip(ctx context.Context, trip *types.Trip) (*types.Trip, error) {
	args := m.Called(ctx, trip)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Trip), args.Error(1)
}

func (m *MockTripModel) GetTripByID(ctx context.Context, tripID, userID string) (*types.Trip, error) {
	args := m.Called(ctx, tripID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Trip), args.Error(1)
}

func (m *MockTripModel) UpdateTrip(ctx context.Context, tripID, userID string, update *types.TripUpdate) (*types.Trip, error) {
	args := m.Called(ctx, tripID, userID, update)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Trip), args.Error(1)
}

func (m *MockTripModel) UpdateTripStatus(ctx context.Context, tripID string, status types.TripStatus) error {
	args := m.Called(ctx, tripID, status)
	return args.Error(0)
}

func (m *MockTripModel) DeleteTrip(ctx context.Context, tripID string) error {
	args := m.Called(ctx, tripID)
	return args.Error(0)
}

func (m *MockTripModel) ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Trip), args.Error(1)
}

func (m *MockTripModel) SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error) {
	args := m.Called(ctx, criteria)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Trip), args.Error(1)
}

func (m *MockTripModel) GetTripWithMembers(ctx context.Context, tripID, userID string) (*types.TripWithMembers, error) {
	args := m.Called(ctx, tripID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.TripWithMembers), args.Error(1)
}

func (m *MockTripModel) GetTripMembers(ctx context.Context, tripID string) ([]types.TripMembership, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.TripMembership), args.Error(1)
}

func (m *MockTripModel) GetInvitationsByTripID(ctx context.Context, tripID string) ([]*types.TripInvitation, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.TripInvitation), args.Error(1)
}

func (m *MockTripModel) AddMember(ctx context.Context, membership *types.TripMembership) error {
	args := m.Called(ctx, membership)
	return args.Error(0)
}

func (m *MockTripModel) GetUserRole(ctx context.Context, tripID, userID string) (types.MemberRole, error) {
	args := m.Called(ctx, tripID, userID)
	return args.Get(0).(types.MemberRole), args.Error(1)
}

func (m *MockTripModel) UpdateMemberRole(ctx context.Context, tripID, userID string, role types.MemberRole) (*interfaces.CommandResult, error) {
	args := m.Called(ctx, tripID, userID, role)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
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
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.TripInvitation), args.Error(1)
}

func (m *MockTripModel) UpdateInvitationStatus(ctx context.Context, invitationID string, status types.InvitationStatus) error {
	args := m.Called(ctx, invitationID, status)
	return args.Error(0)
}

func (m *MockTripModel) AcceptInvitationAtomically(ctx context.Context, invitationID string, membership *types.TripMembership) error {
	args := m.Called(ctx, invitationID, membership)
	return args.Error(0)
}

func (m *MockTripModel) LookupUserByEmail(ctx context.Context, email string) (*types.SupabaseUser, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SupabaseUser), args.Error(1)
}

func (m *MockTripModel) GetTrip(ctx context.Context, tripID string) (*types.Trip, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Trip), args.Error(1)
}

func (m *MockTripModel) GetCommandContext() *interfaces.CommandContext {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*interfaces.CommandContext)
}

// Ensure MockTripModel implements the interface
var _ interfaces.TripModelInterface = (*MockTripModel)(nil)

type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) Publish(ctx context.Context, tripID string, event types.Event) error {
	args := m.Called(ctx, tripID, event)
	return args.Error(0)
}

func (m *MockEventPublisher) PublishBatch(ctx context.Context, tripID string, events []types.Event) error {
	args := m.Called(ctx, tripID, events)
	return args.Error(0)
}

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

type MockWeatherService struct {
	mock.Mock
}

func (m *MockWeatherService) StartWeatherUpdates(ctx context.Context, tripID string, latitude float64, longitude float64) {
	m.Called(ctx, tripID, latitude, longitude)
}

func (m *MockWeatherService) IncrementSubscribers(tripID string, latitude float64, longitude float64) {
	m.Called(tripID, latitude, longitude)
}

func (m *MockWeatherService) DecrementSubscribers(tripID string) {
	m.Called(tripID)
}

func (m *MockWeatherService) TriggerImmediateUpdate(ctx context.Context, tripID string, lat, lon float64) error {
	args := m.Called(ctx, tripID, lat, lon)
	return args.Error(0)
}

func (m *MockWeatherService) GetWeather(ctx context.Context, tripID string) (*types.WeatherInfo, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.WeatherInfo), args.Error(1)
}

func (m *MockWeatherService) GetWeatherByCoords(ctx context.Context, tripID string, latitude, longitude float64) (*types.WeatherInfo, error) {
	args := m.Called(ctx, tripID, latitude, longitude)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.WeatherInfo), args.Error(1)
}

// MockUserService is now defined in mocks_test.go

type MockPexelsClient struct {
	mock.Mock
}

func (m *MockPexelsClient) SearchDestinationImage(ctx context.Context, query string) (string, error) {
	args := m.Called(ctx, query)
	return args.String(0), args.Error(1)
}

// Ensure MockPexelsClient implements pexels.ClientInterface
var _ pexels.ClientInterface = (*MockPexelsClient)(nil)

// Test setup helper
func setupTestHandler() (*TripHandler, *MockTripModel, *MockEventPublisher, *MockWeatherService, *MockUserService, *MockPexelsClient) {
	mockTripModel := new(MockTripModel)
	mockEventPublisher := new(MockEventPublisher)
	mockWeatherService := new(MockWeatherService)
	mockUserService := new(MockUserService)
	mockPexelsClient := new(MockPexelsClient)

	handler := NewTripHandler(
		mockTripModel,
		mockEventPublisher,
		&supabase.Client{},
		&config.ServerConfig{},
		mockWeatherService,
		mockUserService,
		mockPexelsClient,
	)

	return handler, mockTripModel, mockEventPublisher, mockWeatherService, mockUserService, mockPexelsClient
}

// buildTripRouter wraps a handler in a Gin router with the error handler middleware,
// matching the production setup so c.Error() calls produce the correct HTTP status.
func buildTripRouter(path, method string, handler gin.HandlerFunc, userID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		if userID != "" {
			c.Set(string(middleware.UserIDKey), userID)
		}
		c.Next()
	})
	switch method {
	case http.MethodGet:
		r.GET(path, handler)
	case http.MethodPost:
		r.POST(path, handler)
	case http.MethodPut:
		r.PUT(path, handler)
	case http.MethodPatch:
		r.PATCH(path, handler)
	case http.MethodDelete:
		r.DELETE(path, handler)
	}
	return r
}

// Helper function to setup gin context with authentication
func setupGinContext(method, path string, body interface{}) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	
	var bodyBytes []byte
	if body != nil {
		bodyBytes, _ = json.Marshal(body)
	}
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, bytes.NewReader(bodyBytes))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	
	return c, w
}

// Test CreateTripHandler
func TestCreateTripHandler(t *testing.T) {
	tests := []struct {
		name           string
		userID         string
		requestBody    interface{}
		setupMocks     func(*MockTripModel, *MockPexelsClient)
		expectedStatus int
		expectedError  bool
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name:   "Success - Create trip with all fields",
			userID: "user123",
			requestBody: CreateTripRequest{
				Name:                 "Test Trip",
				Description:          "Test Description",
				DestinationPlaceID:   stringPtr("place123"),
				DestinationAddress:   stringPtr("123 Test St"),
				DestinationName:      stringPtr("Test Destination"),
				DestinationLatitude:  40.7128,
				DestinationLongitude: -74.0060,
				StartDate:            time.Now().Add(24 * time.Hour),
				EndDate:              time.Now().Add(48 * time.Hour),
				Status:               types.TripStatusPlanning,
				BackgroundImageURL:   "https://example.com/image.jpg",
			},
			setupMocks: func(mockTrip *MockTripModel, mockPexels *MockPexelsClient) {
				createdTrip := &types.Trip{
					ID:                   "trip123",
					Name:                 "Test Trip",
					Description:          "Test Description",
					DestinationPlaceID:   stringPtr("place123"),
					DestinationAddress:   stringPtr("123 Test St"),
					DestinationName:      stringPtr("Test Destination"),
					DestinationLatitude:  40.7128,
					DestinationLongitude: -74.0060,
					StartDate:            time.Now().Add(24 * time.Hour),
					EndDate:              time.Now().Add(48 * time.Hour),
					Status:               types.TripStatusPlanning,
					BackgroundImageURL:   "https://example.com/image.jpg",
					CreatedBy:            stringPtr("user123"),
					CreatedAt:            time.Now(),
					UpdatedAt:            time.Now(),
				}
				
				mockTrip.On("CreateTrip", mock.Anything, mock.AnythingOfType("*types.Trip")).Return(createdTrip, nil)
				mockTrip.On("GetTripMembers", mock.Anything, "trip123").Return([]types.TripMembership{
					{
						TripID: "trip123",
						UserID: "user123",
						Role:   types.MemberRoleOwner,
						Status: types.MembershipStatusActive,
					},
				}, nil)
				mockTrip.On("GetInvitationsByTripID", mock.Anything, "trip123").Return([]*types.TripInvitation{}, nil)
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "trip123", resp["id"])
				assert.Equal(t, "Test Trip", resp["name"])
				assert.Equal(t, "Test Description", resp["description"])
				assert.Equal(t, "PLANNING", resp["status"])
				assert.Equal(t, "user123", resp["createdBy"])
				
				// Check destination structure
				dest := resp["destination"].(map[string]interface{})
				assert.Equal(t, "123 Test St", dest["address"])
				assert.Equal(t, "place123", dest["placeId"])
				assert.Equal(t, "Test Destination", dest["name"])
				
				coords := dest["coordinates"].(map[string]interface{})
				assert.Equal(t, 40.7128, coords["lat"])
				assert.Equal(t, -74.0060, coords["lng"])
				
				// Check members
				members := resp["members"].([]interface{})
				assert.Len(t, members, 1)
			},
		},
		{
			name:   "Success - Create trip without background image (fetch from Pexels)",
			userID: "user123",
			requestBody: CreateTripRequest{
				Name:                 "Paris Trip",
				DestinationLatitude:  48.8566,
				DestinationLongitude: 2.3522,
				StartDate:            time.Now().Add(24 * time.Hour),
				EndDate:              time.Now().Add(48 * time.Hour),
			},
			setupMocks: func(mockTrip *MockTripModel, mockPexels *MockPexelsClient) {
				mockPexels.On("SearchDestinationImage", mock.Anything, mock.AnythingOfType("string")).Return("https://pexels.com/photo.jpg", nil)
				
				createdTrip := &types.Trip{
					ID:                   "trip124",
					Name:                 "Paris Trip",
					DestinationLatitude:  48.8566,
					DestinationLongitude: 2.3522,
					StartDate:            time.Now().Add(24 * time.Hour),
					EndDate:              time.Now().Add(48 * time.Hour),
					Status:               types.TripStatusPlanning,
					BackgroundImageURL:   "https://pexels.com/photo.jpg",
					CreatedBy:            stringPtr("user123"),
					CreatedAt:            time.Now(),
					UpdatedAt:            time.Now(),
				}
				
				mockTrip.On("CreateTrip", mock.Anything, mock.AnythingOfType("*types.Trip")).Return(createdTrip, nil)
				mockTrip.On("GetTripMembers", mock.Anything, "trip124").Return([]types.TripMembership{}, nil)
				mockTrip.On("GetInvitationsByTripID", mock.Anything, "trip124").Return([]*types.TripInvitation{}, nil)
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "trip124", resp["id"])
				assert.Equal(t, "https://pexels.com/photo.jpg", resp["backgroundImageUrl"])
			},
		},
		{
			name:           "Error - Invalid request body",
			userID:         "user123",
			requestBody:    "not json",
			setupMocks:     func(mockTrip *MockTripModel, mockPexels *MockPexelsClient) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  true,
		},
		{
			name:   "Error - No user ID in context",
			userID: "",
			requestBody: CreateTripRequest{
				Name:                 "Test Trip",
				DestinationLatitude:  40.7128,
				DestinationLongitude: -74.0060,
				StartDate:            time.Now().Add(24 * time.Hour),
				EndDate:              time.Now().Add(48 * time.Hour),
			},
			setupMocks:     func(mockTrip *MockTripModel, mockPexels *MockPexelsClient) {},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  true,
		},
		{
			name:   "Error - Trip creation fails",
			userID: "user123",
			requestBody: CreateTripRequest{
				Name:                 "Test Trip",
				DestinationLatitude:  40.7128,
				DestinationLongitude: -74.0060,
				StartDate:            time.Now().Add(24 * time.Hour),
				EndDate:              time.Now().Add(48 * time.Hour),
			},
			setupMocks: func(mockTrip *MockTripModel, mockPexels *MockPexelsClient) {
				mockPexels.On("SearchDestinationImage", mock.Anything, mock.AnythingOfType("string")).Return("", nil)
				mockTrip.On("CreateTrip", mock.Anything, mock.AnythingOfType("*types.Trip")).Return(nil, apperrors.InternalServerError("Database error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  true,
		},
		{
			name:   "Success - Pexels fetch fails but trip creation succeeds",
			userID: "user123",
			requestBody: CreateTripRequest{
				Name:                 "Test Trip",
				DestinationLatitude:  40.7128,
				DestinationLongitude: -74.0060,
				StartDate:            time.Now().Add(24 * time.Hour),
				EndDate:              time.Now().Add(48 * time.Hour),
			},
			setupMocks: func(mockTrip *MockTripModel, mockPexels *MockPexelsClient) {
				mockPexels.On("SearchDestinationImage", mock.Anything, mock.AnythingOfType("string")).Return("", errors.New("Pexels API error"))
				
				createdTrip := &types.Trip{
					ID:                   "trip125",
					Name:                 "Test Trip",
					DestinationLatitude:  40.7128,
					DestinationLongitude: -74.0060,
					StartDate:            time.Now().Add(24 * time.Hour),
					EndDate:              time.Now().Add(48 * time.Hour),
					Status:               types.TripStatusPlanning,
					BackgroundImageURL:   "", // No image
					CreatedBy:            stringPtr("user123"),
					CreatedAt:            time.Now(),
					UpdatedAt:            time.Now(),
				}
				
				mockTrip.On("CreateTrip", mock.Anything, mock.AnythingOfType("*types.Trip")).Return(createdTrip, nil)
				mockTrip.On("GetTripMembers", mock.Anything, "trip125").Return([]types.TripMembership{}, nil)
				mockTrip.On("GetInvitationsByTripID", mock.Anything, "trip125").Return([]*types.TripInvitation{}, nil)
			},
			expectedStatus: http.StatusCreated,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockTripModel, _, _, _, mockPexelsClient := setupTestHandler()
			tt.setupMocks(mockTripModel, mockPexelsClient)

			r := buildTripRouter("/trips", http.MethodPost, handler.CreateTripHandler, tt.userID)

			var bodyBytes []byte
			switch v := tt.requestBody.(type) {
			case string:
				bodyBytes = []byte(v)
			default:
				bodyBytes, _ = json.Marshal(tt.requestBody)
			}

			req, _ := http.NewRequest(http.MethodPost, "/trips", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if !tt.expectedError && tt.checkResponse != nil {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				tt.checkResponse(t, response)
			}

			mockTripModel.AssertExpectations(t)
			mockPexelsClient.AssertExpectations(t)
		})
	}
}

// Test GetTripHandler
func TestGetTripHandler(t *testing.T) {
	tests := []struct {
		name           string
		tripID         string
		userID         string
		setupMocks     func(*MockTripModel)
		expectedStatus int
		expectedError  bool
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name:   "Success - Get trip",
			tripID: "trip123",
			userID: "user123",
			setupMocks: func(mockTrip *MockTripModel) {
				trip := &types.Trip{
					ID:                   "trip123",
					Name:                 "Test Trip",
					Description:          "Test Description",
					DestinationLatitude:  40.7128,
					DestinationLongitude: -74.0060,
					StartDate:            time.Now().Add(24 * time.Hour),
					EndDate:              time.Now().Add(48 * time.Hour),
					Status:               types.TripStatusPlanning,
					CreatedBy:            stringPtr("user123"),
					CreatedAt:            time.Now(),
					UpdatedAt:            time.Now(),
				}
				mockTrip.On("GetTripByID", mock.Anything, "trip123", "user123").Return(trip, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "trip123", resp["id"])
				assert.Equal(t, "Test Trip", resp["name"])
			},
		},
		{
			name:   "Error - Trip not found",
			tripID: "nonexistent",
			userID: "user123",
			setupMocks: func(mockTrip *MockTripModel) {
				mockTrip.On("GetTripByID", mock.Anything, "nonexistent", "user123").Return(nil, apperrors.NotFound("Trip", "not found"))
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  true,
		},
		{
			name:   "Error - User not authorized",
			tripID: "trip123",
			userID: "user456",
			setupMocks: func(mockTrip *MockTripModel) {
				mockTrip.On("GetTripByID", mock.Anything, "trip123", "user456").Return(nil, apperrors.Forbidden("not_member", "User is not a member of this trip"))
			},
			expectedStatus: http.StatusForbidden,
			expectedError:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockTripModel, _, _, _, _ := setupTestHandler()
			tt.setupMocks(mockTripModel)
			
			c, w := setupGinContext("GET", fmt.Sprintf("/trips/%s", tt.tripID), nil)
			c.Params = gin.Params{{Key: "id", Value: tt.tripID}}
			c.Set(string(middleware.UserIDKey), tt.userID)
			
			handler.GetTripHandler(c)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			if !tt.expectedError && tt.checkResponse != nil {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				tt.checkResponse(t, response)
			}
			
			mockTripModel.AssertExpectations(t)
		})
	}
}

// Test UpdateTripHandler
func TestUpdateTripHandler(t *testing.T) {
	tests := []struct {
		name           string
		tripID         string
		userID         string
		requestBody    interface{}
		setupMocks     func(*MockTripModel)
		expectedStatus int
		expectedError  bool
	}{
		{
			name:   "Success - Update trip",
			tripID: "trip123",
			userID: "user123",
			requestBody: types.TripUpdate{
				Name:        stringPtr("Updated Trip Name"),
				Description: stringPtr("Updated Description"),
			},
			setupMocks: func(mockTrip *MockTripModel) {
				updatedTrip := &types.Trip{
					ID:          "trip123",
					Name:        "Updated Trip Name",
					Description: "Updated Description",
					UpdatedAt:   time.Now(),
				}
				mockTrip.On("UpdateTrip", mock.Anything, "trip123", "user123", mock.AnythingOfType("*types.TripUpdate")).Return(updatedTrip, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Error - Invalid request body",
			tripID:         "trip123",
			userID:         "user123",
			requestBody:    "not json",
			setupMocks:     func(mockTrip *MockTripModel) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  true,
		},
		{
			name:   "Error - Update fails",
			tripID: "trip123",
			userID: "user123",
			requestBody: types.TripUpdate{
				Name: stringPtr("Updated Trip Name"),
			},
			setupMocks: func(mockTrip *MockTripModel) {
				mockTrip.On("UpdateTrip", mock.Anything, "trip123", "user123", mock.AnythingOfType("*types.TripUpdate")).Return(nil, apperrors.Forbidden("not_owner", "Only trip owner can update"))
			},
			expectedStatus: http.StatusForbidden,
			expectedError:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockTripModel, _, _, _, _ := setupTestHandler()
			tt.setupMocks(mockTripModel)

			r := buildTripRouter("/trips/:id", http.MethodPut, handler.UpdateTripHandler, tt.userID)

			var bodyBytes []byte
			switch v := tt.requestBody.(type) {
			case string:
				bodyBytes = []byte(v)
			default:
				bodyBytes, _ = json.Marshal(tt.requestBody)
			}

			req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("/trips/%s", tt.tripID), bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockTripModel.AssertExpectations(t)
		})
	}
}

// Test UpdateTripStatusHandler
func TestUpdateTripStatusHandler(t *testing.T) {
	tests := []struct {
		name           string
		tripID         string
		userID         string
		requestBody    interface{}
		setupMocks     func(*MockTripModel)
		expectedStatus int
		expectedError  bool
	}{
		{
			name:   "Success - Update trip status",
			tripID: "trip123",
			userID: "user123",
			requestBody: UpdateTripStatusRequest{
				Status: "ACTIVE",
			},
			setupMocks: func(mockTrip *MockTripModel) {
				mockTrip.On("UpdateTripStatus", mock.Anything, "trip123", types.TripStatusActive).Return(nil)
				
				trip := &types.Trip{
					ID:     "trip123",
					Name:   "Test Trip",
					Status: types.TripStatusActive,
				}
				mockTrip.On("GetTripByID", mock.Anything, "trip123", "user123").Return(trip, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "Error - Invalid request body",
			tripID: "trip123",
			userID: "user123",
			requestBody: map[string]interface{}{
				// Missing status field
			},
			setupMocks:     func(mockTrip *MockTripModel) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  true,
		},
		{
			name:   "Error - Status update fails",
			tripID: "trip123",
			userID: "user123",
			requestBody: UpdateTripStatusRequest{
				Status: "ACTIVE",
			},
			setupMocks: func(mockTrip *MockTripModel) {
				mockTrip.On("UpdateTripStatus", mock.Anything, "trip123", types.TripStatusActive).Return(apperrors.Forbidden("invalid_transition", "Cannot transition from COMPLETED to ACTIVE"))
			},
			expectedStatus: http.StatusForbidden,
			expectedError:  true,
		},
		{
			name:   "Success - Update succeeds but fetch updated trip fails",
			tripID: "trip123",
			userID: "user123",
			requestBody: UpdateTripStatusRequest{
				Status: "ACTIVE",
			},
			setupMocks: func(mockTrip *MockTripModel) {
				mockTrip.On("UpdateTripStatus", mock.Anything, "trip123", types.TripStatusActive).Return(nil)
				mockTrip.On("GetTripByID", mock.Anything, "trip123", "user123").Return(nil, errors.New("Failed to fetch"))
			},
			expectedStatus: http.StatusOK,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockTripModel, _, _, _, _ := setupTestHandler()
			tt.setupMocks(mockTripModel)

			r := buildTripRouter("/trips/:id/status", http.MethodPatch, handler.UpdateTripStatusHandler, tt.userID)

			var bodyBytes []byte
			switch v := tt.requestBody.(type) {
			case string:
				bodyBytes = []byte(v)
			default:
				bodyBytes, _ = json.Marshal(tt.requestBody)
			}

			req, _ := http.NewRequest(http.MethodPatch, fmt.Sprintf("/trips/%s/status", tt.tripID), bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockTripModel.AssertExpectations(t)
		})
	}
}

// Test DeleteTripHandler
func TestDeleteTripHandler(t *testing.T) {
	tests := []struct {
		name           string
		tripID         string
		setupMocks     func(*MockTripModel)
		expectedStatus int
	}{
		{
			name:   "Success - Delete trip",
			tripID: "trip123",
			setupMocks: func(mockTrip *MockTripModel) {
				mockTrip.On("DeleteTrip", mock.Anything, "trip123").Return(nil)
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:   "Error - Trip not found",
			tripID: "nonexistent",
			setupMocks: func(mockTrip *MockTripModel) {
				mockTrip.On("DeleteTrip", mock.Anything, "nonexistent").Return(apperrors.NotFound("Trip", "not found"))
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:   "Error - User not authorized",
			tripID: "trip123",
			setupMocks: func(mockTrip *MockTripModel) {
				mockTrip.On("DeleteTrip", mock.Anything, "trip123").Return(apperrors.Forbidden("not_owner", "Only trip owner can delete"))
			},
			expectedStatus: http.StatusForbidden,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockTripModel, _, _, _, _ := setupTestHandler()
			tt.setupMocks(mockTripModel)

			r := buildTripRouter("/trips/:id", http.MethodDelete, handler.DeleteTripHandler, "user123")

			req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("/trips/%s", tt.tripID), nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockTripModel.AssertExpectations(t)
		})
	}
}

// Test ListUserTripsHandler
func TestListUserTripsHandler(t *testing.T) {
	tests := []struct {
		name           string
		userID         string
		setupMocks     func(*MockTripModel)
		expectedStatus int
		expectedError  bool
		checkResponse  func(*testing.T, []interface{})
	}{
		{
			name:   "Success - List user trips",
			userID: "user123",
			setupMocks: func(mockTrip *MockTripModel) {
				trips := []*types.Trip{
					{
						ID:     "trip1",
						Name:   "Trip 1",
						Status: types.TripStatusPlanning,
					},
					{
						ID:     "trip2",
						Name:   "Trip 2",
						Status: types.TripStatusActive,
					},
				}
				mockTrip.On("ListUserTrips", mock.Anything, "user123").Return(trips, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, trips []interface{}) {
				assert.Len(t, trips, 2)
				trip1 := trips[0].(map[string]interface{})
				assert.Equal(t, "trip1", trip1["id"])
				assert.Equal(t, "Trip 1", trip1["name"])
			},
		},
		{
			name:   "Success - Empty trip list",
			userID: "user123",
			setupMocks: func(mockTrip *MockTripModel) {
				mockTrip.On("ListUserTrips", mock.Anything, "user123").Return([]*types.Trip{}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, trips []interface{}) {
				assert.Len(t, trips, 0)
			},
		},
		{
			name:           "Error - No user ID",
			userID:         "",
			setupMocks:     func(mockTrip *MockTripModel) {},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  true,
		},
		{
			name:   "Error - Database error",
			userID: "user123",
			setupMocks: func(mockTrip *MockTripModel) {
				mockTrip.On("ListUserTrips", mock.Anything, "user123").Return(nil, apperrors.InternalServerError("Database error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockTripModel, _, _, _, _ := setupTestHandler()
			tt.setupMocks(mockTripModel)
			
			c, w := setupGinContext("GET", "/trips", nil)
			if tt.userID != "" {
				c.Set(string(middleware.UserIDKey), tt.userID)
			}
			
			handler.ListUserTripsHandler(c)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			if !tt.expectedError && tt.checkResponse != nil {
				var response []interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				tt.checkResponse(t, response)
			}
			
			mockTripModel.AssertExpectations(t)
		})
	}
}

// Test SearchTripsHandler
func TestSearchTripsHandler(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		setupMocks     func(*MockTripModel)
		expectedStatus int
		expectedError  bool
		checkResponse  func(*testing.T, []interface{})
	}{
		{
			name: "Success - Search trips by destination",
			requestBody: types.TripSearchCriteria{
				Destination: "Paris",
			},
			setupMocks: func(mockTrip *MockTripModel) {
				trips := []*types.Trip{
					{
						ID:                 "trip1",
						Name:               "Paris Trip",
						DestinationAddress: stringPtr("Paris, France"),
					},
				}
				mockTrip.On("SearchTrips", mock.Anything, mock.AnythingOfType("types.TripSearchCriteria")).Return(trips, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, trips []interface{}) {
				assert.Len(t, trips, 1)
			},
		},
		{
			name: "Success - Search trips by date range",
			requestBody: types.TripSearchCriteria{
				StartDateFrom: time.Now(),
				StartDateTo:   time.Now().Add(7 * 24 * time.Hour),
			},
			setupMocks: func(mockTrip *MockTripModel) {
				mockTrip.On("SearchTrips", mock.Anything, mock.AnythingOfType("types.TripSearchCriteria")).Return([]*types.Trip{}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Error - Invalid request body",
			requestBody:    "invalid",
			setupMocks:     func(mockTrip *MockTripModel) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  true,
		},
		{
			name: "Error - Search fails",
			requestBody: types.TripSearchCriteria{
				Destination: "Paris",
			},
			setupMocks: func(mockTrip *MockTripModel) {
				mockTrip.On("SearchTrips", mock.Anything, mock.AnythingOfType("types.TripSearchCriteria")).Return(nil, apperrors.InternalServerError("Database error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockTripModel, _, _, _, _ := setupTestHandler()
			tt.setupMocks(mockTripModel)

			r := buildTripRouter("/trips/search", http.MethodPost, handler.SearchTripsHandler, "user123")

			var bodyBytes []byte
			switch v := tt.requestBody.(type) {
			case string:
				bodyBytes = []byte(v)
			default:
				bodyBytes, _ = json.Marshal(tt.requestBody)
			}

			req, _ := http.NewRequest(http.MethodPost, "/trips/search", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if !tt.expectedError && tt.checkResponse != nil {
				var response []interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				tt.checkResponse(t, response)
			}

			mockTripModel.AssertExpectations(t)
		})
	}
}

// Test GetTripWithMembersHandler
func TestGetTripWithMembersHandler(t *testing.T) {
	tests := []struct {
		name           string
		tripID         string
		userID         string
		setupMocks     func(*MockTripModel)
		expectedStatus int
		expectedError  bool
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name:   "Success - Get trip with members",
			tripID: "trip123",
			userID: "user123",
			setupMocks: func(mockTrip *MockTripModel) {
				tripWithMembers := &types.TripWithMembers{
					Trip: types.Trip{
						ID:     "trip123",
						Name:   "Test Trip",
						Status: types.TripStatusActive,
					},
					Members: []*types.TripMembership{
						{
							TripID: "trip123",
							UserID: "user123",
							Role:   types.MemberRoleOwner,
							Status: types.MembershipStatusActive,
						},
						{
							TripID: "trip123",
							UserID: "user456",
							Role:   types.MemberRoleMember,
							Status: types.MembershipStatusActive,
						},
					},
				}
				mockTrip.On("GetTripWithMembers", mock.Anything, "trip123", "user123").Return(tripWithMembers, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				// TripWithMembers serializes as {"trip": {...}, "members": [...]}
				trip := resp["trip"].(map[string]interface{})
				assert.Equal(t, "trip123", trip["id"])
				members := resp["members"].([]interface{})
				assert.Len(t, members, 2)
			},
		},
		{
			name:   "Error - Trip not found",
			tripID: "nonexistent",
			userID: "user123",
			setupMocks: func(mockTrip *MockTripModel) {
				mockTrip.On("GetTripWithMembers", mock.Anything, "nonexistent", "user123").Return(nil, apperrors.NotFound("Trip", "not found"))
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  true,
		},
		{
			name:   "Error - User not authorized",
			tripID: "trip123",
			userID: "unauthorized",
			setupMocks: func(mockTrip *MockTripModel) {
				mockTrip.On("GetTripWithMembers", mock.Anything, "trip123", "unauthorized").Return(nil, apperrors.Forbidden("not_member", "User is not a member"))
			},
			expectedStatus: http.StatusForbidden,
			expectedError:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockTripModel, _, _, _, _ := setupTestHandler()
			tt.setupMocks(mockTripModel)

			r := buildTripRouter("/trips/:id/details", http.MethodGet, handler.GetTripWithMembersHandler, tt.userID)

			req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/trips/%s/details", tt.tripID), nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if !tt.expectedError && tt.checkResponse != nil {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				tt.checkResponse(t, response)
			}

			mockTripModel.AssertExpectations(t)
		})
	}
}

// Test TriggerWeatherUpdateHandler
func TestTriggerWeatherUpdateHandler(t *testing.T) {
	tests := []struct {
		name           string
		tripID         string
		userID         string
		setupMocks     func(*MockTripModel, *MockWeatherService)
		expectedStatus int
		expectedError  bool
	}{
		{
			name:   "Success - Trigger weather update",
			tripID: "trip123",
			userID: "user123",
			setupMocks: func(mockTrip *MockTripModel, mockWeather *MockWeatherService) {
				trip := &types.Trip{
					ID:                   "trip123",
					Name:                 "Test Trip",
					DestinationLatitude:  40.7128,
					DestinationLongitude: -74.0060,
				}
				mockTrip.On("GetTripByID", mock.Anything, "trip123", "user123").Return(trip, nil)
				mockWeather.On("TriggerImmediateUpdate", mock.Anything, "trip123", 40.7128, -74.0060).Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "Error - Trip not found",
			tripID: "nonexistent",
			userID: "user123",
			setupMocks: func(mockTrip *MockTripModel, mockWeather *MockWeatherService) {
				mockTrip.On("GetTripByID", mock.Anything, "nonexistent", "user123").Return(nil, apperrors.NotFound("Trip", "not found"))
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  true,
		},
		{
			name:   "Error - No destination coordinates",
			tripID: "trip123",
			userID: "user123",
			setupMocks: func(mockTrip *MockTripModel, mockWeather *MockWeatherService) {
				trip := &types.Trip{
					ID:                   "trip123",
					Name:                 "Test Trip",
					DestinationLatitude:  0,
					DestinationLongitude: 0,
				}
				mockTrip.On("GetTripByID", mock.Anything, "trip123", "user123").Return(trip, nil)
			},
			expectedStatus: http.StatusForbidden,
			expectedError:  true,
		},
		{
			name:   "Error - Weather update fails",
			tripID: "trip123",
			userID: "user123",
			setupMocks: func(mockTrip *MockTripModel, mockWeather *MockWeatherService) {
				trip := &types.Trip{
					ID:                   "trip123",
					Name:                 "Test Trip",
					DestinationLatitude:  40.7128,
					DestinationLongitude: -74.0060,
				}
				mockTrip.On("GetTripByID", mock.Anything, "trip123", "user123").Return(trip, nil)
				mockWeather.On("TriggerImmediateUpdate", mock.Anything, "trip123", 40.7128, -74.0060).Return(apperrors.InternalServerError("Weather API error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  true,
		},
		{
			name:   "Error - Weather update fails with non-AppError",
			tripID: "trip123",
			userID: "user123",
			setupMocks: func(mockTrip *MockTripModel, mockWeather *MockWeatherService) {
				trip := &types.Trip{
					ID:                   "trip123",
					Name:                 "Test Trip",
					DestinationLatitude:  40.7128,
					DestinationLongitude: -74.0060,
				}
				mockTrip.On("GetTripByID", mock.Anything, "trip123", "user123").Return(trip, nil)
				mockWeather.On("TriggerImmediateUpdate", mock.Anything, "trip123", 40.7128, -74.0060).Return(errors.New("Generic error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockTripModel, _, mockWeatherService, _, _ := setupTestHandler()

			// Special case for nil weather service test
			if tt.name == "Error - Weather service not available" {
				handler.weatherService = nil
				trip := &types.Trip{
					ID:                   "trip123",
					Name:                 "Test Trip",
					DestinationLatitude:  40.7128,
					DestinationLongitude: -74.0060,
				}
				mockTripModel.On("GetTripByID", mock.Anything, "trip123", "user123").Return(trip, nil)
			} else {
				tt.setupMocks(mockTripModel, mockWeatherService)
			}

			r := buildTripRouter("/trips/:id/weather/trigger", http.MethodPost, handler.TriggerWeatherUpdateHandler, tt.userID)

			req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/trips/%s/weather/trigger", tt.tripID), nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			mockTripModel.AssertExpectations(t)
			mockWeatherService.AssertExpectations(t)
		})
	}
}

// Test for weather service not available
func TestTriggerWeatherUpdateHandler_NoWeatherService(t *testing.T) {
	handler, mockTripModel, _, _, _, _ := setupTestHandler()
	handler.weatherService = nil // Explicitly set to nil

	trip := &types.Trip{
		ID:                   "trip123",
		Name:                 "Test Trip",
		DestinationLatitude:  40.7128,
		DestinationLongitude: -74.0060,
	}
	mockTripModel.On("GetTripByID", mock.Anything, "trip123", "user123").Return(trip, nil)

	r := buildTripRouter("/trips/:id/weather/trigger", http.MethodPost, handler.TriggerWeatherUpdateHandler, "user123")

	req, _ := http.NewRequest(http.MethodPost, "/trips/trip123/weather/trigger", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockTripModel.AssertExpectations(t)
}

// Test NotImplemented handlers
func TestNotImplementedHandlers(t *testing.T) {
	handler, _, _, _, _, _ := setupTestHandler()
	
	tests := []struct {
		name    string
		handler func(*gin.Context)
		method  string
		path    string
	}{
		{
			name:    "UploadTripImage",
			handler: handler.UploadTripImage,
			method:  "POST",
			path:    "/trips/trip123/images",
		},
		{
			name:    "ListTripImages",
			handler: handler.ListTripImages,
			method:  "GET",
			path:    "/trips/trip123/images",
		},
		{
			name:    "DeleteTripImage",
			handler: handler.DeleteTripImage,
			method:  "DELETE",
			path:    "/trips/trip123/images/image123",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupGinContext(tt.method, tt.path, nil)
			
			tt.handler(c)
			
			assert.Equal(t, http.StatusNotImplemented, w.Code)
			
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, "Not implemented", response["message"])
		})
	}
}

// Test handleModelError
func TestHandleModelError(t *testing.T) {
	handler, _, _, _, _, _ := setupTestHandler()
	
	tests := []struct {
		name           string
		error          error
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "AppError - Not Found",
			error:          apperrors.NotFound("Resource", "not found"),
			expectedStatus: http.StatusNotFound,
			expectedCode:   "NOT_FOUND",
		},
		{
			name:           "AppError - Forbidden",
			error:          apperrors.Forbidden("access_denied", "Access denied"),
			expectedStatus: http.StatusForbidden,
			expectedCode:   "AUTHORIZATION",
		},
		{
			name:           "AppError - Validation Failed",
			error:          apperrors.ValidationFailed("invalid_input", "Invalid input"),
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "VALIDATION",
		},
		{
			name:           "Generic Error",
			error:          errors.New("Something went wrong"),
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "INTERNAL_ERROR",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupGinContext("GET", "/test", nil)
			
			handler.handleModelError(c, tt.error)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			var response types.ErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, response.Code)
		})
	}
}

// Test fetchBackgroundImage
func TestFetchBackgroundImage(t *testing.T) {
	handler, _, _, _, _, mockPexelsClient := setupTestHandler()
	
	tests := []struct {
		name        string
		trip        *types.Trip
		setupMocks  func()
		expectedURL string
	}{
		{
			name: "Success - Fetch image with destination name",
			trip: &types.Trip{
				Name:            "Paris Trip",
				DestinationName: stringPtr("Paris"),
			},
			setupMocks: func() {
				mockPexelsClient.On("SearchDestinationImage", mock.Anything, mock.AnythingOfType("string")).Return("https://pexels.com/paris.jpg", nil)
			},
			expectedURL: "https://pexels.com/paris.jpg",
		},
		{
			name: "Success - Fetch image with destination address",
			trip: &types.Trip{
				Name:               "Test Trip",
				DestinationAddress: stringPtr("123 Main St, New York"),
			},
			setupMocks: func() {
				mockPexelsClient.On("SearchDestinationImage", mock.Anything, mock.AnythingOfType("string")).Return("https://pexels.com/newyork.jpg", nil)
			},
			expectedURL: "https://pexels.com/newyork.jpg",
		},
		{
			name: "Error - API returns error",
			trip: &types.Trip{
				Name:            "Test Trip",
				DestinationName: stringPtr("London"),
			},
			setupMocks: func() {
				mockPexelsClient.On("SearchDestinationImage", mock.Anything, mock.AnythingOfType("string")).Return("", errors.New("API error"))
			},
			expectedURL: "",
		},
		{
			name: "No image found",
			trip: &types.Trip{
				Name:            "Test Trip",
				DestinationName: stringPtr("Unknown Place"),
			},
			setupMocks: func() {
				mockPexelsClient.On("SearchDestinationImage", mock.Anything, mock.AnythingOfType("string")).Return("", nil)
			},
			expectedURL: "",
		},
		{
			name: "No search query available",
			trip: &types.Trip{
				Name: "Test Trip",
				// No destination information — BuildSearchQuery may still return the trip name
			},
			setupMocks: func() {
				mockPexelsClient.On("SearchDestinationImage", mock.Anything, mock.AnythingOfType("string")).Return("", nil)
			},
			expectedURL: "",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()
			
			imageURL := handler.fetchBackgroundImage(context.Background(), tt.trip)
			
			assert.Equal(t, tt.expectedURL, imageURL)
			mockPexelsClient.AssertExpectations(t)
			
			// Reset mock for next test
			mockPexelsClient.ExpectedCalls = nil
		})
	}
}

// Test derefString helper
func TestDerefString(t *testing.T) {
	tests := []struct {
		name     string
		input    *string
		expected string
	}{
		{
			name:     "Nil pointer",
			input:    nil,
			expected: "",
		},
		{
			name:     "Valid string pointer",
			input:    stringPtr("test string"),
			expected: "test string",
		},
		{
			name:     "Empty string pointer",
			input:    stringPtr(""),
			expected: "",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := derefString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}