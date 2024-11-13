package main

import (
	"github.com/gin-gonic/gin"
	"github.com/NomadCrew/nomad-crew-backend/user-service/config"
	"github.com/NomadCrew/nomad-crew-backend/user-service/db"
	"github.com/NomadCrew/nomad-crew-backend/user-service/handlers"
	"github.com/NomadCrew/nomad-crew-backend/user-service/logger"
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
	h := handlers.NewHandler(userDB)
	tripHandler := handlers.NewTripHandler(tripDB)

	// Router setup
	r := gin.Default()
	r.POST("/users", h.CreateUserHandler)
	r.GET("/users/:id", h.GetUserHandler)
	r.PUT("/users/:id", h.UpdateUserHandler)
	r.DELETE("/users/:id", h.DeleteUserHandler)
	r.POST("/login", h.LoginHandler)

	v1 := r.Group("/v1")
	{
		v1.POST("/trips", tripHandler.CreateTripHandler)
		v1.GET("/trips/:id", tripHandler.GetTripHandler)
		v1.PUT("/trips/:id", tripHandler.UpdateTripHandler)
		v1.DELETE("/trips/:id", tripHandler.DeleteTripHandler)
	}

	log.Infof("Starting server on port %s", cfg.Port)
	r.Run(":" + cfg.Port)
}
