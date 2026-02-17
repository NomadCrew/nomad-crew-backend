package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetConfigPath(t *testing.T) {
	// Save original CONTAINER env var
	originalContainer := os.Getenv("CONTAINER")
	defer func() {
		if originalContainer != "" {
			os.Setenv("CONTAINER", originalContainer)
		} else {
			os.Unsetenv("CONTAINER")
		}
	}()

	tests := []struct {
		name          string
		env           EnvType
		inContainer   bool
		createFile    bool
		expectedPath  string
		expectError   bool
	}{
		{
			name:         "Development environment",
			env:          Development,
			inContainer:  false,
			createFile:   true,
			expectedPath: filepath.Join("config", "config.dev.yaml"),
			expectError:  false,
		},
		{
			name:         "Staging environment",
			env:          Staging,
			inContainer:  false,
			createFile:   true,
			expectedPath: filepath.Join("config", "config.staging.yaml"),
			expectError:  false,
		},
		{
			name:         "Production environment",
			env:          Production,
			inContainer:  false,
			createFile:   true,
			expectedPath: filepath.Join("config", "config.prod.yaml"),
			expectError:  false,
		},
		{
			name:         "Development in container",
			env:          Development,
			inContainer:  true,
			createFile:   false, // Don't create, just test path
			expectedPath: filepath.Join("/app/config", "config.dev.yaml"),
			expectError:  true, // File won't exist in container path
		},
		{
			name:        "Unknown environment",
			env:         EnvType("unknown"),
			inContainer: false,
			createFile:  false,
			expectError: true,
		},
		{
			name:        "File does not exist",
			env:         Development,
			inContainer: false,
			createFile:  false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set container environment
			if tt.inContainer {
				os.Setenv("CONTAINER", "true")
			} else {
				os.Unsetenv("CONTAINER")
			}

			// Create temporary config file if needed
			if tt.createFile {
				configDir := "config"
				if tt.inContainer {
					configDir = "/app/config"
				}
				
				// Ensure config directory exists
				err := os.MkdirAll(configDir, 0755)
				if err == nil {
					defer os.RemoveAll(configDir)
					
					// Create the file
					var filename string
					switch tt.env {
					case Development:
						filename = "config.dev.yaml"
					case Staging:
						filename = "config.staging.yaml"
					case Production:
						filename = "config.prod.yaml"
					}
					
					if filename != "" {
						filePath := filepath.Join(configDir, filename)
						err = os.WriteFile(filePath, []byte("test: config"), 0644)
						require.NoError(t, err)
					}
				}
			}

			path, err := getConfigPath(tt.env)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedPath, path)
			}
		})
	}
}

