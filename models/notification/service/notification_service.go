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
	"github.com/NomadCrew/nomad-crew-backend/services"
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
	// DeleteAllNotifications removes all notifications for a user.
	DeleteAllNotifications(ctx context.Context, userID uuid.UUID) (int64, error)
	// SetUserBroadcaster sets the broadcaster for delivering real-time notifications to connected users.
	SetUserBroadcaster(b UserBroadcaster)
}

// UserBroadcaster sends events directly to a specific user's WebSocket connection.
// This decouples the notification service from the concrete WebSocket hub implementation.
type UserBroadcaster interface {
	BroadcastToUser(userID string, event types.Event) error
}

// notificationService implements NotificationService.
type notificationService struct {
	notificationStore store.NotificationStore // Remains old store.NotificationStore
	userStore         istore.UserStore        // Changed to internal/store.UserStore
	tripStore         store.TripStore         // Remains old store.TripStore
	eventPublisher    types.EventPublisher
	pushService       services.PushService // Push notification service (optional)
	userBroadcaster   UserBroadcaster      // WebSocket broadcaster (optional)
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

// NewNotificationServiceWithPush creates a new NotificationService with push notification support.
func NewNotificationServiceWithPush(ns store.NotificationStore, us istore.UserStore, ts store.TripStore, ep types.EventPublisher, ps services.PushService, logger *zap.Logger) NotificationService {
	return &notificationService{
		notificationStore: ns,
		userStore:         us,
		tripStore:         ts,
		eventPublisher:    ep,
		pushService:       ps,
		logger:            logger.Named("NotificationService"),
	}
}

// SetUserBroadcaster sets the broadcaster for delivering real-time notifications.
func (s *notificationService) SetUserBroadcaster(b UserBroadcaster) {
	s.userBroadcaster = b
}

// buildNotificationDTO creates the frontend-expected notification shape from a models.Notification.
// This is the same transformation as the handler's toNotificationResponse but returns a map
// suitable for embedding in event payloads.
func buildNotificationDTO(n *models.Notification) map[string]interface{} {
	var metadata map[string]interface{}
	if len(n.Metadata) > 0 {
		if err := json.Unmarshal(n.Metadata, &metadata); err != nil {
			metadata = make(map[string]interface{})
		}
	} else {
		metadata = make(map[string]interface{})
	}

	return map[string]interface{}{
		"id":        n.ID.String(),
		"message":   generateNotificationMessage(n.Type, metadata),
		"read":      n.IsRead,
		"createdAt": n.CreatedAt.Format(time.RFC3339),
		"type":      n.Type,
		"metadata":  metadata,
	}
}

// generateNotificationMessage produces a human-readable message from the notification type and metadata.
func generateNotificationMessage(notifType string, metadata map[string]interface{}) string {
	tripName := getStringFromMap(metadata, "tripName", "a trip")

	switch notifType {
	case "TRIP_INVITATION", "TRIP_INVITATION_RECEIVED":
		inviterName := getStringFromMap(metadata, "inviterName", "Someone")
		return fmt.Sprintf("%s invited you to %s", inviterName, tripName)
	case "CHAT_MESSAGE", "NEW_CHAT_MESSAGE":
		senderName := getStringFromMap(metadata, "senderName", "Someone")
		return fmt.Sprintf("%s sent a message", senderName)
	case "MEMBER_ADDED", "TRIP_MEMBER_JOINED", "MEMBERSHIP_CHANGE":
		memberName := getStringFromMap(metadata, "addedUserName", getStringFromMap(metadata, "memberName", "A new member"))
		return fmt.Sprintf("%s joined %s", memberName, tripName)
	case "TRIP_UPDATE", "TRIP_UPDATED":
		return fmt.Sprintf("%s has been updated", tripName)
	case "TODO_ASSIGNED", "TASK_ASSIGNED":
		assignerName := getStringFromMap(metadata, "assignerName", "Someone")
		todoTitle := getStringFromMap(metadata, "todoTitle", "a task")
		return fmt.Sprintf("%s assigned you: %s", assignerName, todoTitle)
	case "TODO_COMPLETED", "TASK_COMPLETED":
		completerName := getStringFromMap(metadata, "completerName", "Someone")
		todoTitle := getStringFromMap(metadata, "todoTitle", "a task")
		return fmt.Sprintf("%s completed: %s", completerName, todoTitle)
	case "TRIP_MEMBER_LEFT":
		memberName := getStringFromMap(metadata, "memberName", "A member")
		return fmt.Sprintf("%s left %s", memberName, tripName)
	default:
		return "You have a new notification"
	}
}

// CreateAndPublishNotification constructs, saves, and publishes an event for a notification.
func (s *notificationService) CreateAndPublishNotification(ctx context.Context, userID uuid.UUID, notificationType string, metadataInput interface{}) (*models.Notification, error) {
	log := s.logger.With(zap.String("userID", userID.String()), zap.String("type", notificationType))

	// 1. Marshal the specific metadata input to JSON
	metadataJSON, err := json.Marshal(metadataInput)
	if err != nil {
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
		return nil, fmt.Errorf("failed to save notification: %w", err)
	}

	log.Info("Notification created successfully", zap.String("notificationID", notification.ID.String()))

	// 4. Build the frontend-expected DTO for the event payload
	notificationDTO := buildNotificationDTO(notification)

	// 5. Publish NotificationCreatedEvent via event system (for Redis/distributed listeners)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("Panic in publishNotificationEvent goroutine",
					zap.Any("panic", r),
					zap.String("notificationID", notification.ID.String()))
			}
		}()

		log := s.logger.With(zap.String("operation", "publishNotificationEvent"), zap.String("notificationID", notification.ID.String()))

		eventPayloadData := types.NotificationCreatedEvent{
			Timestamp:      time.Now(),
			NotificationID: notification.ID,
			UserID:         notification.UserID,
			Notification:   notificationDTO,
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
			publishScopeID,              // Use UserID as the scope identifier
			notification.UserID.String(), // UserID who triggered/owns the notification
			payloadMap,
			"NotificationService",
		); pubErr != nil {
			log.Error("Failed to publish NotificationCreatedEvent", zap.String("scopeID", publishScopeID), zap.Error(pubErr))
		} else {
			log.Debug("Published NotificationCreatedEvent", zap.String("scopeID", publishScopeID))
		}
	}()

	// 6. Broadcast directly to user's WebSocket connection (if connected)
	if s.userBroadcaster != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					s.logger.Error("Panic in broadcastToUser goroutine",
						zap.Any("panic", r),
						zap.String("notificationID", notification.ID.String()))
				}
			}()

			payloadJSON, err := json.Marshal(notificationDTO)
			if err != nil {
				s.logger.Error("Failed to marshal notification DTO for WebSocket broadcast",
					zap.Error(err),
					zap.String("notificationID", notification.ID.String()))
				return
			}

			wsEvent := types.Event{
				BaseEvent: types.BaseEvent{
					Type:      types.EventTypeNotificationCreated,
					UserID:    notification.UserID.String(),
					Timestamp: time.Now(),
					Version:   1,
				},
				Metadata: types.EventMetadata{
					Source: "NotificationService",
				},
				Payload: payloadJSON,
			}

			if err := s.userBroadcaster.BroadcastToUser(notification.UserID.String(), wsEvent); err != nil {
				s.logger.Warn("Failed to broadcast notification to user via WebSocket",
					zap.Error(err),
					zap.String("userID", notification.UserID.String()),
					zap.String("notificationID", notification.ID.String()))
			}
		}()
	}

	// 7. Send push notification if push service is configured
	if s.pushService != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					s.logger.Error("Panic in sendPushNotification goroutine",
						zap.Any("panic", r),
						zap.String("notificationID", notification.ID.String()),
						zap.String("userID", notification.UserID.String()))
				}
			}()
			s.sendPushNotification(notification, metadataInput)
		}()
	}

	return notification, nil
}

