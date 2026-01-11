package services

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/internal/notification"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"go.uber.org/zap"
)

// NotificationFacadeService handles sending notifications through the facade API
type NotificationFacadeService struct {
	client  *notification.Client
	enabled bool
	logger  *zap.SugaredLogger
}

// NewNotificationFacadeService creates a new notification service
func NewNotificationFacadeService(cfg *config.NotificationConfig) *NotificationFacadeService {
	log := logger.GetLogger()

	if !cfg.Enabled {
		log.Info("Notification service disabled")
		return &NotificationFacadeService{
			enabled: false,
			logger:  log,
		}
	}

	// Create HTTP client with custom timeout
	httpClient := &http.Client{
		Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
	}

	client := notification.NewClient(
		cfg.APIUrl,
		cfg.APIKey,
		notification.WithHTTPClient(httpClient),
	)

	return &NotificationFacadeService{
		client:  client,
		enabled: true,
		logger:  log,
	}
}

// IsEnabled returns whether notifications are enabled
func (s *NotificationFacadeService) IsEnabled() bool {
	return s.enabled
}

// SendTripUpdate sends a trip update notification to specified users
func (s *NotificationFacadeService) SendTripUpdate(ctx context.Context, userIDs []string, data notification.TripUpdateData, priority notification.Priority) error {
	if !s.enabled {
		s.logger.Debug("Notifications disabled, skipping trip update notification")
		return nil
	}

	errors := make([]error, 0)
	for _, userID := range userIDs {
		resp, err := s.client.SendTripUpdate(ctx, userID, data, priority)
		if err != nil {
			s.logger.Error("Failed to send trip update notification",
				"error", err,
				"userId", userID,
				"tripId", data.TripID,
			)
			errors = append(errors, fmt.Errorf("user %s: %w", userID, err))
		} else {
			s.logger.Info("Trip update notification sent",
				"notificationId", resp.NotificationID,
				"userId", userID,
				"tripId", data.TripID,
				"channels", resp.ChannelsUsed,
			)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to send notifications to %d users", len(errors))
	}

	return nil
}

// SendTripUpdateAsync sends trip update notifications asynchronously
func (s *NotificationFacadeService) SendTripUpdateAsync(ctx context.Context, userIDs []string, data notification.TripUpdateData, priority notification.Priority) {
	if !s.enabled {
		return
	}

	go func() {
		// Create a new context with timeout for async operation
		asyncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.SendTripUpdate(asyncCtx, userIDs, data, priority); err != nil {
			s.logger.Error("Async trip update notification failed", "error", err)
		}
	}()
}

// SendChatMessage sends a chat message notification
func (s *NotificationFacadeService) SendChatMessage(ctx context.Context, recipientIDs []string, data notification.ChatMessageData) error {
	if !s.enabled {
		s.logger.Debug("Notifications disabled, skipping chat message notification")
		return nil
	}

	errors := make([]error, 0)
	for _, userID := range recipientIDs {
		// Don't send notification to the sender
		if userID == data.SenderID {
			continue
		}

		resp, err := s.client.SendChatMessage(ctx, userID, data)
		if err != nil {
			s.logger.Error("Failed to send chat message notification",
				"error", err,
				"userId", userID,
				"chatId", data.ChatID,
			)
			errors = append(errors, fmt.Errorf("user %s: %w", userID, err))
		} else {
			s.logger.Info("Chat message notification sent",
				"notificationId", resp.NotificationID,
				"userId", userID,
				"chatId", data.ChatID,
				"channels", resp.ChannelsUsed,
			)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to send notifications to %d users", len(errors))
	}

	return nil
}

// SendChatMessageAsync sends chat message notifications asynchronously
func (s *NotificationFacadeService) SendChatMessageAsync(ctx context.Context, recipientIDs []string, data notification.ChatMessageData) {
	if !s.enabled {
		return
	}

	go func() {
		asyncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.SendChatMessage(asyncCtx, recipientIDs, data); err != nil {
			s.logger.Error("Async chat message notification failed", "error", err)
		}
	}()
}

// SendWeatherAlert sends weather alert notifications
func (s *NotificationFacadeService) SendWeatherAlert(ctx context.Context, userIDs []string, data notification.WeatherAlertData, priority notification.Priority) error {
	if !s.enabled {
		s.logger.Debug("Notifications disabled, skipping weather alert")
		return nil
	}

	errors := make([]error, 0)
	for _, userID := range userIDs {
		resp, err := s.client.SendWeatherAlert(ctx, userID, data, priority)
		if err != nil {
			s.logger.Error("Failed to send weather alert",
				"error", err,
				"userId", userID,
				"location", data.Location,
			)
			errors = append(errors, fmt.Errorf("user %s: %w", userID, err))
		} else {
			s.logger.Info("Weather alert sent",
				"notificationId", resp.NotificationID,
				"userId", userID,
				"location", data.Location,
				"channels", resp.ChannelsUsed,
			)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to send alerts to %d users", len(errors))
	}

	return nil
}

// SendLocationUpdate sends location update notifications
func (s *NotificationFacadeService) SendLocationUpdate(ctx context.Context, recipientIDs []string, data notification.LocationUpdateData) error {
	if !s.enabled {
		s.logger.Debug("Notifications disabled, skipping location update")
		return nil
	}

	errors := make([]error, 0)
	for _, userID := range recipientIDs {
		// Don't send notification to the person sharing location
		if userID == data.SharedByID {
			continue
		}

		resp, err := s.client.SendLocationUpdate(ctx, userID, data)
		if err != nil {
			s.logger.Error("Failed to send location update",
				"error", err,
				"userId", userID,
				"sharedBy", data.SharedByName,
			)
			errors = append(errors, fmt.Errorf("user %s: %w", userID, err))
		} else {
			s.logger.Info("Location update sent",
				"notificationId", resp.NotificationID,
				"userId", userID,
				"sharedBy", data.SharedByName,
				"channels", resp.ChannelsUsed,
			)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to send location updates to %d users", len(errors))
	}

	return nil
}

// SendSystemAlert sends system alert notifications
func (s *NotificationFacadeService) SendSystemAlert(ctx context.Context, userID string, data notification.SystemAlertData, priority notification.Priority) error {
	if !s.enabled {
		s.logger.Debug("Notifications disabled, skipping system alert")
		return nil
	}

	resp, err := s.client.SendSystemAlert(ctx, userID, data, priority)
	if err != nil {
		s.logger.Error("Failed to send system alert",
			"error", err,
			"userId", userID,
			"alertType", data.AlertType,
		)
		return fmt.Errorf("system alert failed: %w", err)
	}

	s.logger.Info("System alert sent",
		"notificationId", resp.NotificationID,
		"userId", userID,
		"alertType", data.AlertType,
		"channels", resp.ChannelsUsed,
	)

	return nil
}

// SendCustomNotification sends a custom notification
func (s *NotificationFacadeService) SendCustomNotification(ctx context.Context, userID string, eventType notification.EventType, priority notification.Priority, data map[string]interface{}) error {
	if !s.enabled {
		s.logger.Debug("Notifications disabled, skipping custom notification")
		return nil
	}

	req := &notification.Request{
		UserID:    userID,
		EventType: eventType,
		Priority:  priority,
		Data:      data,
	}

	resp, err := s.client.Send(ctx, req)
	if err != nil {
		s.logger.Error("Failed to send custom notification",
			"error", err,
			"userId", userID,
			"eventType", eventType,
		)
		return fmt.Errorf("custom notification failed: %w", err)
	}

	s.logger.Info("Custom notification sent",
		"notificationId", resp.NotificationID,
		"userId", userID,
		"eventType", eventType,
		"channels", resp.ChannelsUsed,
	)

	return nil
}