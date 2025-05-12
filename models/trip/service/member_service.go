package service

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/events"
	istore "github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

// TripMemberService handles operations related to trip members
type TripMemberService struct {
	store          istore.TripStore
	eventPublisher types.EventPublisher
}

// NewTripMemberService creates a new trip member service
func NewTripMemberService(
	store istore.TripStore,
	eventPublisher types.EventPublisher,
) *TripMemberService {
	return &TripMemberService{
		store:          store,
		eventPublisher: eventPublisher,
	}
}

// AddMember adds a new member to a trip
func (s *TripMemberService) AddMember(ctx context.Context, membership *types.TripMembership) error {
	if err := s.store.AddMember(ctx, membership); err != nil {
		return err
	}

	// Publish event for member added
	s.publishEvent(ctx, EventTypeMemberAdded, membership.TripID, membership.UserID, map[string]interface{}{
		"role": membership.Role,
	})

	return nil
}

// UpdateMemberRole updates a member's role in a trip
func (s *TripMemberService) UpdateMemberRole(ctx context.Context, tripID, memberID string, newRole types.MemberRole) (*interfaces.CommandResult, error) {
	// Check if the trip exists
	_, err := s.store.GetTrip(ctx, tripID)
	if err != nil {
		return nil, err
	}

	// Get the user's current role to compare
	oldRole, err := s.GetUserRole(ctx, tripID, memberID)
	if err != nil {
		return nil, err
	}

	// Update the role
	if err := s.store.UpdateMemberRole(ctx, tripID, memberID, newRole); err != nil {
		return nil, err
	}

	// Publish event for role change
	s.publishEvent(ctx, EventTypeMemberRoleChanged, tripID, memberID, map[string]interface{}{
		"old_role": oldRole,
		"new_role": newRole,
	})

	return &interfaces.CommandResult{
		Success: true,
		Data: &types.TripMembership{
			TripID: tripID,
			UserID: memberID,
			Role:   newRole,
		},
	}, nil
}

// RemoveMember removes a member from a trip
func (s *TripMemberService) RemoveMember(ctx context.Context, tripID, userID string) error {
	// Check if the member exists
	_, err := s.GetUserRole(ctx, tripID, userID)
	if err != nil {
		return err
	}

	// Remove the member
	if err := s.store.RemoveMember(ctx, tripID, userID); err != nil {
		return err
	}

	// Publish event for member removed
	s.publishEvent(ctx, EventTypeMemberRemoved, tripID, userID, nil)

	return nil
}

// GetTripMembers gets all members of a trip
func (s *TripMemberService) GetTripMembers(ctx context.Context, tripID string) ([]types.TripMembership, error) {
	members, err := s.store.GetTripMembers(ctx, tripID)
	if err != nil {
		return nil, err
	}

	// No longer convert to slice of pointers, return directly
	return members, nil
}

// GetUserRole gets a user's role in a trip
func (s *TripMemberService) GetUserRole(ctx context.Context, tripID, userID string) (types.MemberRole, error) {
	role, err := s.store.GetUserRole(ctx, tripID, userID)
	if err != nil {
		return types.MemberRoleNone, &errors.AppError{
			Type:    errors.NotFoundError,
			Message: "User is not a member of this trip",
		}
	}
	return role, nil
}

// IsTripMember checks if a user is a member of a specific trip
// This is needed to satisfy the handlers.TripServiceInterface
func (s *TripMemberService) IsTripMember(ctx context.Context, tripID, userID string) (bool, error) {
	role, err := s.store.GetUserRole(ctx, tripID, userID)
	if err != nil {
		// If there's an error, check if it's because the user isn't a member
		if appErr, ok := err.(*errors.AppError); ok && appErr.Type == errors.NotFoundError {
			return false, nil
		}
		return false, err
	}

	return role != types.MemberRoleNone, nil
}

// Helper method to publish events using the centralized helper
func (s *TripMemberService) publishEvent(ctx context.Context, eventType string, tripID string, userID string, data map[string]interface{}) {
	log := logger.GetLogger()

	// Publish using the centralized helper
	if err := events.PublishEventWithContext(
		s.eventPublisher,
		ctx,
		eventType,
		tripID,
		userID,
		data,
		"member-service",
	); err != nil {
		log.Warnw("Failed to publish member event",
			"error", err,
			"eventType", eventType,
			"tripID", tripID,
			"userID", userID,
		)
	}
}
