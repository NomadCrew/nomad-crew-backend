package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/cors"

	"github.com/NomadCrew/nomad-crew-backend/user-service/config"
	"github.com/NomadCrew/nomad-crew-backend/user-service/db"
	"github.com/NomadCrew/nomad-crew-backend/user-service/handlers"
	"github.com/NomadCrew/nomad-crew-backend/user-service/logger"
	"github.com/NomadCrew/nomad-crew-backend/user-service/middleware"
)

func main() {
	logger := logger.GetLogger()
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatalf("Failed to load configuration: %s", err)
	}

	dbPool := db.ConnectToDB(cfg.DatabaseConnectionString)
	server := handlers.Server{DB: dbPool}

	router := chi.NewRouter()

	// Public routes
	router.Post("/v1/register", server.RegisterHandler)
	router.Post("/v1/login", server.LoginHandler)

	// Protected routes
	router.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {+
			return middleware.EnsureValidToken(next)
		})
		// Add your protected routes here
		r.Get("/v1/user", server.GetUserHandler)
		r.Get("/v1/nearby-places", server.GetNearbyPlacesHandler)
	})

	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		Debug:            true,
	}).Handler(router)

	// http.ListenAndServe(":"+cfg.Port, corsHandler)
	err = http.ListenAndServeTLS(":"+cfg.Port, "/secrets/server.crt", "/secrets/myserver.key", corsHandler)
	if err != nil {
		logger.Fatalf("Failed to start TLS server: %s", err)
	}
}
