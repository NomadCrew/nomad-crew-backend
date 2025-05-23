package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/services"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

// SupabaseTripService implements types.TripServiceInterface for testing
type SupabaseTripService struct {
	tripMembers map[string]bool
}

func NewSupabaseTripService(tripID, userID string) *SupabaseTripService {
	return &SupabaseTripService{
		tripMembers: map[string]bool{
			tripID + ":" + userID: true,
		},
	}
}

func (m *SupabaseTripService) IsTripMember(ctx context.Context, tripID, userID string) (bool, error) {
	return m.tripMembers[tripID+":"+userID], nil
}

func (m *SupabaseTripService) GetTripMember(ctx context.Context, tripID, userID string) (*types.TripMembership, error) {
	isMember := m.tripMembers[tripID+":"+userID]
	if !isMember {
		return nil, nil
	}

	return &types.TripMembership{
		TripID: tripID,
		UserID: userID,
		Role:   types.MemberRoleOwner, // Default role for test
	}, nil
}

// MockStorage is a simple struct to store test data
type MockStorage struct {
	ChatMessages    map[string]services.ChatMessage
	Reactions       map[string]services.ChatReaction
	LocationUpdates map[string]services.LocationUpdate
	ReadReceipts    map[string]string // tripID:userID -> messageID
}

// ChatHandlerStub implements a minimal handler for testing
type ChatHandlerStub struct {
	tripService  types.TripServiceInterface
	storage      *MockStorage
	featureFlags config.FeatureFlags
}

// LocationHandlerStub implements a minimal handler for testing
type LocationHandlerStub struct {
	tripService  types.TripServiceInterface
	storage      *MockStorage
	featureFlags config.FeatureFlags
}

// SupabaseHandlerTestSuite defines the test suite for Supabase handlers
type SupabaseHandlerTestSuite struct {
	suite.Suite
	storage         *MockStorage
	tripService     types.TripServiceInterface
	router          *gin.Engine
	chatHandler     *ChatHandlerStub
	locationHandler *LocationHandlerStub
	testTripID      string
	testUserID      string
}

// Custom handler implementations for testing

func (h *ChatHandlerStub) SendMessage(c *gin.Context) {
	tripID := c.Param("tripId")
	userID := c.GetString(string(middleware.UserIDKey))

	// Verify user is a member of the trip
	member, err := h.tripService.GetTripMember(c.Request.Context(), tripID, userID)
	if err != nil || member == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not a trip member"})
		return
	}

	var req struct {
		Message string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	messageID := uuid.New().String()
	now := time.Now()

	// Store the message
	msg := services.ChatMessage{
		ID:        messageID,
		TripID:    tripID,
		UserID:    userID,
		Message:   req.Message,
		CreatedAt: now,
	}
	h.storage.ChatMessages[messageID] = msg

	c.JSON(http.StatusCreated, gin.H{
		"id":        messageID,
		"tripId":    tripID,
		"userId":    userID,
		"message":   req.Message,
		"createdAt": now,
	})
}

func (h *ChatHandlerStub) GetMessages(c *gin.Context) {
	c.JSON(http.StatusOK, []gin.H{})
}

func (h *ChatHandlerStub) UpdateReadStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (h *ChatHandlerStub) AddReaction(c *gin.Context) {
	messageID := c.Param("messageId")
	userID := c.GetString(string(middleware.UserIDKey))

	var req struct {
		Emoji string `json:"emoji" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Store the reaction
	key := fmt.Sprintf("%s:%s:%s", messageID, userID, req.Emoji)
	h.storage.Reactions[key] = services.ChatReaction{
		MessageID: messageID,
		UserID:    userID,
		Emoji:     req.Emoji,
	}

	c.JSON(http.StatusCreated, gin.H{
		"messageId": messageID,
		"userId":    userID,
		"emoji":     req.Emoji,
	})
}

func (h *ChatHandlerStub) RemoveReaction(c *gin.Context) {
	messageID := c.Param("messageId")
	emoji := c.Param("emoji")
	userID := c.GetString(string(middleware.UserIDKey))

	// Remove the reaction
	key := fmt.Sprintf("%s:%s:%s", messageID, userID, emoji)
	delete(h.storage.Reactions, key)

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (h *LocationHandlerStub) UpdateLocation(c *gin.Context) {
	tripID := c.Query("tripId")
	userID := c.GetString(string(middleware.UserIDKey))

	var req struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Accuracy  float32 `json:"accuracy"`
		Privacy   string  `json:"privacy"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Store the location
	key := fmt.Sprintf("%s:%s", tripID, userID)
	h.storage.LocationUpdates[key] = services.LocationUpdate{
		TripID:    tripID,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
		Accuracy:  req.Accuracy,
		Privacy:   req.Privacy,
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (h *LocationHandlerStub) GetTripMemberLocations(c *gin.Context) {
	c.JSON(http.StatusOK, []gin.H{})
}

func (suite *SupabaseHandlerTestSuite) SetupSuite() {
	// Set up test IDs
	suite.testTripID = uuid.New().String()
	suite.testUserID = uuid.New().String()

	// Create storage for test data
	suite.storage = &MockStorage{
		ChatMessages:    make(map[string]services.ChatMessage),
		Reactions:       make(map[string]services.ChatReaction),
		LocationUpdates: make(map[string]services.LocationUpdate),
		ReadReceipts:    make(map[string]string),
	}

	// Create trip service
	suite.tripService = NewSupabaseTripService(suite.testTripID, suite.testUserID)

	// Set up feature flags
	featureFlags := config.FeatureFlags{
		EnableSupabaseRealtime: true,
	}

	// Create handlers
	suite.chatHandler = &ChatHandlerStub{
		tripService:  suite.tripService,
		storage:      suite.storage,
		featureFlags: featureFlags,
	}

	suite.locationHandler = &LocationHandlerStub{
		tripService:  suite.tripService,
		storage:      suite.storage,
		featureFlags: featureFlags,
	}

	// Set up Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Setup middleware to inject user ID
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.UserIDKey), suite.testUserID)
		c.Next()
	})

	// Setup chat routes
	chatGroup := router.Group("/trips/:tripId/chat")
	{
		chatGroup.POST("/messages", suite.chatHandler.SendMessage)
		chatGroup.GET("/messages", suite.chatHandler.GetMessages)
		chatGroup.PUT("/messages/read", suite.chatHandler.UpdateReadStatus)
		chatGroup.POST("/messages/:messageId/reactions", suite.chatHandler.AddReaction)
		chatGroup.DELETE("/messages/:messageId/reactions/:emoji", suite.chatHandler.RemoveReaction)
	}

	// Setup location routes
	locationGroup := router.Group("/locations")
	{
		locationGroup.PUT("", suite.locationHandler.UpdateLocation)
		locationGroup.GET("/trips/:tripId", suite.locationHandler.GetTripMemberLocations)
	}

	suite.router = router
}

