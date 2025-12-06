package types

// PermissionRule defines the authorization rules for an action on a resource.
type PermissionRule struct {
	// MinRole is the minimum role required to perform the action.
	// If nil, the action is not allowed for any role.
	MinRole *MemberRole

	// OwnerOnly means only the resource owner can perform this action.
	// This is checked in addition to MinRole (if specified).
	OwnerOnly bool

	// OwnerOrMinRole means either the resource owner OR someone with MinRole can perform the action.
	// This enables "ADMIN can delete any, MEMBER can delete own" patterns.
	OwnerOrMinRole bool
}

// permissionMatrix defines what roles can perform what actions on which resources.
// Key: Resource -> Action -> PermissionRule
var permissionMatrix = map[Resource]map[Action]PermissionRule{
	ResourceTrip: {
		ActionCreate: {MinRole: rolePtr(MemberRoleMember)}, // Any authenticated user can create trips
		ActionRead:   {MinRole: rolePtr(MemberRoleMember)}, // Any trip member can read
		ActionUpdate: {MinRole: rolePtr(MemberRoleAdmin)},  // ADMIN+ can update trip details
		ActionDelete: {MinRole: rolePtr(MemberRoleOwner)},  // Only OWNER can delete trip
	},
	ResourceMember: {
		ActionRead:       {MinRole: rolePtr(MemberRoleMember)}, // Any member can see member list
		ActionCreate:     {MinRole: rolePtr(MemberRoleOwner)},  // Only OWNER can add members directly (use invitations instead)
		ActionRemove:     {MinRole: rolePtr(MemberRoleOwner)},  // Only OWNER can remove members
		ActionChangeRole: {MinRole: rolePtr(MemberRoleAdmin)},  // ADMIN+ can change roles (but not OWNER's role - checked separately)
		ActionLeave:      {MinRole: rolePtr(MemberRoleMember)}, // Any member can leave
	},
	ResourceInvitation: {
		ActionCreate: {MinRole: rolePtr(MemberRoleAdmin)},  // ADMIN+ can send invitations
		ActionRead:   {MinRole: rolePtr(MemberRoleAdmin)},  // ADMIN+ can view invitations
		ActionDelete: {MinRole: rolePtr(MemberRoleAdmin)},  // ADMIN+ can cancel invitations
	},
	ResourceChat: {
		ActionCreate: {MinRole: rolePtr(MemberRoleMember)},                              // Any member can send messages
		ActionRead:   {MinRole: rolePtr(MemberRoleMember)},                              // Any member can read messages
		ActionUpdate: {MinRole: rolePtr(MemberRoleMember), OwnerOnly: true},             // Only message owner can edit
		ActionDelete: {MinRole: rolePtr(MemberRoleAdmin), OwnerOrMinRole: true},         // ADMIN+ can delete any, MEMBER can delete own
	},
	ResourceTodo: {
		ActionCreate: {MinRole: rolePtr(MemberRoleMember)},                              // Any member can create todos
		ActionRead:   {MinRole: rolePtr(MemberRoleMember)},                              // Any member can read todos
		ActionUpdate: {MinRole: rolePtr(MemberRoleAdmin), OwnerOrMinRole: true},         // ADMIN+ can update any, MEMBER can update own
		ActionDelete: {MinRole: rolePtr(MemberRoleAdmin), OwnerOrMinRole: true},         // ADMIN+ can delete any, MEMBER can delete own
	},
	ResourceExpense: {
		ActionCreate: {MinRole: rolePtr(MemberRoleMember)},                              // Any member can create expenses
		ActionRead:   {MinRole: rolePtr(MemberRoleMember)},                              // Any member can read expenses
		ActionUpdate: {MinRole: rolePtr(MemberRoleAdmin), OwnerOrMinRole: true},         // ADMIN+ can update any, MEMBER can update own
		ActionDelete: {MinRole: rolePtr(MemberRoleAdmin), OwnerOrMinRole: true},         // ADMIN+ can delete any, MEMBER can delete own
	},
	ResourceLocation: {
		ActionCreate: {MinRole: rolePtr(MemberRoleMember)},              // Any member can share location
		ActionRead:   {MinRole: rolePtr(MemberRoleMember)},              // Any member can view locations (if sharing enabled)
		ActionUpdate: {MinRole: rolePtr(MemberRoleMember), OwnerOnly: true}, // Only update own location
		ActionDelete: {MinRole: rolePtr(MemberRoleMember), OwnerOnly: true}, // Only delete own location
	},
}

// rolePtr is a helper to create a pointer to a MemberRole.
func rolePtr(r MemberRole) *MemberRole {
	return &r
}

// CanPerform checks if a role can perform an action on a resource.
// This is the basic role-based check without ownership consideration.
// For ownership-aware checks, use CanPerformWithOwnership.
func CanPerform(role MemberRole, action Action, resource Resource) bool {
	resourcePerms, ok := permissionMatrix[resource]
	if !ok {
		return false // Unknown resource
	}

	rule, ok := resourcePerms[action]
	if !ok {
		return false // Unknown action for this resource
	}

	if rule.MinRole == nil {
		return false // Action not allowed
	}

	// If OwnerOnly is set, this basic check returns false
	// (ownership must be verified separately)
	if rule.OwnerOnly && !rule.OwnerOrMinRole {
		return false
	}

	return role.IsAuthorizedFor(*rule.MinRole)
}

// CanPerformWithOwnership checks if a role can perform an action on a resource,
// considering resource ownership.
//
// Parameters:
//   - role: The user's role in the trip
//   - action: The action being attempted
//   - resource: The resource type
//   - isOwner: Whether the user owns the specific resource instance
//
// Returns true if the action is allowed.
func CanPerformWithOwnership(role MemberRole, action Action, resource Resource, isOwner bool) bool {
	resourcePerms, ok := permissionMatrix[resource]
	if !ok {
		return false // Unknown resource
	}

	rule, ok := resourcePerms[action]
	if !ok {
		return false // Unknown action for this resource
	}

	if rule.MinRole == nil {
		return false // Action not allowed
	}

	// Check if role meets minimum requirement
	hasMinRole := role.IsAuthorizedFor(*rule.MinRole)

	// OwnerOnly: only the resource owner can perform this action
	if rule.OwnerOnly && !rule.OwnerOrMinRole {
		return isOwner
	}

	// OwnerOrMinRole: either owner OR role with MinRole can perform
	if rule.OwnerOrMinRole {
		return hasMinRole || isOwner
	}

	// Standard role-based check
	return hasMinRole
}

// GetPermissionRule returns the permission rule for an action on a resource.
// Returns nil if no rule is defined.
func GetPermissionRule(resource Resource, action Action) *PermissionRule {
	resourcePerms, ok := permissionMatrix[resource]
	if !ok {
		return nil
	}

	rule, ok := resourcePerms[action]
	if !ok {
		return nil
	}

	return &rule
}

// RequiresOwnership returns true if the action requires ownership verification.
func RequiresOwnership(resource Resource, action Action) bool {
	rule := GetPermissionRule(resource, action)
	if rule == nil {
		return false
	}
	return rule.OwnerOnly || rule.OwnerOrMinRole
}

// GetMinRoleForAction returns the minimum role required for an action on a resource.
// Returns nil if no role can perform the action.
func GetMinRoleForAction(resource Resource, action Action) *MemberRole {
	rule := GetPermissionRule(resource, action)
	if rule == nil {
		return nil
	}
	return rule.MinRole
}
