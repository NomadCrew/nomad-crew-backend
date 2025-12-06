package service

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/google/uuid"
)

// NotificationServiceInterface defines the contract for notification business logic.
// This interface allows for dependency injection and easier testing.
type NotificationServiceInterface interface {
	// CreateAndPublishNotification creates a notification, saves it, and publishes an event.
	CreateAndPublishNotification(ctx context.Context, userID uuid.UUID, notificationType string, metadataInput interface{}) (*models.Notification, error)

	// GetNotifications retrieves notifications for a user with pagination and optional status filter.
	GetNotifications(ctx context.Context, userID uuid.UUID, limit, offset int, status *bool) ([]models.Notification, error)

	// MarkNotificationAsRead marks a single notification as read.
	MarkNotificationAsRead(ctx context.Context, userID, notificationID uuid.UUID) error

	// MarkAllNotificationsAsRead marks all of a user's notifications as read.
	MarkAllNotificationsAsRead(ctx context.Context, userID uuid.UUID) (int64, error)

	// GetUnreadNotificationCount retrieves the count of unread notifications for a user.
	GetUnreadNotificationCount(ctx context.Context, userID uuid.UUID) (int64, error)

	// DeleteNotification removes a specific notification.
	DeleteNotification(ctx context.Context, userID, notificationID uuid.UUID) error
}
