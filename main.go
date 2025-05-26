// Package main is the entry point for the NomadCrew backend application.
// It initializes configurations, database connections, services, handlers,
// sets up the HTTP router, starts the server, and handles graceful shutdown.
//
// @title           NomadCrew Backend API
// @version         1.0.0
// @description     NomadCrew RESTful API with authentication and WebSocket support
//
// @contact.name    NomadCrew Team
// @contact.url     https://nomadcrew.uk
// @contact.email   support@nomadcrew.uk
//
// @license.name    MIT
// @license.url     https://opensource.org/licenses/MIT
//
// @host            api.nomadcrew.uk
// @BasePath        /v1
//
// @securityDefinitions.apikey    BearerAuth
// @in                            header
// @name                          Authorization
// @description                   JWT token for authentication
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/db"
	"github.com/NomadCrew/nomad-crew-backend/handlers"
	"github.com/NomadCrew/nomad-crew-backend/internal/events"
	internalService "github.com/NomadCrew/nomad-crew-backend/internal/service"
	internalPgStore "github.com/NomadCrew/nomad-crew-backend/internal/store/postgres"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/models"
	locationSvc "github.com/NomadCrew/nomad-crew-backend/models/location/service"
	"github.com/NomadCrew/nomad-crew-backend/models/trip"
	trip_service "github.com/NomadCrew/nomad-crew-backend/models/trip/service"
	userSvc "github.com/NomadCrew/nomad-crew-backend/models/user/service"
	weatherSvc "github.com/NomadCrew/nomad-crew-backend/models/weather/service"
	"github.com/NomadCrew/nomad-crew-backend/pkg/pexels"
	"github.com/NomadCrew/nomad-crew-backend/router"
	"github.com/NomadCrew/nomad-crew-backend/service"
	services "github.com/NomadCrew/nomad-crew-backend/services"
	dbStore "github.com/NomadCrew/nomad-crew-backend/store/postgres"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/supabase-community/supabase-go"
)