func (suite *SupabaseHandlerTestSuite) TestSendMessage() {
	// Prepare request
	message := gin.H{
		"message": "Hello, this is a test message",
	}
	body, _ := json.Marshal(message)

	// Perform request
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/trips/%s/chat/messages", suite.testTripID), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	// Assert response
	suite.Equal(http.StatusCreated, w.Code)

	// Parse response
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	suite.NoError(err)

	// Verify response contains expected fields
	suite.Contains(response, "id")
	suite.Contains(response, "tripId")
	suite.Contains(response, "userId")
	suite.Contains(response, "message")
	suite.Contains(response, "createdAt")

	// Verify message was stored in mock service
	messageID := response["id"].(string)
	storedMessage, exists := suite.storage.ChatMessages[messageID]
	suite.True(exists)
	suite.Equal(suite.testTripID, storedMessage.TripID)
	suite.Equal(suite.testUserID, storedMessage.UserID)
	suite.Equal("Hello, this is a test message", storedMessage.Message)
}

func (suite *SupabaseHandlerTestSuite) TestAddAndRemoveReaction() {
	// First, create a message to react to
	messageReq := gin.H{
		"message": "A message to react to",
	}
	messageBody, _ := json.Marshal(messageReq)
	msgReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/trips/%s/chat/messages", suite.testTripID), bytes.NewBuffer(messageBody))
	msgReq.Header.Set("Content-Type", "application/json")
	msgW := httptest.NewRecorder()
	suite.router.ServeHTTP(msgW, msgReq)

	// Parse response to get message ID
	var msgResponse map[string]interface{}
	err := json.Unmarshal(msgW.Body.Bytes(), &msgResponse)
	suite.NoError(err)
	messageID := msgResponse["id"].(string)

	// Add a reaction
	reaction := gin.H{
		"emoji": "👍",
	}
	reactionBody, _ := json.Marshal(reaction)
	addReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/trips/%s/chat/messages/%s/reactions", suite.testTripID, messageID), bytes.NewBuffer(reactionBody))
	addReq.Header.Set("Content-Type", "application/json")
	addW := httptest.NewRecorder()
	suite.router.ServeHTTP(addW, addReq)

	// Assert response
	suite.Equal(http.StatusCreated, addW.Code)

	// Verify reaction was added
	reactionKey := fmt.Sprintf("%s:%s:%s", messageID, suite.testUserID, "👍")
	_, exists := suite.storage.Reactions[reactionKey]
	suite.True(exists)

	// Remove the reaction
	removeReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/trips/%s/chat/messages/%s/reactions/%s", suite.testTripID, messageID, "👍"), nil)
	removeW := httptest.NewRecorder()
	suite.router.ServeHTTP(removeW, removeReq)

	// Assert response
	suite.Equal(http.StatusOK, removeW.Code)

	// Verify reaction was removed
	_, exists = suite.storage.Reactions[reactionKey]
	suite.False(exists)
}

func (suite *SupabaseHandlerTestSuite) TestUpdateLocation() {
	// Prepare location update request
	location := gin.H{
		"latitude":  40.7128,
		"longitude": -74.0060,
		"accuracy":  10.5,
		"privacy":   "approximate",
	}
	body, _ := json.Marshal(location)

	// Create request with trip ID in query param
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/locations?tripId=%s", suite.testTripID), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	// Assert response
	suite.Equal(http.StatusOK, w.Code)

	// Verify location was updated in mock service
	locationKey := fmt.Sprintf("%s:%s", suite.testTripID, suite.testUserID)
	storedLocation, exists := suite.storage.LocationUpdates[locationKey]
	suite.True(exists)
	suite.Equal(suite.testTripID, storedLocation.TripID)
	suite.Equal(40.7128, storedLocation.Latitude)
	suite.Equal(-74.0060, storedLocation.Longitude)
	suite.Equal(float32(10.5), storedLocation.Accuracy)
	suite.Equal("approximate", storedLocation.Privacy)
}

// Run the test suite
func TestSupabaseHandlerSuite(t *testing.T) {
	suite.Run(t, new(SupabaseHandlerTestSuite))
}
