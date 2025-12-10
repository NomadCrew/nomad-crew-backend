package store

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/google/uuid"
)

// NotificationStore defines the interface for notification data operations.
type NotificationStore interface {
	Create(ctx context.Context, notification *models.Notification) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Notification, error)
	// GetByUser retrieves notifications for a specific user, ordered by creation date descending.
	// It supports pagination using limit and offset, and filtering by read status.
	GetByUser(ctx context.Context, userID uuid.UUID, limit, offset int, status *bool) ([]models.Notification, error)
	MarkRead(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	MarkAllReadByUser(ctx context.Context, userID uuid.UUID) (int64, error) // Returns the number of rows affected
	GetUnreadCount(ctx context.Context, userID uuid.UUID) (int64, error)
	// Delete removes a notification by its ID, ensuring the operation is performed by the owner.
	Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	// DeleteAllByUser removes all notifications for a user. Returns count of deleted notifications.
	DeleteAllByUser(ctx context.Context, userID uuid.UUID) (int64, error)
}
