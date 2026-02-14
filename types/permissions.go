package types

// Action represents an operation that can be performed on a resource.
type Action string

// Resource represents a type of entity that can be accessed.
type Resource string

// Permission actions
const (
	// Standard CRUD actions
	ActionCreate Action = "create"
	ActionRead   Action = "read"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"

	// Resource-specific actions
	ActionInvite     Action = "invite"      // Send invitation to join trip
	ActionRemove     Action = "remove"      // Remove member from trip
	ActionChangeRole Action = "change_role" // Change member's role
	ActionLeave      Action = "leave"       // Leave trip (self-removal)
)

// Resources
const (
	ResourceTrip       Resource = "trip"
	ResourceMember     Resource = "member"
	ResourceInvitation Resource = "invitation"
	ResourceChat       Resource = "chat"
	ResourceTodo       Resource = "todo"
	ResourceExpense    Resource = "expense"
	ResourceLocation   Resource = "location"
	ResourcePoll       Resource = "poll"
)

// String returns the string representation of an Action.
func (a Action) String() string {
	return string(a)
}

// String returns the string representation of a Resource.
func (r Resource) String() string {
	return string(r)
}

// IsValid checks if the action is a valid permission action.
func (a Action) IsValid() bool {
	switch a {
	case ActionCreate, ActionRead, ActionUpdate, ActionDelete,
		ActionInvite, ActionRemove, ActionChangeRole, ActionLeave:
		return true
	}
	return false
}

// IsValid checks if the resource is a valid permission resource.
func (r Resource) IsValid() bool {
	switch r {
	case ResourceTrip, ResourceMember, ResourceInvitation,
		ResourceChat, ResourceTodo, ResourceExpense, ResourceLocation, ResourcePoll:
		return true
	}
	return false
}
