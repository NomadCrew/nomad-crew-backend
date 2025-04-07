package store

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/google/uuid"
)

// UserStore defines the interface for user data operations.
// This is a basic definition; expand as needed.
type UserStore interface {
	// GetUserByID retrieves basic user details, like name, needed for notifications.
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	// Add other user-related data access methods here...
}
