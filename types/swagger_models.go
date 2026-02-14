package types

import "time"

// This file contains Swagger documentation models.
// These types are used only in Swagger annotations and don't affect application logic.

// RawMessage is a placeholder for json.RawMessage to help Swagger
type RawMessage []byte

// NotificationResponse is used for Swagger documentation
// @Description Notification information
type NotificationResponse struct {
	ID        string     `json:"id" example:"a1b2c3d4-e5f6-7890-abcd-ef1234567890"`
	UserID    string     `json:"userId" example:"a1b2c3d4-e5f6-7890-abcd-ef1234567890"`
	Type      string     `json:"type" example:"TRIP_INVITATION"`
	Title     string     `json:"title" example:"New Trip Invitation"`
	Message   string     `json:"message" example:"You've been invited to join a trip to Paris"`
	Read      bool       `json:"read" example:"false"`
	CreatedAt time.Time  `json:"createdAt" example:"2023-01-01T00:00:00Z"`
	Metadata  RawMessage `json:"metadata,omitempty"`
}

// CreateTripRequest is used for Swagger documentation for creating a trip
// @Description Trip creation information
type CreateTripRequest struct {
	Name        string          `json:"name" binding:"required" example:"Summer Vacation"`
	Description string          `json:"description,omitempty" example:"A relaxing trip to the beach"`
	Destination DestinationInfo `json:"destination" binding:"required"`
	StartDate   time.Time       `json:"startDate" binding:"required" example:"2024-07-01T00:00:00Z"`
	EndDate     time.Time       `json:"endDate" binding:"required" example:"2024-07-10T00:00:00Z"`
	Status      string          `json:"status,omitempty" example:"PLANNING"`
}

// TripUpdateRequest is used for Swagger documentation when updating a trip
// @Description Updatable trip fields. All fields are optional.
type TripUpdateRequest struct {
	Name        *string          `json:"name,omitempty" example:"Updated Summer Vacation"`
	Description *string          `json:"description,omitempty" example:"An even more relaxing trip"`
	Destination *DestinationInfo `json:"destination,omitempty"`
	StartDate   *time.Time       `json:"startDate,omitempty" example:"2024-07-02T00:00:00Z"`
	EndDate     *time.Time       `json:"endDate,omitempty" example:"2024-07-11T00:00:00Z"`
	Status      string           `json:"status,omitempty" example:"PLANNING"`
}

// UpdateTripStatusRequest is used for Swagger documentation when updating a trip's status
// @Description Request body for updating trip status
type UpdateTripStatusRequest struct {
	Status string `json:"status" binding:"required" example:"ACTIVE"`
}

// TripSearchRequest is used for Swagger documentation for searching trips
// @Description Criteria for searching trips. All fields are optional.
type TripSearchRequest struct {
	UserID        *string    `json:"userId,omitempty" example:"user-uuid-123"`
	StartDate     *time.Time `json:"startDate,omitempty" example:"2024-08-01T00:00:00Z"`
	EndDate       *time.Time `json:"endDate,omitempty" example:"2024-08-10T00:00:00Z"`
	StartDateFrom *time.Time `json:"startDateFrom,omitempty" example:"2024-08-01T00:00:00Z"`
	StartDateTo   *time.Time `json:"startDateTo,omitempty" example:"2024-08-31T00:00:00Z"`
	Limit         *int       `json:"limit,omitempty" example:"20"`
	Offset        *int       `json:"offset,omitempty" example:"0"`
	Destination   *string    `json:"destination,omitempty" example:"Paris"`
}