// sendPushNotification sends a push notification for the given notification
func (s *notificationService) sendPushNotification(notification *models.Notification, metadataInput interface{}) {
	log := s.logger.With(
		zap.String("operation", "sendPushNotification"),
		zap.String("notificationID", notification.ID.String()),
		zap.String("userID", notification.UserID.String()),
		zap.String("type", notification.Type),
	)

	// Build push notification content based on notification type
	pushNotification := s.buildPushNotification(notification.Type, metadataInput)
	if pushNotification == nil {
		log.Debug("No push notification configured for this type")
		return
	}

	// Add notification ID to data payload for deep linking
	if pushNotification.Data == nil {
		pushNotification.Data = make(map[string]interface{})
	}
	pushNotification.Data["notificationId"] = notification.ID.String()
	pushNotification.Data["type"] = notification.Type

	// Use background context for async push notification
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.pushService.SendPushNotification(ctx, notification.UserID.String(), pushNotification); err != nil {
		log.Error("Failed to send push notification", zap.Error(err))
	} else {
		log.Debug("Push notification sent successfully")
	}
}

// buildPushNotification creates a push notification based on the notification type
func (s *notificationService) buildPushNotification(notificationType string, metadataInput interface{}) *services.PushNotification {
	// Convert metadata to map for easier access
	var metadata map[string]interface{}
	if metadataInput != nil {
		data, err := json.Marshal(metadataInput)
		if err == nil {
			_ = json.Unmarshal(data, &metadata)
		}
	}

	switch notificationType {
	case "TRIP_INVITATION", "TRIP_INVITATION_RECEIVED":
		inviterName := getStringFromMap(metadata, "inviterName", "Someone")
		tripName := getStringFromMap(metadata, "tripName", "a trip")
		return &services.PushNotification{
			Title: "Trip Invitation",
			Body:  fmt.Sprintf("%s invited you to join %s", inviterName, tripName),
			Data:  metadata,
		}

	case "TRIP_MEMBER_JOINED":
		memberName := getStringFromMap(metadata, "memberName", "A member")
		tripName := getStringFromMap(metadata, "tripName", "your trip")
		return &services.PushNotification{
			Title: "New Member",
			Body:  fmt.Sprintf("%s joined %s", memberName, tripName),
			Data:  metadata,
		}

	case "TRIP_MEMBER_LEFT":
		memberName := getStringFromMap(metadata, "memberName", "A member")
		tripName := getStringFromMap(metadata, "tripName", "your trip")
		return &services.PushNotification{
			Title: "Member Left",
			Body:  fmt.Sprintf("%s left %s", memberName, tripName),
			Data:  metadata,
		}

	case "TRIP_UPDATED":
		tripName := getStringFromMap(metadata, "tripName", "A trip")
		return &services.PushNotification{
			Title: "Trip Updated",
			Body:  fmt.Sprintf("%s has been updated", tripName),
			Data:  metadata,
		}

	case "CHAT_MESSAGE":
		senderName := getStringFromMap(metadata, "senderName", "Someone")
		message := getStringFromMap(metadata, "preview", "sent a message")
		return &services.PushNotification{
			Title: senderName,
			Body:  message,
			Data:  metadata,
		}

	case "TODO_ASSIGNED":
		assignerName := getStringFromMap(metadata, "assignerName", "Someone")
		todoTitle := getStringFromMap(metadata, "todoTitle", "a task")
		return &services.PushNotification{
			Title: "Task Assigned",
			Body:  fmt.Sprintf("%s assigned you: %s", assignerName, todoTitle),
			Data:  metadata,
		}

	case "TODO_COMPLETED":
		completerName := getStringFromMap(metadata, "completerName", "Someone")
		todoTitle := getStringFromMap(metadata, "todoTitle", "a task")
		return &services.PushNotification{
			Title: "Task Completed",
			Body:  fmt.Sprintf("%s completed: %s", completerName, todoTitle),
			Data:  metadata,
		}

	default:
		// For unknown types, create a generic notification
		s.logger.Debug("Unknown notification type for push", zap.String("type", notificationType))
		return &services.PushNotification{
			Title: "NomadCrew",
			Body:  "You have a new notification",
			Data:  metadata,
		}
	}
}

