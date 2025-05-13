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
	"github.com/NomadCrew/nomad-crew-backend/router"
	"github.com/NomadCrew/nomad-crew-backend/service"
	services "github.com/NomadCrew/nomad-crew-backend/services"
	dbStore "github.com/NomadCrew/nomad-crew-backend/store/postgres"

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
	tripStore := dbStore.NewPgTripStore(pool)
	todoStore := db.NewTodoDB(dbClient)
	locationDB := db.NewLocationDB(dbClient)
	notificationDB := dbStore.NewPgNotificationStore(pool)

	// Get Supabase service key from environment variable
	supabaseServiceKey := os.Getenv("SUPABASE_SERVICE_KEY")
	userDB := internalPgStore.NewUserStore(pool, cfg.ExternalServices.SupabaseURL, supabaseServiceKey)

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

	// Initialize JWT Validator
	jwtValidator, err := middleware.NewJWTValidator(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize JWT Validator: %v", err)
	}
	log.Info("JWT Validator initialized successfully")

	// Initialize services
	rateLimitService := services.NewRateLimitService(redisClient)

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
	healthService := services.NewHealthService(pool, redisClient, cfg.Server.Version)

	// Initialize notification service with correct dependencies
	notificationService := service.NewNotificationService(notificationDB, userDB, tripStore, eventService, log.Desugar())

	// Initialize refactored location service
	locationManagementService := locationSvc.NewManagementService(
		locationDB,
		eventService,
	)

	// Initialize Chat Store and Service
	chatStore := internalPgStore.NewChatStore(pool)

	// Use the internal service package's ChatService implementation
	chatService := internalService.NewChatService(chatStore, tripStore, eventService)

	tripMemberService := trip_service.NewTripMemberService(tripStore, eventService)
	chatHandler := handlers.NewChatHandler(
		chatService,
		tripMemberService,
		eventService,
		log.Desugar(),
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
	)
	todoModel := models.NewTodoModel(todoStore, tripModel, eventService)

	// Initialize Handlers
	tripHandler := handlers.NewTripHandler(tripModel, eventService, supabaseClient, &cfg.Server, weatherService)
	todoHandler := handlers.NewTodoHandler(todoModel, eventService, log.Desugar())
	healthHandler := handlers.NewHealthHandler(healthService)
	locationHandler := handlers.NewLocationHandler(locationManagementService, log.Desugar())
	notificationHandler := handlers.NewNotificationHandler(notificationService, log.Desugar())
	wsHandler := handlers.NewWSHandler(rateLimitService, eventService, tripStore)

	// Initialize User Service and Handler
	userService := userSvc.NewUserService(userDB)
	userHandler := handlers.NewUserHandler(userService)

	// Prepare Router Dependencies
	routerDeps := router.Dependencies{
		Config:              cfg,
		JWTValidator:        jwtValidator,
		TripHandler:         tripHandler,
		TodoHandler:         todoHandler,
		HealthHandler:       healthHandler,
		LocationHandler:     locationHandler,
		NotificationHandler: notificationHandler,
		WSHandler:           wsHandler,
		ChatHandler:         chatHandler,
		UserHandler:         userHandler,
		Logger:              log,
	}

	// Setup Router using the new package
	r := router.SetupRouter(routerDeps)

	log.Info("Router setup complete")

	// Initialize WebSocket metrics (if still needed here, or move to where WS handler/manager is initialized)
	wsMetrics := &middleware.WSMetrics{
		ConnectionsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "websocket_active_connections",
			Help: "Current active WebSocket connections",
		}),
		MessagesReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "websocket_messages_received_total",
			Help: "Total number of WebSocket messages received.",
		}),
		MessagesSent: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "websocket_messages_sent_total",
			Help: "Total number of WebSocket messages sent.",
		}),
		ErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "websocket_errors_total",
			Help: "Total number of WebSocket errors.",
		}, []string{"trip_id", "type"}),
	}
	prometheus.MustRegister(wsMetrics.ConnectionsActive, wsMetrics.MessagesReceived, wsMetrics.MessagesSent, wsMetrics.ErrorsTotal)

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
