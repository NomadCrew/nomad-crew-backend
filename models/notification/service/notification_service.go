package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/internal/events"
	istore "github.com/NomadCrew/nomad-crew-backend/internal/store" // Import for internal store types
	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/NomadCrew/nomad-crew-backend/store" // Keep for NotificationStore and TripStore (old)
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NotificationService defines the interface for notification business logic.
type NotificationService interface {
	// CreateAndPublishNotification creates a notification, saves it, and publishes an event.
	CreateAndPublishNotification(ctx context.Context, userID uuid.UUID, notificationType string, metadataInput interface{}) (*models.Notification, error)
	GetNotifications(ctx context.Context, userID uuid.UUID, limit, offset int, status *bool) ([]models.Notification, error)
	MarkNotificationAsRead(ctx context.Context, userID, notificationID uuid.UUID) error
	MarkAllNotificationsAsRead(ctx context.Context, userID uuid.UUID) (int64, error)
	GetUnreadNotificationCount(ctx context.Context, userID uuid.UUID) (int64, error)
	// DeleteNotification removes a specific notification.
	DeleteNotification(ctx context.Context, userID, notificationID uuid.UUID) error
}

// notificationService implements NotificationService.
type notificationService struct {
	notificationStore store.NotificationStore // Remains old store.NotificationStore
	userStore         istore.UserStore        // Changed to internal/store.UserStore
	tripStore         store.TripStore         // Remains old store.TripStore
	eventPublisher    types.EventPublisher
	logger            *zap.Logger
}

// NewNotificationService creates a new NotificationService.
func NewNotificationService(ns store.NotificationStore, us istore.UserStore, ts store.TripStore, ep types.EventPublisher, logger *zap.Logger) NotificationService {
	return &notificationService{
		notificationStore: ns,
		userStore:         us, // us is now istore.UserStore
		tripStore:         ts,
		eventPublisher:    ep,
		logger:            logger.Named("NotificationService"),
	}
}

