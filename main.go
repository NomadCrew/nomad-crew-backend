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
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/NomadCrew/nomad-crew-backend/models/trip"
	"github.com/NomadCrew/nomad-crew-backend/services"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"github.com/supabase-community/supabase-go"
)

func main() {
	// Initialize logger
	logger.InitLogger()
	log := logger.GetLogger()
	defer logger.Close()

	// Handle shutdown signals - Enhanced with graceful WebSocket shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Info("Shutting down gracefully...")
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

	pool, err := pgxpool.ConnectConfig(context.Background(), poolConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Initialize database dependencies with concrete implementation
	dbClient := db.NewDatabaseClient(pool)
	tripDB := db.NewTripDB(dbClient)
	todoDB := db.NewTodoDB(dbClient)
	locationDB := db.NewLocationDB(dbClient)

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
		cfg.ExternalServices.SupabaseAnonKey,
		cfg.ExternalServices.SupabaseURL,
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to initialize Supabase client: %v", err)
	}

	log.Infow("Initialized Supabase client",
		"url", cfg.ExternalServices.SupabaseURL,
		"key", logger.MaskSensitiveString(cfg.ExternalServices.SupabaseAnonKey, 3, 0))

	// Initialize JWT Validator
	jwtValidator, err := middleware.NewJWTValidator(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize JWT Validator: %v", err)
	}
	log.Info("JWT Validator initialized successfully")

	// Initialize services
	rateLimitService := services.NewRateLimitService(redisClient)

	// ---> Configuration loading for Event Service (Using Viper Values) <---
	// Load config from Viper-populated struct cfg.EventService
	eventServiceConfig := services.RedisEventServiceConfig{
		PublishTimeout:   time.Duration(cfg.EventService.PublishTimeoutSeconds) * time.Second,
		SubscribeTimeout: time.Duration(cfg.EventService.SubscribeTimeoutSeconds) * time.Second,
		EventBufferSize:  cfg.EventService.EventBufferSize,
	}

	// Log the actual config being used (read from cfg)
	log.Infow("Initializing RedisEventService with config",
		"publishTimeout", eventServiceConfig.PublishTimeout,
		"subscribeTimeout", eventServiceConfig.SubscribeTimeout,
		"eventBufferSize", eventServiceConfig.EventBufferSize,
	)
	// Pass the loaded config to the constructor
	eventService := services.NewRedisEventService(redisClient, eventServiceConfig)
	// ---> End Event Service configuration <---

	weatherService := services.NewWeatherService(eventService)
	emailService := services.NewEmailService(&cfg.Email)
	// Pass eventService to HealthService if it needs it
	healthService := services.NewHealthService(pool, redisClient, cfg.Server.Version) // healthService := services.NewHealthService(pool, redisClient, cfg.Server.Version, eventService)
	locationService := services.NewLocationService(locationDB, eventService)

	// Initialize chat store
	chatStore := db.NewPostgresChatStore(pool, supabaseClient, os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_SERVICE_KEY"))

	// Initialize offline location service (after locationService)
	offlineLocationService := services.NewOfflineLocationService(redisClient, locationService)
	locationService.SetOfflineService(offlineLocationService)

	// Connect WebSocket metrics to health service
	healthService.SetActiveConnectionsGetter(middleware.GetActiveConnectionCount)

	// Handlers
	tripModel := trip.NewTripModel(
		tripDB,
		eventService,
		weatherService,
		supabaseClient,
		&cfg.Server,
		emailService,
		chatStore,
	)
	todoModel := models.NewTodoModel(todoDB, tripModel)
	tripHandler := handlers.NewTripHandler(tripModel, eventService, supabaseClient)
	todoHandler := handlers.NewTodoHandler(todoModel, eventService)
	healthHandler := handlers.NewHealthHandler(healthService)
	locationHandler := handlers.NewLocationHandler(locationService)

	// Add new WebSocket handler
	wsHandler := handlers.NewWSHandler(rateLimitService, eventService)

	// Router setup
	r := gin.Default()
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.CORSMiddleware(&cfg.Server))

	// WebSocket configuration
	wsConfig := middleware.WSConfig{
		PongWait:        60 * time.Second,
		PingPeriod:      30 * time.Second,
		WriteWait:       10 * time.Second,
		MaxMessageSize:  1024,
		ReauthInterval:  5 * time.Minute,
		BufferHighWater: 256,
		BufferLowWater:  64,
	}

	// Initialize WebSocket metrics
	wsMetrics := &middleware.WSMetrics{
		ConnectionsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "websocket_active_connections",
			Help: "Current active WebSocket connections",
		}),
		MessagesReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "websocket_messages_received_total",
			Help: "Total received WebSocket messages",
		}),
		MessagesSent: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "websocket_messages_sent_total",
			Help: "Total sent WebSocket messages",
		}),
		ErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "websocket_errors_total",
			Help: "Total WebSocket errors by type",
		}, []string{"type"}),
	}

	// Register metrics with Prometheus
	prometheus.MustRegister(
		wsMetrics.ConnectionsActive,
		wsMetrics.MessagesReceived,
		wsMetrics.MessagesSent,
		wsMetrics.ErrorsTotal,
	)

	// Register health routes
	r.GET("/health", healthHandler.DetailedHealth)
	r.GET("/health/liveness", healthHandler.LivenessCheck)
	r.GET("/health/readiness", healthHandler.ReadinessCheck)

	// Debug routes - only available in development and staging
	if cfg.Server.Environment != config.EnvProduction {
		debugRoutes := r.Group("/debug")

		// JWT debug endpoint for troubleshooting token issues
		debugRoutes.GET("/jwt", handlers.DebugJWTHandler())

		// Direct JWT validation with hardcoded secret for testing
		debugRoutes.GET("/jwt/direct", handlers.DebugJWTDirectHandler())

		log.Info("Debug routes enabled in non-production environment")
	}

	// Versioned routes (e.g., /v1)
	v1 := r.Group("/v1")

	// Location routes
	locationRoutes := v1.Group("/location")
	{
		locationRoutes.Use(middleware.AuthMiddleware(jwtValidator))
		locationRoutes.POST("/update", locationHandler.UpdateLocationHandler)
		locationRoutes.POST("/offline", locationHandler.SaveOfflineLocationsHandler)
		locationRoutes.POST("/process-offline", locationHandler.ProcessOfflineLocationsHandler)
	}

	// WebSocket routes with new optimized auth
	wsRoutes := v1.Group("/ws")
	{
		// Use optimized WebSocket JWT auth
		wsRoutes.Use(middleware.WSJwtAuth(jwtValidator))
		// Apply WebSocket handler with rate limiting
		wsRoutes.GET("/connect", wsHandler.HandleWebSocketConnection)
	}

	trips := v1.Group("/trips")
	{
		trips.Use(middleware.AuthMiddleware(jwtValidator))

		// List and search endpoints
		trips.GET("/list", tripHandler.ListUserTripsHandler)  // GET all trips related to the user
		trips.POST("/search", tripHandler.SearchTripsHandler) // POST /v1/trips/search with request body

		// Core trip management
		trips.POST("", tripHandler.CreateTripHandler)       // Create trip
		trips.GET("/:id", tripHandler.GetTripHandler)       // Get trip by ID
		trips.PUT("/:id", tripHandler.UpdateTripHandler)    // Update trip
		trips.DELETE("/:id", tripHandler.DeleteTripHandler) // Delete trip
		trips.PATCH("/:id/status", tripHandler.UpdateTripStatusHandler)

		// Event streaming endpoints:
		// New WebSocket endpoint:
		trips.GET("/:id/ws",
			middleware.WSRateLimiter(
				rateLimitService.GetRedisClient(),
				5,              // Max connections per user
				30*time.Second, // Window duration
			),
			middleware.RequireRole(tripModel, types.MemberRoleMember),
			middleware.WSMiddleware(wsConfig, wsMetrics),
			tripHandler.WSStreamEvents,
		)

		// Member management routes
		memberRoutes := trips.Group("/:id/members")
		{
			// Add members (owner only)
			memberRoutes.POST("",
				middleware.RequireRole(tripModel, types.MemberRoleOwner),
				tripHandler.AddMemberHandler,
			)

			// Update member role (owner only)
			memberRoutes.PUT("/:userId/role",
				middleware.RequireRole(tripModel, types.MemberRoleOwner),
				tripHandler.UpdateMemberRoleHandler,
			)

			// Remove member (owner or self)
			memberRoutes.DELETE("/:userId", tripHandler.RemoveMemberHandler)

			// Get trip members (any member)
			memberRoutes.GET("",
				middleware.RequireRole(tripModel, types.MemberRoleMember),
				tripHandler.GetTripMembersHandler,
			)
		}

		trips.PATCH("/:id/trigger-weather-update", tripHandler.TriggerWeatherUpdateHandler)
		trips.POST("/:id/invitations", tripHandler.InviteMemberHandler)

		// Todo routes setup
		todoRoutes := trips.Group("/:id/todos")
		{
			todoRoutes.GET("",
				middleware.RequireRole(tripModel, types.MemberRoleMember),
				todoHandler.ListTodosHandler,
			)
			todoRoutes.POST("",
				middleware.RequireRole(tripModel, types.MemberRoleMember),
				todoHandler.CreateTodoHandler,
			)
			// Support both PUT and POST for updates to accommodate frontend
			todoRoutes.POST("/:todoID",
				middleware.RequireRole(tripModel, types.MemberRoleMember),
				todoHandler.UpdateTodoHandler,
			)
			todoRoutes.PUT("/:todoID",
				middleware.RequireRole(tripModel, types.MemberRoleMember),
				todoHandler.UpdateTodoHandler,
			)
			todoRoutes.DELETE("/:todoID",
				middleware.RequireRole(tripModel, types.MemberRoleMember),
				todoHandler.DeleteTodoHandler,
			)
		}

		// Location routes for trips
		trips.GET("/:id/locations",
			middleware.RequireRole(tripModel, types.MemberRoleMember),
			locationHandler.GetTripMemberLocationsHandler,
		)

		// Chat routes for trips
		trips.GET("/:id/messages",
			middleware.RequireRole(tripModel, types.MemberRoleMember),
			tripHandler.ListTripMessages,
		)
		trips.POST("/:id/messages/read",
			middleware.RequireRole(tripModel, types.MemberRoleMember),
			tripHandler.UpdateLastReadMessage,
		)
	}

	// Create auth handler
	authHandler := handlers.NewAuthHandler(supabaseClient, cfg)

	// Auth routes that don't require authentication
	authRoutes := v1.Group("/auth")
	// Don't add AuthMiddleware here - these routes are for unauthenticated users
	{
		authRoutes.POST("/refresh", authHandler.RefreshTokenHandler)
	}

	// Invitation acceptance routes - don't require authentication initially
	// as the JWT token itself contains the necessary validation information
	inviteRoutes := v1.Group("/trips/invitations")
	{
		// POST endpoint for API calls from the app
		inviteRoutes.POST("/accept", tripHandler.AcceptInvitationHandler)

		// GET endpoint for handling direct URL access and deep links
		// This will redirect to the app with the token as a parameter
		inviteRoutes.GET("/accept/:token", tripHandler.HandleInvitationDeepLink)
	}

	// Add a root-level route for accessing deep links via a cleaner format
	r.GET("/invite/:token", tripHandler.HandleInvitationDeepLink)

	// Initialize chat service
	chatService := services.NewChatService(chatStore, tripModel.GetTripStore(), eventService)
	chatHandler := handlers.NewChatHandler(chatService, chatStore)

	// Chat routes
	chatRoutes := v1.Group("/chats")
	{
		chatRoutes.Use(middleware.AuthMiddleware(jwtValidator))

		// Create a chat group
		chatRoutes.POST("/groups", chatHandler.CreateChatGroup)

		// Get chat groups for a user
		chatRoutes.GET("/groups", chatHandler.ListChatGroups)

		// Get a chat group
		chatRoutes.GET("/groups/:groupID", chatHandler.GetChatGroup)

		// Update a chat group
		chatRoutes.PUT("/groups/:groupID", chatHandler.UpdateChatGroup)

		// Delete a chat group
		chatRoutes.DELETE("/groups/:groupID", chatHandler.DeleteChatGroup)

		// Get messages for a chat group
		chatRoutes.GET("/groups/:groupID/messages", chatHandler.ListChatMessages)

		// Get members of a chat group
		chatRoutes.GET("/groups/:groupID/members", chatHandler.ListChatGroupMembers)

		// Update last read message
		chatRoutes.PUT("/groups/:groupID/read", chatHandler.UpdateLastReadMessage)

		// Note: WebSocket connection for chat is now handled through the trip websocket
		// The separate chat websocket endpoint has been removed
	}

	// Configure server with graceful shutdown
	server := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second, // Protection against Slowloris attacks
	}

	// Create a context for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Start server in a goroutine
	go func() {
		log.Infow("Starting server",
			"port", cfg.Server.Port,
			"environment", cfg.Server.Environment,
			"version", cfg.Server.Version)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-sigChan
	log.Info("Shutdown signal received, gracefully terminating connections...")

	// First close all WebSocket connections
	log.Info("Closing active WebSocket connections...")
	// Give WebSockets time to close gracefully before server shutdown
	time.Sleep(2 * time.Second)

	// Then shut down HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Errorf("Server shutdown error: %v", err)
	}

	// Close Redis event service connections
	if err := eventService.Shutdown(shutdownCtx); err != nil {
		log.Errorf("Event service shutdown error: %v", err)
	}

	log.Info("Server gracefully stopped")
}