// getStringFromMap safely extracts a string from a map
func getStringFromMap(m map[string]interface{}, key, defaultValue string) string {
	if m == nil {
		return defaultValue
	}
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

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
		return nil, fmt.Errorf("failed to get notifications: %w", err)
	}
	return notifications, nil
}

// MarkNotificationAsRead marks a single notification as read.
func (s *notificationService) MarkNotificationAsRead(ctx context.Context, userID, notificationID uuid.UUID) error {
	err := s.notificationStore.MarkRead(ctx, notificationID, userID)
	if err != nil {
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
		return 0, fmt.Errorf("failed to mark all notifications as read: %w", err)
	}
	s.logger.Info("All notifications marked as read", zap.String("userID", userID.String()), zap.Int64("affectedRows", affectedRows))
	return affectedRows, nil
}

// GetUnreadNotificationCount retrieves the count of unread notifications.
func (s *notificationService) GetUnreadNotificationCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	count, err := s.notificationStore.GetUnreadCount(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}
	return count, nil
}

// DeleteNotification removes a notification by its ID, ensuring the user owns it.
func (s *notificationService) DeleteNotification(ctx context.Context, userID, notificationID uuid.UUID) error {
	log := s.logger.With(zap.String("userID", userID.String()), zap.String("notificationID", notificationID.String()))

	err := s.notificationStore.Delete(ctx, notificationID, userID)
	if err != nil {
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

// DeleteAllNotifications removes all notifications for a user.
func (s *notificationService) DeleteAllNotifications(ctx context.Context, userID uuid.UUID) (int64, error) {
	deletedCount, err := s.notificationStore.DeleteAllByUser(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to delete all notifications: %w", err)
	}
	s.logger.Info("All notifications deleted", zap.String("userID", userID.String()), zap.Int64("deletedCount", deletedCount))
	return deletedCount, nil
}
