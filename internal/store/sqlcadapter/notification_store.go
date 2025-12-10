package sqlcadapter

import (
	"context"
	"errors"
	"fmt"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/sqlc"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/NomadCrew/nomad-crew-backend/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Ensure sqlcNotificationStore implements store.NotificationStore
var _ store.NotificationStore = (*sqlcNotificationStore)(nil)

type sqlcNotificationStore struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

// NewSqlcNotificationStore creates a new SQLC-based notification store
func NewSqlcNotificationStore(pool *pgxpool.Pool) store.NotificationStore {
	return &sqlcNotificationStore{
		pool:    pool,
		queries: sqlc.New(pool),
	}
}

// GetPool returns the underlying connection pool
func (s *sqlcNotificationStore) GetPool() *pgxpool.Pool {
	return s.pool
}

// Create inserts a new notification into the database
func (s *sqlcNotificationStore) Create(ctx context.Context, notification *models.Notification) error {
	log := logger.GetLogger()

	// Convert notification type from string to sqlc.NotificationType
	notificationType := sqlc.NotificationType(notification.Type)

	id, err := s.queries.CreateNotification(ctx, sqlc.CreateNotificationParams{
		UserID:   notification.UserID.String(),
		Type:     notificationType,
		Metadata: notification.Metadata,
	})
	if err != nil {
		log.Errorw("Failed to create notification", "error", err, "userID", notification.UserID)
		return fmt.Errorf("failed to create notification: %w", err)
	}

	// Parse the returned ID and set it on the notification
	parsedID, err := uuid.Parse(id)
	if err != nil {
		log.Errorw("Failed to parse notification ID", "id", id, "error", err)
		return fmt.Errorf("failed to parse notification ID: %w", err)
	}
	notification.ID = parsedID

	log.Infow("Successfully created notification", "notificationID", id, "userID", notification.UserID)
	return nil
}

// GetByID retrieves a notification by its ID
func (s *sqlcNotificationStore) GetByID(ctx context.Context, id uuid.UUID) (*models.Notification, error) {
	log := logger.GetLogger()

	notification, err := s.queries.GetNotification(ctx, id.String())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Warnw("Notification not found", "notificationID", id)
			return nil, apperrors.NotFound("notification", id.String())
		}
		log.Errorw("Failed to get notification", "notificationID", id, "error", err)
		return nil, fmt.Errorf("failed to get notification: %w", err)
	}

	return sqlcNotificationToModels(notification), nil
}

// GetByUser retrieves notifications for a specific user with pagination and optional status filtering
func (s *sqlcNotificationStore) GetByUser(ctx context.Context, userID uuid.UUID, limit, offset int, status *bool) ([]models.Notification, error) {
	log := logger.GetLogger()

	var notifications []*sqlc.Notification
	var err error

	// If status filter is provided and is false (unread only)
	if status != nil && !*status {
		notifications, err = s.queries.GetUnreadNotifications(ctx, userID.String())
		if err != nil {
			log.Errorw("Failed to get unread notifications", "userID", userID, "error", err)
			return nil, fmt.Errorf("failed to get unread notifications: %w", err)
		}
		// Apply pagination manually for unread notifications
		start := offset
		end := offset + limit
		if start > len(notifications) {
			notifications = []*sqlc.Notification{}
		} else {
			if end > len(notifications) {
				end = len(notifications)
			}
			notifications = notifications[start:end]
		}
	} else {
		// For all notifications or read-only, use GetUserNotifications with pagination
		notifications, err = s.queries.GetUserNotifications(ctx, sqlc.GetUserNotificationsParams{
			UserID: userID.String(),
			Limit:  int32(limit),
			Offset: int32(offset),
		})
		if err != nil {
			log.Errorw("Failed to get user notifications", "userID", userID, "error", err)
			return nil, fmt.Errorf("failed to get user notifications: %w", err)
		}

		// If status filter is provided and is true (read only), filter the results
		if status != nil && *status {
			filtered := []*sqlc.Notification{}
			for _, n := range notifications {
				if n.IsRead {
					filtered = append(filtered, n)
				}
			}
			notifications = filtered
		}
	}

	// Convert to models.Notification (note: returning slice, not slice of pointers)
	result := make([]models.Notification, 0, len(notifications))
	for _, n := range notifications {
		notification := sqlcNotificationToModels(n)
		if notification != nil {
			result = append(result, *notification)
		}
	}

	log.Infow("Successfully retrieved notifications for user", "userID", userID, "count", len(result))
	return result, nil
}

