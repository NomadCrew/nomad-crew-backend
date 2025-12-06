package validation

import (
	"testing"

	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/stretchr/testify/assert"
)

func TestValidateRoleTransition_OwnerImmutable(t *testing.T) {
	tests := []struct {
		name        string
		oldRole     types.MemberRole
		newRole     types.MemberRole
		expectError bool
		errorCode   string
	}{
		// OWNER role cannot be changed
		{
			name:        "cannot change OWNER to ADMIN",
			oldRole:     types.MemberRoleOwner,
			newRole:     types.MemberRoleAdmin,
			expectError: true,
			errorCode:   "owner_immutable",
		},
		{
			name:        "cannot change OWNER to MEMBER",
			oldRole:     types.MemberRoleOwner,
			newRole:     types.MemberRoleMember,
			expectError: true,
			errorCode:   "owner_immutable",
		},
		// Cannot assign OWNER role
		{
			name:        "cannot promote ADMIN to OWNER",
			oldRole:     types.MemberRoleAdmin,
			newRole:     types.MemberRoleOwner,
			expectError: true,
			errorCode:   "owner_immutable",
		},
		{
			name:        "cannot promote MEMBER to OWNER",
			oldRole:     types.MemberRoleMember,
			newRole:     types.MemberRoleOwner,
			expectError: true,
			errorCode:   "owner_immutable",
		},
		// Valid transitions (ADMIN <-> MEMBER)
		{
			name:        "can promote MEMBER to ADMIN",
			oldRole:     types.MemberRoleMember,
			newRole:     types.MemberRoleAdmin,
			expectError: false,
		},
		{
			name:        "can demote ADMIN to MEMBER",
			oldRole:     types.MemberRoleAdmin,
			newRole:     types.MemberRoleMember,
			expectError: false,
		},
		// Same role transitions (no-op)
		{
			name:        "ADMIN to ADMIN is allowed",
			oldRole:     types.MemberRoleAdmin,
			newRole:     types.MemberRoleAdmin,
			expectError: false,
		},
		{
			name:        "MEMBER to MEMBER is allowed",
			oldRole:     types.MemberRoleMember,
			newRole:     types.MemberRoleMember,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRoleTransition(tt.oldRole, tt.newRole)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorCode)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateMembershipStatus(t *testing.T) {
	tests := []struct {
		name        string
		oldStatus   types.MembershipStatus
		newStatus   types.MembershipStatus
		expectError bool
	}{
		{
			name:        "ACTIVE to INACTIVE is allowed",
			oldStatus:   types.MembershipStatusActive,
			newStatus:   types.MembershipStatusInactive,
			expectError: false,
		},
		{
			name:        "INACTIVE to ACTIVE is allowed",
			oldStatus:   types.MembershipStatusInactive,
			newStatus:   types.MembershipStatusActive,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMembershipStatus(tt.oldStatus, tt.newStatus)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
