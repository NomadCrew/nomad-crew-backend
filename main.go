// Package main is the entry point for the NomadCrew backend application.
// It initializes configurations, database connections, services, handlers,
// sets up the HTTP router, starts the server, and handles graceful shutdown.
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

	// Import the new location service package
	locationSvc "github.com/NomadCrew/nomad-crew-backend/models/location/service"
	"github.com/NomadCrew/nomad-crew-backend/models/trip"
	"github.com/NomadCrew/nomad-crew-backend/router"                 // Import the new router package
	service "github.com/NomadCrew/nomad-crew-backend/service"        // New service package
	"github.com/NomadCrew/nomad-crew-backend/services"               // Old services package - Keep for now if other services remain
	dbStore "github.com/NomadCrew/nomad-crew-backend/store/postgres" // Alias for postgres store implementations

	// Alias for store interfaces

	// "github.com/gin-gonic/gin" // Gin is now primarily used within the router package
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
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

	pool, err := pgxpool.ConnectConfig(context.Background(), poolConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Initialize database dependencies with concrete implementation
	dbClient := db.NewDatabaseClient(pool)
	tripDB := db.NewTripDB(dbClient)
	todoDB := db.NewTodoDB(dbClient)
	locationDB := db.NewLocationDB(dbClient)               // This implements store.LocationStore
	notificationDB := dbStore.NewPgNotificationStore(pool) // Assuming this exists based on pattern
	userDB := dbStore.NewPgUserStore(pool)                 // Based on grep results

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
		cfg.ExternalServices.SupabaseURL, // Changed from AnonKey to ServiceKey if admin actions needed?
		cfg.ExternalServices.SupabaseAnonKey,
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
	eventServiceConfig := services.RedisEventServiceConfig{
		PublishTimeout:   time.Duration(cfg.EventService.PublishTimeoutSeconds) * time.Second,
		SubscribeTimeout: time.Duration(cfg.EventService.SubscribeTimeoutSeconds) * time.Second,
		EventBufferSize:  cfg.EventService.EventBufferSize,
	}
	eventService := services.NewRedisEventService(redisClient, eventServiceConfig)
	weatherService := services.NewWeatherService(eventService)
	emailService := services.NewEmailService(&cfg.Email)
	healthService := services.NewHealthService(pool, redisClient, cfg.Server.Version)
	notificationService := service.NewNotificationService(notificationDB, userDB, tripDB, eventService, log.Desugar())

	// Initialize offline location service (pass nil initially for location service dependency)
	offlineLocationService := services.NewOfflineLocationService(redisClient, nil)

	// Initialize refactored location service
	// It needs the OfflineLocationServiceInterface, which offlineLocationService implements
	locationManagementService := locationSvc.NewManagementService(
		locationDB, // Implements store.LocationStore now
		eventService,
		offlineLocationService, // Pass the instance
	)

	// Now, set the location service dependency in the offline service
	offlineLocationService.SetLocationService(locationManagementService) // Use the setter method

	// Initialize Chat Store (Needs Supabase Client)
	chatStore := db.NewPostgresChatStore(pool, supabaseClient, os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_SERVICE_KEY"))

	// Initialize Models / Facades
	tripModel := trip.NewTripModel(
		tripDB,
		eventService,
		weatherService,
		supabaseClient,
		&cfg.Server,
		emailService,
		chatStore, // Pass initialized chat store
	)
	todoModel := models.NewTodoModel(todoDB, tripModel)

	// Initialize Handlers
	tripHandler := handlers.NewTripHandler(tripModel, eventService, supabaseClient, &cfg.Server, weatherService, chatStore)
	todoHandler := handlers.NewTodoHandler(todoModel, eventService)
	healthHandler := handlers.NewHealthHandler(healthService)
	locationHandler := handlers.NewLocationHandler(locationManagementService)
	notificationHandler := handlers.NewNotificationHandler(notificationService, log.Desugar())
	wsHandler := handlers.NewWSHandler(rateLimitService, eventService) // Ensure dependencies are correct

	// Prepare Router Dependencies
	routerDeps := router.Dependencies{
		Config:              cfg,
		JWTValidator:        jwtValidator,
		TripHandler:         tripHandler,
		TodoHandler:         todoHandler,
		HealthHandler:       healthHandler,
		LocationHandler:     locationHandler, // Ensure handler uses the refactored service
		NotificationHandler: notificationHandler,
		WSHandler:           wsHandler,
	}

	// Setup Router using the new package
	r := router.SetupRouter(routerDeps)

	// Initialize WebSocket metrics (if still needed here, or move to where WS handler/manager is initialized)
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
	prometheus.MustRegister(
		wsMetrics.ConnectionsActive,
		wsMetrics.MessagesReceived,
		wsMetrics.MessagesSent,
		wsMetrics.ErrorsTotal,
	)
	// Connect WebSocket metrics to health service (if GetActiveConnectionCount is still global/accessible)
	// This might need adjustment depending on how WS connections are managed now
	healthService.SetActiveConnectionsGetter(middleware.GetActiveConnectionCount)

	// Start the server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Server.Port),
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()
	log.Infof("Server started on port %s", cfg.Server.Port)

	// Wait for shutdown signal
	<-shutdownCtx.Done()

	// Perform graceful shutdown
	log.Info("Shutting down server...")

	// Create a deadline context for shutdown
	shutdownTimeout := 5 * time.Second // Configurable?
	shutdownDeadlineCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	// Attempt graceful server shutdown
	if err := srv.Shutdown(shutdownDeadlineCtx); err != nil {
		log.Errorf("Server shutdown failed: %v", err)
	} else {
		log.Info("Server gracefully stopped")
	}

	// Add any other cleanup needed (e.g., closing WebSocket hub/manager)
	// wsManager.Shutdown() // Example

	log.Info("Application shut down complete.")
}