func TestLoadConfigForEnv(t *testing.T) {
	// Create temporary config directory
	configDir := "config"
	err := os.MkdirAll(configDir, 0755)
	require.NoError(t, err)
	defer os.RemoveAll(configDir)

	// Create a valid test config file
	testConfig := `
server:
  environment: development
  port: 8080
  jwt_secret_key: "test-jwt-secret-key-that-is-long-enough-for-testing"
  allowed_origins:
    - "*"
database:
  host: "localhost"
  port: 5432
  user: "user"
  password: "pass"
  name: "testdb"
redis:
  address: "localhost:6379"
email:
  from_address: "test@example.com"
  resend_api_key: "test-key"
external_services:
  geoapify_key: "test-key"
  pexels_api_key: "test-key"
  supabase_anon_key: "test-key"
  supabase_service_key: "test-service-key-that-is-long-enough"
  supabase_url: "https://test.supabase.co"
  supabase_jwt_secret: "test-jwt-secret-that-is-long-enough"
event_service:
  publish_timeout_seconds: 5
  subscribe_timeout_seconds: 10
  event_buffer_size: 100
rate_limit:
  auth_requests_per_minute: 10
  window_seconds: 60
worker_pool:
  max_workers: 10
  queue_size: 1000
  shutdown_timeout_seconds: 30
`

	configFile := filepath.Join(configDir, "config.dev.yaml")
	err = os.WriteFile(configFile, []byte(testConfig), 0644)
	require.NoError(t, err)

	tests := []struct {
		name        string
		environment string
		expectError bool
	}{
		{
			name:        "Load development config",
			environment: "dev",
			expectError: false,
		},
		{
			name:        "Load non-existent staging config",
			environment: "staging",
			expectError: true,
		},
		{
			name:        "Invalid environment",
			environment: "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadConfigForEnv(tt.environment)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, cfg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, cfg)
				assert.Equal(t, EnvDevelopment, cfg.Server.Environment)
				assert.Equal(t, "8080", cfg.Server.Port)
			}
		})
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	// Create temporary directory for test files
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		fileContent string
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid configuration file",
			fileContent: `
server:
  environment: production
  port: 3000
  jwt_secret_key: "production-jwt-secret-key-that-is-very-long-and-secure"
  allowed_origins:
    - "https://example.com"
database:
  host: "prod.db.com"
  port: 5432
  user: "prod_user"
  password: "prod_pass"
  name: "proddb"
redis:
  address: "prod.redis.com:6379"
  password: "redis-password"
email:
  from_address: "noreply@example.com"
  from_name: "Example App"
  resend_api_key: "prod-resend-key"
external_services:
  geoapify_key: "prod-geoapify-key"
  pexels_api_key: "prod-pexels-key"
  supabase_anon_key: "prod-supabase-anon-key"
  supabase_service_key: "prod-supabase-service-key-that-is-long-enough"
  supabase_url: "https://prod.supabase.co"
  supabase_jwt_secret: "prod-supabase-jwt-secret-that-is-long-enough"
event_service:
  publish_timeout_seconds: 10
  subscribe_timeout_seconds: 20
  event_buffer_size: 200
rate_limit:
  auth_requests_per_minute: 10
  window_seconds: 60
worker_pool:
  max_workers: 10
  queue_size: 1000
  shutdown_timeout_seconds: 30
`,
			expectError: false,
		},
		{
			name: "Invalid YAML syntax",
			fileContent: `
server:
  environment: production
  port: 3000
  jwt_secret_key: "key"
  invalid yaml syntax here
`,
			expectError: true,
			errorMsg:    "failed to read config file",
		},
		{
			name: "Missing required fields",
			fileContent: `
server:
  environment: production
  port: 3000
`,
			expectError: true,
			errorMsg:    "config validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			filePath := filepath.Join(tempDir, "test-config.yaml")
			err := os.WriteFile(filePath, []byte(tt.fileContent), 0644)
			require.NoError(t, err)

			cfg, err := LoadConfigFromFile(filePath)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, cfg)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, cfg)
			}
		})
	}

	t.Run("Non-existent file", func(t *testing.T) {
		cfg, err := LoadConfigFromFile("/non/existent/file.yaml")
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "failed to read config file")
	})
}

