package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAction_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		action   Action
		expected bool
	}{
		{"create is valid", ActionCreate, true},
		{"read is valid", ActionRead, true},
		{"update is valid", ActionUpdate, true},
		{"delete is valid", ActionDelete, true},
		{"invite is valid", ActionInvite, true},
		{"remove is valid", ActionRemove, true},
		{"change_role is valid", ActionChangeRole, true},
		{"leave is valid", ActionLeave, true},
		{"invalid action", Action("invalid"), false},
		{"empty action", Action(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.action.IsValid())
		})
	}
}

func TestResource_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		resource Resource
		expected bool
	}{
		{"trip is valid", ResourceTrip, true},
		{"member is valid", ResourceMember, true},
		{"invitation is valid", ResourceInvitation, true},
		{"chat is valid", ResourceChat, true},
		{"todo is valid", ResourceTodo, true},
		{"expense is valid", ResourceExpense, true},
		{"location is valid", ResourceLocation, true},
		{"invalid resource", Resource("invalid"), false},
		{"empty resource", Resource(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.resource.IsValid())
		})
	}
}

func TestCanPerform_TripPermissions(t *testing.T) {
	tests := []struct {
		name     string
		role     MemberRole
		action   Action
		expected bool
	}{
		// Trip READ - all members can read
		{"owner can read trip", MemberRoleOwner, ActionRead, true},
		{"admin can read trip", MemberRoleAdmin, ActionRead, true},
		{"member can read trip", MemberRoleMember, ActionRead, true},

		// Trip UPDATE - admin+ only
		{"owner can update trip", MemberRoleOwner, ActionUpdate, true},
		{"admin can update trip", MemberRoleAdmin, ActionUpdate, true},
		{"member cannot update trip", MemberRoleMember, ActionUpdate, false},

		// Trip DELETE - owner only
		{"owner can delete trip", MemberRoleOwner, ActionDelete, true},
		{"admin cannot delete trip", MemberRoleAdmin, ActionDelete, false},
		{"member cannot delete trip", MemberRoleMember, ActionDelete, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanPerform(tt.role, tt.action, ResourceTrip)
			assert.Equal(t, tt.expected, result, "Role %s, Action %s on Trip", tt.role, tt.action)
		})
	}
}

func TestCanPerform_MemberPermissions(t *testing.T) {
	tests := []struct {
		name     string
		role     MemberRole
		action   Action
		expected bool
	}{
		// Member READ - all members can see member list
		{"owner can read members", MemberRoleOwner, ActionRead, true},
		{"admin can read members", MemberRoleAdmin, ActionRead, true},
		{"member can read members", MemberRoleMember, ActionRead, true},

		// Member REMOVE - owner only
		{"owner can remove members", MemberRoleOwner, ActionRemove, true},
		{"admin cannot remove members", MemberRoleAdmin, ActionRemove, false},
		{"member cannot remove members", MemberRoleMember, ActionRemove, false},

		// Member CHANGE_ROLE - admin+
		{"owner can change roles", MemberRoleOwner, ActionChangeRole, true},
		{"admin can change roles", MemberRoleAdmin, ActionChangeRole, true},
		{"member cannot change roles", MemberRoleMember, ActionChangeRole, false},

		// Member LEAVE - all members can leave
		{"owner can leave", MemberRoleOwner, ActionLeave, true},
		{"admin can leave", MemberRoleAdmin, ActionLeave, true},
		{"member can leave", MemberRoleMember, ActionLeave, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanPerform(tt.role, tt.action, ResourceMember)
			assert.Equal(t, tt.expected, result, "Role %s, Action %s on Member", tt.role, tt.action)
		})
	}
}

func TestCanPerform_InvitationPermissions(t *testing.T) {
	tests := []struct {
		name     string
		role     MemberRole
		action   Action
		expected bool
	}{
		// Invitation CREATE - admin+ can invite
		{"owner can create invitations", MemberRoleOwner, ActionCreate, true},
		{"admin can create invitations", MemberRoleAdmin, ActionCreate, true},
		{"member cannot create invitations", MemberRoleMember, ActionCreate, false},

		// Invitation DELETE - admin+ can cancel
		{"owner can delete invitations", MemberRoleOwner, ActionDelete, true},
		{"admin can delete invitations", MemberRoleAdmin, ActionDelete, true},
		{"member cannot delete invitations", MemberRoleMember, ActionDelete, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanPerform(tt.role, tt.action, ResourceInvitation)
			assert.Equal(t, tt.expected, result, "Role %s, Action %s on Invitation", tt.role, tt.action)
		})
	}
}

