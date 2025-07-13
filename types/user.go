package types

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// SupabaseUser represents the minimal user information we get from Supabase
type SupabaseUser struct {
	ID           string       `json:"id"`
	Email        string       `json:"email"`
	UserMetadata UserMetadata `json:"user_metadata"`
}

// UserMetadata represents the custom user metadata stored in Supabase
type UserMetadata struct {
	Username       string `json:"username"`
	FirstName      string `json:"firstName,omitempty"`
	LastName       string `json:"lastName,omitempty"`
	ProfilePicture string `json:"avatar_url,omitempty"`
}

// UserResponse represents a simplified user object for API responses
type UserResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Username    string `json:"username"`
	FirstName   string `json:"firstName,omitempty"`
	LastName    string `json:"lastName,omitempty"`
	AvatarURL   string `json:"avatarUrl,omitempty"`   // Changed from ProfilePicture to match Supabase
	DisplayName string `json:"displayName,omitempty"` // Added for UI display purposes
}

// UserProfile represents the user profile information in the system
type UserProfile struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	Username    string    `json:"username"`
	FirstName   string    `json:"firstName,omitempty"`
	LastName    string    `json:"lastName,omitempty"`
	AvatarURL   string    `json:"avatarUrl,omitempty"`
	DisplayName string    `json:"displayName,omitempty"`
	LastSeenAt  time.Time `json:"lastSeenAt,omitempty"`
	IsOnline    bool      `json:"isOnline,omitempty"`
	// Add any additional profile fields as needed
}

// JWTClaims represents the claims structure for access tokens
type JWTClaims struct {
	UserID    string   `json:"sub"`
	Email     string   `json:"email,omitempty"`
	Username  string   `json:"username,omitempty"`
	Roles     []string `json:"roles,omitempty"`
	SessionID string   `json:"sid,omitempty"`
	jwt.RegisteredClaims
}

// User represents a user within the application
type User struct {
	ID                string                 `json:"id" db:"id"`
	Username          string                 `json:"username" db:"username"`
	FirstName         string                 `json:"firstName,omitempty" db:"first_name"`
	LastName          string                 `json:"lastName,omitempty" db:"last_name"`
	Email             string                 `json:"email" db:"email"`
	CreatedAt         time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at" db:"updated_at"`
	ProfilePictureURL string                 `json:"profilePictureUrl,omitempty" db:"profile_picture_url"`
	RawUserMetaData   []byte                 `json:"-" db:"raw_user_meta_data"`
	LastSeenAt        *time.Time             `json:"lastSeenAt,omitempty" db:"last_seen_at"`
	IsOnline          bool                   `json:"isOnline,omitempty" db:"is_online"`
	Preferences       map[string]interface{} `json:"preferences,omitempty" db:"-"` // JSON data for user preferences
}

// GetFullName returns the user's full name if available, otherwise username.
func (u *User) GetFullName() string {
	if u.FirstName != "" && u.LastName != "" {
		return u.FirstName + " " + u.LastName
	}
	if u.FirstName != "" {
		return u.FirstName
	}
	if u.LastName != "" {
		return u.LastName
	}
	return u.Username // Fallback to username
}

// GetDisplayName returns the user's display name for UI purposes
func (u *User) GetDisplayName() string {
	return u.GetFullName()
}

// IsComplete returns true if the user has all required fields populated
func (u *User) IsComplete() bool {
	return u.ID != "" && u.Email != "" && u.Username != ""
}

// ShouldSync returns true if this user should be synced with Supabase
func (u *User) ShouldSync() bool {
	// Check if the user was updated recently (last 24 hours)
	oneDayAgo := time.Now().Add(-24 * time.Hour)
	return u.UpdatedAt.Before(oneDayAgo) || !u.IsComplete()
}

// UserUpdateRequest represents a request to update user information
type UserUpdateRequest struct {
	Username          *string `json:"username,omitempty"`
	FirstName         *string `json:"firstName,omitempty"`
	LastName          *string `json:"lastName,omitempty"`
	ProfilePictureURL *string `json:"profilePictureUrl,omitempty"`
}

// CreateUserRequest represents the request to create a new user
type CreateUserRequest struct {
	Username          string                 `json:"username" binding:"required"`
	Email             string                 `json:"email" binding:"required,email"`
	FirstName         string                 `json:"first_name"`
	LastName          string                 `json:"last_name"`
	ProfilePictureURL string                 `json:"profile_picture_url"`
	Preferences       map[string]interface{} `json:"preferences"`
}
