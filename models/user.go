package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user within the application's domain model,
// potentially distinct from external representations like SupabaseUser.
type User struct {
	ID                uuid.UUID `json:"id" db:"id"`
	SupabaseID        string    `json:"supabaseId" db:"supabase_id"`
	Username          string    `json:"username" db:"username"`
	FirstName         string    `json:"firstName,omitempty" db:"first_name"`
	LastName          string    `json:"lastName,omitempty" db:"last_name"`
	Email             string    `json:"email" db:"email"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
	ProfilePictureURL string    `json:"profilePictureUrl,omitempty" db:"profile_picture_url"`
	RawUserMetaData   []byte    `json:"-" db:"raw_user_meta_data"`
	LastSeenAt        time.Time `json:"lastSeenAt,omitempty" db:"last_seen_at"`
	IsOnline          bool      `json:"isOnline,omitempty" db:"is_online"`
	Preferences       []byte    `json:"-" db:"preferences"` // JSON data for user preferences
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
	return u.SupabaseID != "" && u.Email != "" && u.Username != ""
}

// ShouldSync returns true if this user should be synced with Supabase
func (u *User) ShouldSync() bool {
	// Check if the user was updated recently (last 24 hours)
	oneDayAgo := time.Now().Add(-24 * time.Hour)
	return u.UpdatedAt.Before(oneDayAgo) || !u.IsComplete()
}

// UserPreference represents a user preference setting
type UserPreference struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

// UserUpdateRequest represents a request to update user information
type UserUpdateRequest struct {
	Username          *string `json:"username,omitempty"`
	FirstName         *string `json:"firstName,omitempty"`
	LastName          *string `json:"lastName,omitempty"`
	ProfilePictureURL *string `json:"profilePictureUrl,omitempty"`
}
