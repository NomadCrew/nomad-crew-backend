package types

import (
	"time"
)

// Location represents a user's geographic location at a specific point in time
type Location struct {
	ID        string    `json:"id"`
	TripID    string    `json:"tripId"`
	UserID    string    `json:"userId"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Accuracy  float64   `json:"accuracy"`
	Timestamp time.Time `json:"timestamp"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// LocationUpdate represents the payload for updating a user's location
type LocationUpdate struct {
	Latitude  float64 `json:"latitude" binding:"required"`
	Longitude float64 `json:"longitude" binding:"required"`
	Accuracy  float64 `json:"accuracy" binding:"required"`
	Timestamp int64   `json:"timestamp" binding:"required"` // Unix timestamp in milliseconds
}

// MemberLocation represents a trip member's location with additional user information
type MemberLocation struct {
	Location
	UserName string `json:"userName"`
	UserRole string `json:"userRole"`
}

// LocationServiceInterface defines the interface for location-related operations
type LocationServiceInterface interface {
	UpdateLocation(ctx interface{}, userID string, update LocationUpdate) (*Location, error)
	GetTripMemberLocations(ctx interface{}, tripID string) ([]MemberLocation, error)
}
