package notification

// EventType represents the type of notification event
type EventType string

const (
	EventTypeTripUpdate     EventType = "TRIP_UPDATE"
	EventTypeChatMessage    EventType = "CHAT_MESSAGE"
	EventTypeWeatherAlert   EventType = "WEATHER_ALERT"
	EventTypeLocationUpdate EventType = "LOCATION_UPDATE"
	EventTypeSystemAlert    EventType = "SYSTEM_ALERT"
)

// Priority represents the notification priority level
type Priority string

const (
	PriorityCritical Priority = "CRITICAL"
	PriorityHigh     Priority = "HIGH"
	PriorityMedium   Priority = "MEDIUM"
	PriorityLow      Priority = "LOW"
)

// Request represents a notification request to the facade API
type Request struct {
	UserID         string                 `json:"userId"`
	EventType      EventType              `json:"eventType"`
	Priority       Priority               `json:"priority,omitempty"`
	NotificationID string                 `json:"notificationId,omitempty"`
	Data           map[string]interface{} `json:"data"`
}

// Response represents the response from the notification facade API
type Response struct {
	NotificationID string   `json:"notificationId"`
	MessageID      string   `json:"messageId"`
	Status         string   `json:"status"`
	ChannelsUsed   []string `json:"channelsUsed"`
	Error          string   `json:"error,omitempty"`
}

// TripUpdateData represents data for trip update notifications
type TripUpdateData struct {
	TripID      string `json:"tripId"`
	TripName    string `json:"tripName"`
	Message     string `json:"message"`
	UpdateType  string `json:"updateType"`
	UpdatedBy   string `json:"updatedBy,omitempty"`
	ChangesMade string `json:"changesMade,omitempty"`
}

// ChatMessageData represents data for chat message notifications
type ChatMessageData struct {
	TripID         string `json:"tripId"`
	ChatID         string `json:"chatId"`
	SenderID       string `json:"senderId"`
	SenderName     string `json:"senderName"`
	Message        string `json:"message"`
	MessagePreview string `json:"messagePreview"`
}

// WeatherAlertData represents data for weather alert notifications
type WeatherAlertData struct {
	TripID    string `json:"tripId"`
	Location  string `json:"location"`
	AlertType string `json:"alertType"`
	Message   string `json:"message"`
	Date      string `json:"date"`
	Severity  string `json:"severity,omitempty"`
}

// LocationUpdateData represents data for location update notifications
type LocationUpdateData struct {
	TripID       string             `json:"tripId"`
	SharedByID   string             `json:"sharedById"`
	SharedByName string             `json:"sharedByName"`
	Location     LocationCoordinate `json:"location"`
}

// LocationCoordinate represents geographic coordinates
type LocationCoordinate struct {
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
	Name string  `json:"name,omitempty"`
}

// SystemAlertData represents data for system alert notifications
type SystemAlertData struct {
	AlertType      string `json:"alertType"`
	Message        string `json:"message"`
	ActionRequired bool   `json:"actionRequired"`
	ActionURL      string `json:"actionUrl,omitempty"`
}

// NotificationQueryOptions represents query options for retrieving notifications
type NotificationQueryOptions struct {
	Limit      int       `json:"limit,omitempty"`
	ReadStatus string    `json:"readStatus,omitempty"` // "READ" or "UNREAD"
	EventType  EventType `json:"eventType,omitempty"`
	LastKey    string    `json:"lastKey,omitempty"` // For pagination
}

// Notification represents a notification in the history
type Notification struct {
	NotificationID string                 `json:"notificationId"`
	EventType      EventType              `json:"eventType"`
	Priority       Priority               `json:"priority"`
	ReadStatus     string                 `json:"readStatus"`
	ReadAt         *string                `json:"readAt,omitempty"`
	Timestamp      string                 `json:"timestamp"`
	Data           map[string]interface{} `json:"data"`
	Channels       []string               `json:"channels"`
}

// NotificationList represents a list of notifications with pagination
type NotificationList struct {
	Notifications []Notification `json:"notifications"`
	Count         int            `json:"count"`
	UnreadCount   int            `json:"unreadCount,omitempty"`
	HasMore       bool           `json:"hasMore"`
	LastKey       string         `json:"lastKey,omitempty"`
}

// UserPreferences represents user notification preferences
type UserPreferences struct {
	Channels         map[string]ChannelPreference         `json:"channels"`
	QuietHours       QuietHoursConfig                     `json:"quietHours"`
	NotificationTypes map[EventType]NotificationTypeConfig `json:"notificationTypes"`
}

// ChannelPreference represents preferences for a notification channel
type ChannelPreference struct {
	Enabled bool `json:"enabled"`
}

// QuietHoursConfig represents quiet hours configuration
type QuietHoursConfig struct {
	Enabled bool   `json:"enabled"`
	Start   string `json:"start"` // HH:MM format
	End     string `json:"end"`   // HH:MM format
}

// NotificationTypeConfig represents preferences for a notification type
type NotificationTypeConfig struct {
	Enabled bool `json:"enabled"`
}