package types

import "time"

// DeviceType represents the type of device for push notifications
type DeviceType string

const (
	DeviceTypeIOS     DeviceType = "ios"
	DeviceTypeAndroid DeviceType = "android"
)

// PushToken represents a user's push notification token
type PushToken struct {
	ID         string     `json:"id"`
	UserID     string     `json:"userId"`
	Token      string     `json:"token"`
	DeviceType DeviceType `json:"deviceType"`
	IsActive   bool       `json:"isActive"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
}

// RegisterPushTokenRequest is the request body for registering a push token
type RegisterPushTokenRequest struct {
	Token      string `json:"token" binding:"required"`
	DeviceType string `json:"deviceType" binding:"required,oneof=ios android"`
}

// DeregisterPushTokenRequest is the request body for deregistering a push token
type DeregisterPushTokenRequest struct {
	Token string `json:"token" binding:"required"`
}
