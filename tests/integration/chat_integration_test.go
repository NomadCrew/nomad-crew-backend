package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/handlers"
	"github.com/NomadCrew/nomad-crew-backend/internal/auth"
	"github.com/NomadCrew/nomad-crew-backend/internal/events"
	"github.com/NomadCrew/nomad-crew-backend/internal/service"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	internalPostgresStore "github.com/NomadCrew/nomad-crew-backend/internal/store/postgres"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	rootPostgresStore "github.com/NomadCrew/nomad-crew-backend/store/postgres"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	postgresContainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	redisTC "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
)

// MockTripService implements TripServiceInterface for testing
type MockTripService struct {
	tripMembers map[string]bool
}

func NewMockTripService(tripID, userID string) *MockTripService {
	return &MockTripService{
		tripMembers: map[string]bool{
			tripID + ":" + userID: true,
		},
	}
}

func (m *MockTripService) IsTripMember(ctx context.Context, tripID, userID string) (bool, error) {
	// Debug log to check context
	if ctxUserID, ok := ctx.Value(middleware.UserIDKey).(string); ok && ctxUserID != "" {
		fmt.Printf("MockTripService.IsTripMember: Found userID in context: %s (param userID: %s)\n",
			ctxUserID, userID)
	} else {
		fmt.Printf("MockTripService.IsTripMember: No userID in context! (param userID: %s)\n", userID)
	}

	return m.tripMembers[tripID+":"+userID], nil
}

// MockJWTValidator implements middleware.Validator for testing
type MockJWTValidator struct {
	userID string
}

func NewMockJWTValidator(userID string) *MockJWTValidator {
	return &MockJWTValidator{userID: userID}
}

func (m *MockJWTValidator) Validate(tokenString string) (string, error) {
	return m.userID, nil
}

// ChatIntegrationTestSuite tests chat functionality end-to-end
type ChatIntegrationTestSuite struct {
	suite.Suite
	ctx            context.Context
	pgContainer    *postgresContainer.PostgresContainer
	pgPool         *pgxpool.Pool
	redisContainer *redisTC.RedisContainer
	redisClient    *redis.Client
	chatStore      store.ChatStore
	tripStore      store.TripStore
	userStore      store.UserStore
	eventService   types.EventPublisher
	chatService    service.ChatService
	testTripID     string
	testUserID     string
	testGroupID    string   // Added for clarity
	cleanupFuncs   []func() // Store cleanup functions
	testServer     *httptest.Server
	httpClient     *http.Client
	jwtToken       string
	jwtSecret      string
	router         *gin.Engine
}

// setupPostgresContainer sets up a PostgreSQL container using testcontainers.
func setupPostgresContainer(ctx context.Context, t *testing.T) (*postgresContainer.PostgresContainer, *pgxpool.Pool, func()) {
	pgContainer, err := postgresContainer.Run(ctx,
		"postgres:14",
		postgresContainer.WithDatabase("testdb"),
		postgresContainer.WithUsername("testuser"),
		postgresContainer.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp"),
		),
	)
	require.NoError(t, err)

	// Allow extra time for the container to fully initialize
	time.Sleep(4 * time.Second)

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Retry logic for database connection
	var pool *pgxpool.Pool
	var connectErr error
	for retries := 0; retries < 3; retries++ {
		pool, connectErr = pgxpool.Connect(ctx, connStr)
		if connectErr == nil {
			break
		}
		t.Logf("Database connection attempt %d failed: %v, retrying...", retries+1, connectErr)
		time.Sleep(time.Duration(retries+1) * time.Second)
	}
	require.NoError(t, connectErr, "Failed to connect to database after multiple attempts")

	// Run migrations
	migrationSQL, err := os.ReadFile("../../db/migrations/000001_init.up.sql")
	require.NoError(t, err, "Failed to read migration file")
	_, err = pool.Exec(ctx, string(migrationSQL))
	require.NoError(t, err, "Failed to apply migration")

	cleanup := func() {
		if pool != nil {
			pool.Close()
		}
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate postgres container: %v", err)
		}
	}

	return pgContainer, pool, cleanup
}

