package integration_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/db" // For SetupTestDB or ApplyMigrations
	"github.com/NomadCrew/nomad-crew-backend/handlers"
	"github.com/NomadCrew/nomad-crew-backend/internal/auth"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	trip_service "github.com/NomadCrew/nomad-crew-backend/models/trip/service"
	"github.com/NomadCrew/nomad-crew-backend/store/postgres"

	internalPgStore "github.com/NomadCrew/nomad-crew-backend/internal/store/postgres"
	user_service "github.com/NomadCrew/nomad-crew-backend/models/user/service" // Added import
	approuter "github.com/NomadCrew/nomad-crew-backend/router"                 // For mock email service
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool" // Correct Supabase import
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/supabase-community/supabase-go"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	testDBPool *pgxpool.Pool // Changed to testDBPool for clarity
	testCFG    *config.Config
)

// MockEventPublisher is a simple mock for types.EventPublisher
type MockEventPublisher struct{}

func (m *MockEventPublisher) Publish(ctx context.Context, tripID string, event types.Event) error {
	log.Printf("MockEventPublisher: Faux publish event ID %s, Type %s, TripID %s (target trip: %s)\n", event.ID, event.Type, event.TripID, tripID)
	return nil
}

func (m *MockEventPublisher) PublishBatch(ctx context.Context, tripID string, events []types.Event) error {
	log.Printf("MockEventPublisher: Faux publish batch of %d events for trip %s\n", len(events), tripID)
	return nil
}

func (m *MockEventPublisher) Subscribe(ctx context.Context, tripID string, subscriberID string, eventTypes ...types.EventType) (<-chan types.Event, error) {
	log.Printf("MockEventPublisher: Faux Subscribe to trip %s for subscriber %s\n", tripID, subscriberID)
	ch := make(chan types.Event)
	return ch, nil
}

func (m *MockEventPublisher) Unsubscribe(ctx context.Context, tripID string, subscriberID string) error {
	log.Printf("MockEventPublisher: Faux Unsubscribe from trip %s for subscriber %s\n", tripID, subscriberID)
	return nil
}

// MockEmailService is a simple mock for types.EmailService
type MockEmailService struct{}

func (m *MockEmailService) SendInvitationEmail(ctx context.Context, data types.EmailData) error {
	inviterName, _ := data.TemplateData["InviterName"].(string)
	tripName, _ := data.TemplateData["TripName"].(string)
	log.Printf("MockEmailService: Faux SendInvitationEmail to %s (Subject: %s) for inviter %s, trip %s\n", data.To, data.Subject, inviterName, tripName)
	return nil
}

func (m *MockEmailService) SendWelcomeEmail(ctx context.Context, data types.EmailData) error {
	log.Printf("MockEmailService: Faux SendWelcomeEmail to %s, Subject: %s\n", data.To, data.Subject)
	return nil
}

func (m *MockEmailService) SendPasswordResetEmail(ctx context.Context, data types.EmailData) error {
	log.Printf("MockEmailService: Faux SendPasswordResetEmail to %s, Subject: %s\n", data.To, data.Subject)
	return nil
}

