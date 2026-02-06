package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"go.uber.org/zap"
)

const (
	// ExpoPushURL is the Expo Push API endpoint
	ExpoPushURL = "https://exp.host/--/api/v2/push/send"

	// MaxBatchSize is the maximum number of notifications per request (Expo limit)
	MaxBatchSize = 100

	// HTTP client timeout
	pushTimeout = 30 * time.Second
)

// PushService handles sending push notifications via Expo
type PushService interface {
	// SendPushNotification sends a push notification to a single user
	SendPushNotification(ctx context.Context, userID string, notification *PushNotification) error

	// SendPushNotificationToUsers sends a push notification to multiple users
	SendPushNotificationToUsers(ctx context.Context, userIDs []string, notification *PushNotification) error

	// SendPushNotificationToToken sends directly to a specific token (for testing)
	SendPushNotificationToToken(ctx context.Context, token string, notification *PushNotification) error
}

// PushNotification represents a push notification payload
type PushNotification struct {
	Title    string                 `json:"title"`
	Body     string                 `json:"body"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Sound    string                 `json:"sound,omitempty"`    // "default" or custom sound
	Badge    *int                   `json:"badge,omitempty"`    // iOS badge count
	Priority string                 `json:"priority,omitempty"` // "default", "normal", "high"
	TTL      int                    `json:"ttl,omitempty"`      // Time to live in seconds
}

// ExpoMessage is the Expo push API message format
type ExpoMessage struct {
	To       string                 `json:"to"`
	Title    string                 `json:"title,omitempty"`
	Body     string                 `json:"body,omitempty"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Sound    string                 `json:"sound,omitempty"`
	Badge    *int                   `json:"badge,omitempty"`
	Priority string                 `json:"priority,omitempty"`
	TTLSec   int                    `json:"ttl,omitempty"`
}

// ExpoResponse represents the Expo Push API response
type ExpoResponse struct {
	Data []ExpoTicket `json:"data"`
}

// ExpoTicket represents a single push ticket from Expo
type ExpoTicket struct {
	Status  string          `json:"status"` // "ok" or "error"
	ID      string          `json:"id,omitempty"`
	Message string          `json:"message,omitempty"`
	Details *ExpoErrorDetails `json:"details,omitempty"`
}

// ExpoErrorDetails contains details about push errors
type ExpoErrorDetails struct {
	Error string `json:"error,omitempty"` // "DeviceNotRegistered", "InvalidCredentials", etc.
}

// expoPushService implements PushService
type expoPushService struct {
	pushTokenStore store.PushTokenStore
	httpClient     *http.Client
	logger         *zap.Logger
}

// NewExpoPushService creates a new Expo push notification service
func NewExpoPushService(pts store.PushTokenStore, logger *zap.Logger) PushService {
	return &expoPushService{
		pushTokenStore: pts,
		httpClient: &http.Client{
			Timeout: pushTimeout,
		},
		logger: logger.Named("ExpoPushService"),
	}
}

// SendPushNotification sends a push notification to a single user
func (s *expoPushService) SendPushNotification(ctx context.Context, userID string, notification *PushNotification) error {
	tokens, err := s.pushTokenStore.GetActiveTokensForUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get tokens for user %s: %w", userID, err)
	}

	if len(tokens) == 0 {
		s.logger.Debug("No active push tokens found for user", zap.String("userID", userID))
		return nil // Not an error - user just doesn't have push enabled
	}

	return s.sendToTokens(ctx, tokens, notification)
}

// SendPushNotificationToUsers sends a push notification to multiple users
func (s *expoPushService) SendPushNotificationToUsers(ctx context.Context, userIDs []string, notification *PushNotification) error {
	if len(userIDs) == 0 {
		return nil
	}

	tokens, err := s.pushTokenStore.GetActiveTokensForUsers(ctx, userIDs)
	if err != nil {
		return fmt.Errorf("failed to get tokens for users: %w", err)
	}

	if len(tokens) == 0 {
		s.logger.Debug("No active push tokens found for users", zap.Int("userCount", len(userIDs)))
		return nil
	}

	return s.sendToTokens(ctx, tokens, notification)
}

// SendPushNotificationToToken sends directly to a specific token
func (s *expoPushService) SendPushNotificationToToken(ctx context.Context, token string, notification *PushNotification) error {
	messages := []ExpoMessage{s.buildExpoMessage(token, notification)}
	return s.sendBatch(ctx, messages)
}