// setupRedisContainer sets up a Redis container using testcontainers.
func setupRedisContainer(ctx context.Context, t *testing.T) (*redisTC.RedisContainer, *redis.Client, func()) {
	redisContainer, err := redisTC.Run(ctx,
		"docker.io/redis:7",
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("6379/tcp"),
		),
	)
	require.NoError(t, err)

	// Allow extra time for the container to fully initialize
	time.Sleep(2 * time.Second)

	host, err := redisContainer.Host(ctx)
	require.NoError(t, err)
	port, err := redisContainer.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)

	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", host, port.Port()),
	})
	require.NoError(t, redisClient.Ping(ctx).Err())

	cleanup := func() {
		if err := redisContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate redis container: %v", err)
		}
	}

	return redisContainer, redisClient, cleanup
}

// setupTestRouter creates a Gin router with required handlers and middleware for testing
func (suite *ChatIntegrationTestSuite) setupTestRouter() *gin.Engine {
	logger, _ := zap.NewDevelopment()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// Setup chat handler
	mockTripService := NewMockTripService(suite.testTripID, suite.testUserID)
	chatHandler := handlers.NewChatHandler(
		suite.chatService,
		mockTripService,
		suite.eventService,
		logger,
	)

	// TEMPORARY TEST SOLUTION:
	// Set the user ID using the standard key defined in middleware package
	// This matches what the real middleware uses
	authTestMiddleware := func(c *gin.Context) {
		// First check if the userID was passed in the request context
		userID := ""
		if ctxUserID, ok := c.Request.Context().Value(middleware.UserIDKey).(string); ok && ctxUserID != "" {
			userID = ctxUserID
			fmt.Printf("Test middleware found userID in Request.Context(): %s\n", userID)
		} else {
			userID = suite.testUserID
			fmt.Printf("Test middleware using default suite.testUserID: %s\n", userID)
		}

		// Set the key that middleware.AuthMiddleware sets
		c.Set(string(middleware.UserIDKey), userID)

		// Set directly in request context too
		newCtx := context.WithValue(c.Request.Context(), middleware.UserIDKey, userID)
		c.Request = c.Request.WithContext(newCtx)

		fmt.Printf("Auth middleware set userID '%s' in Gin context and Request context\n", userID)
		c.Next()
	}

	// Create authenticated group
	auth := r.Group("")
	auth.Use(authTestMiddleware)

	// Add chat routes
	tripGroup := auth.Group("/trips/:id")
	chatGroup := tripGroup.Group("/chat")

	chatGroup.GET("/messages", chatHandler.ListMessages)
	chatGroup.POST("/messages", chatHandler.SendMessage)
	chatGroup.PUT("/messages/:messageId", chatHandler.UpdateMessage)
	chatGroup.DELETE("/messages/:messageId", chatHandler.DeleteMessage)
	chatGroup.POST("/messages/:messageId/reactions", chatHandler.AddReaction)
	chatGroup.DELETE("/messages/:messageId/reactions/:reactionType", chatHandler.RemoveReaction)
	chatGroup.PUT("/read", chatHandler.UpdateLastRead)

	return r
}

// generateTestUserToken generates a JWT token for the test user
func (suite *ChatIntegrationTestSuite) generateTestUserToken() string {
	// Use a fixed secret for tests
	suite.jwtSecret = "test_jwt_secret_for_integration_tests"
	token, err := auth.GenerateJWT(
		suite.testUserID,
		"test@example.com",
		suite.jwtSecret,
		time.Hour,
	)
	require.NoError(suite.T(), err, "Failed to generate test JWT token")
	return token
}

// makeAuthenticatedRequest is a helper to make HTTP requests with authentication
func (suite *ChatIntegrationTestSuite) makeAuthenticatedRequest(method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, suite.testServer.URL+path, body)
	if err != nil {
		return nil, err
	}

	// Add auth header
	req.Header.Set("Authorization", "Bearer "+suite.jwtToken)
	req.Header.Set("Content-Type", "application/json")

	// Add debug context for integration tests to track context flow
	debugCtx := context.WithValue(req.Context(), middleware.UserIDKey, suite.testUserID)
	reqWithDebugCtx := req.WithContext(debugCtx)

	// Log for debugging
	fmt.Printf("Integration test making %s request to %s with userID: %s in context\n",
		method, path, debugCtx.Value(middleware.UserIDKey))

	return suite.httpClient.Do(reqWithDebugCtx)
}