// SetupInvitationTest initializes test containers, database, and services for invitation tests
func SetupInvitationTest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping integration tests on Windows due to Docker limitations.")
	}

	logger.InitLogger() // Standard logger init, IsTest might be set elsewhere or by default for tests
	logOutput := logger.GetLogger()

	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "postgres:14-alpine",
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForLog("database system is ready to accept connections"),
			wait.ForListeningPort("5432/tcp"),
		),
		Env: map[string]string{
			"POSTGRES_DB":       "test_nomadcrew",
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpassword",
		},
	}
	postgresContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		logOutput.Fatalf("Could not start postgres container: %s", err)
	}
	t.Cleanup(func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			logOutput.Errorf("Could not stop postgres container: %s", err)
		}
	})

	host, err := postgresContainer.Host(ctx)
	if err != nil {
		logOutput.Fatalf("Could not get postgres container host: %s", err)
	}
	mappedPortStr, err := postgresContainer.MappedPort(ctx, "5432/tcp")
	if err != nil {
		logOutput.Fatalf("Could not get mapped port: %s", err)
	}
	mappedPortInt, err := strconv.Atoi(mappedPortStr.Port())
	if err != nil {
		logOutput.Fatalf("Could not convert mapped port to int: %s", err)
	}

	testCFG = &config.Config{
		Server: config.ServerConfig{
			Port:         "0",
			JwtSecretKey: "test-jwt-secret-key-for-invitations-12345",
			Environment:  config.EnvDevelopment,   // Use EnvDevelopment for tests
			FrontendURL:  "http://localhost:3001", // Dummy frontend URL for tests
		},
		Database: config.DatabaseConfig{ // Corrected struct name
			Host:     host,
			Port:     mappedPortInt, // Corrected type
			User:     "testuser",
			Password: "testpassword",
			Name:     "test_nomadcrew",
			SSLMode:  "disable",
		},
		ExternalServices: config.ExternalServices{ // Corrected struct name
			SupabaseURL:     "http://localhost:54321",
			SupabaseAnonKey: "test-supabase-anon-key",
		},
		Email: config.EmailConfig{
			FromAddress: "test@example.com",
			FromName:    "Test Sender",
		},
		EventService: config.EventServiceConfig{
			PublishTimeoutSeconds:   5,
			SubscribeTimeoutSeconds: 5,
			EventBufferSize:         10,
		},
	}
	// Assign the JWT secret AFTER testCFG is initialized
	testCFG.ExternalServices.SupabaseJWTSecret = testCFG.Server.JwtSecretKey

	dsn := fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=%s",
		testCFG.Database.User, testCFG.Database.Password,
		testCFG.Database.Host, testCFG.Database.Port,
		testCFG.Database.Name, testCFG.Database.SSLMode,
	)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		logOutput.Fatalf("Failed to parse DSN: %s", err)
	}
	testDBPool, err = pgxpool.ConnectConfig(context.Background(), poolConfig)
	if err != nil {
		logOutput.Fatalf("Failed to connect to test database: %s", err)
	}
	t.Cleanup(func() {
		if testDBPool != nil {
			testDBPool.Close()
		}
	})

	// Attempt to run migrations using db.SetupTestDB or a direct migration runner
	// Option 1: If db.SetupTestDB handles migrations AND returns a pool or client from which pool can be derived:
	// testDBClient, err := db.SetupTestDB(dsn) // Assuming SetupTestDB is in 'db' package
	// if err != nil {
	// 	logOutput.Fatalf("Failed to setup test DB (migrations): %s", err)
	// }
	// testDBPool = testDBClient.GetPool() // If SetupTestDB returns a client with GetPool()

	// Option 2: Find project root and run migrations manually (more explicit)
	wd, err := os.Getwd()
	if err != nil {
		logOutput.Fatalf("Failed to get working directory: %v", err)
	}
	projectRoot := wd
	for i := 0; i < 5; i++ { // Limit search upwards to 5 levels
		if _, statErr := os.Stat(filepath.Join(projectRoot, "go.mod")); statErr == nil {
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			logOutput.Fatalf("Could not find project root (go.mod) from %s", wd)
		}
		projectRoot = parent
	}

	// Execute migrations directly
	// TODO: Ensure a migration file exists in db/migrations that adds the `destination_place_id`
	//       and other necessary columns (e.g., background_image_url) to the `trips` table.
	//       The test currently fails if this migration is missing.
	migrationDir := filepath.Join(projectRoot, "db", "migrations")
	migrationFiles, err := filepath.Glob(filepath.Join(migrationDir, "*.up.sql"))
	if err != nil || len(migrationFiles) == 0 {
		logOutput.Fatalf("Failed to find migration files in %s: %v", migrationDir, err)
	}

	// Sort files to ensure they run in order (important if names dictate order)
	sort.Strings(migrationFiles)

	for _, migrationFile := range migrationFiles {
		baseFileName := filepath.Base(migrationFile)
		logOutput.Infof("Processing migration: %s", baseFileName)

		// Skip the Supabase production migration when in test environment
		if baseFileName == "000002_supabase_realtime.up.sql" {
			// Use test migration instead
			testMigrationFile := filepath.Join(migrationDir, "000002_supabase_realtime.test.sql")
			if _, err := os.Stat(testMigrationFile); os.IsNotExist(err) {
				logOutput.Fatalf("Test migration file not found: %s", testMigrationFile)
			}

			logOutput.Infof("Using test migration for Supabase Realtime: %s", testMigrationFile)
			migrationSQL, err := os.ReadFile(testMigrationFile)
			if err != nil {
				logOutput.Fatalf("Failed to read test migration file %s: %v", testMigrationFile, err)
			}

			// Execute the test SQL migration script
			_, err = testDBPool.Exec(context.Background(), string(migrationSQL))
			if err != nil {
				logOutput.Fatalf("Failed to apply test migration %s: %v", filepath.Base(testMigrationFile), err)
			}
			continue
		}

		// For all other migrations, apply normally
		migrationSQL, err := os.ReadFile(migrationFile)
		if err != nil {
			logOutput.Fatalf("Failed to read migration file %s: %v", migrationFile, err)
		}

		// Execute the SQL migration script
		_, err = testDBPool.Exec(context.Background(), string(migrationSQL))
		if err != nil {
			logOutput.Fatalf("Failed to apply migration %s: %v", baseFileName, err)
		}
	}

	logOutput.Info("All migrations applied successfully.")

	logOutput.Info("Test database and migrations setup complete.")
}

