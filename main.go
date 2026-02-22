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
	"github.com/NomadCrew/nomad-crew-backend/internal/store/sqlcadapter"
	"github.com/NomadCrew/nomad-crew-backend/internal/websocket"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/models"
	notificationSvc "github.com/NomadCrew/nomad-crew-backend/models/notification/service"
	"github.com/NomadCrew/nomad-crew-backend/models/trip"
	trip_service "github.com/NomadCrew/nomad-crew-backend/models/trip/service"
	userSvc "github.com/NomadCrew/nomad-crew-backend/models/user/service"
	expenseSvc "github.com/NomadCrew/nomad-crew-backend/models/expense/service"
	walletSvc "github.com/NomadCrew/nomad-crew-backend/models/wallet/service"
	weatherSvc "github.com/NomadCrew/nomad-crew-backend/models/weather/service"
	"github.com/NomadCrew/nomad-crew-backend/pkg/pexels"
	"github.com/NomadCrew/nomad-crew-backend/router"
	services "github.com/NomadCrew/nomad-crew-backend/services"

	"github.com/jackc/pgx/v5/pgxpool"
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
	pool, err := pgxpool.NewWithConfig(dbCtx, poolConfig)
	dbCancel()

	if err != nil {
		log.Errorw("Failed to establish initial database connection, will retry during operation",
			"error", err)
		// Don't fatal here, the app will retry connections
	} else {
		log.Info("Successfully established initial database connection")
		defer pool.Close()

		// Run embedded migrations (safe on every startup — already-applied are skipped).
		if err := db.RunMigrations(cfg.Database.URL()); err != nil {
			log.Warnw("Migration error (continuing — tables may already exist)", "error", err)
		}
	}

	// Initialize database dependencies with enhanced resilient client
	dbClient := db.NewDatabaseClientWithConfig(pool, poolConfig)

	// Configure more aggressive reconnection for serverless environment
	if config.IsRunningInServerless() {
		dbClient.SetMaxRetries(5)           // Increase max retries
		dbClient.SetRetryDelay(time.Second) // Start with shorter delay
		log.Info("Configured database client with serverless-optimized reconnection settings")
	}

	// Store initializations using SQLC-based implementations
	// All stores now use the type-safe SQLC generated code
	tripStore := sqlcadapter.NewSqlcTripStore(dbClient.GetPool())
	log.Info("Using SQLC-based trip store")

	todoStore := sqlcadapter.NewSqlcTodoStore(dbClient.GetPool())
	log.Info("Using SQLC-based todo store")

	notificationDB := sqlcadapter.NewSqlcNotificationStore(dbClient.GetPool())
	log.Info("Using SQLC-based notification store")

	// Get Supabase service key from config
	supabaseServiceKey := cfg.ExternalServices.SupabaseServiceKey
	userDB := sqlcadapter.NewSqlcUserStore(dbClient.GetPool(), cfg.ExternalServices.SupabaseURL, cfg.ExternalServices.SupabaseJWTSecret)
	log.Info("Using SQLC-based user store")

	pushTokenStore := sqlcadapter.NewSqlcPushTokenStore(dbClient.GetPool())
	log.Info("Using SQLC-based push token store")

	feedbackStore := sqlcadapter.NewSqlcFeedbackStore(dbClient.GetPool())
	log.Info("Using feedback store")

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

	weatherService := weatherSvc.NewWeatherService()
	emailService := services.NewEmailService(&cfg.Email)
	healthService := services.NewHealthService(dbClient.GetPool(), redisClient, cfg.Server.Version)

	// Initialize push notification service
	pushService := services.NewExpoPushService(pushTokenStore, log.Desugar())
	log.Info("Push notification service initialized")

	// Initialize notification worker pool for bounded async operations
	notificationWorkerPool := services.NewWorkerPool(cfg.WorkerPool)
	notificationWorkerPool.Start()
	log.Infow("Notification worker pool started",
		"maxWorkers", cfg.WorkerPool.MaxWorkers,
		"queueSize", cfg.WorkerPool.QueueSize)

	// Initialize notification facade service (for chat and other notifications)
	notificationFacadeService := services.NewNotificationFacadeService(&cfg.Notification, notificationWorkerPool)
	log.Infow("Notification facade service initialized", "enabled", notificationFacadeService.IsEnabled())

	// Initialize notification service with push notification support
	notificationService := notificationSvc.NewNotificationServiceWithPush(notificationDB, userDB, tripStore, eventService, pushService, log.Desugar())

	tripMemberService := trip_service.NewTripMemberService(tripStore, eventService, supabaseService)

	// Initialize trip model with new store (removed chatStore dependency)
	tripModel := trip.NewTripModel(
		tripStore,
		nil, // chatStore removed - using Supabase for chat
		userDB,
		eventService,
		weatherService,
		supabaseClient,
		&cfg.Server,
		emailService,
		supabaseService,
		notificationFacadeService,
	)
	todoModel := models.NewTodoModel(todoStore, tripModel, eventService)

	// Poll store, model, and handler
	pollStore := sqlcadapter.NewSqlcPollStore(dbClient.GetPool())
	log.Info("Using SQLC-based poll store")
	pollModel := models.NewPollModel(pollStore, tripModel, eventService)
	pollHandler := handlers.NewPollHandler(pollModel)

	// Wallet store, service, and handler
	walletStore := sqlcadapter.NewSqlcWalletStore(dbClient.GetPool())
	log.Info("Using wallet store")
	var walletFileStorage walletSvc.FileStorage
	switch cfg.Server.WalletStorageBackend {
	case "r2":
		r2Storage, err := walletSvc.NewR2FileStorage(
			cfg.R2.AccountID,
			cfg.R2.BucketName,
			cfg.R2.AccessKeyID,
			cfg.R2.SecretAccessKey,
		)
		if err != nil {
			log.Fatalf("Failed to initialize R2 file storage: %v", err)
		}
		walletFileStorage = r2Storage
		log.Info("Using R2 file storage for wallet documents")
	default:
		walletFileStorage = walletSvc.NewLocalFileStorage(cfg.Server.WalletStoragePath)
		log.Info("Using local file storage for wallet documents")
	}
	walletService := walletSvc.NewWalletService(walletStore, tripStore, walletFileStorage, cfg.EffectiveWalletSigningKey())
	walletHandler := handlers.NewWalletHandler(walletService)

	// Poll image handler (reuses wallet file storage and signing key)
	pollImageHandler := handlers.NewPollImageHandler(walletFileStorage, cfg.EffectiveWalletSigningKey())

	// Expense store, service, and handler
	expenseStore := sqlcadapter.NewSqlcExpenseStore(dbClient.GetPool())
	log.Info("Using SQLC-based expense store")
	expenseService := expenseSvc.NewExpenseService(expenseStore, tripStore, eventService)
	expenseHandler := handlers.NewExpenseHandler(expenseService)

	// Initialize User Service and Handler
	// Pass jwtValidator to enable JWKS validation for onboarding (new Supabase API keys)
	userService := userSvc.NewUserService(userDB, cfg.ExternalServices.SupabaseJWTSecret, supabaseService, jwtValidator)
	userHandler := handlers.NewUserHandler(userService)

	// Initialize Pexels client
	pexelsClient := pexels.NewClient(cfg.ExternalServices.PexelsAPIKey)
	log.Infow("Initialized Pexels client", "hasAPIKey", cfg.ExternalServices.PexelsAPIKey != "")

	// Initialize Handlers
	tripHandler := handlers.NewTripHandler(tripModel, eventService, supabaseClient, &cfg.Server, weatherService, userService, pexelsClient)
	todoHandler := handlers.NewTodoHandler(todoModel, eventService, log.Desugar())
	healthHandler := handlers.NewHealthHandler(healthService)
	notificationHandler := handlers.NewNotificationHandler(notificationService, log.Desugar())
	memberHandler := handlers.NewMemberHandler(tripModel, userDB, eventService)
	invitationHandler := handlers.NewInvitationHandlerWithNotifications(tripModel, userDB, eventService, &cfg.Server, notificationService)
	pushTokenHandler := handlers.NewPushTokenHandler(pushTokenStore, log.Desugar())
	feedbackHandler := handlers.NewFeedbackHandler(feedbackStore)

	// Initialize Supabase Realtime handlers (always enabled)
	chatHandlerSupabase := handlers.NewChatHandlerSupabase(
		tripMemberService,
		supabaseService,
		notificationFacadeService,
	)
	locationHandlerSupabase := handlers.NewLocationHandlerSupabase(
		tripMemberService,
		supabaseService,
		userDB,
	)

	// Initialize WebSocket hub and handler for real-time events
	wsHub := websocket.NewHub(eventService, tripStore)
	wsHandler := websocket.NewHandler(wsHub, &cfg.Server, tripMemberService)
	log.Info("WebSocket hub and handler initialized")

	// Wire WebSocket hub as the broadcaster for real-time notification delivery
	notificationService.SetUserBroadcaster(wsHub)

	// Prepare Router Dependencies
	routerDeps := router.Dependencies{
		Config:              cfg,
		JWTValidator:        jwtValidator,
		UserService:         userService, // Add UserService for enhanced auth middleware
		TripHandler:         tripHandler,
		TodoHandler:         todoHandler,
		HealthHandler:       healthHandler,
		NotificationHandler: notificationHandler,
		UserHandler:         userHandler,
		MemberHandler:       memberHandler,
		InvitationHandler:   invitationHandler,
		Logger:              log,
		SupabaseService:     supabaseService,
		RedisClient:         redisClient, // Add Redis client for rate limiting
		TripModel:           tripModel,   // Add TripModel for RBAC middleware
	}

	// Add Supabase Realtime handlers (always enabled in development)
	routerDeps.ChatHandlerSupabase = chatHandlerSupabase
	routerDeps.LocationHandlerSupabase = locationHandlerSupabase
	routerDeps.WebSocketHandler = wsHandler
	routerDeps.PushTokenHandler = pushTokenHandler
	routerDeps.PollHandler = pollHandler
	routerDeps.PollImageHandler = pollImageHandler
	routerDeps.FeedbackHandler = feedbackHandler
	routerDeps.WalletHandler = walletHandler
	routerDeps.ExpenseHandler = expenseHandler

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

	// Start wallet document purge goroutine (GDPR: hard-delete soft-deleted docs after retention period)
	go func() {
		const retentionDays = 30
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		purge := func() {
			purgeCtx, purgeCancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer purgeCancel()
			count, err := walletService.PurgeExpiredDocuments(purgeCtx, retentionDays)
			if err != nil {
				log.Errorw("Wallet document purge failed", "error", err)
				return
			}
			if count > 0 {
				log.Infow("Purged expired wallet documents", "count", count, "retentionDays", retentionDays)
			}
		}

		// Run immediately on startup
		purge()

		for {
			select {
			case <-shutdownCtx.Done():
				return
			case <-ticker.C:
				purge()
			}
		}
	}()
	log.Info("Wallet document purge goroutine started (30-day retention)")

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
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.WorkerPool.ShutdownTimeoutSeconds)*time.Second)
	defer cancel()

	// Shutdown notification worker pool first to drain pending notifications
	log.Info("Shutting down notification worker pool...")
	if err := notificationWorkerPool.Shutdown(ctx); err != nil {
		log.Errorw("Error during notification worker pool shutdown", "error", err)
	}

	// Shutdown WebSocket hub to close all connections gracefully
	log.Info("Shutting down WebSocket hub...")
	if err := wsHub.Shutdown(ctx); err != nil {
		log.Errorw("Error during WebSocket hub shutdown", "error", err)
	}

	// Shutdown event service to stop processing new events
	log.Info("Shutting down event service...")
	if err := eventService.Shutdown(ctx); err != nil {
		log.Errorw("Error during event service shutdown", "error", err)
	}

	// Then shutdown HTTP server
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalw("Server forced to shutdown", "error", err)
	}

	log.Info("Server has been gracefully shut down")
}

