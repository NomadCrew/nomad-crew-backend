package utils

import (
	"context"
	"time"

	internal_errors "github.com/NomadCrew/nomad-crew-backend/internal/errors"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/google/uuid"
)

// GetUserIDFromContext extracts the user ID from the context
// Shared utility function for all services
func GetUserIDFromContext(ctx context.Context) (string, error) {
	// Check using the defined middleware key first
	if userID, ok := ctx.Value(middleware.UserIDKey).(string); ok && userID != "" {
		return userID, nil
	}

	// Optional: Keep checks for raw strings for backward compatibility if needed
	// // First check for "userID" (camelCase)
	// if userID, ok := ctx.Value("userID").(string); ok && userID != "" {
	// 	return userID, nil
	// }
	// // Then check for "user_id" (snake_case)
	// if userID, ok := ctx.Value("user_id").(string); ok && userID != "" {
	// 	return userID, nil
	// }

	return "", internal_errors.NewUnauthorizedError("User not authenticated (key missing or invalid type)")
}

// generateEventID creates a unique ID for events
func GenerateEventID() string {
	return time.Now().UTC().Format("20060102150405") + "-" + uuid.New().String()[:8]
}

// RandomString produces a random string of specified length
// Useful for generating IDs or tokens
func RandomString(length int) string {
	return uuid.New().String()[:length]
}