func TestCanPerformWithOwnership_TodoPermissions(t *testing.T) {
	tests := []struct {
		name     string
		role     MemberRole
		action   Action
		isOwner  bool
		expected bool
	}{
		// Todo CREATE - all members can create
		{"owner can create todos", MemberRoleOwner, ActionCreate, false, true},
		{"admin can create todos", MemberRoleAdmin, ActionCreate, false, true},
		{"member can create todos", MemberRoleMember, ActionCreate, false, true},

		// Todo READ - all members can read
		{"owner can read todos", MemberRoleOwner, ActionRead, false, true},
		{"admin can read todos", MemberRoleAdmin, ActionRead, false, true},
		{"member can read todos", MemberRoleMember, ActionRead, false, true},

		// Todo UPDATE - admin+ can update any, member can update own
		{"owner can update any todo", MemberRoleOwner, ActionUpdate, false, true},
		{"admin can update any todo", MemberRoleAdmin, ActionUpdate, false, true},
		{"member can update own todo", MemberRoleMember, ActionUpdate, true, true},
		{"member cannot update others todo", MemberRoleMember, ActionUpdate, false, false},

		// Todo DELETE - admin+ can delete any, member can delete own
		{"owner can delete any todo", MemberRoleOwner, ActionDelete, false, true},
		{"admin can delete any todo", MemberRoleAdmin, ActionDelete, false, true},
		{"member can delete own todo", MemberRoleMember, ActionDelete, true, true},
		{"member cannot delete others todo", MemberRoleMember, ActionDelete, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanPerformWithOwnership(tt.role, tt.action, ResourceTodo, tt.isOwner)
			assert.Equal(t, tt.expected, result, "Role %s, Action %s on Todo, IsOwner %v", tt.role, tt.action, tt.isOwner)
		})
	}
}

func TestCanPerformWithOwnership_ChatPermissions(t *testing.T) {
	tests := []struct {
		name     string
		role     MemberRole
		action   Action
		isOwner  bool
		expected bool
	}{
		// Chat CREATE - all members can send messages
		{"owner can send messages", MemberRoleOwner, ActionCreate, false, true},
		{"admin can send messages", MemberRoleAdmin, ActionCreate, false, true},
		{"member can send messages", MemberRoleMember, ActionCreate, false, true},

		// Chat UPDATE - only message owner can edit
		{"owner can update own message", MemberRoleOwner, ActionUpdate, true, true},
		{"owner cannot update others message", MemberRoleOwner, ActionUpdate, false, false},
		{"admin can update own message", MemberRoleAdmin, ActionUpdate, true, true},
		{"admin cannot update others message", MemberRoleAdmin, ActionUpdate, false, false},
		{"member can update own message", MemberRoleMember, ActionUpdate, true, true},
		{"member cannot update others message", MemberRoleMember, ActionUpdate, false, false},

		// Chat DELETE - admin+ can delete any, member can delete own
		{"owner can delete any message", MemberRoleOwner, ActionDelete, false, true},
		{"admin can delete any message", MemberRoleAdmin, ActionDelete, false, true},
		{"member can delete own message", MemberRoleMember, ActionDelete, true, true},
		{"member cannot delete others message", MemberRoleMember, ActionDelete, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanPerformWithOwnership(tt.role, tt.action, ResourceChat, tt.isOwner)
			assert.Equal(t, tt.expected, result, "Role %s, Action %s on Chat, IsOwner %v", tt.role, tt.action, tt.isOwner)
		})
	}
}

