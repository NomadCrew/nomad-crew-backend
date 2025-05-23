package config

import (
	"os"
	"strconv"
	"strings"
)

// FeatureFlags holds all feature flags for the application
type FeatureFlags struct {
	EnableSupabaseRealtime bool // Controls whether to use Supabase Realtime
}

// GetFeatureFlags loads feature flags from environment variables
func GetFeatureFlags() FeatureFlags {
	return FeatureFlags{
		EnableSupabaseRealtime: getBoolEnv("ENABLE_SUPABASE_REALTIME", false),
	}
}

// getBoolEnv retrieves a boolean environment variable with a default value
func getBoolEnv(key string, defaultVal bool) bool {
	val, exists := os.LookupEnv(key)
	if !exists {
		return defaultVal
	}

	// Convert to lowercase for case-insensitive comparison
	val = strings.ToLower(val)

	// Check for truthy values
	if val == "true" || val == "yes" || val == "1" || val == "on" {
		return true
	}

	// Try parsing as int
	if intVal, err := strconv.Atoi(val); err == nil {
		return intVal != 0
	}

	return false
} 