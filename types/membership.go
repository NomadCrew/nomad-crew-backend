package types

import "time"

type MemberRole string

const (
	MemberRoleNone   MemberRole = "NONE"
	MemberRoleOwner  MemberRole = "OWNER"
	MemberRoleMember MemberRole = "MEMBER"
	MemberRoleAdmin  MemberRole = "ADMIN"
)

type MembershipStatus string

const (
	MembershipStatusActive   MembershipStatus = "ACTIVE"
	MembershipStatusInactive MembershipStatus = "INACTIVE"
	MembershipStatusInvited  MembershipStatus = "INVITED"
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

func (r MemberRole) IsAuthorizedFor(requiredRole MemberRole) bool {
	roleHierarchy := map[MemberRole]int{
		MemberRoleNone:   0,
		MemberRoleMember: 1,
		MemberRoleOwner:  2,
	}

	currentLevel, ok := roleHierarchy[r]
	if !ok {
		return false
	}

	requiredLevel, ok := roleHierarchy[requiredRole]
	if !ok {
		return false
	}

	return currentLevel >= requiredLevel
}

func (r MemberRole) IsValid() bool {
	switch r {
	case MemberRoleOwner, MemberRoleAdmin, MemberRoleMember:
		return true
	}
	return false
}