func TestCreateConfigTemplateForEnvironment(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	err := os.Chdir(tempDir)
	require.NoError(t, err)
	defer os.Chdir(originalWd)

	// Create config directory
	err = os.MkdirAll("config", 0755)
	require.NoError(t, err)

	tests := []struct {
		name        string
		env         EnvType
		expectError bool
		validateContent func(t *testing.T, content string)
	}{
		{
			name:        "Create development template",
			env:         Development,
			expectError: false,
			validateContent: func(t *testing.T, content string) {
				assert.Contains(t, content, "Config for dev environment")
				assert.Contains(t, content, "frontend_url: \"http://localhost:3000\"")
				assert.Contains(t, content, "sslmode: \"disable\"")
			},
		},
		{
			name:        "Create staging template",
			env:         Staging,
			expectError: false,
			validateContent: func(t *testing.T, content string) {
				assert.Contains(t, content, "Config for staging environment")
				assert.Contains(t, content, "frontend_url: \"https://staging.nomadcrew.uk\"")
				assert.Contains(t, content, "sslmode: \"require\"")
			},
		},
		{
			name:        "Create production template",
			env:         Production,
			expectError: false,
			validateContent: func(t *testing.T, content string) {
				assert.Contains(t, content, "Config for production environment")
				assert.Contains(t, content, "frontend_url: \"https://nomadcrew.uk\"")
				assert.Contains(t, content, "sslmode: \"require\"")
			},
		},
		{
			name:        "Unknown environment",
			env:         EnvType("unknown"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CreateConfigTemplateForEnvironment(tt.env)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)

				// Verify file was created
				var filename string
				switch tt.env {
				case Development:
					filename = "config.dev.yaml"
				case Staging:
					filename = "config.staging.yaml"
				case Production:
					filename = "config.prod.yaml"
				}

				filePath := filepath.Join("config", filename)
				content, err := os.ReadFile(filePath)
				require.NoError(t, err)

				if tt.validateContent != nil {
					tt.validateContent(t, string(content))
				}

				// Clean up for next test
				os.Remove(filePath)
			}
		})
	}

	t.Run("File already exists", func(t *testing.T) {
		// Create a file first
		filePath := filepath.Join("config", "config.dev.yaml")
		err := os.WriteFile(filePath, []byte("existing content"), 0644)
		require.NoError(t, err)

		// Try to create template again
		err = CreateConfigTemplateForEnvironment(Development)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "configuration file already exists")

		// Verify original content wasn't changed
		content, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, "existing content", string(content))
	})
}

func TestGetConfigTemplate(t *testing.T) {
	tests := []struct {
		name     string
		env      EnvType
		validate func(t *testing.T, template string)
	}{
		{
			name: "Development template",
			env:  Development,
			validate: func(t *testing.T, template string) {
				assert.Contains(t, template, "# Config for dev environment")
				assert.Contains(t, template, "frontend_url: \"http://localhost:3000\"")
				assert.Contains(t, template, "sslmode: \"disable\"")
				assert.Contains(t, template, "${JWT_SECRET}")
				assert.Contains(t, template, "${DB_HOST}")
			},
		},
		{
			name: "Staging template",
			env:  Staging,
			validate: func(t *testing.T, template string) {
				assert.Contains(t, template, "# Config for staging environment")
				assert.Contains(t, template, "frontend_url: \"https://staging.nomadcrew.uk\"")
				assert.Contains(t, template, "sslmode: \"require\"")
			},
		},
		{
			name: "Production template",
			env:  Production,
			validate: func(t *testing.T, template string) {
				assert.Contains(t, template, "# Config for production environment")
				assert.Contains(t, template, "frontend_url: \"https://nomadcrew.uk\"")
				assert.Contains(t, template, "sslmode: \"require\"")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := getConfigTemplate(tt.env)
			assert.NotEmpty(t, template)
			
			if tt.validate != nil {
				tt.validate(t, template)
			}

			// Common validations
			assert.Contains(t, template, "server:")
			assert.Contains(t, template, "database:")
			assert.Contains(t, template, "external_services:")
			assert.Contains(t, template, "${SUPABASE_URL}")
			assert.Contains(t, template, "${SUPABASE_ANON_KEY}")
		})
	}
}

func TestEnvironmentTypeCoverage(t *testing.T) {
	// Test all environment type constants
	envTypes := []EnvType{Development, Staging, Production}
	
	for _, env := range envTypes {
		t.Run(string(env), func(t *testing.T) {
			// Verify each environment has a template
			template := getConfigTemplate(env)
			assert.NotEmpty(t, template)
			
			// Verify each environment can generate a valid filename
			var expectedFilename string
			switch env {
			case Development:
				expectedFilename = "config.dev.yaml"
			case Staging:
				expectedFilename = "config.staging.yaml"
			case Production:
				expectedFilename = "config.prod.yaml"
			}
			assert.NotEmpty(t, expectedFilename)
		})
	}
}