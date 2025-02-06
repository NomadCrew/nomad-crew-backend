package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/db"
	"github.com/NomadCrew/nomad-crew-backend/handlers"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/NomadCrew/nomad-crew-backend/services"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Initialize logger
	logger.InitLogger()
	log := logger.GetLogger()
	defer logger.Close()

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

	// Initialize services
	rateLimitService := services.NewRateLimitService(redisClient)
	// Use the new Redis-based event service.
	eventService := services.NewRedisEventService(redisClient)

	// Handlers
	tripModel := models.NewTripModel(tripDB)
	todoModel := models.NewTodoModel(todoDB, tripModel)
	tripHandler := handlers.NewTripHandler(tripModel, eventService)
	todoHandler := handlers.NewTodoHandler(todoModel, eventService)

	// Router setup
	r := gin.Default()
	r.Use(middleware.ErrorHandler())

	// WebSocket configuration
	wsConfig := middleware.WSConfig{
		AllowedOrigins: cfg.Server.AllowedOrigins,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			for _, allowed := range cfg.Server.AllowedOrigins {
				if origin == allowed {
					return true
				}
			}
			return false
		},
	}

	// Versioned routes (e.g., /v1)
	v1 := r.Group("/v1")
	trips := v1.Group("/trips")
	{
		trips.Use(middleware.AuthMiddleware(cfg))

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
			middleware.AuthMiddleware(cfg),
			middleware.RequireRole(tripModel, types.MemberRoleMember),
			middleware.WSMiddleware(wsConfig),
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
	}

	// Todo routes setup
	setupTodoRoutes(r, todoHandler, cfg, rateLimitService)

	log.Infof("Starting server on port %s", cfg.Server.Port)
	if err := r.Run(":" + cfg.Server.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// setupTodoRoutes sets up todo-related routes.
func setupTodoRoutes(r *gin.Engine, th *handlers.TodoHandler, cfg *config.Config, rateLimitService *services.RateLimitService) {
	// Use consistent parameter name with trip routes.
	todos := r.Group("/v1/trips/:id/todos")
	todos.Use(
		middleware.AuthMiddleware(cfg),
		middleware.RateLimiter(
			rateLimitService.GetRedisClient(),
			100,
			time.Minute,
		),
	)
	{
		todos.POST("", th.CreateTodoHandler)
		todos.GET("", th.ListTodosHandler)
		// Use distinct parameter name for todo ID.
		todos.PUT("/:todoId", th.UpdateTodoHandler)
		todos.DELETE("/:todoId", th.DeleteTodoHandler)
	}
}
