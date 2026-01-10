package middleware

// contextKey defines a type for context keys to avoid collisions.
type contextKey string

// Defines context keys used within the application middleware and handlers.
const (
	// UserIDKey is the context key for the authenticated user's Supabase ID (string).
	// This is kept for any legacy compatibility during transition.
	UserIDKey contextKey = "userID"

	// AuthenticatedUserKey is the context key for the full authenticated user object (*models.User).
	// This provides access to the complete user information when needed.
	AuthenticatedUserKey contextKey = "authenticatedUser"

	// IsAdminKey is the context key for the authenticated user's system-wide admin status (bool).
	// Extracted from JWT app_metadata.is_admin. Defaults to false if not present.
	// This is different from trip-level MemberRoleAdmin.
	IsAdminKey contextKey = "isAdmin"
)
