package service

import (
	"context"

	internal_errors "github.com/NomadCrew/nomad-crew-backend/internal/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/events"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

// TripChatService handles chat operations for trips
type TripChatService struct {
	chatStore      store.ChatStore
	tripStore      store.TripStore
	eventPublisher types.EventPublisher
}

// NewTripChatService creates a new trip chat service
func NewTripChatService(
	chatStore store.ChatStore,
	tripStore store.TripStore,
	eventPublisher types.EventPublisher,
) *TripChatService {
	return &TripChatService{
		chatStore:      chatStore,
		tripStore:      tripStore,
		eventPublisher: eventPublisher,
	}
}

// ListMessages lists messages for a trip
func (s *TripChatService) ListMessages(ctx context.Context, tripID string, userID string, limit int, before string) ([]*types.ChatMessage, error) {
	// Check if the user is a member of the trip (based on membership data)
	_, err := s.tripStore.GetUserRole(ctx, tripID, userID) // We only need to check the error
	if err != nil {
		// Consider wrapping the error or returning a specific "access denied" error.
		// For now, returning the original error which might indicate "not found" or other issues.
		return nil, internal_errors.NewForbiddenError("User is not a member of this trip or access denied")
	}

	// Using appropriate method from the store
	paginationParams := types.PaginationParams{
		Limit:  limit,
		Offset: 0, // Assuming offset is 0 for now, 'before' parameter is not directly used
		// Before/After cursor logic could be added here if needed based on 'before' param
	}
	messages, _, err := s.chatStore.ListChatMessages(ctx, tripID, paginationParams)
	if err != nil {
		return nil, err
	}

	// Convert to proper return type
	result := make([]*types.ChatMessage, len(messages))
	for i, msg := range messages {
		msgCopy := msg
		result[i] = &msgCopy
	}

	return result, nil
}

// UpdateLastReadMessage updates the last read message for a user in a trip
func (s *TripChatService) UpdateLastReadMessage(ctx context.Context, tripID string, userID string, messageID string) error {
	// Check if the user is a member of the trip
	_, err := s.tripStore.GetUserRole(ctx, tripID, userID) // We only need to check the error
	if err != nil {
		// Consider wrapping the error or returning a specific "access denied" error.
		return internal_errors.NewForbiddenError("User is not a member of this trip or access denied")
	}

	// Since the ChatStore doesn't have a direct method for trip message read status,
	// we could use an appropriate method or implement a proper one
	// For now, using UpdateLastReadMessage with group ID (which may need to be refactored)
	if err := s.chatStore.UpdateLastReadMessage(ctx, tripID, userID, messageID); err != nil {
		return err
	}

	// Publish event for last read updated
	data := map[string]interface{}{
		"message_id": messageID,
	}

	return events.PublishEventWithContext(
		s.eventPublisher,
		ctx,
		"chat.last_read_updated",
		tripID,
		userID,
		data,
		"chat-service",
	)
}
