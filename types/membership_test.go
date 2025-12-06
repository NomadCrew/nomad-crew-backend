package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemberRole_RoleLevel(t *testing.T) {
	tests := []struct {
		name          string
		role          MemberRole
		expectedLevel int
	}{
		{"owner has level 3", MemberRoleOwner, 3},
		{"admin has level 2", MemberRoleAdmin, 2},
		{"member has level 1", MemberRoleMember, 1},
		{"invalid role has level 0", MemberRole("INVALID"), 0},
		{"empty role has level 0", MemberRole(""), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.role.RoleLevel()
			assert.Equal(t, tt.expectedLevel, result)
		})
	}
}

func TestMemberRole_IsAuthorizedFor(t *testing.T) {
	tests := []struct {
		name             string
		currentRole      MemberRole
		requiredRole     MemberRole
		expectAuthorized bool
	}{
		// Owner tests
		{
			name:             "Owner can access Owner resources",
			currentRole:      MemberRoleOwner,
			requiredRole:     MemberRoleOwner,
			expectAuthorized: true,
		},
		{
			name:             "Owner can access Admin resources",
			currentRole:      MemberRoleOwner,
			requiredRole:     MemberRoleAdmin,
			expectAuthorized: true,
		},
		{
			name:             "Owner can access Member resources",
			currentRole:      MemberRoleOwner,
			requiredRole:     MemberRoleMember,
			expectAuthorized: true,
		},
		// Admin tests
		{
			name:             "Admin cannot access Owner resources",
			currentRole:      MemberRoleAdmin,
			requiredRole:     MemberRoleOwner,
			expectAuthorized: false,
		},
		{
			name:             "Admin can access Admin resources",
			currentRole:      MemberRoleAdmin,
			requiredRole:     MemberRoleAdmin,
			expectAuthorized: true,
		},
		{
			name:             "Admin can access Member resources",
			currentRole:      MemberRoleAdmin,
			requiredRole:     MemberRoleMember,
			expectAuthorized: true,
		},
		// Member tests
		{
			name:             "Member cannot access Owner resources",
			currentRole:      MemberRoleMember,
			requiredRole:     MemberRoleOwner,
			expectAuthorized: false,
		},
		{
			name:             "Member cannot access Admin resources",
			currentRole:      MemberRoleMember,
			requiredRole:     MemberRoleAdmin,
			expectAuthorized: false,
		},
		{
			name:             "Member can access Member resources",
			currentRole:      MemberRoleMember,
			requiredRole:     MemberRoleMember,
			expectAuthorized: true,
		},
		// Invalid role tests
		{
			name:             "Invalid role cannot access Member resources",
			currentRole:      "INVALID",
			requiredRole:     MemberRoleMember,
			expectAuthorized: false,
		},
		{
			name:             "Member cannot access invalid role",
			currentRole:      MemberRoleMember,
			requiredRole:     "INVALID",
			expectAuthorized: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.currentRole.IsAuthorizedFor(tt.requiredRole)
			assert.Equal(t, tt.expectAuthorized, result)
		})
	}
}

func TestMemberRole_IsOwner(t *testing.T) {
	assert.True(t, MemberRoleOwner.IsOwner())
	assert.False(t, MemberRoleAdmin.IsOwner())
	assert.False(t, MemberRoleMember.IsOwner())
	assert.False(t, MemberRole("INVALID").IsOwner())
}

func TestMemberRole_IsAdmin(t *testing.T) {
	// IsAdmin returns true for ADMIN or OWNER
	assert.True(t, MemberRoleOwner.IsAdmin())
	assert.True(t, MemberRoleAdmin.IsAdmin())
	assert.False(t, MemberRoleMember.IsAdmin())
	assert.False(t, MemberRole("INVALID").IsAdmin())
}

func TestMemberRole_IsMember(t *testing.T) {
	// IsMember returns true for any valid role
	assert.True(t, MemberRoleOwner.IsMember())
	assert.True(t, MemberRoleAdmin.IsMember())
	assert.True(t, MemberRoleMember.IsMember())
	assert.False(t, MemberRole("INVALID").IsMember())
}

func TestMemberRole_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		role     MemberRole
		expected bool
	}{
		{"owner is valid", MemberRoleOwner, true},
		{"admin is valid", MemberRoleAdmin, true},
		{"member is valid", MemberRoleMember, true},
		{"invalid role is not valid", MemberRole("INVALID"), false},
		{"empty role is not valid", MemberRole(""), false},
		{"lowercase is not valid", MemberRole("owner"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.role.IsValid())
		})
	}
}

func TestMembershipStatus_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		status   MembershipStatus
		expected bool
	}{
		{"active is valid", MembershipStatusActive, true},
		{"inactive is valid", MembershipStatusInactive, true},
		{"invalid status is not valid", MembershipStatus("INVALID"), false},
		{"empty status is not valid", MembershipStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.IsValid())
		})
	}
}

// TestRoleHierarchy verifies the OWNER > ADMIN > MEMBER hierarchy is correct
func TestRoleHierarchy(t *testing.T) {
	// OWNER has highest level
	assert.Greater(t, MemberRoleOwner.RoleLevel(), MemberRoleAdmin.RoleLevel())
	assert.Greater(t, MemberRoleOwner.RoleLevel(), MemberRoleMember.RoleLevel())

	// ADMIN is higher than MEMBER but lower than OWNER
	assert.Greater(t, MemberRoleAdmin.RoleLevel(), MemberRoleMember.RoleLevel())
	assert.Less(t, MemberRoleAdmin.RoleLevel(), MemberRoleOwner.RoleLevel())

	// MEMBER is lowest
	assert.Less(t, MemberRoleMember.RoleLevel(), MemberRoleAdmin.RoleLevel())
	assert.Less(t, MemberRoleMember.RoleLevel(), MemberRoleOwner.RoleLevel())
}