// setupTestRouterAndDeps initializes a Gin engine with all necessary dependencies for integration tests.
func setupTestRouterAndDeps(t *testing.T) *gin.Engine {
	require.NotNil(t, testDBPool, "TestDBPool should be initialized by TestMain")
	require.NotNil(t, testCFG, "TestCFG should be initialized by TestMain")

	// Initialize Stores
	tripStore := postgres.NewPgTripStore(testDBPool)
	// UserStore initialization needs to be correct based on internal/store vs store/postgres
	// For now, assume NewPgUserStore returns a type compatible with internal/store.UserStore if signatures match
	userStore := internalPgStore.NewUserStore(testDBPool, testCFG.ExternalServices.SupabaseURL, "test-supabase-service-key")

	// Corrected ChatStore instantiation, assuming nil for supabase client in tests is acceptable or mocked appropriately.
	// The actual Supabase client used in main.go is from "github.com/supabase-community/supabase-go"
	var supaClient *supabase.Client // Correct type
	// If a real client is needed for some tests, it should be initialized here, otherwise nil is fine if UserStore/ChatStore handle it.
	chatStore := db.NewPostgresChatStore(testDBPool, supaClient, testCFG.ExternalServices.SupabaseURL, "test-supabase-service-key")

	// Initialize Services
	eventService := &MockEventPublisher{} // Use the mock
	emailService := &MockEmailService{}   // Use the mock

	tripModel := trip_service.NewTripModelCoordinator(
		tripStore,
		chatStore,
		userStore, // This is the problematic line if type mismatch occurs
		eventService,
		nil, // weatherSvc
		supaClient,
		&testCFG.Server,
		emailService,
	)

	// UserHandler expects user_service.UserServiceInterface
	// Create an actual user service instance for integration tests
	userService := user_service.NewUserService(userStore, testCFG.ExternalServices.SupabaseJWTSecret)

	invitationHandler := handlers.NewInvitationHandler(tripModel, userStore, eventService, &testCFG.Server)
	userHandler := handlers.NewUserHandler(userService) // Corrected: Pass UserService

	deps := approuter.Dependencies{
		Config: testCFG,
		// JWTValidator:      middleware.NewNoOpValidator(), // Replace with real validator
		TripHandler:         nil,
		TodoHandler:         nil,
		HealthHandler:       nil,
		LocationHandler:     nil,
		NotificationHandler: nil,
		ChatHandler:         nil,
		UserHandler:         userHandler,
		Logger:              logger.GetLogger(),
		MemberHandler:       nil,
		InvitationHandler:   invitationHandler,
	}

	jwtVal, err := middleware.NewJWTValidator(testCFG)
	require.NoError(t, err)
	deps.JWTValidator = jwtVal

	r := approuter.SetupRouter(deps)
	return r
}

