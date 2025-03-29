package middleware

// contextKey defines a type for context keys to avoid collisions.
type contextKey string

// Defines context keys used within the application middleware and handlers.
const (
	// UserIDKey is the context key for the authenticated user's ID (string).
	UserIDKey contextKey = "userID"
	// UserRolesKey could be added here if roles are extracted during auth.
	// UserRolesKey contextKey = "userRoles"
	// AuthenticatedUserKey could hold the full user model.
	// AuthenticatedUserKey contextKey = "authenticatedUser"
)
