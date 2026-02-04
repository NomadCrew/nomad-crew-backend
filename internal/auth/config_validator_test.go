package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/stretchr/testify/assert"
)

func TestNewConfigValidator(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			JwtSecretKey: "test-secret-key-that-is-long-enough",
		},
	}

	validator := NewConfigValidator(cfg)
	
	assert.NotNil(t, validator)
	assert.Equal(t, cfg, validator.config)
	assert.NotNil(t, validator.client)
	assert.Equal(t, 10*time.Second, validator.client.Timeout)
}

func TestValidateAuthConfig(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.Config
		setupMockServer func() *httptest.Server
		expectedErrors []string
	}{
		{
			name: "Valid configuration",
			config: &config.Config{
				Server: config.ServerConfig{
					JwtSecretKey: "this-is-a-very-long-secret-key-that-meets-requirements",
				},
				ExternalServices: config.ExternalServices{
					SupabaseURL:     "",
					SupabaseAnonKey: "",
				},
			},
			expectedErrors: []string{},
		},
		{
			name: "Missing JWT secret",
			config: &config.Config{
				Server: config.ServerConfig{
					JwtSecretKey: "",
				},
			},
			expectedErrors: []string{"JWT secret key is not configured"},
		},
		{
			name: "JWT secret too short",
			config: &config.Config{
				Server: config.ServerConfig{
					JwtSecretKey: "short",
				},
			},
			expectedErrors: []string{"JWT secret key is too short"},
		},
		{
			name: "Invalid Supabase URL",
			config: &config.Config{
				Server: config.ServerConfig{
					JwtSecretKey: "this-is-a-very-long-secret-key-that-meets-requirements",
				},
				ExternalServices: config.ExternalServices{
					SupabaseURL:     "not-a-valid-url",
					SupabaseAnonKey: "test-key",
				},
			},
			expectedErrors: []string{"invalid Supabase URL"},
		},
		{
			name: "Missing Supabase anon key",
			config: &config.Config{
				Server: config.ServerConfig{
					JwtSecretKey: "this-is-a-very-long-secret-key-that-meets-requirements",
				},
				ExternalServices: config.ExternalServices{
					SupabaseURL:     "https://test.supabase.co",
					SupabaseAnonKey: "",
				},
			},
			expectedErrors: []string{"Supabase anon key is not configured"},
		},
		{
			name: "Valid Supabase config with accessible JWKS",
			config: &config.Config{
				Server: config.ServerConfig{
					JwtSecretKey: "this-is-a-very-long-secret-key-that-meets-requirements",
				},
				ExternalServices: config.ExternalServices{
					SupabaseURL:     "", // Will be set to mock server URL
					SupabaseAnonKey: "test-anon-key",
				},
			},
			setupMockServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/auth/v1/jwks" {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{"keys": []}`))
					} else {
						w.WriteHeader(http.StatusNotFound)
					}
				}))
			},
			expectedErrors: []string{},
		},
		{
			name: "Supabase JWKS endpoint not accessible",
			config: &config.Config{
				Server: config.ServerConfig{
					JwtSecretKey: "this-is-a-very-long-secret-key-that-meets-requirements",
				},
				ExternalServices: config.ExternalServices{
					SupabaseURL:     "", // Will be set to mock server URL
					SupabaseAnonKey: "test-anon-key",
				},
			},
			setupMockServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
			},
			expectedErrors: []string{"JWKS endpoint not accessible"},
		},
		{
			name: "Multiple validation errors",
			config: &config.Config{
				Server: config.ServerConfig{
					JwtSecretKey: "short",
				},
				ExternalServices: config.ExternalServices{
					SupabaseURL:     "not-a-url",
					SupabaseAnonKey: "",
				},
			},
			expectedErrors: []string{
				"JWT secret key is too short",
				"invalid Supabase URL",
				"Supabase anon key is not configured",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.config
			
			// Setup mock server if needed
			if tt.setupMockServer != nil {
				server := tt.setupMockServer()
				defer server.Close()
				cfg.ExternalServices.SupabaseURL = server.URL
			}

			validator := NewConfigValidator(cfg)
			errors := validator.ValidateAuthConfig()

			if len(tt.expectedErrors) == 0 {
				assert.Empty(t, errors)
			} else {
				assert.Len(t, errors, len(tt.expectedErrors))
				
				// Check that all expected errors are present
				for _, expectedErr := range tt.expectedErrors {
					found := false
					for _, err := range errors {
						if err != nil && contains(err.Error(), expectedErr) {
							found = true
							break
						}
					}
					assert.True(t, found, "Expected error not found: %s", expectedErr)
				}
			}
		})
	}
}

func TestCheckEndpointAvailability(t *testing.T) {
	tests := []struct {
		name         string
		setupServer  func() *httptest.Server
		useSupabase  bool
		expectError  bool
	}{
		{
			name: "Successful endpoint check",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
			},
			expectError: false,
		},
		{
			name: "Endpoint returns 404",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			expectError: true,
		},
		{
			name: "Endpoint returns 500",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
			},
			expectError: true,
		},
		{
			name: "Supabase endpoint with API key",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Check for API key header
					if r.Header.Get("Apikey") == "test-anon-key" {
						w.WriteHeader(http.StatusOK)
					} else {
						w.WriteHeader(http.StatusUnauthorized)
					}
				}))
			},
			useSupabase: true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			cfg := &config.Config{
				ExternalServices: config.ExternalServices{},
			}

			endpoint := server.URL + "/test"
			if tt.useSupabase {
				cfg.ExternalServices.SupabaseURL = server.URL
				cfg.ExternalServices.SupabaseAnonKey = "test-anon-key"
				endpoint = fmt.Sprintf("%s/auth/v1/jwks", server.URL)
			}

			validator := NewConfigValidator(cfg)
			err := validator.checkEndpointAvailability(endpoint)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTestTokenCreation(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			JwtSecretKey: "this-is-a-very-long-secret-key-that-meets-requirements",
		},
	}

	validator := NewConfigValidator(cfg)
	
	// Currently returns nil as per the implementation comment
	err := validator.TestTokenCreation()
	assert.NoError(t, err)
}

func TestPrintValidationResults(t *testing.T) {
	cfg := &config.Config{}
	validator := NewConfigValidator(cfg)

	t.Run("No errors", func(t *testing.T) {
		// Should log success message without panicking
		validator.PrintValidationResults([]error{})
	})

	t.Run("With errors", func(t *testing.T) {
		errors := []error{
			fmt.Errorf("first error"),
			fmt.Errorf("second error"),
			fmt.Errorf("third error"),
		}
		
		// Should log errors without panicking
		validator.PrintValidationResults(errors)
	})
}

func TestEndpointTimeouts(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(15 * time.Second) // Longer than client timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{
		ExternalServices: config.ExternalServices{
			SupabaseURL:     server.URL,
			SupabaseAnonKey: "test-key",
		},
	}

	validator := NewConfigValidator(cfg)
	
	// Should timeout and return error
	err := validator.checkEndpointAvailability(server.URL + "/test")
	assert.Error(t, err)
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && s[0:len(substr)] == substr || (len(substr) < len(s) && containsHelper(s[1:], substr)))
}

func containsHelper(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	if s[0:len(substr)] == substr {
		return true
	}
	return containsHelper(s[1:], substr)
}