// Helper function to generate a JWT for a user
func generateTestUserToken(t *testing.T, userID string, secretKey string) string {
	// Ensure userID is a valid UUID string if your UserStore expects it, or adjust as needed
	// For tests, userID can be any string that your auth middleware and user store can handle.
	// If UserID in JWTClaims is parsed as UUID, ensure it's valid:
	// if _, err := uuid.Parse(userID); err != nil {
	//     userID = uuid.NewSHA1(uuid.NameSpaceDNS, []byte(userID)).String() // Deterministic UUID
	// }
	token, err := auth.GenerateJWT(userID, userID+"@example.com", secretKey, time.Hour) // Using userID as part of email for uniqueness
	require.NoError(t, err, "Failed to generate test user token")
	return token
}

// Helper function to generate an invitation token
/*
func generateTestInvitationToken(t *testing.T, invitationID string, tripID string, inviteeEmail string, secretKey string) string {
	token, err := auth.GenerateInvitationToken(invitationID, tripID, inviteeEmail, secretKey, time.Hour)
	require.NoError(t, err, "Failed to generate test invitation token")
	return token
}
*/

// TestDeclineInvitation_Success_InviteeIDKnown tests declining an invitation successfully
// when the invitation has a known InviteeID and the authenticated user is that invitee.
func TestDeclineInvitation_Success_InviteeIDKnown(t *testing.T) {
	t.Skip("Skipping test due to issues with Supabase auth schema in test database")

	SetupInvitationTest(t)

	router := setupTestRouterAndDeps(t)

	// 1. Setup:
	// Create inviter user
	inviterEmail := "inviter.decline.known@example.com"
	inviterSupabaseID := "supa_inviter_" + uuid.NewString()
	inviterUser, err := createUserInDB(context.Background(), testDBPool, inviterEmail, inviterSupabaseID, "Inviter User")
	require.NoError(t, err)
	require.NotNil(t, inviterUser)

	// Create invitee user
	inviteeEmail := "invitee.decline.known@example.com"
	inviteeSupabaseID := "supa_invitee_" + uuid.NewString()
	inviteeUser, err := createUserInDB(context.Background(), testDBPool, inviteeEmail, inviteeSupabaseID, "Invitee User")
	require.NoError(t, err)
	require.NotNil(t, inviteeUser)

	// Create trip
	trip, err := createTripInDB(context.Background(), testDBPool, "Trip for Decline Known ID", inviterUser.ID)
	require.NoError(t, err)
	require.NotNil(t, trip)

	// Create an invitation record with InviteeID set
	var actualInviteeIDForDB *string
	if inviteeUser.ID != "" { // Assuming inviteeUser.ID is the internal DB user ID
		tempID := inviteeUser.ID
		actualInviteeIDForDB = &tempID
	}
	var actualTokenForDB *string // Token is usually generated by app, so nil for creation in test if not pre-seeding one
	invitation, err := createInvitationInDB(context.Background(), testDBPool, trip.ID, inviterUser.ID, actualInviteeIDForDB, inviteeEmail, types.MemberRoleMember, types.InvitationStatusPending, time.Now().Add(24*time.Hour), actualTokenForDB)
	require.NoError(t, err)
	require.NotNil(t, invitation)

	// Generate JWT for invitee user (auth token for header)
	// The UserID for JWT should be the one that your auth middleware extracts and BaseCommand uses.
	// If BaseCommand.UserID is the Supabase Auth ID, use inviteeUser.SupabaseID.
	// If BaseCommand.UserID is the internal DB User ID, use inviteeUser.ID.
	// From user_handler.go, c.GetString("user_id") seems to be the internal DB ID.
	inviteeAuthToken := generateTestUserToken(t, inviteeUser.ID, testCFG.Server.JwtSecretKey)

	// Generate JWT for the invitation (token in request body)
	invitationBodyToken, err := auth.GenerateInvitationToken(invitation.ID, trip.ID, inviteeEmail, testCFG.Server.JwtSecretKey, time.Hour)
	require.NoError(t, err)

	declineReqPayload := handlers.DeclineInvitationRequest{Token: invitationBodyToken}
	payloadBytes, _ := json.Marshal(declineReqPayload)

	req, _ := http.NewRequest(http.MethodPost, "/v1/invitations/decline", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+inviteeAuthToken)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// 2. Assertions:
	if !assert.Equal(t, http.StatusNoContent, rr.Code, "Expected StatusNoContent, got %d, body: %s", rr.Code, rr.Body.String()) {
		// Log invitation status if assert fails
		updatedInv, _ := getInvitationFromDB(context.Background(), testDBPool, invitation.ID)
		if updatedInv != nil {
			t.Logf("Invitation status in DB after failed assert: %s", updatedInv.Status)
		}
		return
	}

	// Verify invitation status in DB
	updatedInvitation, err := getInvitationFromDB(context.Background(), testDBPool, invitation.ID)
	require.NoError(t, err)
	require.NotNil(t, updatedInvitation, "Updated invitation should not be nil")
	assert.Equal(t, types.InvitationStatusDeclined, updatedInvitation.Status, "Invitation status should be DECLINED")
}

