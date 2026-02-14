package docs

import (
	"time"

	"github.com/NomadCrew/nomad-crew-backend/types"
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

// CreateTripRequest is used for Swagger documentation for creating a trip
// @Description Trip creation information
type CreateTripRequest struct {
	// The trip name
	Name string `json:"name" binding:"required" example:"Summer Vacation"`

	// Trip description
	Description string `json:"description,omitempty" example:"A relaxing trip to the beach"`

	// Trip destination information
	Destination DestinationInfo `json:"destination" binding:"required"`

	// Trip start date
	StartDate time.Time `json:"startDate" binding:"required" example:"2024-07-01T00:00:00Z"`

	// Trip end date
	EndDate time.Time `json:"endDate" binding:"required" example:"2024-07-10T00:00:00Z"`

	// Trip status (PLANNING, ACTIVE, COMPLETED, CANCELLED)
	// Defaults to PLANNING if not provided.
	Status string `json:"status,omitempty" example:"PLANNING"`
}

// TripUpdateRequest is used for Swagger documentation when updating a trip
// @Description Updatable trip fields. All fields are optional.
type TripUpdateRequest struct {
	// The new trip name
	Name *string `json:"name,omitempty" example:"Updated Summer Vacation"`

	// New trip description
	Description *string `json:"description,omitempty" example:"An even more relaxing trip"`

	// New trip destination information
	Destination *DestinationInfo `json:"destination,omitempty"`

	// New trip start date
	StartDate *time.Time `json:"startDate,omitempty" example:"2024-07-02T00:00:00Z"`

	// New trip end date
	EndDate *time.Time `json:"endDate,omitempty" example:"2024-07-11T00:00:00Z"`

	// New trip status (PLANNING, ACTIVE, COMPLETED, CANCELLED)
	Status string `json:"status,omitempty" example:"PLANNING"`
}

// UpdateTripStatusRequest is used for Swagger documentation when updating a trip's status
// @Description Request body for updating trip status
type UpdateTripStatusRequest struct {
	// The new trip status (e.g., PLANNING, ACTIVE, COMPLETED, CANCELLED)
	Status string `json:"status" binding:"required" example:"ACTIVE"`
}


// TripSearchRequest is used for Swagger documentation for searching trips
// @Description Criteria for searching trips. All fields are optional.
type TripSearchRequest struct {
	// Optional: User ID to filter trips by (admin capability or specific use cases)
	UserID *string `json:"userId,omitempty" example:"user-uuid-123"`

	// Optional: Filter by exact start date
	StartDate *time.Time `json:"startDate,omitempty" example:"2024-08-01T00:00:00Z"`
	// Optional: Filter by exact end date
	EndDate *time.Time `json:"endDate,omitempty" example:"2024-08-10T00:00:00Z"`

	// Optional: Filter by start date range (from this date)
	StartDateFrom *time.Time `json:"startDateFrom,omitempty" example:"2024-08-01T00:00:00Z"`
	// Optional: Filter by start date range (to this date)
	StartDateTo *time.Time `json:"startDateTo,omitempty" example:"2024-08-31T00:00:00Z"`

	// Optional: Limit number of results
	Limit *int `json:"limit,omitempty" example:"20"`
	// Optional: Offset for pagination
	Offset *int `json:"offset,omitempty" example:"0"`

	// Optional: Filter by destination (e.g., address, place name)
	Destination *string `json:"destination,omitempty" example:"Paris"`
}

// TripMemberResponse is used for Swagger documentation for trip member details
// @Description Detailed information about a trip member
type TripMemberResponse struct {
	ID     string `json:"id" example:"membership-uuid-123"`
	TripID string `json:"tripId" example:"trip-uuid-456"`
	UserID string `json:"userId" example:"user-uuid-789"`
	// Role of the member in the trip (e.g., OWNER, MEMBER, ADMIN)
	Role string `json:"role" example:"MEMBER"`
	// Status of the membership (e.g., ACTIVE, INACTIVE, INVITED)
	Status    string    `json:"status" example:"ACTIVE"`
	CreatedAt time.Time `json:"createdAt" example:"2023-01-01T10:00:00Z"`
	UpdatedAt time.Time `json:"updatedAt" example:"2023-01-01T11:00:00Z"`
}

// TripWithMembersResponse is used for Swagger documentation of a trip and its members
// @Description Detailed trip information along with a list of its members
type TripWithMembersResponse struct {
	Trip    TripResponse          `json:"trip"`
	Members []*TripMemberResponse `json:"members"`
}

// StatusResponse is a generic response for status messages
// @Description Generic status message response
type StatusResponse struct {
	Message string `json:"message" example:"Operation successful"`
}

// ImageResponse defines the structure for image details
// @Description Image details
type ImageResponse struct {
	ID         string    `json:"id" example:"image-uuid-123"`
	URL        string    `json:"url" example:"https://example.com/image.jpg"`
	UploadedAt time.Time `json:"uploadedAt" example:"2023-01-01T12:00:00Z"`
	// Add other fields like FileName, Size if available and relevant
}

// ImageUploadResponse defines the structure for response after image upload
// @Description Response after successful image upload
type ImageUploadResponse struct {
	ID  string `json:"id" example:"image-uuid-123"`
	URL string `json:"url" example:"https://example.com/image.jpg"`
}

// UserListResponse for paginated list of users
// @Description A paginated list of user profiles.
type UserListResponse struct {
	Users  []*types.UserProfile `json:"users"`
	Total  int                  `json:"total" example:"100"`
	Offset int                  `json:"offset" example:"0"`
	Limit  int                  `json:"limit" example:"20"`
}

// UserUpdateRequest for updating user profile
// @Description Fields for updating a user profile. All fields are optional.
type UserUpdateRequest struct {
	Username          *string `json:"username,omitempty" example:"new.john.doe"`
	FirstName         *string `json:"firstName,omitempty" example:"Jonathan"`
	LastName          *string `json:"lastName,omitempty" example:"Doer"`
	ProfilePictureURL *string `json:"profilePictureUrl,omitempty" example:"https://example.com/new_avatar.png"`
}

// UserPreferencesRequest for updating user preferences
// @Description A map of preference keys to their values.
// Example: {"theme": "dark", "notifications": {"email": true, "push": false}}
type UserPreferencesRequest map[string]interface{}

// ChatMessageUpdateRequest for updating chat message content
// @Description Request body for updating a chat message.
type ChatMessageUpdateRequest struct {
	Content string `json:"content" binding:"required" example:"Updated message content here."`
}

// ChatLastReadRequest for updating the last read message ID
// @Description Request to update the last read message ID for the user in a trip's chat.
type ChatLastReadRequest struct {
	LastReadMessageID string `json:"lastReadMessageId" binding:"required" example:"message-uuid-xyz"`
}

// MemberLocationListResponse for a list of member locations
// @Description A list of locations for trip members.
type MemberLocationListResponse struct {
	Locations []*types.MemberLocation `json:"locations"`
}

// TodoCreateRequest for creating a new todo item
// @Description Request body for creating a new todo. TripID is taken from the path.
type TodoCreateRequest struct {
	Text string `json:"text" binding:"required" example:"Buy groceries"`
}

// TodoUpdateRequest for updating an existing todo item
// @Description Fields for updating a todo. All fields are optional.
type TodoUpdateRequest struct {
	// New text for the todo item
	Text *string `json:"text,omitempty" example:"Buy milk and eggs"`
	// New status for the todo item (e.g., COMPLETE, INCOMPLETE)
	Status *string `json:"status,omitempty" example:"COMPLETE"` // Using string for example as TodoStatus is an alias
}

// TodoResponse for representing a todo item in API responses
// @Description Detailed information about a todo item.
type TodoResponse struct {
	ID     string `json:"id" example:"todo-uuid-123"`
	TripID string `json:"tripId" example:"trip-uuid-456"`
	Text   string `json:"text" example:"Buy groceries"`
	// Status of the todo item (e.g., COMPLETE, INCOMPLETE)
	Status    string    `json:"status" example:"INCOMPLETE"` // Using string for example
	CreatedBy string    `json:"createdBy" example:"user-uuid-789"`
	CreatedAt time.Time `json:"createdAt" example:"2023-10-26T10:00:00Z"`
	UpdatedAt time.Time `json:"updatedAt" example:"2023-10-26T10:05:00Z"`
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

// AddMemberRequest for adding a new member to a trip
// @Description Request body for adding a member to a trip.
type AddMemberRequest struct {
	// User ID of the member to add
	UserID string `json:"userId" binding:"required" example:"user-uuid-abc"`
	// Role to assign to the new member (e.g., MEMBER, ADMIN, OWNER)
	Role string `json:"role" binding:"required" example:"MEMBER"` // types.MemberRole is a string
}

// UpdateMemberRoleRequest for updating a member's role in a trip
// @Description Request body for updating a trip member's role.
type UpdateMemberRoleRequest struct {
	// New role for the member (e.g., MEMBER, ADMIN, OWNER)
	Role string `json:"role" binding:"required" example:"ADMIN"` // types.MemberRole is a string
}

// TripMemberDetailResponse for representing a trip member with their profile for API responses
// @Description Detailed information about a trip member, including their membership details and user profile.
type TripMemberDetailResponse struct {
	Membership TripMemberResponse `json:"membership"` // Reusing existing docs.TripMemberResponse for membership details
	User       types.UserResponse `json:"user"`       // Assuming types.UserResponse is suitable for direct use here
}

// ChatMessageReactionResponse for representing a chat message reaction in API responses
// @Description Detailed information about a chat message reaction.
type ChatMessageReactionResponse struct {
	ID        string `json:"id" example:"reaction-uuid-123"`
	MessageID string `json:"messageId" example:"message-uuid-456"`
	UserID    string `json:"userId" example:"user-uuid-789"`
	// The emoji character(s)
	Reaction  string    `json:"reaction" example:"👍"`
	CreatedAt time.Time `json:"createdAt" example:"2023-10-27T10:00:00Z"`
}