// SetupSuite prepares the test suite once before all tests
func (suite *ChatIntegrationTestSuite) SetupSuite() {
	t := suite.T() // Get the testing.T object
	suite.ctx = context.Background()

	// Skip container tests on Windows
	if runtime.GOOS == "windows" {
		t.Skip("Skipping integration test on Windows - testcontainers setup issues")
	}

	suite.cleanupFuncs = make([]func(), 0)

	// Setup PostgreSQL
	var pgCleanup func()
	suite.pgContainer, suite.pgPool, pgCleanup = setupPostgresContainer(suite.ctx, t)
	suite.cleanupFuncs = append(suite.cleanupFuncs, pgCleanup)

	// Setup Redis
	var redisCleanup func()
	suite.redisContainer, suite.redisClient, redisCleanup = setupRedisContainer(suite.ctx, t)
	suite.cleanupFuncs = append(suite.cleanupFuncs, redisCleanup)

	// Instantiate Stores
	suite.chatStore = internalPostgresStore.NewChatStore(suite.pgPool)
	suite.tripStore = rootPostgresStore.NewPgTripStore(suite.pgPool)
	// Use placeholder Supabase creds for UserStore as they are not directly needed for core chat tests
	suite.userStore = internalPostgresStore.NewUserStore(suite.pgPool, "placeholder_supabase_url", "placeholder_supabase_key")

	// Instantiate Event Publisher
	suite.eventService = events.NewRedisPublisher(suite.redisClient)

	// Instantiate Chat Service
	// Explicitly create the concrete type
	chatServiceImpl := service.NewChatService(suite.chatStore, suite.tripStore, suite.eventService)
	chatServiceImpl.SetUserStore(suite.userStore)
	// Assign the concrete type that implements the interface
	suite.chatService = chatServiceImpl

	require.NotNil(t, suite.chatService, "Chat service should be initialized")
}

// TearDownSuite cleans up after all tests
func (suite *ChatIntegrationTestSuite) TearDownSuite() {
	if suite.testServer != nil {
		suite.testServer.Close()
	}

	for i := len(suite.cleanupFuncs) - 1; i >= 0; i-- {
		suite.cleanupFuncs[i]()
	}
}

// SetupTest runs before each test
func (suite *ChatIntegrationTestSuite) SetupTest() {
	t := suite.T()
	// Setup test data for each test
	preGeneratedUserID := uuid.NewString() // Keep for email/username generation if needed, but don't assume it's the final ID

	// Create test user
	// Capture the actual ID returned by the store
	actualUserID, err := suite.userStore.CreateUser(suite.ctx, &types.User{
		// ID:         preGeneratedUserID, // Do not set ID here, let the DB generate it if that's the store's behavior
		SupabaseID: "supabase|" + preGeneratedUserID, // Ensure unique Supabase ID
		Email:      fmt.Sprintf("test-%s@example.com", preGeneratedUserID),
		Username:   fmt.Sprintf("testuser-%s", preGeneratedUserID),
	})
	require.NoError(t, err, "Failed to create test user")
	require.NotEmpty(t, actualUserID, "CreateUser should return a valid user ID")
	suite.testUserID = actualUserID // Use the actual ID from the database

	t.Logf("User %s created, waiting briefly before trip/group creation...", suite.testUserID)
	time.Sleep(100 * time.Millisecond) // Small delay to ensure user commit is visible

	// Create test trip
	// Note: The trip ID is generated by the store, not pre-assigned here.
	t.Logf("Attempting to create trip for UserID: %s", suite.testUserID)
	// Store the returned actual trip ID
	actualTripID, err := suite.tripStore.CreateTrip(suite.ctx, types.Trip{
		Name:        "Chat Integration Test Trip",
		CreatedBy:   &suite.testUserID,
		StartDate:   time.Now(),
		EndDate:     time.Now().Add(24 * time.Hour),
		Status:      types.TripStatusPlanning,
		Description: "Trip for testing chat",
	})
	if err != nil {
		t.Logf("CreateTrip failed: %v", err)
	}
	require.NoError(t, err, "Failed to create test trip")
	require.NotEmpty(t, actualTripID, "CreateTrip should return a valid trip ID")

	// Fetch the created trip to ensure we have the definitive DB-generated ID
	createdTrip, errGet := suite.tripStore.GetTrip(suite.ctx, actualTripID)
	require.NoError(t, errGet, "Failed to get created trip")
	require.NotNil(t, createdTrip, "Fetched created trip should not be nil")

	suite.testTripID = createdTrip.ID // Update suite's trip ID to the actual created one from GetTrip
	t.Logf("CreateTrip succeeded. Actual Trip ID from GetTrip: %s, UserID: %s", suite.testTripID, suite.testUserID)

	// Verify the member was added by CreateTrip (optional check)
	members, err := suite.tripStore.GetTripMembers(suite.ctx, suite.testTripID)
	require.NoError(t, err, "Failed to get trip members after trip creation")
	foundOwner := false
	for _, member := range members {
		if member.UserID == suite.testUserID && member.Role == types.MemberRoleOwner {
			foundOwner = true
			break
		}
	}
	require.True(t, foundOwner, "Test user should be owner of the trip after CreateTrip")

	// Create a chat group for the test trip
	t.Logf("Attempting to create chat group for TripID: %s by UserID: %s", suite.testTripID, suite.testUserID)
	group, err := suite.chatService.CreateGroup(suite.ctx, suite.testTripID, "Test Chat Group", suite.testUserID)
	require.NoError(t, err, "Failed to create chat group")
	require.NotNil(t, group, "Created group should not be nil")
	suite.testGroupID = group.ID // Store the created group ID

	// Setup HTTP test server with authentication
	suite.router = suite.setupTestRouter()
	suite.testServer = httptest.NewServer(suite.router)
	suite.httpClient = &http.Client{
		Timeout: 5 * time.Second,
	}

	// Generate JWT token for authentication
	suite.jwtToken = suite.generateTestUserToken()
}