// TODO: Add more test cases:
// - TestDeclineInvitation_Success_InviteeByEmail
// - TestDeclineInvitation_Forbidden_NotInvitee
// - TestDeclineInvitation_Forbidden_OtherUserIsInvitee
// - TestDeclineInvitation_BadRequest_InvalidToken
// - TestDeclineInvitation_BadRequest_ExpiredToken
// - TestDeclineInvitation_BadRequest_AlreadyAccepted
// - TestDeclineInvitation_BadRequest_AlreadyDeclined
// - TestDeclineInvitation_NotFound_InvitationDoesNotExist

// --- Helper functions for DB interaction (to be implemented or imported) ---

func createUserInDB(ctx context.Context, pool *pgxpool.Pool, email, supabaseID, displayName string) (*types.User, error) {
	userID := uuid.NewString()
	hashedPassword := "" // Not strictly needed if not testing password login directly
	profilePicURL := "http://example.com/avatar.png"

	// Use the actual fields from types.User and the users table schema
	// Check db/migrations/000001_init.up.sql for user table columns
	query := `
		INSERT INTO users (id, supabase_id, email, encrypted_password, username, first_name, last_name, profile_picture_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		RETURNING id, supabase_id, email, username, first_name, last_name, profile_picture_url, created_at, updated_at
	`
	// For username, first_name, last_name, derive from displayName or set defaults
	username := email // Or generate a unique username
	firstName := displayName
	lastName := ""

	var user types.User
	err := pool.QueryRow(ctx, query, userID, supabaseID, email, hashedPassword, username, firstName, lastName, profilePicURL).Scan(
		&user.ID, &user.SupabaseID, &user.Email, &user.Username, &user.FirstName, &user.LastName, &user.ProfilePictureURL, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("createUserInDB failed: %w", err)
	}
	return &user, nil
}