// MarkRead marks a single notification as read for a specific user
func (s *sqlcNotificationStore) MarkRead(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	log := logger.GetLogger()

	err := s.queries.MarkNotificationAsRead(ctx, sqlc.MarkNotificationAsReadParams{
		ID:     id.String(),
		UserID: userID.String(),
	})
	if err != nil {
		log.Errorw("Failed to mark notification as read", "notificationID", id, "userID", userID, "error", err)
		return fmt.Errorf("failed to mark notification as read: %w", err)
	}

	log.Infow("Successfully marked notification as read", "notificationID", id, "userID", userID)
	return nil
}

// MarkAllReadByUser marks all unread notifications as read for a specific user
func (s *sqlcNotificationStore) MarkAllReadByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	log := logger.GetLogger()

	// Get count before marking as read
	count, err := s.queries.CountUnreadNotifications(ctx, userID.String())
	if err != nil {
		log.Errorw("Failed to count unread notifications", "userID", userID, "error", err)
		return 0, fmt.Errorf("failed to count unread notifications: %w", err)
	}

	err = s.queries.MarkAllNotificationsRead(ctx, userID.String())
	if err != nil {
		log.Errorw("Failed to mark all notifications as read", "userID", userID, "error", err)
		return 0, fmt.Errorf("failed to mark all notifications as read: %w", err)
	}

	log.Infow("Successfully marked all notifications as read", "userID", userID, "count", count)
	return count, nil
}

// GetUnreadCount retrieves the count of unread notifications for a specific user
func (s *sqlcNotificationStore) GetUnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	log := logger.GetLogger()

	count, err := s.queries.CountUnreadNotifications(ctx, userID.String())
	if err != nil {
		log.Errorw("Failed to get unread notification count", "userID", userID, "error", err)
		return 0, fmt.Errorf("failed to get unread notification count: %w", err)
	}

	log.Infow("Successfully retrieved unread notification count", "userID", userID, "count", count)
	return count, nil
}

// Delete removes a notification by its ID, ensuring the operation is performed by the owner
func (s *sqlcNotificationStore) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	log := logger.GetLogger()

	err := s.queries.DeleteNotification(ctx, sqlc.DeleteNotificationParams{
		ID:     id.String(),
		UserID: userID.String(),
	})
	if err != nil {
		log.Errorw("Failed to delete notification", "notificationID", id, "userID", userID, "error", err)
		return fmt.Errorf("failed to delete notification: %w", err)
	}

	log.Infow("Successfully deleted notification", "notificationID", id, "userID", userID)
	return nil
}

// DeleteAllByUser removes all notifications for a specific user
func (s *sqlcNotificationStore) DeleteAllByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	log := logger.GetLogger()

	// Get count before deleting
	count, err := s.queries.CountAllNotifications(ctx, userID.String())
	if err != nil {
		log.Errorw("Failed to count notifications", "userID", userID, "error", err)
		return 0, fmt.Errorf("failed to count notifications: %w", err)
	}

	err = s.queries.DeleteAllNotificationsByUser(ctx, userID.String())
	if err != nil {
		log.Errorw("Failed to delete all notifications", "userID", userID, "error", err)
		return 0, fmt.Errorf("failed to delete all notifications: %w", err)
	}

	log.Infow("Successfully deleted all notifications", "userID", userID, "count", count)
	return count, nil
}

// sqlcNotificationToModels converts a SQLC notification to models.Notification
func sqlcNotificationToModels(n *sqlc.Notification) *models.Notification {
	if n == nil {
		return nil
	}

	// Parse UUID from string
	id, err := uuid.Parse(n.ID)
	if err != nil {
		logger.GetLogger().Errorw("Failed to parse notification ID", "id", n.ID, "error", err)
		return nil
	}

	userID, err := uuid.Parse(n.UserID)
	if err != nil {
		logger.GetLogger().Errorw("Failed to parse user ID", "userID", n.UserID, "error", err)
		return nil
	}

	result := &models.Notification{
		ID:       id,
		UserID:   userID,
		Type:     string(n.Type),
		Metadata: n.Metadata,
		IsRead:   n.IsRead,
	}

	if n.CreatedAt.Valid {
		result.CreatedAt = n.CreatedAt.Time
	}
	if n.UpdatedAt.Valid {
		result.UpdatedAt = n.UpdatedAt.Time
	}

	return result
}
