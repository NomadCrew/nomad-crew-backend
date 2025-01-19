package main

import (
    "github.com/gin-gonic/gin"
    "github.com/NomadCrew/nomad-crew-backend/config"
    "github.com/NomadCrew/nomad-crew-backend/db"
    "github.com/NomadCrew/nomad-crew-backend/handlers"
    "github.com/NomadCrew/nomad-crew-backend/logger"
    "github.com/NomadCrew/nomad-crew-backend/middleware"
    "github.com/NomadCrew/nomad-crew-backend/models"
    "github.com/NomadCrew/nomad-crew-backend/types"
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

    // Handlers
    tripHandler := handlers.NewTripHandler(models.NewTripModel(tripDB))

    // Router setup
    r := gin.Default()
    r.Use(middleware.ErrorHandler())

    // Versioned routes (e.g., /v1)
    v1 := r.Group("/v1")
    trips := v1.Group("/trips")
{
    trips.Use(middleware.AuthMiddleware(cfg))
    
    // Core trip routes
    trips.POST("", tripHandler.CreateTripHandler)            // Create trip
    trips.GET("/:id", tripHandler.GetTripHandler)           // Get trip by ID (now includes members)
    trips.PUT("/:id", tripHandler.UpdateTripHandler)        // Update trip
    trips.DELETE("/:id", tripHandler.DeleteTripHandler)     // Delete trip
    trips.GET("/list", tripHandler.ListUserTripsHandler)    // List trips
    trips.POST("/search", tripHandler.SearchTripsHandler)   // Search trips
    trips.PATCH("/:id/status", tripHandler.UpdateTripStatusHandler)
    

    tripModel := models.NewTripModel(tripDB)

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
        memberRoutes.DELETE("/:userId",
            tripHandler.RemoveMemberHandler)

        // Get trip members (any member)
        memberRoutes.GET("",
            middleware.RequireRole(tripModel, types.MemberRoleMember),
            tripHandler.GetTripMembersHandler)
    }
}

    log.Infof("Starting server on port %s", cfg.Port)
    if err := r.Run(":" + cfg.Port); err != nil {
        log.Fatalf("Failed to start server: %v", err)
    }
}