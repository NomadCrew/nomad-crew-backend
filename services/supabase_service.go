package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"go.uber.org/zap"
)

// ChatMessage is the payload structure for chat messages
type ChatMessage struct {
	ID        string    `json:"id"`
	TripID    string    `json:"trip_id"`
	UserID    string    `json:"user_id"`
	Message   string    `json:"message"`
	ReplyToID *string   `json:"reply_to_id,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// ChatReaction is the payload structure for chat reactions
type ChatReaction struct {
	MessageID string `json:"message_id"`
	UserID    string `json:"user_id"`
	Emoji     string `json:"emoji"`
}

// ReadReceipt represents the last message read by a user in a trip
type ReadReceipt struct {
	ID                string    `json:"id"`
	TripID            string    `json:"trip_id"`
	UserID            string    `json:"user_id"`
	LastReadMessageID string    `json:"last_read_message_id"`
	ReadAt            time.Time `json:"read_at"`
}

// UserPresence represents a user's online status
type UserPresence struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	TripID         *string   `json:"trip_id,omitempty"`
	Status         string    `json:"status"`
	LastSeen       time.Time `json:"last_seen"`
	IsTyping       bool      `json:"is_typing"`
	TypingInTripID *string   `json:"typing_in_trip_id,omitempty"`
}

// LocationUpdate is the payload structure for location updates
type LocationUpdate struct {
	TripID           string        `json:"trip_id,omitempty"`
	Latitude         float64       `json:"latitude"`
	Longitude        float64       `json:"longitude"`
	Accuracy         float32       `json:"accuracy"`
	SharingEnabled   bool          `json:"is_sharing_enabled"`
	SharingExpiresIn time.Duration `json:"sharing_expires_in,omitempty"`
	Privacy          string        `json:"privacy,omitempty"`
}

// OrderOpts contains options for ordering query results
type OrderOpts struct {
	Ascending bool
}

// SupabaseService provides integration with Supabase for realtime features
type SupabaseService struct {
	supabaseURL string
	supabaseKey string
	httpClient  *http.Client
	logger      *zap.SugaredLogger
	isEnabled   bool
}

// SupabaseServiceConfig contains configuration for the Supabase service
type SupabaseServiceConfig struct {
	IsEnabled   bool
	SupabaseURL string
	SupabaseKey string
}

// NewSupabaseService creates a new Supabase service instance
func NewSupabaseService(config SupabaseServiceConfig) *SupabaseService {
	return &SupabaseService{
		supabaseURL: config.SupabaseURL,
		supabaseKey: config.SupabaseKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger:    logger.GetLogger(),
		isEnabled: config.IsEnabled,
	}
}

// IsEnabled returns whether the Supabase integration is enabled
func (s *SupabaseService) IsEnabled() bool {
	return s.isEnabled
}

// UpdateLocation updates a user's location in Supabase
func (s *SupabaseService) UpdateLocation(ctx context.Context, userID string, update LocationUpdate) error {
	if !s.isEnabled {
		return nil
	}

	// Calculate expiration time if provided
	var expiresAt *time.Time
	if update.SharingExpiresIn > 0 {
		t := time.Now().Add(update.SharingExpiresIn)
		expiresAt = &t
	}

	// Prepare data for Supabase
	payload := map[string]interface{}{
		"user_id":            userID,
		"trip_id":            update.TripID,
		"latitude":           update.Latitude,
		"longitude":          update.Longitude,
		"accuracy":           update.Accuracy,
		"is_sharing_enabled": update.SharingEnabled,
		"privacy":            update.Privacy,
	}

	if expiresAt != nil {
		payload["sharing_expires_at"] = expiresAt
	}

	return s.postToSupabase(ctx, "locations", payload)
}

// SendChatMessage sends a chat message to Supabase
func (s *SupabaseService) SendChatMessage(ctx context.Context, msg ChatMessage) error {
	if !s.isEnabled {
		return nil
	}

	// Prepare data for Supabase
	payload := map[string]interface{}{
		"id":          msg.ID,
		"trip_id":     msg.TripID,
		"user_id":     msg.UserID,
		"message":     msg.Message,
		"reply_to_id": msg.ReplyToID,
		"created_at":  msg.CreatedAt,
	}

	return s.postToSupabase(ctx, "supabase_chat_messages", payload)
}

// AddChatReaction adds a reaction to a chat message in Supabase
func (s *SupabaseService) AddChatReaction(ctx context.Context, reaction ChatReaction) error {
	if !s.isEnabled {
		return nil
	}

	payload := map[string]interface{}{
		"message_id": reaction.MessageID,
		"user_id":    reaction.UserID,
		"emoji":      reaction.Emoji,
	}

	return s.postToSupabase(ctx, "supabase_chat_reactions", payload)
}

// RemoveChatReaction removes a reaction from a chat message in Supabase
func (s *SupabaseService) RemoveChatReaction(ctx context.Context, reaction ChatReaction) error {
	if !s.isEnabled {
		return nil
	}

	// Properly escape URL parameters to prevent injection and handle special characters
	messageID := url.QueryEscape(reaction.MessageID)
	userID := url.QueryEscape(reaction.UserID)
	emoji := url.QueryEscape(reaction.Emoji)

	deleteURL := fmt.Sprintf("%s/rest/v1/supabase_chat_reactions?message_id=eq.%s&user_id=eq.%s&emoji=eq.%s",
		s.supabaseURL, messageID, userID, emoji)

	req, err := http.NewRequestWithContext(ctx, "DELETE", deleteURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create DELETE request: %w", err)
	}

	req.Header.Set("apikey", s.supabaseKey)
	req.Header.Set("Authorization", "Bearer "+s.supabaseKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send DELETE request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("Supabase DELETE returned status code %d", resp.StatusCode)
	}

	return nil
}

// UpdatePresence updates a user's presence status in Supabase
func (s *SupabaseService) UpdatePresence(ctx context.Context, userID string, tripID string, isOnline bool) error {
	if !s.isEnabled {
		return nil
	}

	status := "online"
	if !isOnline {
		status = "offline"
	}

	payload := map[string]interface{}{
		"user_id":   userID,
		"trip_id":   tripID,
		"status":    status,
		"last_seen": time.Now(),
	}

	return s.postToSupabase(ctx, "supabase_user_presence", payload)
}

// UpdateTypingStatus updates a user's typing status in Supabase
func (s *SupabaseService) UpdateTypingStatus(ctx context.Context, userID string, tripID string, isTyping bool) error {
	if !s.isEnabled {
		return nil
	}

	payload := map[string]interface{}{
		"user_id":           userID,
		"trip_id":           tripID,
		"is_typing":         isTyping,
		"typing_in_trip_id": tripID,
		"last_seen":         time.Now(),
	}

	return s.postToSupabase(ctx, "supabase_user_presence", payload)
}

// postToSupabase sends data to Supabase REST API
func (s *SupabaseService) postToSupabase(ctx context.Context, table string, data map[string]interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	url := fmt.Sprintf("%s/rest/v1/%s", s.supabaseURL, table)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", s.supabaseKey)
	req.Header.Set("Authorization", "Bearer "+s.supabaseKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "resolution=merge-duplicates,return=minimal")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("Supabase returned status code %d", resp.StatusCode)
	}

	return nil
}

// GetChatHistory retrieves chat messages for a trip
func (s *SupabaseService) GetChatHistory(
	ctx context.Context,
	tripID string,
	limit int,
	before *time.Time,
) ([]ChatMessage, error) {
	// Implementation depends on the actual Supabase client
	// For now, return a placeholder to satisfy the interface
	return []ChatMessage{}, nil
}

// MarkMessagesAsRead updates read receipt
func (s *SupabaseService) MarkMessagesAsRead(
	ctx context.Context,
	tripID, userID, lastMessageID string,
) error {
	// Implementation depends on the actual Supabase client
	return nil
}

// AddReaction adds a reaction to a message
func (s *SupabaseService) AddReaction(
	ctx context.Context,
	messageID, userID, emoji string,
) error {
	// Delegate to the implemented AddChatReaction method
	return s.AddChatReaction(ctx, ChatReaction{
		MessageID: messageID,
		UserID:    userID,
		Emoji:     emoji,
	})
}

// RemoveReaction removes a reaction from a message
func (s *SupabaseService) RemoveReaction(
	ctx context.Context,
	messageID, userID, emoji string,
) error {
	// Delegate to the implemented RemoveChatReaction method
	return s.RemoveChatReaction(ctx, ChatReaction{
		MessageID: messageID,
		UserID:    userID,
		Emoji:     emoji,
	})
}

// GetTripMemberLocations fetches locations for members of a trip
func (s *SupabaseService) GetTripMemberLocations(
	ctx context.Context,
	tripID string,
) ([]map[string]interface{}, error) {
	// Implementation depends on the actual Supabase client
	return []map[string]interface{}{}, nil
}

// GetTripMemberPresence fetches presence information for members of a trip
func (s *SupabaseService) GetTripMemberPresence(
	ctx context.Context,
	tripID string,
) ([]UserPresence, error) {
	// Implementation depends on the actual Supabase client
	return []UserPresence{}, nil
}
