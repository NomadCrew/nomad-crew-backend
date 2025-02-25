package main

import (
	"context"
	"crypto/tls"
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

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Info("Shutting down gracefully...")
	}()

	// Load configuration and DB connection
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database connection directly
	var poolConfig *pgxpool.Config
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Name,
		cfg.Database.SSLMode,
	)

	if cfg.Server.Environment == config.EnvProduction {
		poolConfig, err = pgxpool.ParseConfig(connStr)
		if err != nil {
			log.Fatalf("Failed to parse database config: %v", err)
		}
		poolConfig.ConnConfig.TLSConfig = &tls.Config{
			ServerName: cfg.Database.Host,
			MinVersion: tls.VersionTLS12,
		}
	} else {
		// Development configuration with plain TCP connection
		devConnStr := connStr

		poolConfig, err = pgxpool.ParseConfig(devConnStr)
		if err != nil {
			log.Fatalf("Failed to parse database config: %v", err)
		}
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

	// Initialize Redis client with TLS in production
	redisOptions := &redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}

	if cfg.Server.Environment == config.EnvProduction {
		redisOptions.TLSConfig = &tls.Config{
			ServerName: cfg.Redis.Address,
			MinVersion: tls.VersionTLS12,
		}
	}

	redisClient := redis.NewClient(redisOptions)
	defer redisClient.Close()

	// Initialize Supabase client
	supabaseClient, err := supabase.NewClient(
		cfg.ExternalServices.SupabaseAnonKey,
		cfg.ExternalServices.SupabaseURL,
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to initialize Supabase client: %v", err)
	}

	// Initialize services
	rateLimitService := services.NewRateLimitService(redisClient)
	eventService := services.NewRedisEventService(redisClient)
	weatherService := services.NewWeatherService(eventService)
	emailService := services.NewEmailService(&cfg.Email)
	healthService := services.NewHealthService(pool, redisClient, cfg.Server.Version)

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
	)
	todoModel := models.NewTodoModel(todoDB, tripModel)
	tripHandler := handlers.NewTripHandler(tripModel, eventService, supabaseClient)
	todoHandler := handlers.NewTodoHandler(todoModel, eventService)
	healthHandler := handlers.NewHealthHandler(healthService)

	// Router setup
	r := gin.Default()
	r.Use(middleware.ErrorHandler())

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

	// Versioned routes (e.g., /v1)
	v1 := r.Group("/v1")
	trips := v1.Group("/trips")
	{
		trips.Use(middleware.AuthMiddleware(&cfg.Server))

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
			middleware.AuthMiddleware(&cfg.Server),
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
			todoRoutes.Use(
				middleware.AuthMiddleware(&cfg.Server), // Auth first
			)

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
	}

	// Create auth handler
	authHandler := handlers.NewAuthHandler(supabaseClient, cfg)

	// Auth routes that don't require authentication
	authRoutes := v1.Group("/auth")
	// Don't add AuthMiddleware here - these routes are for unauthenticated users
	{
		authRoutes.POST("/refresh", authHandler.RefreshTokenHandler)
	}

	// Create server
	srv := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Run server in goroutine
	go func() {
		log.Infof("Starting server on port %s", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-sigChan
	log.Info("Shutting down gracefully...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	// Shutdown event service first to stop new events
	if err := eventService.Shutdown(shutdownCtx); err != nil {
		log.Warnf("Event service shutdown error: %v", err)
	}

	// Shutdown server
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
}