// sendToTokens sends notifications to all provided tokens
func (s *expoPushService) sendToTokens(ctx context.Context, tokens []*types.PushToken, notification *PushNotification) error {
	if len(tokens) == 0 {
		return nil
	}

	// Build messages for all tokens
	messages := make([]ExpoMessage, 0, len(tokens))
	for _, token := range tokens {
		messages = append(messages, s.buildExpoMessage(token.Token, notification))
	}

	// Send in batches
	for i := 0; i < len(messages); i += MaxBatchSize {
		end := i + MaxBatchSize
		if end > len(messages) {
			end = len(messages)
		}

		batch := messages[i:end]
		if err := s.sendBatch(ctx, batch); err != nil {
			s.logger.Error("Failed to send push notification batch",
				zap.Int("batchStart", i),
				zap.Int("batchEnd", end),
				zap.Error(err))
			// Continue with other batches even if one fails
		}
	}

	return nil
}

// buildExpoMessage constructs an Expo message from our notification format
func (s *expoPushService) buildExpoMessage(token string, notification *PushNotification) ExpoMessage {
	msg := ExpoMessage{
		To:    token,
		Title: notification.Title,
		Body:  notification.Body,
		Data:  notification.Data,
	}

	if notification.Sound != "" {
		msg.Sound = notification.Sound
	} else {
		msg.Sound = "default"
	}

	if notification.Badge != nil {
		msg.Badge = notification.Badge
	}

	if notification.Priority != "" {
		msg.Priority = notification.Priority
	} else {
		msg.Priority = "high"
	}

	if notification.TTL > 0 {
		msg.TTLSec = notification.TTL
	}

	return msg
}

// sendBatch sends a batch of messages to Expo's push API
func (s *expoPushService) sendBatch(ctx context.Context, messages []ExpoMessage) error {
	if len(messages) == 0 {
		return nil
	}

	body, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("failed to marshal messages: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ExpoPushURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send push notification: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		s.logger.Error("Expo push API returned non-OK status",
			zap.Int("statusCode", resp.StatusCode),
			zap.String("response", string(respBody)))
		return fmt.Errorf("expo push API returned status %d", resp.StatusCode)
	}

	// Parse response to check for individual ticket errors
	var expoResp ExpoResponse
	if err := json.Unmarshal(respBody, &expoResp); err != nil {
		s.logger.Warn("Failed to parse Expo response", zap.Error(err), zap.String("responseBody", string(respBody)))
		return nil // Don't fail if we can't parse, the push was likely successful
	}

	// Log the raw Expo response for debugging
	s.logger.Debug("Expo push response received",
		zap.Int("ticketCount", len(expoResp.Data)),
		zap.Int("messageCount", len(messages)))

	// Process tickets to handle errors
	s.processTickets(ctx, messages, expoResp.Data)

	return nil
}

// processTickets handles the response tickets from Expo
func (s *expoPushService) processTickets(ctx context.Context, messages []ExpoMessage, tickets []ExpoTicket) {
	var okCount, errCount int
	for i, ticket := range tickets {
		if i >= len(messages) {
			break
		}

		token := messages[i].To

		if ticket.Status == "error" {
			errCount++
			errorDetails := ""
			if ticket.Details != nil {
				errorDetails = ticket.Details.Error
			}
			s.logger.Warn("Push notification failed",
				zap.String("token", s.maskToken(token)),
				zap.String("status", ticket.Status),
				zap.String("message", ticket.Message),
				zap.String("errorDetails", errorDetails))

			// If the device is not registered, invalidate the token
			if ticket.Details != nil && ticket.Details.Error == "DeviceNotRegistered" {
				s.logger.Debug("Invalidating unregistered token", zap.String("token", s.maskToken(token)))
				if err := s.pushTokenStore.InvalidateToken(ctx, token); err != nil {
					s.logger.Error("Failed to invalidate token", zap.Error(err))
				}
			}
		} else if ticket.Status == "ok" {
			okCount++
			// Log successful ticket at debug level to avoid per-token noise
			s.logger.Debug("Push notification ticket successful",
				zap.String("token", s.maskToken(token)),
				zap.String("ticketId", ticket.ID))

			// Update last used timestamp for successful sends
			if err := s.pushTokenStore.UpdateTokenLastUsed(ctx, token); err != nil {
				s.logger.Warn("Failed to update token last used", zap.Error(err))
			}
		} else {
			// Log unexpected status
			s.logger.Warn("Unexpected push ticket status",
				zap.String("token", s.maskToken(token)),
				zap.String("status", ticket.Status),
				zap.String("message", ticket.Message))
		}
	}

	s.logger.Info("Push notification batch processed",
		zap.Int("total", len(tickets)),
		zap.Int("ok", okCount),
		zap.Int("errors", errCount))
}

// maskToken masks a token for logging (shows first and last few characters)
func (s *expoPushService) maskToken(token string) string {
	if len(token) <= 10 {
		return "***"
	}
	// Show first 8 and last 4 characters
	prefix := token[:8]
	suffix := token[len(token)-4:]
	masked := strings.Repeat("*", 8)
	return prefix + masked + suffix
}
