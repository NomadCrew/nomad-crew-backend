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
    r.POST("/login", userHandler.LoginHandler)
    v1 := r.Group("/v1")
    users := v1.Group("/users")
    {
        users.POST("", userHandler.CreateUserHandler)
        users.GET("/:id", userHandler.GetUserHandler)
        users.PUT("/:id", userHandler.UpdateUserHandler)
        users.DELETE("/:id", userHandler.DeleteUserHandler)
    }

    trips := v1.Group("/trips")
    {
        trips.Use(middleware.AuthMiddleware())
        trips.POST("", tripHandler.CreateTripHandler)
        trips.GET("/:id", tripHandler.GetTripHandler)
        trips.PUT("/:id", tripHandler.UpdateTripHandler)
        trips.DELETE("/:id", tripHandler.DeleteTripHandler)
        trips.GET("/list", tripHandler.ListUserTripsHandler)
        trips.POST("/search", tripHandler.SearchTripsHandler)
    }

    log.Infof("Starting server on port %s", cfg.Port)
    if err := r.Run(":" + cfg.Port); err != nil {
        log.Fatalf("Failed to start server: %v", err)
    }
}
