package service

import (
	"github.com/google/uuid"
)

// GetUserIDFromContext is moved to internal/utils
// generateEventID is moved to internal/utils

// getCurrentTime returns the current time in UTC - REMOVED as unused
/*
func getCurrentTime() time.Time {
	return time.Now().UTC()
}
*/

// Helper function to produce random strings for IDs
func RandomString(length int) string {
	return uuid.New().String()[:length]
}