func TestCanPerformWithOwnership_LocationPermissions(t *testing.T) {
	tests := []struct {
		name     string
		role     MemberRole
		action   Action
		isOwner  bool
		expected bool
	}{
		// Location - only own location can be updated/deleted
		{"owner can update own location", MemberRoleOwner, ActionUpdate, true, true},
		{"owner cannot update others location", MemberRoleOwner, ActionUpdate, false, false},
		{"admin can update own location", MemberRoleAdmin, ActionUpdate, true, true},
		{"member can update own location", MemberRoleMember, ActionUpdate, true, true},
		{"member cannot update others location", MemberRoleMember, ActionUpdate, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanPerformWithOwnership(tt.role, tt.action, ResourceLocation, tt.isOwner)
			assert.Equal(t, tt.expected, result, "Role %s, Action %s on Location, IsOwner %v", tt.role, tt.action, tt.isOwner)
		})
	}
}

func TestRequiresOwnership(t *testing.T) {
	tests := []struct {
		name     string
		resource Resource
		action   Action
		expected bool
	}{
		// Actions that require ownership checks
		{"todo update requires ownership", ResourceTodo, ActionUpdate, true},
		{"todo delete requires ownership", ResourceTodo, ActionDelete, true},
		{"chat update requires ownership", ResourceChat, ActionUpdate, true},
		{"chat delete requires ownership", ResourceChat, ActionDelete, true},
		{"location update requires ownership", ResourceLocation, ActionUpdate, true},
		{"location delete requires ownership", ResourceLocation, ActionDelete, true},

		// Actions that don't require ownership checks
		{"trip delete doesn't require ownership", ResourceTrip, ActionDelete, false},
		{"trip update doesn't require ownership", ResourceTrip, ActionUpdate, false},
		{"todo create doesn't require ownership", ResourceTodo, ActionCreate, false},
		{"member remove doesn't require ownership", ResourceMember, ActionRemove, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RequiresOwnership(tt.resource, tt.action)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetMinRoleForAction(t *testing.T) {
	tests := []struct {
		name         string
		resource     Resource
		action       Action
		expectedRole *MemberRole
	}{
		{"trip delete requires owner", ResourceTrip, ActionDelete, rolePtr(MemberRoleOwner)},
		{"trip update requires admin", ResourceTrip, ActionUpdate, rolePtr(MemberRoleAdmin)},
		{"trip read requires member", ResourceTrip, ActionRead, rolePtr(MemberRoleMember)},
		{"member remove requires owner", ResourceMember, ActionRemove, rolePtr(MemberRoleOwner)},
		{"invitation create requires admin", ResourceInvitation, ActionCreate, rolePtr(MemberRoleAdmin)},
		{"unknown action returns nil", ResourceTrip, Action("unknown"), nil},
		{"unknown resource returns nil", Resource("unknown"), ActionRead, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetMinRoleForAction(tt.resource, tt.action)
			if tt.expectedRole == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, *tt.expectedRole, *result)
			}
		})
	}
}

func TestGetPermissionRule(t *testing.T) {
	// Test existing rule
	rule := GetPermissionRule(ResourceTrip, ActionDelete)
	assert.NotNil(t, rule)
	assert.NotNil(t, rule.MinRole)
	assert.Equal(t, MemberRoleOwner, *rule.MinRole)
	assert.False(t, rule.OwnerOnly)
	assert.False(t, rule.OwnerOrMinRole)

	// Test rule with OwnerOrMinRole
	rule = GetPermissionRule(ResourceTodo, ActionDelete)
	assert.NotNil(t, rule)
	assert.True(t, rule.OwnerOrMinRole)

	// Test rule with OwnerOnly
	rule = GetPermissionRule(ResourceLocation, ActionUpdate)
	assert.NotNil(t, rule)
	assert.True(t, rule.OwnerOnly)

	// Test non-existent resource
	rule = GetPermissionRule(Resource("nonexistent"), ActionRead)
	assert.Nil(t, rule)

	// Test non-existent action
	rule = GetPermissionRule(ResourceTrip, Action("nonexistent"))
	assert.Nil(t, rule)
}

func TestCanPerform_InvalidInputs(t *testing.T) {
	// Invalid role
	result := CanPerform(MemberRole("invalid"), ActionRead, ResourceTrip)
	assert.False(t, result)

	// Invalid action
	result = CanPerform(MemberRoleOwner, Action("invalid"), ResourceTrip)
	assert.False(t, result)

	// Invalid resource
	result = CanPerform(MemberRoleOwner, ActionRead, Resource("invalid"))
	assert.False(t, result)
}
