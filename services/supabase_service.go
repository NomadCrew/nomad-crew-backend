package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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
	SharingExpiresIn time.Duration `json:"sharing_expires_in,omitempty"` // Duration in seconds, converted from client-provided value
	Privacy          string        `json:"privacy,omitempty"`
}

// Sync data structures for minimal data synchronization
type UserSyncData struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

type TripSyncData struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	CreatedBy            string    `json:"created_by"`
	StartDate            time.Time `json:"start_date"`
	EndDate              time.Time `json:"end_date"`
	DestinationLatitude  float64   `json:"destination_latitude"`
	DestinationLongitude float64   `json:"destination_longitude"`
}

// TripMembershipSyncData represents the minimal membership data needed for Supabase RLS
type TripMembershipSyncData struct {
	TripID string `json:"trip_id"`
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

type MembershipSyncData struct {
	TripID string `json:"trip_id"`
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	Status string `json:"status"`
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

// Sync Methods for RLS Support

// SyncUser syncs minimal user data to Supabase for RLS validation
func (s *SupabaseService) SyncUser(ctx context.Context, userData UserSyncData) error {
	if !s.isEnabled {
		s.logger.Debug("Supabase sync disabled, skipping user sync")
		return nil
	}

	s.logger.Infow("Syncing user to Supabase", "userID", userData.ID, "username", userData.Username)

	payload := map[string]interface{}{
		"id":          userData.ID,
		"supabase_id": userData.ID,
		"email":       userData.Email,
		"username":    userData.Username,
	}

	return s.upsertToSupabase(ctx, "users", payload, "id")
}

// SyncTrip syncs minimal trip data to Supabase for RLS validation
func (s *SupabaseService) SyncTrip(ctx context.Context, tripData TripSyncData) error {
	if !s.isEnabled {
		s.logger.Debug("Supabase sync disabled, skipping trip sync")
		return nil
	}

	s.logger.Infow("Syncing trip to Supabase", "tripID", tripData.ID, "name", tripData.Name)

	payload := map[string]interface{}{
		"id":                    tripData.ID,
		"name":                  tripData.Name,
		"created_by":            tripData.CreatedBy,
		"start_date":            tripData.StartDate,
		"end_date":              tripData.EndDate,
		"destination_latitude":  tripData.DestinationLatitude,
		"destination_longitude": tripData.DestinationLongitude,
	}

	return s.upsertToSupabase(ctx, "trips", payload, "id")
}

// SyncMembership syncs trip membership data to Supabase for RLS validation
func (s *SupabaseService) SyncMembership(ctx context.Context, membershipData MembershipSyncData) error {
	if !s.isEnabled {
		s.logger.Debug("Supabase sync disabled, skipping membership sync")
		return nil
	}

	s.logger.Infow("Syncing membership to Supabase",
		"tripID", membershipData.TripID,
		"userID", membershipData.UserID,
		"role", membershipData.Role)

	payload := map[string]interface{}{
		"trip_id": membershipData.TripID,
		"user_id": membershipData.UserID,
		"role":    membershipData.Role,
		"status":  membershipData.Status,
	}

	return s.upsertToSupabase(ctx, "trip_memberships", payload, "trip_id,user_id")
}

// DeleteTrip removes trip data from Supabase
func (s *SupabaseService) DeleteTrip(ctx context.Context, tripID string) error {
	if !s.isEnabled {
		s.logger.Debug("Supabase sync disabled, skipping trip deletion")
		return nil
	}

	s.logger.Infow("Deleting trip from Supabase", "tripID", tripID)

	return s.deleteFromSupabase(ctx, "trips", "id", tripID)
}

// DeleteMembership removes membership data from Supabase
func (s *SupabaseService) DeleteMembership(ctx context.Context, tripID, userID string) error {
	if !s.isEnabled {
		s.logger.Debug("Supabase sync disabled, skipping membership deletion")
		return nil
	}

	s.logger.Infow("Deleting membership from Supabase", "tripID", tripID, "userID", userID)

	// Use the enhanced deleteFromSupabase method with multiple filters
	return s.deleteFromSupabaseWithFilters(ctx, "trip_memberships", map[string]string{
		"trip_id": tripID,
		"user_id": userID,
	})
}

// Helper method for upsert operations
func (s *SupabaseService) upsertToSupabase(ctx context.Context, table string, data map[string]interface{}, onConflict string) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	// Build URL with on_conflict parameter if provided
	upsertURL := fmt.Sprintf("%s/rest/v1/%s", s.supabaseURL, table)
	if onConflict != "" {
		params := url.Values{}
		params.Add("on_conflict", onConflict)
		upsertURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "POST", upsertURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Validate and clean the API key value (same logic as postToSupabase)
	cleanedKey, err := s.validateAndCleanAPIKey("upsertToSupabase")
	if err != nil {
		return err
	}

	req.Header.Set("Apikey", cleanedKey)
	req.Header.Set("Authorization", "Bearer "+cleanedKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "resolution=merge-duplicates")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		s.logger.Errorw("Failed to upsert to Supabase",
			"table", table,
			"status", resp.StatusCode,
			"response", string(body),
			"data", string(jsonData),
			"onConflict", onConflict)
		return fmt.Errorf("failed to upsert to %s: status %d", table, resp.StatusCode)
	}

	s.logger.Debugw("Successfully synced to Supabase", "table", table, "data", string(jsonData), "onConflict", onConflict)
	return nil
}

// Helper method for delete operations
func (s *SupabaseService) deleteFromSupabase(ctx context.Context, table, column, value string) error {
	return s.deleteFromSupabaseWithFilters(ctx, table, map[string]string{
		column: value,
	})
}

// Helper method for delete operations with multiple filters
func (s *SupabaseService) deleteFromSupabaseWithFilters(ctx context.Context, table string, filters map[string]string) error {
	// Build query parameters
	params := url.Values{}
	for column, value := range filters {
		params.Add(column, "eq."+value)
	}

	deleteURL := fmt.Sprintf("%s/rest/v1/%s?%s", s.supabaseURL, table, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "DELETE", deleteURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create DELETE request: %w", err)
	}

	// Validate and clean the API key value (same logic as postToSupabase)
	cleanedKey, err := s.validateAndCleanAPIKey("deleteFromSupabaseWithFilters")
	if err != nil {
		return err
	}

	req.Header.Set("Apikey", cleanedKey)
	req.Header.Set("Authorization", "Bearer "+cleanedKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send DELETE request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		s.logger.Errorw("Failed to delete from Supabase",
			"table", table,
			"status", resp.StatusCode,
			"response", string(body),
			"filters", filters)
		return fmt.Errorf("failed to delete from %s: status %d", table, resp.StatusCode)
	}

	s.logger.Debugw("Successfully deleted from Supabase", "table", table, "filters", filters)
	return nil
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
		"latitude":           update.Latitude,
		"longitude":          update.Longitude,
		"accuracy":           update.Accuracy,
		"is_sharing_enabled": update.SharingEnabled,
		"privacy":            update.Privacy,
		"timestamp":          time.Now(),
	}

	// Only include trip_id if it's not empty
	if update.TripID != "" {
		payload["trip_id"] = update.TripID
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

	// Validate and clean the API key value (same logic as postToSupabase)
	cleanedKey, err := s.validateAndCleanAPIKey("RemoveChatReaction")
	if err != nil {
		return err
	}

	req.Header.Set("Apikey", cleanedKey)
	req.Header.Set("Authorization", "Bearer "+cleanedKey)
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
	// Log function entry with payload data
	if s.logger != nil {
		s.logger.Errorw("DIAGNOSTIC: postToSupabase function entry",
			"table", table,
			"data_keys", func() []string {
				keys := make([]string, 0, len(data))
				for k := range data {
					keys = append(keys, k)
				}
				return keys
			}(),
			"payload_summary", func() string {
				jsonBytes, err := json.Marshal(data)
				if err != nil {
					return fmt.Sprintf("JSON marshal error: %v", err)
				}
				if len(jsonBytes) > 200 {
					return string(jsonBytes[:200]) + "..."
				}
				return string(jsonBytes)
			}())
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		if s.logger != nil {
			s.logger.Errorw("DIAGNOSTIC: JSON marshal failed",
				"error", err.Error(),
				"data", data)
		}
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if s.logger != nil {
		s.logger.Errorw("DIAGNOSTIC: JSON marshal successful",
			"json_length", len(jsonData))
	}

	url := fmt.Sprintf("%s/rest/v1/%s", s.supabaseURL, table)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		if s.logger != nil {
			s.logger.Errorw("DIAGNOSTIC: HTTP request creation failed",
				"error", err.Error(),
				"url", url)
		}
		return fmt.Errorf("failed to create request: %w", err)
	}

	if s.logger != nil {
		s.logger.Errorw("DIAGNOSTIC: HTTP request created successfully")
	}

	// PRODUCTION DIAGNOSTIC: Force ERROR level logging to appear in production logs
	if s.logger != nil {
		s.logger.Errorw("DIAGNOSTIC: Starting Supabase request validation",
			"table", table,
			"url", url)
	}

	cleanedKey, err := s.validateAndCleanAPIKey("postToSupabase")
	if err != nil {
		return err
	}

	if s.logger != nil {
		s.logger.Errorw("DIAGNOSTIC: Supabase key validation passed, setting headers",
			"cleaned_key_length", len(cleanedKey))
	}

	req.Header.Set("Apikey", cleanedKey)
	req.Header.Set("Authorization", "Bearer "+cleanedKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "resolution=merge-duplicates,return=minimal")

	if s.logger != nil {
		s.logger.Errorw("DIAGNOSTIC: Headers set, making HTTP request to Supabase")
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		if s.logger != nil {
			s.logger.Errorw("DIAGNOSTIC: HTTP request failed",
				"error", err.Error(),
				"error_type", fmt.Sprintf("%T", err))
		}
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if s.logger != nil {
		s.logger.Errorw("DIAGNOSTIC: HTTP request completed",
			"status_code", resp.StatusCode,
			"status", resp.Status)
	}

	if resp.StatusCode >= 300 {
		// Read response body with size limit to avoid huge payloads
		// Create a limited reader to cap the response size at 8KB
		limitedReader := &io.LimitedReader{R: resp.Body, N: 8192} // 8KB limit
		bodyBytes, err := io.ReadAll(limitedReader)

		errorDetails := "could not read response body"
		if err == nil && len(bodyBytes) > 0 {
			// Try to format as JSON if possible
			var prettyJSON bytes.Buffer
			if json.Indent(&prettyJSON, bodyBytes, "", "  ") == nil {
				errorDetails = prettyJSON.String()
			} else {
				errorDetails = string(bodyBytes)
			}
		}

		if s.logger != nil {
			s.logger.Errorw("DIAGNOSTIC: Supabase returned error status",
				"status_code", resp.StatusCode,
				"response_body", errorDetails)
		}

		return fmt.Errorf("Supabase returned status code %d: %s", resp.StatusCode, errorDetails)
	}

	if s.logger != nil {
		s.logger.Errorw("DIAGNOSTIC: Supabase request completed successfully")
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

// CheckTripExists verifies if a trip exists in Supabase trips table
func (s *SupabaseService) CheckTripExists(ctx context.Context, tripID string) (bool, error) {
	if !s.isEnabled {
		// If Supabase is disabled, assume trip exists locally
		return true, nil
	}

	if strings.TrimSpace(tripID) == "" {
		return false, fmt.Errorf("trip ID cannot be empty")
	}

	// Query the trips table to check if the trip exists
	queryURL := fmt.Sprintf("%s/rest/v1/trips?id=eq.%s&select=id", s.supabaseURL, url.QueryEscape(tripID))

	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	cleanedKey, err := s.validateAndCleanAPIKey("CheckTripExists")
	if err != nil {
		return false, err
	}

	req.Header.Set("Apikey", cleanedKey)
	req.Header.Set("Authorization", "Bearer "+cleanedKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("Supabase returned status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse JSON response - should be an array
	var trips []map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &trips); err != nil {
		return false, fmt.Errorf("failed to parse response JSON: %w", err)
	}

	exists := len(trips) > 0
	if s.logger != nil {
		s.logger.Infow("Trip existence check completed", "tripID", tripID, "exists", exists)
	}

	return exists, nil
}

// SyncTripImmediate performs immediate synchronous trip sync to Supabase
// This is used when we need to ensure a trip exists in Supabase before dependent operations
func (s *SupabaseService) SyncTripImmediate(ctx context.Context, tripData TripSyncData) error {
	if !s.isEnabled {
		return nil
	}

	if strings.TrimSpace(tripData.ID) == "" {
		return fmt.Errorf("trip ID cannot be empty")
	}

	// Sync trip data directly using the provided data
	if err := s.SyncTrip(ctx, tripData); err != nil {
		return fmt.Errorf("failed to sync trip to Supabase: %w", err)
	}

	s.logger.Infow("Successfully completed immediate trip sync", "tripID", tripData.ID)
	return nil
}

// validateAndCleanAPIKey validates and cleans the Supabase API key for HTTP headers
// Returns the cleaned key or an error if validation fails
func (s *SupabaseService) validateAndCleanAPIKey(methodName string) (string, error) {
	// Validate and clean the API key value
	cleanedKey := strings.TrimSpace(s.supabaseKey)
	if cleanedKey != s.supabaseKey {
		if s.logger != nil {
			s.logger.Errorw("DIAGNOSTIC: Supabase key had leading/trailing whitespace - TRIMMED",
				"method", methodName,
				"original_length", len(s.supabaseKey),
				"cleaned_length", len(cleanedKey))
		}
	}

	// Check for invalid characters that would cause header validation to fail
	for i, r := range cleanedKey {
		if r < 32 || r > 126 { // ASCII printable characters only
			if s.logger != nil {
				// Log more context around the invalid character
				contextStart := i - 5
				if contextStart < 0 {
					contextStart = 0
				}
				contextEnd := i + 6
				if contextEnd > len(cleanedKey) {
					contextEnd = len(cleanedKey)
				}
				s.logger.Errorw("DIAGNOSTIC: Invalid character found in Supabase key",
					"method", methodName,
					"position", i,
					"character_code", int(r),
					"character", string(r),
					"context_before", cleanedKey[contextStart:i],
					"context_after", cleanedKey[i+1:contextEnd],
					"total_key_length", len(cleanedKey))
			}
			return "", fmt.Errorf("invalid character in Supabase key at position %d: character code %d", i, int(r))
		}
	}

	// Add validation for empty key after cleaning
	if len(cleanedKey) == 0 {
		if s.logger != nil {
			s.logger.Errorw("DIAGNOSTIC: Supabase key is empty after cleaning",
				"method", methodName)
		}
		return "", fmt.Errorf("Supabase key is empty after cleaning")
	}

	return cleanedKey, nil
}
