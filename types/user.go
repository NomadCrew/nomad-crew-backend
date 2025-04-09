package types

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
