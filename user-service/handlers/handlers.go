package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/NomadCrew/nomad-crew-backend/user-service/models"
	"github.com/jackc/pgx/v4/pgxpool"
)

type Server struct {
	DB *pgxpool.Pool
}

func (s *Server) RegisterHandler(w http.ResponseWriter, r *http.Request) {
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
	if err := u.SaveUser(ctx, s.DB); err != nil {
		http.Error(w, "Failed to save user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	u.Password = ""
	json.NewEncoder(w).Encode(u)
}

func (s *Server) LoginHandler(w http.ResponseWriter, r *http.Request) {
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
	user, err := models.AuthenticateUser(ctx, s.DB, credentials.Email, credentials.Password)
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

func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	jsonResp, _ := json.Marshal(map[string]string{"status": "ok"})
	_, _ = w.Write(jsonResp)
}