// CreateAndPublishNotification constructs, saves, and publishes an event for a notification.
func (s *notificationService) CreateAndPublishNotification(ctx context.Context, userID uuid.UUID, notificationType string, metadataInput interface{}) (*models.Notification, error) {
	log := s.logger.With(zap.String("userID", userID.String()), zap.String("type", notificationType))

	// 1. Marshal the specific metadata input to JSON
	metadataJSON, err := json.Marshal(metadataInput)
	if err != nil {
		log.Error("Failed to marshal notification metadata", zap.Any("metadataInput", metadataInput), zap.Error(err))
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// 2. Create the notification model
	notification := &models.Notification{
		UserID:    userID,
		Type:      notificationType,
		Metadata:  metadataJSON,
		IsRead:    false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 3. Save the notification to the store
	if err := s.notificationStore.Create(ctx, notification); err != nil {
		log.Error("Failed to save notification to store", zap.Error(err))
		return nil, fmt.Errorf("failed to save notification: %w", err)
	}

	log.Info("Notification created successfully", zap.String("notificationID", notification.ID.String()))

	// 4. Prepare and publish NotificationCreatedEvent using the centralized helper
	go func() {
		log := s.logger.With(zap.String("operation", "publishNotificationEvent"), zap.String("notificationID", notification.ID.String()))

		// Prepare the specific event payload structure
		eventPayloadData := types.NotificationCreatedEvent{
			Timestamp:      time.Now(), // Use current time for event payload timestamp
			NotificationID: notification.ID,
			UserID:         notification.UserID,
		}

		// Convert payload struct to map[string]interface{}
		var payloadMap map[string]interface{}
		eventPayloadJSON, err := json.Marshal(eventPayloadData)
		if err != nil {
			log.Error("Failed to marshal NotificationCreatedEvent payload for publishing", zap.Error(err))
			return
		}
		if err := json.Unmarshal(eventPayloadJSON, &payloadMap); err != nil {
			log.Error("Failed to unmarshal NotificationCreatedEvent payload into map for publishing", zap.Error(err))
			return
		}

		// Use UserID as the publishing scope/key since notifications are user-centric
		publishScopeID := notification.UserID.String()

		if pubErr := events.PublishEventWithContext(
			s.eventPublisher,
			context.Background(), // Use background context for async
			string(types.EventTypeNotificationCreated),
			publishScopeID,               // Use UserID as the scope identifier
			notification.UserID.String(), // UserID who triggered/owns the notification
			payloadMap,
			"NotificationService",
		); pubErr != nil {
			// Log error but don't fail the original operation
			log.Error("Failed to publish NotificationCreatedEvent", zap.String("scopeID", publishScopeID), zap.Error(pubErr))
		} else {
			log.Debug("Published NotificationCreatedEvent", zap.String("scopeID", publishScopeID))
		}
	}()

	return notification, nil
}

// Helper function example: Populate metadata and trigger notification creation/publish
// This logic should reside in the service responsible for the action (e.g., TripService).
// func (tripSvc *tripService) InviteUserToTrip(ctx context.Context, tripID, inviterID, invitedUserID uuid.UUID) error {
// 	 // ... perform invitation logic ...

// 	 // Fetch necessary details for notification
// 	 inviter, err := tripSvc.userStore.GetUserByID(ctx, inviterID)
// 	 if err != nil { /* handle error */ }
// 	 trip, err := tripSvc.tripStore.GetTripByID(ctx, tripID)
// 	 if err != nil { /* handle error */ }

// 	 metadata := models.TripInvitationMetadata{
// 	 	Type:        "TRIP_INVITATION",
// 	 	InviterID:   inviterID,
// 	 	InviterName: inviter.Name,
// 	 	TripID:      tripID,
// 	 	TripName:    trip.Name,
// 	 }

// 	 // Call the NotificationService to create and publish
// 	 _, err = tripSvc.notificationService.CreateAndPublishNotification(ctx, invitedUserID, metadata.Type, metadata)
// 	 if err != nil {
// 	 	 // Log error, maybe compensate, but potentially continue
// 	 	 tripSvc.logger.Error("Failed to create/publish trip invitation notification", zap.Error(err))
// 	 }

// 	 return nil // or original error if invitation logic failed
// }

// GetNotifications retrieves notifications for a user.
func (s *notificationService) GetNotifications(ctx context.Context, userID uuid.UUID, limit, offset int, status *bool) ([]models.Notification, error) {
	// Default limit if not provided or invalid
	if limit <= 0 || limit > 100 { // Set a max limit
		limit = 20
	}
	// Default offset
	if offset < 0 {
		offset = 0
	}

	notifications, err := s.notificationStore.GetByUser(ctx, userID, limit, offset, status)
	if err != nil {
		s.logger.Error("Failed to get notifications from store", zap.String("userID", userID.String()), zap.Error(err))
		return nil, fmt.Errorf("failed to get notifications: %w", err)
	}
	return notifications, nil
}

// MarkNotificationAsRead marks a single notification as read.
func (s *notificationService) MarkNotificationAsRead(ctx context.Context, userID, notificationID uuid.UUID) error {
	err := s.notificationStore.MarkRead(ctx, notificationID, userID)
	if err != nil {
		s.logger.Error("Failed to mark notification as read",
			zap.String("userID", userID.String()),
			zap.String("notificationID", notificationID.String()),
			zap.Error(err))

		if errors.Is(err, store.ErrNotFound) {
			return err
		}
		if errors.Is(err, store.ErrForbidden) {
			return err
		}
		return fmt.Errorf("failed to mark notification as read: %w", err)
	}
	s.logger.Info("Notification marked as read", zap.String("notificationID", notificationID.String()))
	return nil
}

// MarkAllNotificationsAsRead marks all of a user's notifications as read.
func (s *notificationService) MarkAllNotificationsAsRead(ctx context.Context, userID uuid.UUID) (int64, error) {
	affectedRows, err := s.notificationStore.MarkAllReadByUser(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to mark all notifications as read", zap.String("userID", userID.String()), zap.Error(err))
		return 0, fmt.Errorf("failed to mark all notifications as read: %w", err)
	}
	s.logger.Info("All notifications marked as read", zap.String("userID", userID.String()), zap.Int64("affectedRows", affectedRows))
	return affectedRows, nil
}

// GetUnreadNotificationCount retrieves the count of unread notifications.
func (s *notificationService) GetUnreadNotificationCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	count, err := s.notificationStore.GetUnreadCount(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get unread notification count", zap.String("userID", userID.String()), zap.Error(err))
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}
	return count, nil
}

// DeleteNotification removes a notification by its ID, ensuring the user owns it.
func (s *notificationService) DeleteNotification(ctx context.Context, userID, notificationID uuid.UUID) error {
	log := s.logger.With(zap.String("userID", userID.String()), zap.String("notificationID", notificationID.String()))

	err := s.notificationStore.Delete(ctx, notificationID, userID)
	if err != nil {
		log.Error("Failed to delete notification from store", zap.Error(err))

		// Propagate specific errors (NotFound, Forbidden) for the handler
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrForbidden) {
			return err
		}
		// Wrap generic errors
		return fmt.Errorf("failed to delete notification: %w", err)
	}

	log.Info("Notification deleted successfully")
	return nil
}