// TearDownTest cleans up after each test
func (suite *ChatIntegrationTestSuite) TearDownTest() {
	// Use DELETE FROM ... WHERE ... for more targeted cleanup
	// Order matters due to foreign key constraints
	// Use suite.testTripID which now holds the actual ID

	_, err := suite.pgPool.Exec(suite.ctx, "DELETE FROM chat_message_reactions")
	suite.NoError(err, "Cleanup chat_message_reactions failed")

	_, err = suite.pgPool.Exec(suite.ctx, "DELETE FROM chat_group_members")
	suite.NoError(err, "Cleanup chat_group_members failed")

	_, err = suite.pgPool.Exec(suite.ctx, "DELETE FROM chat_messages")
	suite.NoError(err, "Cleanup chat_messages failed")

	_, err = suite.pgPool.Exec(suite.ctx, "DELETE FROM chat_groups WHERE trip_id = $1", suite.testTripID)
	suite.NoError(err, "Cleanup chat_groups failed")

	_, err = suite.pgPool.Exec(suite.ctx, "DELETE FROM trip_memberships WHERE trip_id = $1", suite.testTripID)
	suite.NoError(err, "Cleanup trip_memberships failed")

	_, err = suite.pgPool.Exec(suite.ctx, "DELETE FROM trips WHERE id = $1", suite.testTripID)
	suite.NoError(err, "Cleanup trips failed")

	_, err = suite.pgPool.Exec(suite.ctx, "DELETE FROM users WHERE id = $1", suite.testUserID)
	suite.NoError(err, "Cleanup users failed")
}

