package main

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/NomadCrew/nomad-crew-backend/db"
)

type Server struct {
	db db.Datastore
}

func main() {
	db := db.NewDB("postgres", "host=localhost port=5432 user=postgres password=admin123 dbname=nomadcrew sslmode=disable")
	server := &Server{db: db}

	http.HandleFunc("/register", server.registerHandler)
	http.HandleFunc("/login", server.loginHandler)
	http.ListenAndServe(":8080", nil)
}

func (s *Server) registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var u User
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	if err := u.SaveUser(ctx, s.db); err != nil {
		http.Error(w, "Failed to save user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(u) // Send back the user details (excluding password)
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
	user, err := AuthenticateUser(ctx, s.db, credentials.Email, credentials.Password)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		} else {
			http.Error(w, "Server error", http.StatusInternalServerError)
		}
		return
	}

	// Generate JWT token (not implemented here)
	// ...

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(user) // Send back the user details (excluding password)
}
