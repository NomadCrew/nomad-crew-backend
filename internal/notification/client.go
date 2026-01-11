package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client represents a client for the notification facade API
type Client struct {
	apiURL     string
	apiKey     string
	httpClient *http.Client
}

// ClientOption is a function that configures the client
type ClientOption func(*Client)

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// NewClient creates a new notification client
func NewClient(apiURL, apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		apiURL: apiURL,
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Send sends a notification request to the facade API
func (c *Client) Send(ctx context.Context, req *Request) (*Response, error) {
	if err := c.validateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.apiURL
	if !isFullURL(c.apiURL) {
		url = fmt.Sprintf("%s/notify", c.apiURL)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var notifResp Response
	if err := json.NewDecoder(resp.Body).Decode(&notifResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if notifResp.Error != "" {
			return &notifResp, fmt.Errorf("notification failed with status %d: %s", resp.StatusCode, notifResp.Error)
		}
		return &notifResp, fmt.Errorf("notification failed with status %d", resp.StatusCode)
	}

	return &notifResp, nil
}

// SendAsync sends a notification asynchronously
func (c *Client) SendAsync(ctx context.Context, req *Request) <-chan error {
	errChan := make(chan error, 1)

	go func() {
		_, err := c.Send(ctx, req)
		errChan <- err
		close(errChan)
	}()

	return errChan
}

// SendBatch sends multiple notifications (currently sends them individually).
// NOTE: This is for the external notification API (NotificationFacadeService).
// The Expo push service (services/push_service.go) already has proper batching
// with MaxBatchSize=100 for actual push notification delivery.
// Enhancement: Implement batch endpoint when available in external notification API.
func (c *Client) SendBatch(ctx context.Context, requests []*Request) ([]error, error) {
	errors := make([]error, len(requests))
	
	for i, req := range requests {
		_, err := c.Send(ctx, req)
		errors[i] = err
	}
	
	return errors, nil
}

// validateRequest validates the notification request
func (c *Client) validateRequest(req *Request) error {
	if req.UserID == "" {
		return fmt.Errorf("userId is required")
	}

	if req.EventType == "" {
		return fmt.Errorf("eventType is required")
	}

	validEventTypes := map[EventType]bool{
		EventTypeTripUpdate:     true,
		EventTypeChatMessage:    true,
		EventTypeWeatherAlert:   true,
		EventTypeLocationUpdate: true,
		EventTypeSystemAlert:    true,
	}

	if !validEventTypes[req.EventType] {
		return fmt.Errorf("invalid eventType: %s", req.EventType)
	}

	if req.Priority != "" {
		validPriorities := map[Priority]bool{
			PriorityCritical: true,
			PriorityHigh:     true,
			PriorityMedium:   true,
			PriorityLow:      true,
		}

		if !validPriorities[req.Priority] {
			return fmt.Errorf("invalid priority: %s", req.Priority)
		}
	}

	if req.Data == nil {
		req.Data = make(map[string]interface{})
	}

	return nil
}

// Helper methods for creating typed notifications

// SendTripUpdate sends a trip update notification
func (c *Client) SendTripUpdate(ctx context.Context, userID string, data TripUpdateData, priority Priority) (*Response, error) {
	req := &Request{
		UserID:    userID,
		EventType: EventTypeTripUpdate,
		Priority:  priority,
		Data: map[string]interface{}{
			"tripId":      data.TripID,
			"tripName":    data.TripName,
			"message":     data.Message,
			"updateType":  data.UpdateType,
			"updatedBy":   data.UpdatedBy,
			"changesMade": data.ChangesMade,
		},
	}

	return c.Send(ctx, req)
}

// SendChatMessage sends a chat message notification
func (c *Client) SendChatMessage(ctx context.Context, userID string, data ChatMessageData) (*Response, error) {
	req := &Request{
		UserID:    userID,
		EventType: EventTypeChatMessage,
		Priority:  PriorityMedium,
		Data: map[string]interface{}{
			"tripId":         data.TripID,
			"chatId":         data.ChatID,
			"senderId":       data.SenderID,
			"senderName":     data.SenderName,
			"message":        data.Message,
			"messagePreview": data.MessagePreview,
		},
	}

	return c.Send(ctx, req)
}

// SendWeatherAlert sends a weather alert notification
func (c *Client) SendWeatherAlert(ctx context.Context, userID string, data WeatherAlertData, priority Priority) (*Response, error) {
	req := &Request{
		UserID:    userID,
		EventType: EventTypeWeatherAlert,
		Priority:  priority,
		Data: map[string]interface{}{
			"tripId":    data.TripID,
			"location":  data.Location,
			"alertType": data.AlertType,
			"message":   data.Message,
			"date":      data.Date,
			"severity":  data.Severity,
		},
	}

	return c.Send(ctx, req)
}

// SendLocationUpdate sends a location update notification
func (c *Client) SendLocationUpdate(ctx context.Context, userID string, data LocationUpdateData) (*Response, error) {
	req := &Request{
		UserID:    userID,
		EventType: EventTypeLocationUpdate,
		Priority:  PriorityLow,
		Data: map[string]interface{}{
			"tripId":       data.TripID,
			"sharedById":   data.SharedByID,
			"sharedByName": data.SharedByName,
			"location":     data.Location,
		},
	}

	return c.Send(ctx, req)
}

// SendSystemAlert sends a system alert notification
func (c *Client) SendSystemAlert(ctx context.Context, userID string, data SystemAlertData, priority Priority) (*Response, error) {
	req := &Request{
		UserID:    userID,
		EventType: EventTypeSystemAlert,
		Priority:  priority,
		Data: map[string]interface{}{
			"alertType":      data.AlertType,
			"message":        data.Message,
			"actionRequired": data.ActionRequired,
			"actionUrl":      data.ActionURL,
		},
	}

	return c.Send(ctx, req)
}

// MarkNotificationRead marks a notification as read
func (c *Client) MarkNotificationRead(ctx context.Context, notificationID, userID string) error {
	url := fmt.Sprintf("%s/notifications/%s/read", c.getBaseURL(), notificationID)
	
	reqBody := map[string]string{
		"userId": userID,
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	
	httpReq, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errMsg, ok := errResp["error"].(string); ok {
			return fmt.Errorf("failed to mark notification as read: %s", errMsg)
		}
		return fmt.Errorf("failed to mark notification as read with status %d", resp.StatusCode)
	}
	
	return nil
}

// GetUserNotifications retrieves notifications for a user
func (c *Client) GetUserNotifications(ctx context.Context, userID string, opts *NotificationQueryOptions) (*NotificationList, error) {
	url := fmt.Sprintf("%s/users/%s/notifications", c.getBaseURL(), userID)
	
	// Build query parameters
	params := "?"
	if opts != nil {
		if opts.Limit > 0 {
			params += fmt.Sprintf("limit=%d&", opts.Limit)
		}
		if opts.ReadStatus != "" {
			params += fmt.Sprintf("readStatus=%s&", opts.ReadStatus)
		}
		if opts.EventType != "" {
			params += fmt.Sprintf("eventType=%s&", opts.EventType)
		}
		if opts.LastKey != "" {
			params += fmt.Sprintf("lastKey=%s&", opts.LastKey)
		}
	}
	
	if params != "?" {
		url += params[:len(params)-1] // Remove trailing &
	}
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	httpReq.Header.Set("x-api-key", c.apiKey)
	
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	var notifList NotificationList
	if err := json.NewDecoder(resp.Body).Decode(&notifList); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get notifications with status %d", resp.StatusCode)
	}
	
	return &notifList, nil
}

// GetUserPreferences retrieves notification preferences for a user
func (c *Client) GetUserPreferences(ctx context.Context, userID string) (*UserPreferences, error) {
	url := fmt.Sprintf("%s/users/%s/preferences", c.getBaseURL(), userID)
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	httpReq.Header.Set("x-api-key", c.apiKey)
	
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	var prefsResp struct {
		UserID      string          `json:"userId"`
		Preferences UserPreferences `json:"preferences"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&prefsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get preferences with status %d", resp.StatusCode)
	}
	
	return &prefsResp.Preferences, nil
}

// UpdateUserPreferences updates notification preferences for a user
func (c *Client) UpdateUserPreferences(ctx context.Context, userID string, prefs *UserPreferences) error {
	url := fmt.Sprintf("%s/users/%s/preferences", c.getBaseURL(), userID)
	
	jsonData, err := json.Marshal(prefs)
	if err != nil {
		return fmt.Errorf("failed to marshal preferences: %w", err)
	}
	
	httpReq, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errMsg, ok := errResp["error"].(string); ok {
			return fmt.Errorf("failed to update preferences: %s", errMsg)
		}
		return fmt.Errorf("failed to update preferences with status %d", resp.StatusCode)
	}
	
	return nil
}

// isFullURL checks if the URL already contains the /notify path
func isFullURL(url string) bool {
	return strings.HasSuffix(url, "/notify")
}

// getBaseURL returns the base URL without /notify
func (c *Client) getBaseURL() string {
	if strings.HasSuffix(c.apiURL, "/notify") {
		return strings.TrimSuffix(c.apiURL, "/notify")
	}
	return c.apiURL
}