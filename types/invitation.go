package types

import (
	"database/sql"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// InvitationStatus represents the status of a trip invitation
type InvitationStatus string

const (
	InvitationStatusPending  InvitationStatus = "PENDING"
	InvitationStatusAccepted InvitationStatus = "ACCEPTED"
	InvitationStatusDeclined InvitationStatus = "DECLINED"
)

// TripInvitation represents an invitation to join a trip
type TripInvitation struct {
	ID           string           `json:"id"`
	TripID       string           `json:"tripId"`
	InviterID    string           `json:"inviterId"`
	InviteeEmail string           `json:"inviteeEmail"`
	InviteeID    *string          `json:"inviteeId,omitempty"` // Optional, may be nil if user not in system yet
	Role         MemberRole       `json:"role"`
	Status       InvitationStatus `json:"status"`
	CreatedAt    time.Time        `json:"createdAt"`
	UpdatedAt    time.Time        `json:"updatedAt"`
	ExpiresAt    *time.Time       `json:"expiresAt,omitempty"`
	Token        sql.NullString   `json:"token,omitempty"` // Changed to sql.NullString
}

// InvitationClaims represents the data stored in a JWT for invitation links
type InvitationClaims struct {
	InvitationID string `json:"invitationId"`
	TripID       string `json:"tripId"`
	Email        string `json:"email"`
	InviteeEmail string `json:"inviteeEmail"`
	jwt.RegisteredClaims
}

// InvitationDetailsResponse defines the structure for detailed invitation responses
type InvitationDetailsResponse struct {
	ID          string           `json:"id"`
	TripID      string           `json:"tripId"`
	Email       string           `json:"email"` // Represents InviteeEmail
	Status      InvitationStatus `json:"status"`
	Role        MemberRole       `json:"role"`
	CreatedAt   time.Time        `json:"createdAt"`
	ExpiresAt   *time.Time       `json:"expiresAt,omitempty"`
	Trip        *TripBasicInfo   `json:"trip,omitempty"`    // Assumes TripBasicInfo will be defined
	Inviter     *UserResponse    `json:"inviter,omitempty"` // Assumes UserResponse is defined
	MemberCount int              `json:"memberCount"`
}
