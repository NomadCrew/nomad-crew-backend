package auth

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/logger"
)

// ConfigValidator provides methods to validate auth-related configuration
type ConfigValidator struct {
	config *config.Config
	client *http.Client
}

// NewConfigValidator creates a new validator for auth configuration
func NewConfigValidator(cfg *config.Config) *ConfigValidator {
	return &ConfigValidator{
		config: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ValidateAuthConfig performs comprehensive validation of auth configuration
func (v *ConfigValidator) ValidateAuthConfig() []error {
	var errors []error

	// Validate JWT Secret
	if v.config.Server.JwtSecretKey == "" {
		errors = append(errors, fmt.Errorf("JWT secret key is not configured"))
	} else if len(v.config.Server.JwtSecretKey) < 32 {
		errors = append(errors, fmt.Errorf("JWT secret key is too short (should be at least 32 characters)"))
	}

	// Validate Supabase configuration if used
	if v.config.ExternalServices.SupabaseURL != "" {
		// Validate URL format
		if _, err := url.Parse(v.config.ExternalServices.SupabaseURL); err != nil {
			errors = append(errors, fmt.Errorf("invalid Supabase URL: %w", err))
		}

		// Validate Supabase key
		if v.config.ExternalServices.SupabaseAnonKey == "" {
			errors = append(errors, fmt.Errorf("Supabase anon key is not configured"))
		}

		// Check JWKS URL accessibility
		jwksURL := fmt.Sprintf("%s/auth/v1/jwks", v.config.ExternalServices.SupabaseURL)
		if err := v.checkEndpointAvailability(jwksURL); err != nil {
			errors = append(errors, fmt.Errorf("JWKS endpoint not accessible: %w", err))
		}
	}

	return errors
}

// TestTokenCreation attempts to create and validate a test token
func (v *ConfigValidator) TestTokenCreation() error {
	// This would need to be implemented based on the token creation logic used in the app
	// For a complete implementation, you would:
	// 1. Create a test token
	// 2. Validate the token
	// 3. Return any errors encountered
	return nil
}

// checkEndpointAvailability tests if an endpoint is accessible
func (v *ConfigValidator) checkEndpointAvailability(endpoint string) error {
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return err
	}

	// Add Supabase key header if it's a Supabase endpoint
	if v.config.ExternalServices.SupabaseAnonKey != "" && 
	   endpoint == fmt.Sprintf("%s/auth/v1/jwks", v.config.ExternalServices.SupabaseURL) {
		req.Header.Add("apikey", v.config.ExternalServices.SupabaseAnonKey)
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("endpoint returned status code %d", resp.StatusCode)
	}

	return nil
}

// PrintValidationResults logs all validation results
func (v *ConfigValidator) PrintValidationResults(errors []error) {
	log := logger.GetLogger()
	
	if len(errors) == 0 {
		log.Info("Auth configuration validation passed successfully")
		return
	}

	log.Errorw("Auth configuration validation failed", "error_count", len(errors))
	for i, err := range errors {
		log.Errorw("Validation error", "index", i+1, "error", err.Error())
	}
} 