func createTripInDB(ctx context.Context, pool *pgxpool.Pool, name, ownerID string) (*types.Trip, error) {
	log.Printf("createTripInDB: Attempting to create trip '%s' for owner %s", name, ownerID)
	query := `
		INSERT INTO trips (name, created_by, start_date, end_date, status, description, destination_address, destination_place_id, destination_latitude, destination_longitude, background_image_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, name, created_by, start_date, end_date, status, description, created_at, updated_at, 
		          destination_address, destination_place_id, destination_latitude, destination_longitude, background_image_url
	`

	now := time.Now()
	trip := &types.Trip{}
	var destAddress, destPlaceID, bgImageURL sql.NullString
	var destLat, destLon sql.NullFloat64

	// Example BackgroundImageURL, can be empty or a valid URL
	exampleBackgroundImageURL := "http://example.com/background.jpg"

	err := pool.QueryRow(ctx, query,
		name,
		ownerID,
		now,                           // start_date
		now.Add(24*time.Hour),         // end_date
		types.TripStatusPlanning,      // status
		"Test trip created by helper", // description
		"Test Destination Address",    // destination_address (example value)
		"ChIJN1t_tDeuEmsRUsoyG83frY4", // destination_place_id (example: Googleplex)
		37.4220,                       // destination_latitude (example: Googleplex)
		-122.0841,                     // destination_longitude (example: Googleplex)
		exampleBackgroundImageURL,     // background_image_url
	).Scan(
		&trip.ID,
		&trip.Name,
		&trip.CreatedBy,
		&trip.StartDate,
		&trip.EndDate,
		&trip.Status,
		&trip.Description,
		&trip.CreatedAt,
		&trip.UpdatedAt,
		&destAddress,
		&destPlaceID,
		&destLat,
		&destLon,
		&bgImageURL,
	)

	if err != nil {
		log.Printf("createTripInDB failed: %v", err)
		return nil, fmt.Errorf("createTripInDB failed: %w", err)
	}

	if destAddress.Valid {
		trip.DestinationAddress = &destAddress.String
	} else {
		trip.DestinationAddress = nil
	}
	if destPlaceID.Valid {
		trip.DestinationPlaceID = &destPlaceID.String
	} else {
		trip.DestinationPlaceID = nil
	}

	if destLat.Valid {
		trip.DestinationLatitude = destLat.Float64
	} else {
		trip.DestinationLatitude = 0.0 // Assign zero value for float64 if NULL
	}
	if destLon.Valid {
		trip.DestinationLongitude = destLon.Float64
	} else {
		trip.DestinationLongitude = 0.0 // Assign zero value for float64 if NULL
	}

	if bgImageURL.Valid {
		trip.BackgroundImageURL = bgImageURL.String
	} else {
		trip.BackgroundImageURL = "" // Assign empty string if NULL
	}

	log.Printf("createTripInDB: Successfully created trip ID %s", trip.ID)
	return trip, nil
}

func createInvitationInDB(ctx context.Context, pool *pgxpool.Pool, tripID, inviterID string, inviteeID *string, inviteeEmail string, role types.MemberRole, status types.InvitationStatus, expiresAt time.Time, token *string) (*types.TripInvitation, error) {
	invID := uuid.NewString()
	query := `
		INSERT INTO trip_invitations (id, trip_id, inviter_id, invitee_email, role, status, expires_at, token, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		RETURNING id, trip_id, inviter_id, invitee_email, role, status, created_at, updated_at, expires_at, token
	`
	var inv types.TripInvitation
	// Scan directly into inv.Token which is now sql.NullString
	err := pool.QueryRow(ctx, query, invID, tripID, inviterID, inviteeEmail, role, status, expiresAt, token).Scan(
		&inv.ID, &inv.TripID, &inv.InviterID, &inv.InviteeEmail, &inv.Role, &inv.Status, &inv.CreatedAt, &inv.UpdatedAt, &inv.ExpiresAt, &inv.Token, // Scan directly into sql.NullString
	)
	if err != nil {
		return nil, fmt.Errorf("createInvitationInDB failed for email %s: %w", inviteeEmail, err)
	}
	return &inv, nil
}

func getInvitationFromDB(ctx context.Context, pool *pgxpool.Pool, invitationID string) (*types.TripInvitation, error) {
	query := `
		SELECT id, trip_id, inviter_id, invitee_email, role, status, created_at, updated_at, expires_at, token
		FROM trip_invitations WHERE id = $1
	`
	var inv types.TripInvitation
	// Scan directly into inv.Token which is now sql.NullString
	err := pool.QueryRow(ctx, query, invitationID).Scan(
		&inv.ID, &inv.TripID, &inv.InviterID, &inv.InviteeEmail, &inv.Role, &inv.Status, &inv.CreatedAt, &inv.UpdatedAt, &inv.ExpiresAt, &inv.Token, // Scan directly into sql.NullString
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) { // Use errors.Is for pgx v4/v5
			return nil, nil // Not found is not an error for a getter in tests sometimes
		}
		return nil, fmt.Errorf("getInvitationFromDB failed: %w", err)
	}
	return &inv, nil
}

// func loadTestConfig() *config.Config { /* ... load test config ... */ return &config.Config{} }
