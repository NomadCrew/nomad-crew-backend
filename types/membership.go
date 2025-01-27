package types

import "time"

type MemberRole string

const (
    MemberRoleNone   MemberRole = "NONE"
    MemberRoleOwner  MemberRole = "OWNER"
    MemberRoleMember MemberRole = "MEMBER"
)

type MembershipStatus string

const (
    MembershipStatusActive   MembershipStatus = "ACTIVE"
    MembershipStatusInactive MembershipStatus = "INACTIVE"
)

type TripMembership struct {
    ID        string           `json:"id"`
    TripID    string           `json:"tripId"`
    UserID    string           `json:"userId"`
    Role      MemberRole       `json:"role"`
    Status    MembershipStatus `json:"status"`
    CreatedAt time.Time        `json:"createdAt"`
    UpdatedAt time.Time        `json:"updatedAt"`
}
