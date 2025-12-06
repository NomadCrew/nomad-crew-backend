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

// RoleLevel returns the hierarchical level of a role.
// Higher level = more permissions.
// OWNER (3) > ADMIN (2) > MEMBER (1)
func (r MemberRole) RoleLevel() int {
	roleHierarchy := map[MemberRole]int{
		MemberRoleMember: 1,
		MemberRoleAdmin:  2,
		MemberRoleOwner:  3, // OWNER has highest privileges
	}

	level, ok := roleHierarchy[r]
	if !ok {
		return 0 // Unknown role has no privileges
	}
	return level
}

// IsAuthorizedFor checks if this role has sufficient privileges for the required role.
// Returns true if the current role's level is >= the required role's level.
func (r MemberRole) IsAuthorizedFor(requiredRole MemberRole) bool {
	currentLevel := r.RoleLevel()
	requiredLevel := requiredRole.RoleLevel()

	if currentLevel == 0 || requiredLevel == 0 {
		return false // Unknown role
	}

	return currentLevel >= requiredLevel
}

// IsOwner returns true if this role is OWNER.
func (r MemberRole) IsOwner() bool {
	return r == MemberRoleOwner
}

// IsAdmin returns true if this role is ADMIN or higher.
func (r MemberRole) IsAdmin() bool {
	return r == MemberRoleAdmin || r == MemberRoleOwner
}

// IsMember returns true if this role is a valid member role (any role).
func (r MemberRole) IsMember() bool {
	return r.IsValid()
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
