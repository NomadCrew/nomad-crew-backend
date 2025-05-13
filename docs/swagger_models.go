package docs

import (
	"time"
)

// This file contains models used by Swagger documentation
// It doesn't affect the actual application logic, just documentation

// RawMessage is a placeholder for json.RawMessage to help Swagger
type RawMessage []byte

// NotificationResponse is used for Swagger documentation
// @Description Notification information
type NotificationResponse struct {
	// The notification ID
	ID string `json:"id" example:"a1b2c3d4-e5f6-7890-abcd-ef1234567890"`

	// The user ID this notification belongs to
	UserID string `json:"userId" example:"a1b2c3d4-e5f6-7890-abcd-ef1234567890"`

	// The notification type
	Type string `json:"type" example:"TRIP_INVITATION"`

	// The notification title
	Title string `json:"title" example:"New Trip Invitation"`

	// The notification message
	Message string `json:"message" example:"You've been invited to join a trip to Paris"`

	// Whether the notification has been read
	Read bool `json:"read" example:"false"`

	// When the notification was created
	CreatedAt time.Time `json:"createdAt" example:"2023-01-01T00:00:00Z"`

	// Additional data related to the notification (if any)
	Metadata RawMessage `json:"metadata,omitempty"`
}

// ErrorResponse represents an error response
// @Description Error information
type ErrorResponse struct {
	// Error code
	Code string `json:"code" example:"VALIDATION_ERROR"`

	// Error message
	Message string `json:"message" example:"Invalid request parameters"`

	// Detailed error information
	Error string `json:"error,omitempty" example:"Field 'name' is required"`
}

// TripResponse is used for Swagger documentation
// @Description Trip information
type TripResponse struct {
	// The trip ID
	ID string `json:"id" example:"a1b2c3d4-e5f6-7890-abcd-ef1234567890"`

	// The trip name
	Name string `json:"name" example:"Trip to Paris"`

	// Trip description
	Description string `json:"description" example:"A wonderful week in Paris"`

	// Trip destination information
	Destination DestinationInfo `json:"destination"`

	// Trip start date
	StartDate time.Time `json:"startDate" example:"2023-06-01T00:00:00Z"`

	// Trip end date
	EndDate time.Time `json:"endDate" example:"2023-06-07T00:00:00Z"`

	// Trip status (PLANNING, ACTIVE, COMPLETED, CANCELLED)
	Status string `json:"status" example:"PLANNING"`

	// ID of user who created the trip
	CreatedBy string `json:"createdBy" example:"a1b2c3d4-e5f6-7890-abcd-ef1234567890"`

	// Trip creation time
	CreatedAt time.Time `json:"createdAt" example:"2023-01-01T00:00:00Z"`

	// Trip last update time
	UpdatedAt time.Time `json:"updatedAt" example:"2023-01-02T00:00:00Z"`

	// URL to trip background image
	BackgroundImageURL string `json:"backgroundImageUrl,omitempty" example:"https://example.com/images/paris.jpg"`
}

// DestinationInfo is used for Swagger documentation
// @Description Destination information
type DestinationInfo struct {
	// Destination address
	Address string `json:"address" example:"Paris, France"`

	// Google Maps place ID
	PlaceID string `json:"placeId" example:"ChIJD7fiBh9u5kcRYJSMaMOCCwQ"`

	// Coordinates information
	Coordinates Coordinates `json:"coordinates"`
}

// Coordinates is used for Swagger documentation
// @Description Geographic coordinates
type Coordinates struct {
	// Latitude
	Lat float64 `json:"lat" example:"48.8566"`

	// Longitude
	Lng float64 `json:"lng" example:"2.3522"`
}
