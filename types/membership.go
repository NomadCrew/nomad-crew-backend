package types

import "time"

type MemberRole string

const (
	// MemberRoleNone   MemberRole = "NONE" // Removed as it's not in the DB ENUM
	MemberRoleOwner  MemberRole = "OWNER"
	MemberRoleMember MemberRole = "MEMBER"
	MemberRoleAdmin  MemberRole = "ADMIN"
)

type MembershipStatus string

const (
	MembershipStatusActive   MembershipStatus = "ACTIVE"
	MembershipStatusInactive MembershipStatus = "INACTIVE"
	// MembershipStatusInvited  MembershipStatus = "INVITED" // Removed as it's not in the DB ENUM
)

type TripMembership struct {
	ID        string           `json:"id"`
	TripID    string           `json:"tripId"`
	UserID    string           `json:"userId"`
	Role      MemberRole       `json:"role"`
	Status    MembershipStatus `json:"status"`
	CreatedAt time.Time        `json:"createdAt"`
	UpdatedAt time.Time        `json:"updatedAt"`
	DeletedAt *time.Time       `json:"deletedAt,omitempty"` // Added for soft delete
}

func (r MemberRole) IsAuthorizedFor(requiredRole MemberRole) bool {
	roleHierarchy := map[MemberRole]int{
		// MemberRoleNone:   0, // Removed
		MemberRoleMember: 1,
		MemberRoleAdmin:  2, // Assuming Admin has higher or equal privileges to Owner for some actions
		MemberRoleOwner:  2, // Owner and Admin can be at the same level or adjusted as per logic
	}

	currentLevel, ok := roleHierarchy[r]
	if !ok {
		return false
	}

	requiredLevel, ok := roleHierarchy[requiredRole]
	if !ok {
		return false // Or handle as an error, depending on desired behavior for unknown roles
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

// IsValid checks if the status is a valid membership status
func (ms MembershipStatus) IsValid() bool {
	switch ms {
	case MembershipStatusActive, MembershipStatusInactive:
		return true
	default:
		return false
	}
}
