package main

import (
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

	// Initialize database dependencies
	tripDB := db.NewTripDB(cfg.DB)
	todoDB := db.NewTodoDB(cfg.DB)

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Initialize services
	rateLimitService := services.NewRateLimitService(redisClient)
    eventService := services.NewRedisEventService(redisClient)

	// Handlers
	tripModel := models.NewTripModel(tripDB)
    todoModel := models.NewTodoModel(todoDB, tripModel)
	tripHandler := handlers.NewTripHandler(tripModel, eventService)
	todoHandler := handlers.NewTodoHandler(todoModel, eventService)

	// Router setup
	r := gin.Default()
	r.Use(middleware.ErrorHandler())

	// Versioned routes (e.g., /v1)
	v1 := r.Group("/v1")
	trips := v1.Group("/trips")
	{
		trips.Use(middleware.AuthMiddleware(cfg))

		// Core trip routes
		trips.POST("", tripHandler.CreateTripHandler)         // Create trip
		trips.GET("/:id", tripHandler.GetTripHandler)         // Get trip by ID
		trips.PUT("/:id", tripHandler.UpdateTripHandler)      // Update trip
		trips.DELETE("/:id", tripHandler.DeleteTripHandler)   // Delete trip
		trips.GET("/list", tripHandler.ListUserTripsHandler)  // List trips
		trips.POST("/search", tripHandler.SearchTripsHandler) // Search trips
		trips.PATCH("/:id/status", tripHandler.UpdateTripStatusHandler)
		trips.GET("/:id/stream",
			middleware.RequireRole(tripModel, types.MemberRoleMember),
			tripHandler.StreamEvents)

		// Member management routes
		memberRoutes := trips.Group("/:id/members")
		{
			// Add members (admin only)
			memberRoutes.POST("",
				middleware.RequireRole(tripModel, types.MemberRoleAdmin),
				tripHandler.AddMemberHandler)

			// Update member role (admin only)
			memberRoutes.PUT("/:userId/role",
				middleware.RequireRole(tripModel, types.MemberRoleAdmin),
				tripHandler.UpdateMemberRoleHandler)

			// Remove member (admin or self)
			memberRoutes.DELETE("/:userId", tripHandler.RemoveMemberHandler)

			// Get trip members (any member)
			memberRoutes.GET("",
				middleware.RequireRole(tripModel, types.MemberRoleMember),
				tripHandler.GetTripMembersHandler)
		}
	}

	// Todo routes setup
	setupTodoRoutes(r, todoHandler, cfg, tripModel, rateLimitService)

	log.Infof("Starting server on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// setupTodoRoutes sets up todo-related routes
func setupTodoRoutes(r *gin.Engine, th *handlers.TodoHandler, cfg *config.Config, tripModel *models.TripModel, rateLimitService *services.RateLimitService) {
	todos := r.Group("/v1/trips/:tripId/todos")
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
		todos.PUT("/:id", th.UpdateTodoHandler)
		todos.DELETE("/:id", th.DeleteTodoHandler)
		todos.GET("/stream",
			middleware.RequireRole(tripModel, types.MemberRoleMember),
			th.StreamTodoEvents)
	}
}
