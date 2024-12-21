package main

import (
	"github.com/gin-gonic/gin"
	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/db"
	"github.com/NomadCrew/nomad-crew-backend/handlers"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/models"
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
    userDB := db.NewUserDB(cfg.DB)
    tripDB := db.NewTripDB(cfg.DB)

    // Handlers
    userHandler := handlers.NewUserHandler(models.NewUserModel(userDB))
    tripHandler := handlers.NewTripHandler(models.NewTripModel(tripDB))

    // Router setup
    r := gin.Default()
    r.Use(middleware.ErrorHandler())

    // Routes that are outside versioning
    r.POST("/login", userHandler.LoginHandler)           // Login endpoint
    r.POST("/users", userHandler.CreateUserHandler)      // Create user endpoint

    // Versioned routes (e.g., /v1)
    v1 := r.Group("/v1")
    users := v1.Group("/users")
    {
        users.GET("/:id", userHandler.GetUserHandler)     // Get user by ID
        users.PUT("/:id", userHandler.UpdateUserHandler)  // Update user
        users.DELETE("/:id", userHandler.DeleteUserHandler) // Delete user
    }

    trips := v1.Group("/trips")
    {
        trips.Use(middleware.AuthMiddleware())
        trips.POST("", tripHandler.CreateTripHandler)     // Create trip
        trips.GET("/:id", tripHandler.GetTripHandler)     // Get trip by ID
        trips.PUT("/:id", tripHandler.UpdateTripHandler)  // Update trip
        trips.DELETE("/:id", tripHandler.DeleteTripHandler) // Delete trip
        trips.GET("/list", tripHandler.ListUserTripsHandler) // List trips
        trips.POST("/search", tripHandler.SearchTripsHandler) // Search trips
    }

    log.Infof("Starting server on port %s", cfg.Port)
    if err := r.Run(":" + cfg.Port); err != nil {
        log.Fatalf("Failed to start server: %v", err)
    }
}
