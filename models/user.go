package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user within the application's domain model,
// potentially distinct from external representations like SupabaseUser.
type User struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Username  string    `json:"username" db:"username"`
	FirstName string    `json:"firstName,omitempty" db:"first_name"`
	LastName  string    `json:"lastName,omitempty" db:"last_name"`
	Email     string    `json:"email" db:"email"` // Assuming email is also stored locally
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
	// Add other fields as necessary, e.g., ProfilePictureURL
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