// TestChatMessageLifecycle tests the full lifecycle of a chat message using HTTP API
func (suite *ChatIntegrationTestSuite) TestChatMessageLifecycle() {
	t := suite.T()

	// 1. Create a new message via HTTP API
	messageText := "Hello, integration test!"
	reqBody := fmt.Sprintf(`{"content":"%s"}`, messageText)
	createURL := fmt.Sprintf("/trips/%s/chat/messages", suite.testTripID)

	resp, err := suite.makeAuthenticatedRequest(http.MethodPost, createURL, strings.NewReader(reqBody))
	require.NoError(t, err, "Error making POST request to create message")
	require.Equal(t, http.StatusCreated, resp.StatusCode, "Expected status 201 Created")

	// Parse response - Changed from ChatMessage to ChatMessageWithUser to match the API response structure
	var createResponse types.ChatMessageWithUser
	err = json.NewDecoder(resp.Body).Decode(&createResponse)
	resp.Body.Close()
	require.NoError(t, err, "Error decoding create message response")

	// Access ID through the Message field
	require.NotEmpty(t, createResponse.Message.ID, "Message ID should not be empty")
	messageID := createResponse.Message.ID

	// 2. Retrieve the message via HTTP API
	getURL := fmt.Sprintf("/trips/%s/chat/messages?limit=10&offset=0", suite.testTripID)
	resp, err = suite.makeAuthenticatedRequest(http.MethodGet, getURL, nil)
	require.NoError(t, err, "Error making GET request")
	require.Equal(t, http.StatusOK, resp.StatusCode, "Expected status 200 OK")

	// Parse response
	var listResponse types.ChatMessagePaginatedResponse
	err = json.NewDecoder(resp.Body).Decode(&listResponse)
	resp.Body.Close()
	require.NoError(t, err, "Error decoding list messages response")

	// Find our message
	var foundMessage *types.ChatMessageWithUser
	for _, msg := range listResponse.Messages {
		if msg.Message.ID == messageID {
			foundMessage = &msg
			break
		}
	}
	require.NotNil(t, foundMessage, "Created message not found in list response")
	assert.Equal(t, messageText, foundMessage.Message.Content)

	// 3. Update the message via HTTP API
	updatedText := "Updated message content"
	updateBody := fmt.Sprintf(`{"content":"%s"}`, updatedText)
	updateURL := fmt.Sprintf("/trips/%s/chat/messages/%s", suite.testTripID, messageID)

	resp, err = suite.makeAuthenticatedRequest(http.MethodPut, updateURL, strings.NewReader(updateBody))
	require.NoError(t, err, "Error making PUT request")
	require.Equal(t, http.StatusOK, resp.StatusCode, "Expected status 200 OK")

	// 4. Delete the message via HTTP API
	deleteURL := fmt.Sprintf("/trips/%s/chat/messages/%s", suite.testTripID, messageID)
	resp, err = suite.makeAuthenticatedRequest(http.MethodDelete, deleteURL, nil)
	require.NoError(t, err, "Error making DELETE request")
	require.Equal(t, http.StatusOK, resp.StatusCode, "Expected status 200 OK")
	resp.Body.Close()

	// Verify deletion by trying to get the message again
	resp, err = suite.makeAuthenticatedRequest(http.MethodGet, getURL, nil)
	require.NoError(t, err, "Error making GET request after deletion")

	var listResponse2 types.ChatMessagePaginatedResponse
	err = json.NewDecoder(resp.Body).Decode(&listResponse2)
	resp.Body.Close()
	require.NoError(t, err, "Error decoding list response after deletion")

	// Message should not be in the list
	var foundDeletedMessage bool
	for _, msg := range listResponse2.Messages {
		if msg.Message.ID == messageID {
			foundDeletedMessage = true
			break
		}
	}
	assert.False(t, foundDeletedMessage, "Deleted message should not be found")
}

// TestChatReactions tests adding and removing reactions to messages
func (suite *ChatIntegrationTestSuite) TestChatReactions() {
	t := suite.T()

	// 1. Create a test message via HTTP API
	messageText := "React to this message"
	reqBody := fmt.Sprintf(`{"content":"%s"}`, messageText)
	createURL := fmt.Sprintf("/trips/%s/chat/messages", suite.testTripID)

	resp, err := suite.makeAuthenticatedRequest(http.MethodPost, createURL, strings.NewReader(reqBody))
	require.NoError(t, err, "Error making POST request to create message")
	require.Equal(t, http.StatusCreated, resp.StatusCode, "Expected status 201 Created")

	// Parse response - Changed from ChatMessage to ChatMessageWithUser to match the API response structure
	var createResponse types.ChatMessageWithUser
	err = json.NewDecoder(resp.Body).Decode(&createResponse)
	resp.Body.Close()
	require.NoError(t, err, "Error decoding create message response")

	// Access ID through the Message field
	messageID := createResponse.Message.ID

	// 2. Add a reaction via HTTP API
	reaction := "👍"
	reactionBody := fmt.Sprintf(`{"reaction":"%s"}`, reaction)
	addReactionURL := fmt.Sprintf("/trips/%s/chat/messages/%s/reactions", suite.testTripID, messageID)

	resp, err = suite.makeAuthenticatedRequest(http.MethodPost, addReactionURL, strings.NewReader(reactionBody))
	require.NoError(t, err, "Error making POST request to add reaction")
	require.Equal(t, http.StatusOK, resp.StatusCode, "Expected status 200 OK")
	resp.Body.Close()

	// 3. Remove the reaction via HTTP API
	removeReactionURL := fmt.Sprintf("/trips/%s/chat/messages/%s/reactions/%s", suite.testTripID, messageID, reaction)

	resp, err = suite.makeAuthenticatedRequest(http.MethodDelete, removeReactionURL, nil)
	require.NoError(t, err, "Error making DELETE request to remove reaction")
	require.Equal(t, http.StatusOK, resp.StatusCode, "Expected status 200 OK")
	resp.Body.Close()
}