// main initializes and runs the NomadCrew backend application.
// It sets up logging, configuration, database connections (PostgreSQL, Redis),
// Supabase client, JWT validation, various application services and handlers,
// configures the Gin router, starts the HTTP server, and manages graceful shutdown
// upon receiving SIGINT or SIGTERM signals.
func main() {
	// Initialize logger
	logger.InitLogger()
	log := logger.GetLogger()
	defer logger.Close()

	// Handle shutdown signals - Enhanced with graceful WebSocket shutdown
	shutdownCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Info("Received shutdown signal. Initiating graceful shutdown...")
		cancel() // Trigger context cancellation
	}()

	configEnv := os.Getenv("SERVER_ENVIRONMENT")
	if configEnv == "" {
		configEnv = "development"
	}
	configFilePath := fmt.Sprintf("config/config.%s.yaml", configEnv)

	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		log.Infow("Config file not found, generating from environment variables",
			"path", configFilePath,
			"environment", configEnv)
	}

	// Load configuration and DB connection
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database connection directly
	poolConfig, err := config.ConfigureNeonPostgresPool(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to configure database: %v", err)
	}

	// Create context with timeout for initial connection
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
	pool, err := pgxpool.ConnectConfig(dbCtx, poolConfig)
	dbCancel()

	if err != nil {
		log.Errorw("Failed to establish initial database connection, will retry during operation",
			"error", err)
		// Don't fatal here, the app will retry connections
	} else {
		log.Info("Successfully established initial database connection")
		defer pool.Close()
	}

	// Initialize database dependencies with enhanced resilient client
	dbClient := db.NewDatabaseClientWithConfig(pool, poolConfig)

	// Configure more aggressive reconnection for serverless environment
	if config.IsRunningInServerless() {
		dbClient.SetMaxRetries(5)           // Increase max retries
		dbClient.SetRetryDelay(time.Second) // Start with shorter delay
		log.Info("Configured database client with serverless-optimized reconnection settings")
	}

	// Other store initializations using the database pool
	tripStore := dbStore.NewPgTripStore(dbClient.GetPool())
	todoStore := db.NewTodoDB(dbClient)
	locationDB := db.NewLocationDB(dbClient)
	notificationDB := dbStore.NewPgNotificationStore(dbClient.GetPool())

	// Get Supabase service key from config
	supabaseServiceKey := cfg.ExternalServices.SupabaseServiceKey
	userDB := internalPgStore.NewUserStore(dbClient.GetPool(), cfg.ExternalServices.SupabaseURL, cfg.ExternalServices.SupabaseJWTSecret)

	// Initialize Redis client with TLS in production
	redisOptions := config.ConfigureUpstashRedisOptions(&cfg.Redis)
	redisClient := redis.NewClient(redisOptions)
	defer redisClient.Close()

	// Test Redis connection
	if err := config.TestRedisConnection(redisClient); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Initialize Supabase client
	supabaseClient, err := supabase.NewClient(
		cfg.ExternalServices.SupabaseURL,
		cfg.ExternalServices.SupabaseAnonKey,
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to initialize Supabase client: %v", err)
	}

	log.Infow("Initialized Supabase client",
		"url", cfg.ExternalServices.SupabaseURL,
		"key", logger.MaskSensitiveString(cfg.ExternalServices.SupabaseAnonKey, 3, 0))

	// Initialize Supabase service for Realtime (always enabled)
	supabaseService := services.NewSupabaseService(services.SupabaseServiceConfig{
		IsEnabled:   true, // Always enabled - no feature flag dependency
		SupabaseURL: cfg.ExternalServices.SupabaseURL,
		SupabaseKey: supabaseServiceKey,
	})
	log.Info("Supabase service initialized successfully (sync always enabled)")

	// Initialize JWT Validator
	jwtValidator, err := middleware.NewJWTValidator(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize JWT Validator: %v", err)
	}
	log.Info("JWT Validator initialized successfully")

	// Initialize services
	// rateLimitService := services.NewRateLimitService(redisClient) - removed as it's no longer used after WebSocket removal

	// Create event service config based on application configuration
	eventServiceConfig := events.Config{
		PublishTimeout:   time.Duration(cfg.EventService.PublishTimeoutSeconds) * time.Second,
		SubscribeTimeout: time.Duration(cfg.EventService.SubscribeTimeoutSeconds) * time.Second,
		EventBufferSize:  cfg.EventService.EventBufferSize,
	}

	// Initialize the event service from the internal/events package
	eventService := events.NewService(redisClient, eventServiceConfig)

	weatherService := weatherSvc.NewWeatherService(eventService)
	emailService := services.NewEmailService(&cfg.Email)
	healthService := services.NewHealthService(dbClient.GetPool(), redisClient, cfg.Server.Version)

	// Initialize notification service with correct dependencies
	notificationService := service.NewNotificationService(notificationDB, userDB, tripStore, eventService, log.Desugar())

	// Initialize refactored location service
	locationManagementService := locationSvc.NewManagementService(
		locationDB,
		eventService,
	)

	// Initialize Chat Store and Service
	chatStore := internalPgStore.NewChatStore(dbClient.GetPool())

	// Use the internal service package's ChatService implementation
	chatService := internalService.NewChatService(chatStore, tripStore, eventService)

	tripMemberService := trip_service.NewTripMemberService(tripStore, eventService, supabaseService)
	chatHandler := handlers.NewChatHandler(
		chatService,
		tripMemberService,
		eventService,
		log.Desugar(),
		supabaseService,
	)

	// Initialize trip model with new store
	tripModel := trip.NewTripModel(
		tripStore,
		chatStore,
		userDB,
		eventService,
		weatherService,
		supabaseClient,
		&cfg.Server,
		emailService,
		supabaseService,
	)
	todoModel := models.NewTodoModel(todoStore, tripModel, eventService)

	// Initialize User Service and Handler
	userService := userSvc.NewUserService(userDB, cfg.ExternalServices.SupabaseJWTSecret, supabaseService)
	userHandler := handlers.NewUserHandler(userService)

	// Initialize Pexels client
	pexelsClient := pexels.NewClient(cfg.ExternalServices.PexelsAPIKey)
	log.Infow("Initialized Pexels client", "hasAPIKey", cfg.ExternalServices.PexelsAPIKey != "")

	// Initialize Handlers
	tripHandler := handlers.NewTripHandler(tripModel, eventService, supabaseClient, &cfg.Server, weatherService, userService, pexelsClient)
	todoHandler := handlers.NewTodoHandler(todoModel, eventService, log.Desugar())
	healthHandler := handlers.NewHealthHandler(healthService)
	locationHandler := handlers.NewLocationHandler(
		locationManagementService,
		tripMemberService,
		supabaseService,
		log.Desugar(),
	)
	notificationHandler := handlers.NewNotificationHandler(notificationService, log.Desugar())
	memberHandler := handlers.NewMemberHandler(tripModel, userDB, eventService)
	invitationHandler := handlers.NewInvitationHandler(tripModel, userDB, eventService, &cfg.Server)

	// Initialize Supabase Realtime handlers (always enabled)
	chatHandlerSupabase := handlers.NewChatHandlerSupabase(
		tripMemberService,
		supabaseService,
	)
	locationHandlerSupabase := handlers.NewLocationHandlerSupabase(
		tripMemberService,
		supabaseService,
	)

	// Prepare Router Dependencies
	routerDeps := router.Dependencies{
		Config:              cfg,
		JWTValidator:        jwtValidator,
		UserService:         userService, // Add UserService for enhanced auth middleware
		TripHandler:         tripHandler,
		TodoHandler:         todoHandler,
		HealthHandler:       healthHandler,
		LocationHandler:     locationHandler,
		NotificationHandler: notificationHandler,
		ChatHandler:         chatHandler,
		UserHandler:         userHandler,
		MemberHandler:       memberHandler,
		InvitationHandler:   invitationHandler,
		Logger:              log,
		SupabaseService:     supabaseService,
	}

	// Add Supabase Realtime handlers (always enabled in development)
	routerDeps.ChatHandlerSupabase = chatHandlerSupabase
	routerDeps.LocationHandlerSupabase = locationHandlerSupabase

	// Setup Router using the new package
	r := router.SetupRouter(routerDeps)

	log.Info("Router setup complete")

	// Start the server
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.Server.Port),
		Handler:           r,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Start server in a goroutine so it doesn't block shutdown handling
	go func() {
		log.Infow("Starting server", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for shutdown signal and then gracefully shut down
	<-shutdownCtx.Done()
	log.Info("Shutting down server...")

	// Create a deadline to wait for current operations to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalw("Server forced to shutdown", "error", err)
	}

	log.Info("Server has been gracefully shut down")
}
