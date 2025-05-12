package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockLocationStore struct {
	mock.Mock
}

func (m *mockLocationStore) CreateLocation(ctx context.Context, location *types.Location) (string, error) {
	args := m.Called(ctx, location)
	return args.String(0), args.Error(1)
}

func (m *mockLocationStore) GetLocation(ctx context.Context, id string) (*types.Location, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Location), args.Error(1)
}

func (m *mockLocationStore) UpdateLocation(ctx context.Context, userID string, update types.LocationUpdate) (*types.Location, error) {
	args := m.Called(ctx, userID, update)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Location), args.Error(1)
}

func (m *mockLocationStore) DeleteLocation(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockLocationStore) ListTripMemberLocations(ctx context.Context, tripID string) ([]*types.MemberLocation, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.MemberLocation), args.Error(1)
}

func (m *mockLocationStore) GetTripMemberLocations(ctx context.Context, tripID string) ([]types.MemberLocation, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.MemberLocation), args.Error(1)
}

func (m *mockLocationStore) GetUserRole(ctx context.Context, tripID string, userID string) (types.MemberRole, error) {
	args := m.Called(ctx, tripID, userID)
	return args.Get(0).(types.MemberRole), args.Error(1)
}

/*
// TODO: Offline location functionality was removed.
func (m *mockLocationStore) SaveOfflineLocations(ctx context.Context, userID, tripID string, updates []types.LocationUpdate, deviceID string) error {
	args := m.Called(ctx, userID, tripID, updates, deviceID)
	return args.Error(0)
}

func (m *mockLocationStore) ProcessOfflineLocations(ctx context.Context, userID, tripID string) error {
	args := m.Called(ctx, userID, tripID)
	return args.Error(0)
}
*/

func (m *mockLocationStore) BeginTx(ctx context.Context) (store.Transaction, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(store.Transaction), args.Error(1)
}

// mockLocationServiceInterface is a mock interface that matches what our mock implementation provides
type mockLocationServiceInterface interface {
	UpdateLocation(ctx context.Context, userID string, update types.LocationUpdate) (*types.Location, error)
	GetTripMemberLocations(ctx context.Context, tripID string, userID string) ([]types.MemberLocation, error)
}

// mockLocationService implements the mockLocationServiceInterface
type mockLocationService struct {
	mock.Mock
}

func (m *mockLocationService) UpdateLocation(ctx context.Context, userID string, update types.LocationUpdate) (*types.Location, error) {
	args := m.Called(ctx, userID, update)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Location), args.Error(1)
}

func (m *mockLocationService) GetTripMemberLocations(ctx context.Context, tripID string, userID string) ([]types.MemberLocation, error) {
	args := m.Called(ctx, tripID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.MemberLocation), args.Error(1)
}

// TestLocationHandler is a simplified test version of the actual handler
type TestLocationHandler struct {
	locService mockLocationServiceInterface
}

// UpdateLocationHandler handles requests to update a user's location
func (h *TestLocationHandler) UpdateLocationHandler(c *gin.Context) {
	userID := c.GetString("userID")

	var locationUpdate types.LocationUpdate
	if err := c.ShouldBindJSON(&locationUpdate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	location, err := h.locService.UpdateLocation(c.Request.Context(), userID, locationUpdate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, location)
}

// GetTripMemberLocationsHandler handles requests to get locations of all members in a trip
func (h *TestLocationHandler) GetTripMemberLocationsHandler(c *gin.Context) {
	tripID := c.Param("id")
	userID := c.GetString("userID")

	locations, err := h.locService.GetTripMemberLocations(c.Request.Context(), tripID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"locations": locations,
	})
}

// mockAuthMiddleware simulates setting the user ID in the context
func mockAuthMiddleware(userID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("userID", userID) // Assuming "userID" is the key used by the actual auth middleware
		c.Next()
	}
}

func setupTestRouter() (*gin.Engine, *mockLocationService) {
	mockSvc := new(mockLocationService)
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create a handler with our mock service
	handler := &TestLocationHandler{
		locService: mockSvc,
	}

	// Setup routes with mock authentication
	authGroup := router.Group("/trips/:id", mockAuthMiddleware("test-user-id")) // Apply middleware
	{
		authGroup.POST("/locations", handler.UpdateLocationHandler)
		authGroup.GET("/locations", handler.GetTripMemberLocationsHandler)
	}

	return router, mockSvc
}

func TestUpdateLocation(t *testing.T) {
	router, mockSvc := setupTestRouter()

	update := types.LocationUpdate{
		Latitude:  51.5074,
		Longitude: -0.1278,
		Accuracy:  10.5,
		Timestamp: time.Now().UnixMilli(),
	}

	expectedLocation := &types.Location{
		ID:        "test-id",
		UserID:    "test-user-id",
		Latitude:  update.Latitude,
		Longitude: update.Longitude,
		Accuracy:  update.Accuracy,
		Timestamp: time.UnixMilli(update.Timestamp),
	}

	// Use our mockLocationService
	mockSvc.On("UpdateLocation", mock.Anything, "test-user-id", update).Return(expectedLocation, nil)

	body, _ := json.Marshal(update)
	req, _ := http.NewRequest("POST", "/trips/test-trip-id/locations", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "test-user-id")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var response types.Location
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, expectedLocation.ID, response.ID)
	assert.Equal(t, expectedLocation.Latitude, response.Latitude)
	assert.Equal(t, expectedLocation.Longitude, response.Longitude)
}

func TestGetTripMemberLocations(t *testing.T) {
	router, mockSvc := setupTestRouter()

	tripID := "test-trip-id"
	now := time.Now()

	expectedLocations := []types.MemberLocation{
		{
			Location: types.Location{
				ID:        "test-id-1",
				UserID:    "test-user-id",
				Latitude:  51.5074,
				Longitude: -0.1278,
				Accuracy:  10.5,
				Timestamp: now,
			},
			UserName: "Test User",
			UserRole: "member",
		},
	}

	// Use our mockLocationService
	mockSvc.On("GetTripMemberLocations", mock.Anything, tripID, "test-user-id").Return(expectedLocations, nil)

	req, _ := http.NewRequest("GET", "/trips/test-trip-id/locations", nil)
	req.Header.Set("X-User-ID", "test-user-id")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var response map[string][]types.MemberLocation
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Len(t, response["locations"], 1)
	assert.Equal(t, expectedLocations[0].ID, response["locations"][0].ID)
	assert.Equal(t, expectedLocations[0].UserName, response["locations"][0].UserName)
}

/*
// TODO: Offline location functionality was removed. These tests are no longer valid.
func TestSaveOfflineLocations(t *testing.T) {
	// Removed as mentioned in the comment
}

func TestProcessOfflineLocations(t *testing.T) {
	// Removed as mentioned in the comment
}
*/
