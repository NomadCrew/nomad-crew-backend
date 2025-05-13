package service

// Event topics for publishing
const (
	EventTopicTrips       = "trips"
	EventTopicChat        = "chat"
	EventTopicInvitation  = "invitation"
	EventTopicTodo        = "todo"
	EventTopicWeather     = "weather"
	EventTopicDestination = "destination"
)

// Event types for trips
const (
	// Trip events
	EventTypeTripCreated       = "trip.created"
	EventTypeTripUpdated       = "trip.updated"
	EventTypeTripDeleted       = "trip.deleted"
	EventTypeTripStatusChanged = "trip.status.changed"

	// Member events
	EventTypeMemberAdded       = "member.added"
	EventTypeMemberRemoved     = "member.removed"
	EventTypeMemberRoleChanged = "member.role.changed"

	// Invitation events
	EventTypeInvitationCreated  = "invitation.created"
	EventTypeInvitationAccepted = "invitation.accepted"
	EventTypeInvitationRejected = "invitation.rejected"

	// Chat events
	EventTypeMessageSent     = "chat.message.sent"
	EventTypeLastReadUpdated = "chat.last_read.updated"

	// Todo events
	EventTypeTodoCreated   = "todo.created"
	EventTypeTodoUpdated   = "todo.updated"
	EventTypeTodoDeleted   = "todo.deleted"
	EventTypeTodoCompleted = "todo.completed"

	// Weather events
	EventTypeWeatherUpdated = "weather.updated"

	// Location/destination events
	EventTypeDestinationChanged = "destination.changed"
)

// publishEventWithContext has been moved to internal/events/publisher.go
