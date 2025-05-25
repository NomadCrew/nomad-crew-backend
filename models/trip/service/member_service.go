package service

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/events"
	istore "github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/services"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

// TripMemberService handles operations related to trip members
type TripMemberService struct {
	store           istore.TripStore
	eventPublisher  types.EventPublisher
	supabaseService *services.SupabaseService
}

// NewTripMemberService creates a new trip member service
func NewTripMemberService(
	store istore.TripStore,
	eventPublisher types.EventPublisher,
	supabaseService *services.SupabaseService,
) *TripMemberService {
	return &TripMemberService{
		store:           store,
		eventPublisher:  eventPublisher,
		supabaseService: supabaseService,
	}
}

// AddMember adds a new member to a trip
func (s *TripMemberService) AddMember(ctx context.Context, membership *types.TripMembership) error {
	if err := s.store.AddMember(ctx, membership); err != nil {
		return err
	}

	// Sync membership to Supabase for RLS validation
	if s.supabaseService != nil && s.supabaseService.IsEnabled() {
		syncData := services.MembershipSyncData{
			TripID: membership.TripID,
			UserID: membership.UserID,
			Role:   string(membership.Role),
			Status: string(membership.Status),
		}

		// Sync asynchronously to avoid blocking member addition
		go func() {
			syncCtx := context.Background()
			if err := s.supabaseService.SyncMembership(syncCtx, syncData); err != nil {
				log := logger.GetLogger()
				log.Errorw("Failed to sync trip membership to Supabase", "error", err, "tripID", membership.TripID, "userID", membership.UserID)
			} else {
				log := logger.GetLogger()
				log.Infow("Successfully synced trip membership to Supabase", "tripID", membership.TripID, "userID", membership.UserID)
			}
		}()
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
		// If GetUserRole fails, the user is likely not a member or an error occurred.
		return nil, err
	}

	// Update the role
	if err := s.store.UpdateMemberRole(ctx, tripID, memberID, newRole); err != nil {
		return nil, err
	}

	// Sync updated membership to Supabase for RLS validation
	if s.supabaseService != nil && s.supabaseService.IsEnabled() {
		syncData := services.MembershipSyncData{
			TripID: tripID,
			UserID: memberID,
			Role:   string(newRole),
			Status: "active", // Assume active status for role updates
		}

		// Sync asynchronously to avoid blocking role update
		go func() {
			syncCtx := context.Background()
			if err := s.supabaseService.SyncMembership(syncCtx, syncData); err != nil {
				log := logger.GetLogger()
				log.Errorw("Failed to sync updated trip membership to Supabase", "error", err, "tripID", tripID, "userID", memberID)
			} else {
				log := logger.GetLogger()
				log.Infow("Successfully synced updated trip membership to Supabase", "tripID", tripID, "userID", memberID)
			}
		}()
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
	// Check if the member exists by trying to get their role.
	// If GetUserRole returns an error, the member is not found or another error occurred.
	_, err := s.GetUserRole(ctx, tripID, userID)
	if err != nil {
		return err
	}

	// Remove the member
	if err := s.store.RemoveMember(ctx, tripID, userID); err != nil {
		return err
	}

	// Remove membership from Supabase
	if s.supabaseService != nil && s.supabaseService.IsEnabled() {
		// Sync asynchronously to avoid blocking member removal
		go func() {
			syncCtx := context.Background()
			if err := s.supabaseService.DeleteMembership(syncCtx, tripID, userID); err != nil {
				log := logger.GetLogger()
				log.Errorw("Failed to delete trip membership from Supabase", "error", err, "tripID", tripID, "userID", userID)
			} else {
				log := logger.GetLogger()
				log.Infow("Successfully deleted trip membership from Supabase", "tripID", tripID, "userID", userID)
			}
		}()
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

// GetUserRole gets a user's role in a trip.
// It returns an empty role string and the error if the user is not found or another error occurs.
func (s *TripMemberService) GetUserRole(ctx context.Context, tripID, userID string) (types.MemberRole, error) {
	role, err := s.store.GetUserRole(ctx, tripID, userID)
	if err != nil {
		return "", err // Return empty role and the original error from the store
	}
	return role, nil
}

// IsTripMember checks if a user is a member of a specific trip.
// This is needed to satisfy the handlers.TripServiceInterface
func (s *TripMemberService) IsTripMember(ctx context.Context, tripID, userID string) (bool, error) {
	_, err := s.store.GetUserRole(ctx, tripID, userID)
	if err != nil {
		// Check if the error is specifically a "not found" type error from the store.
		// The exact error type or code to check might depend on the store's implementation.
		if appErr, ok := err.(*errors.AppError); ok && appErr.Type == errors.NotFoundError {
			return false, nil // User not found, so not a member, no error to bubble up for this specific check.
		}
		// For other types of errors, return false and the error.
		return false, err
	}
	// If GetUserRole succeeded without error, the user is a member.
	return true, nil
}

// GetTripMember retrieves a trip membership record for a user
// This is also needed to satisfy the handlers.TripServiceInterface
func (s *TripMemberService) GetTripMember(ctx context.Context, tripID, userID string) (*types.TripMembership, error) {
	// Get all members from the trip
	members, err := s.store.GetTripMembers(ctx, tripID)
	if err != nil {
		return nil, err
	}

	// Filter to find the specific member
	for _, membership := range members {
		if membership.UserID == userID {
			// Return a copy to avoid any potential issues with the slice being modified
			membershipCopy := membership
			return &membershipCopy, nil
		}
	}

	// If we get here, the user is not a member of this trip
	// Try to get their role which will return the appropriate "not found" error
	_, err = s.store.GetUserRole(ctx, tripID, userID)
	return nil, err
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
