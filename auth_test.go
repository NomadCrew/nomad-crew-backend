package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
)

func main() {
	// Initialize logger
	logger.InitLogger()
	log := logger.GetLogger()
	log.Info("Starting JWT validation test")

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Errorw("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Test token from the error log
	token := "eyJhbGciOiJIUzI1NiIsImtpZCI6ImVaQmVmbjlyZTJUVmFsdlMiLCJ0eXAiOiJKV1QifQ.eyJpc3MiOiJodHRwczovL2VmbXFpbHRkYWp2cWVubmRteWx6LnN1cGFiYXNlLmNvL2F1dGgvdjEiLCJzdWIiOiIwYzViYzQ2My1kNTE0LTQ0NTItYjc3NC02ZDk2ZTlmMjRmYzIiLCJhdWQiOiJhdXRoZW50aWNhdGVkIiwiZXhwIjoxNzQxOTg0NDAwLCJpYXQiOjE3NDE5ODA4MDAsImVtYWlsIjoibmFxZWViYWxpLnNoYW1zaUBnbWFpbC5jb20iLCJwaG9uZSI6IiIsImFwcF9tZXRhZGF0YSI6eyJwcm92aWRlciI6ImFwcGxlIiwicHJvdmlkZXJzIjpbImFwcGxlIl19LCJ1c2VyX21ldGFkYXRhIjp7ImN1c3RvbV9jbGFpbXMiOnsiYXV0aF90aW1lIjoxNzQxOTY2MjQyfSwiZW1haWwiOiJuYXFlZWJhbGkuc2hhbXNpQGdtYWlsLmNvbSIsImVtYWlsX3ZlcmlmaWVkIjp0cnVlLCJpc3MiOiJodHRwczovL2FwcGxlaWQuYXBwbGUuY29tIiwicGhvbmVfdmVyaWZpZWQiOmZhbHNlLCJwcm92aWRlcl9pZCI6IjAwMDI5MC43ZmEyOTcyOTU4ODI0MjE2YWY0MjU1YmNlYmY0OTZkYi4yMTQ4Iiwic3ViIjoiMDAwMjkwLjdmYTI5NzI5NTg4MjQyMTZhZjQyNTViY2ViZjQ5NmRiLjIxNDgifSwicm9sZSI6ImF1dGhlbnRpY2F0ZWQiLCJhYWwiOiJhYWwxIiwiYW1yIjpbeyJtZXRob2QiOiJvYXV0aCIsInRpbWVzdGFtcCI6MTc0MTk2NjI0NX1dLCJzZXNzaW9uX2lkIjoiZGFjOWRmYmYtZDFkZS00ZTdhLWI5MDYtOTcyM2E0YTQ4NWZkIiwiaXNfYW5vbnltb3VzIjpmYWxzZX0.qo1nN44o9F7bxRmu3C3OLiuuOBNIi7x3qO5wtegUJKA"

	// Print token info
	parts := strings.Split(token, ".")
	fmt.Printf("Token has %d parts\n", len(parts))
	fmt.Printf("Token length: %d\n", len(token))

	// Print config info
	fmt.Printf("Supabase URL: %s\n", cfg.ExternalServices.SupabaseURL)
	fmt.Printf("Supabase Anon Key length: %d\n", len(cfg.ExternalServices.SupabaseAnonKey))
	fmt.Printf("Supabase JWT Secret length: %d\n", len(cfg.ExternalServices.SupabaseJWTSecret))

	// Test JWKS cache
	jwksURL := fmt.Sprintf("%s/auth/v1/jwks", strings.TrimSuffix(cfg.ExternalServices.SupabaseURL, "/"))
	cache := middleware.GetJWKSCache(jwksURL)

	// Try to get the key with the kid from the token
	kid := "eZBefn9re2TValvS"
	fmt.Printf("Attempting to fetch key with kid: %s\n", kid)

	start := time.Now()
	key, err := cache.GetKey(kid)
	elapsed := time.Since(start)

	if err != nil {
		fmt.Printf("Error fetching key: %v (in %v)\n", err, elapsed)
		os.Exit(1)
	}

	fmt.Printf("Successfully fetched key in %v\n", elapsed)
	fmt.Printf("Key ID: %s\n", key.KeyID())
	fmt.Printf("Key Type: %s\n", key.KeyType())

	// Test token validation
	fmt.Println("\nTesting token validation...")
	start = time.Now()
	userID, err := middleware.ValidateTokenWithoutAbort(token)
	elapsed = time.Since(start)

	if err != nil {
		fmt.Printf("Token validation failed: %v (in %v)\n", err, elapsed)
		os.Exit(1)
	}

	fmt.Printf("Token validation successful in %v\n", elapsed)
	fmt.Printf("User ID: %s\n", userID)
}