// TestChatMessagePagination tests fetching messages with pagination via HTTP API
func (suite *ChatIntegrationTestSuite) TestChatMessagePagination() {
	t := suite.T()

	// 1. Send multiple messages via HTTP API
	numMessages := 25
	for i := 0; i < numMessages; i++ {
		messageText := fmt.Sprintf("Pagination test message %d", i)
		reqBody := fmt.Sprintf(`{"content":"%s"}`, messageText)
		createURL := fmt.Sprintf("/trips/%s/chat/messages", suite.testTripID)

		resp, err := suite.makeAuthenticatedRequest(http.MethodPost, createURL, strings.NewReader(reqBody))
		require.NoError(t, err, "Error making POST request to create message")
		require.Equal(t, http.StatusCreated, resp.StatusCode, "Expected status 201 Created")
		resp.Body.Close()
	}

	// 2. Test pagination - first page
	getFirstPageURL := fmt.Sprintf("/trips/%s/chat/messages?limit=10&offset=0", suite.testTripID)
	resp, err := suite.makeAuthenticatedRequest(http.MethodGet, getFirstPageURL, nil)
	require.NoError(t, err, "Error making GET request for first page")
	require.Equal(t, http.StatusOK, resp.StatusCode, "Expected status 200 OK")

	var firstPageResponse types.ChatMessagePaginatedResponse
	err = json.NewDecoder(resp.Body).Decode(&firstPageResponse)
	resp.Body.Close()
	require.NoError(t, err, "Error decoding first page response")
	assert.Len(t, firstPageResponse.Messages, 10, "First page should have 10 messages")

	// 3. Test pagination - second page
	getSecondPageURL := fmt.Sprintf("/trips/%s/chat/messages?limit=10&offset=10", suite.testTripID)
	resp, err = suite.makeAuthenticatedRequest(http.MethodGet, getSecondPageURL, nil)
	require.NoError(t, err, "Error making GET request for second page")
	require.Equal(t, http.StatusOK, resp.StatusCode, "Expected status 200 OK")

	var secondPageResponse types.ChatMessagePaginatedResponse
	err = json.NewDecoder(resp.Body).Decode(&secondPageResponse)
	resp.Body.Close()
	require.NoError(t, err, "Error decoding second page response")
	assert.Len(t, secondPageResponse.Messages, 10, "Second page should have 10 messages")

	// 4. Test pagination - third page (remaining 5)
	getThirdPageURL := fmt.Sprintf("/trips/%s/chat/messages?limit=10&offset=20", suite.testTripID)
	resp, err = suite.makeAuthenticatedRequest(http.MethodGet, getThirdPageURL, nil)
	require.NoError(t, err, "Error making GET request for third page")
	require.Equal(t, http.StatusOK, resp.StatusCode, "Expected status 200 OK")

	var thirdPageResponse types.ChatMessagePaginatedResponse
	err = json.NewDecoder(resp.Body).Decode(&thirdPageResponse)
	resp.Body.Close()
	require.NoError(t, err, "Error decoding third page response")
	assert.Len(t, thirdPageResponse.Messages, 5, "Third page should have 5 messages")
}

// Skip WebSocketIntegration test as it requires a more complex setup
func (suite *ChatIntegrationTestSuite) TestWebSocketIntegration() {
	suite.T().Skip("WebSocket testing requires a more complex setup with WebSocket client")
}

// Ensure the test suite is run
func TestChatIntegrationSuite(t *testing.T) {
	suite.Run(t, new(ChatIntegrationTestSuite))
}
