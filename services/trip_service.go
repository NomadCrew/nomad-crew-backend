package services

import (
	"context"
	"errors"
	"strings"

	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
)

// Common errors for validation
var (
	ErrEmptyTripID   = errors.New("trip ID cannot be empty")
	ErrEmptyUserID   = errors.New("user ID cannot be empty")
	ErrInvalidTripID = errors.New("invalid trip ID format")
	ErrInvalidUserID = errors.New("invalid user ID format")
)

// TripService implements the types.TripServiceInterface
type TripService struct {
	tripStore store.TripStore
}

// NewTripService creates a new TripService
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
	// Validate tripID
	if strings.TrimSpace(tripID) == "" {
		return nil, ErrEmptyTripID
	}

	// Validate userID
	if strings.TrimSpace(userID) == "" {
		return nil, ErrEmptyUserID
	}

	// Validate tripID is a valid UUID
	if _, err := uuid.Parse(tripID); err != nil {
		return nil, ErrInvalidTripID
	}

	// Validate userID is a valid UUID
	if _, err := uuid.Parse(userID); err != nil {
		return nil, ErrInvalidUserID
	}

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
