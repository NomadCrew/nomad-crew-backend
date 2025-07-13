// Package main provides a simple utility to check authentication configuration.
// It can be run with: go run scripts/auth_verification/auth_config/verify.go
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/internal/auth"
	"github.com/NomadCrew/nomad-crew-backend/logger"
)

func main() {
	fmt.Println("NomadCrew Auth Configuration Check")
	fmt.Println("=================================")

	// Define command line flags
	environment := flag.String("env", "dev", "Environment to verify (dev, staging, production)")
	configFile := flag.String("config", "", "Path to config file (if not using environment defaults)")
	verbose := flag.Bool("verbose", false, "Enable verbose output")

	flag.Parse()

	// Initialize logger
	log := logger.GetLogger()
	log.Info("Starting auth configuration check")

	// Load configuration
	var cfg *config.Config
	var err error

	if *configFile != "" {
		cfg, err = config.LoadConfigFromFile(*configFile)
	} else {
		cfg, err = config.LoadConfigForEnv(*environment)
	}

	if err != nil {
		fmt.Printf("❌ Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Checking auth configuration for environment: %s\n\n", *environment)
	if *verbose {
		fmt.Printf("Using Supabase URL: %s\n", cfg.ExternalServices.SupabaseURL)
		fmt.Println("JWT secret configured:", cfg.Server.JwtSecretKey != "")
	}

	// Create validator
	validator := auth.NewConfigValidator(cfg)

	// Validate auth configuration
	errors := validator.ValidateAuthConfig()

	// Print results
	if len(errors) > 0 {
		fmt.Printf("❌ Auth configuration validation failed with %d errors:\n", len(errors))
		for i, err := range errors {
			fmt.Printf("  %d. %s\n", i+1, err)
		}
		os.Exit(1)
	}

	// Test token creation and validation
	err = validator.TestTokenCreation()
	if err != nil {
		fmt.Printf("❌ Token creation/validation test failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Auth configuration is valid")
	fmt.Println("✅ Token creation and validation working correctly")
	fmt.Println("✅ Auth configuration check completed successfully")
}