// TripMemberResponse is used for Swagger documentation for trip member details
// @Description Detailed information about a trip member
type TripMemberResponse struct {
	ID        string    `json:"id" example:"membership-uuid-123"`
	TripID    string    `json:"tripId" example:"trip-uuid-456"`
	UserID    string    `json:"userId" example:"user-uuid-789"`
	Role      string    `json:"role" example:"MEMBER"`
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

// ImageResponse defines the structure for image details
// @Description Image details
type ImageResponse struct {
	ID         string    `json:"id" example:"image-uuid-123"`
	URL        string    `json:"url" example:"https://example.com/image.jpg"`
	UploadedAt time.Time `json:"uploadedAt" example:"2023-01-01T12:00:00Z"`
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
	Users  []*UserProfile `json:"users"`
	Total  int            `json:"total" example:"100"`
	Offset int            `json:"offset" example:"0"`
	Limit  int            `json:"limit" example:"20"`
}

// UserPreferencesRequest for updating user preferences
// @Description A map of preference keys to their values.
type UserPreferencesRequest map[string]interface{}

// MemberLocationListResponse for a list of member locations
// @Description A list of locations for trip members.
type MemberLocationListResponse struct {
	Locations []*MemberLocation `json:"locations"`
}

// TodoCreateRequest for creating a new todo item
// @Description Request body for creating a new todo. TripID is taken from the path.
type TodoCreateRequest struct {
	Text string `json:"text" binding:"required" example:"Buy groceries"`
}

// TodoUpdateRequest for updating an existing todo item
// @Description Fields for updating a todo. All fields are optional.
type TodoUpdateRequest struct {
	Text   *string `json:"text,omitempty" example:"Buy milk and eggs"`
	Status *string `json:"status,omitempty" example:"COMPLETE"`
}

// TodoResponse for representing a todo item in API responses
// @Description Detailed information about a todo item.
type TodoResponse struct {
	ID        string    `json:"id" example:"todo-uuid-123"`
	TripID    string    `json:"tripId" example:"trip-uuid-456"`
	Text      string    `json:"text" example:"Buy groceries"`
	Status    string    `json:"status" example:"INCOMPLETE"`
	CreatedBy string    `json:"createdBy" example:"user-uuid-789"`
	CreatedAt time.Time `json:"createdAt" example:"2023-10-26T10:00:00Z"`
	UpdatedAt time.Time `json:"updatedAt" example:"2023-10-26T10:05:00Z"`
}

// TripResponse is used for Swagger documentation
// @Description Trip information
type TripResponse struct {
	ID                 string          `json:"id" example:"a1b2c3d4-e5f6-7890-abcd-ef1234567890"`
	Name               string          `json:"name" example:"Trip to Paris"`
	Description        string          `json:"description" example:"A wonderful week in Paris"`
	Destination        DestinationInfo `json:"destination"`
	StartDate          time.Time       `json:"startDate" example:"2023-06-01T00:00:00Z"`
	EndDate            time.Time       `json:"endDate" example:"2023-06-07T00:00:00Z"`
	Status             string          `json:"status" example:"PLANNING"`
	CreatedBy          string          `json:"createdBy" example:"a1b2c3d4-e5f6-7890-abcd-ef1234567890"`
	CreatedAt          time.Time       `json:"createdAt" example:"2023-01-01T00:00:00Z"`
	UpdatedAt          time.Time       `json:"updatedAt" example:"2023-01-02T00:00:00Z"`
	BackgroundImageURL string          `json:"backgroundImageUrl,omitempty" example:"https://example.com/images/paris.jpg"`
}

// DestinationInfo is used for Swagger documentation
// @Description Destination information
type DestinationInfo struct {
	Address     string      `json:"address" example:"Paris, France"`
	PlaceID     string      `json:"placeId" example:"ChIJD7fiBh9u5kcRYJSMaMOCCwQ"`
	Coordinates Coordinates `json:"coordinates"`
}

// AddMemberRequest for adding a new member to a trip
// @Description Request body for adding a member to a trip.
type AddMemberRequest struct {
	UserID string `json:"userId" binding:"required" example:"user-uuid-abc"`
	Role   string `json:"role" binding:"required" example:"MEMBER"`
}

// UpdateMemberRoleRequest for updating a member's role in a trip
// @Description Request body for updating a trip member's role.
type UpdateMemberRoleRequest struct {
	Role string `json:"role" binding:"required" example:"ADMIN"`
}

// TripMemberDetailResponse for representing a trip member with their profile
// @Description Detailed information about a trip member, including their membership details and user profile.
type TripMemberDetailResponse struct {
	Membership TripMemberResponse `json:"membership"`
	User       UserResponse       `json:"user"`
}

// ChatMessageReactionResponse for representing a chat message reaction
// @Description Detailed information about a chat message reaction.
type ChatMessageReactionResponse struct {
	ID        string    `json:"id" example:"reaction-uuid-123"`
	MessageID string    `json:"messageId" example:"message-uuid-456"`
	UserID    string    `json:"userId" example:"user-uuid-789"`
	Reaction  string    `json:"reaction" example:"👍"`
	CreatedAt time.Time `json:"createdAt" example:"2023-10-27T10:00:00Z"`
}
