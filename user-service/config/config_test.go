package config

import (
    "os"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
    tests := []struct {
        name        string
        envVars     map[string]string
        expectError bool
    }{
        {
            name: "valid configuration",
            envVars: map[string]string{
                "DB_CONNECTION_STRING": "postgresql://test:test@localhost:5432/testdb",
                "JWT_SECRET_KEY":      "test-secret",
                "PORT":               "8080",
            },
            expectError: false,
        },
        {
            name: "missing database connection",
            envVars: map[string]string{
                "JWT_SECRET_KEY": "test-secret",
                "PORT":          "8080",
            },
            expectError: true,
        },
        {
            name: "missing JWT secret",
            envVars: map[string]string{
                "DB_CONNECTION_STRING": "postgresql://test:test@localhost:5432/testdb",
                "PORT":                "8080",
            },
            expectError: true,
        },
        {
            name: "missing port",
            envVars: map[string]string{
                "DB_CONNECTION_STRING": "postgresql://test:test@localhost:5432/testdb",
                "JWT_SECRET_KEY":      "test-secret",
            },
            expectError: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Clear environment before each test
            os.Clearenv()

            // Set up test environment
            for key, value := range tt.envVars {
                os.Setenv(key, value)
            }

            // Run test
            cfg, err := LoadConfig()

            // Verify results
            if tt.expectError {
                assert.Error(t, err)
                assert.Nil(t, cfg)
            } else {
                require.NoError(t, err)
                assert.NotNil(t, cfg)
                assert.Equal(t, tt.envVars["DB_CONNECTION_STRING"], cfg.DatabaseConnectionString)
                assert.Equal(t, tt.envVars["JWT_SECRET_KEY"], cfg.JwtSecretKey)
                assert.Equal(t, tt.envVars["PORT"], cfg.Port)
                assert.NotNil(t, cfg.DB)
            }
        })
    }
}

func TestMaskSensitiveURL(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {
            name:     "valid postgresql url",
            input:    "postgresql://user:password@localhost:5432/dbname",
            expected: "postgresql://*****:*****@localhost:5432/dbname",
        },
        {
            name:     "empty url",
            input:    "",
            expected: "",
        },
        {
            name:     "invalid url format",
            input:    "invalid-url",
            expected: "invalid-url-format",
        },
        {
            name:     "url without credentials",
            input:    "localhost:5432/dbname",
            expected: "invalid-url-format",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := maskSensitiveURL(tt.input)
            assert.Equal(t, tt.expected, result)
        })
    }
}