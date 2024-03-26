package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"

	"github.com/NomadCrew/nomad-crew-backend/user-service/db"
	"github.com/NomadCrew/nomad-crew-backend/user-service/logger"
	"github.com/NomadCrew/nomad-crew-backend/user-service/models"
	"github.com/jackc/pgx/v4/pgxpool"
)

type Server struct {
	db *pgxpool.Pool
}

type Config struct {
	DatabaseConnectionString string `json:"databaseConnectionString"`
	JwtSecretKey             string `json:"jwtSecretKey"`
}

func loadConfig() Config {
	logger := logger.GetLogger()
	configFile, err := os.Open("config.json")
	if err != nil {
		logger.Fatal("Error opening config file:", err)
	}
	defer configFile.Close()

	var config Config
	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(&config)
	if err != nil {
		logger.Error("Error decoding config file:", err)
	}

	return config
}


func main() {
	logger := logger.GetLogger()
	dbConnectionString := os.Getenv("DB_CONNECTION_STRING")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	dbPool := db.ConnectToDB(dbConnectionString)
	server := &Server{db: dbPool}
	logger.Info(server.db.Stat())

	http.HandleFunc("/v1/register", server.registerHandler)
	http.HandleFunc("/v1/login", server.loginHandler)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		logger.Errorf("Failed to start server: %s", err)
	}
}

func (s *Server) registerHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var u models.User
    if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    if err := u.HashPassword(u.Password); err != nil {
        http.Error(w, "Failed to hash password", http.StatusInternalServerError)
        return
    }

    ctx := r.Context()
    if err := u.SaveUser(ctx, s.db); err != nil {
        http.Error(w, "Failed to save user", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
    u.Password = ""
    json.NewEncoder(w).Encode(u)
}

func (s *Server) loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var credentials struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	user, err := models.AuthenticateUser(ctx, s.db, credentials.Email, credentials.Password)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		} else {
			http.Error(w, "Server error", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(user)
}
