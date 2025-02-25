package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemberRole_IsAuthorizedFor(t *testing.T) {
	tests := []struct {
		name             string
		currentRole      MemberRole
		requiredRole     MemberRole
		expectAuthorized bool
	}{
		{
			name:             "Owner can access Owner resources",
			currentRole:      MemberRoleOwner,
			requiredRole:     MemberRoleOwner,
			expectAuthorized: true,
		},
		{
			name:             "Owner can access Member resources",
			currentRole:      MemberRoleOwner,
			requiredRole:     MemberRoleMember,
			expectAuthorized: true,
		},
		{
			name:             "Member cannot access Owner resources",
			currentRole:      MemberRoleMember,
			requiredRole:     MemberRoleOwner,
			expectAuthorized: false,
		},
		{
			name:             "Member can access Member resources",
			currentRole:      MemberRoleMember,
			requiredRole:     MemberRoleMember,
			expectAuthorized: true,
		},
		{
			name:             "None cannot access Member resources",
			currentRole:      MemberRoleNone,
			requiredRole:     MemberRoleMember,
			expectAuthorized: false,
		},
		{
			name:             "None cannot access Owner resources",
			currentRole:      MemberRoleNone,
			requiredRole:     MemberRoleOwner,
			expectAuthorized: false,
		},
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
