package services

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

// TripService implements the types.TripServiceInterface
type TripService struct {
	tripStore store.TripStore
}

// NewTripService returns a new TripService initialized with the provided TripStore.
func NewTripService(tripStore store.TripStore) *TripService {
	return &TripService{
		tripStore: tripStore,
	}
}

// IsTripMember checks if a user is a member of a trip
func (s *TripService) IsTripMember(ctx context.Context, tripID, userID string) (bool, error) {
	role, err := s.tripStore.GetUserRole(ctx, tripID, userID)
	if err != nil {
		return false, err
	}
	return role != "", nil
}

// GetTripMember retrieves trip membership details for a user
func (s *TripService) GetTripMember(ctx context.Context, tripID, userID string) (*types.TripMembership, error) {
	// Get user role
	role, err := s.tripStore.GetUserRole(ctx, tripID, userID)
	if err != nil {
		return nil, err
	}

	// Create and return a membership object
	return &types.TripMembership{
		TripID: tripID,
		UserID: userID,
		Role:   role,
	}, nil
}
