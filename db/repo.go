// Package db provides implementations for data access interfaces defined in internal/store.
// It interacts with the PostgreSQL database using pgxpool and potentially other external services like Supabase.
package db

import (
	"context"
	"fmt"

	"github.com/NomadCrew/nomad-crew-backend/types"
)

// TripRepo implements the deprecated TripRepository interface using the deprecated Store.
// Deprecated: This implementation is likely unused and superseded by TripDB in trip.go.
// Most methods are stubs.
type TripRepo struct {
	store *Store // Reference to the deprecated Store
}

// Create is a stub implementation for creating a trip.
// Deprecated: Use TripDB.CreateTrip instead.
func (r *TripRepo) Create(ctx context.Context, trip *types.Trip) (string, error) {
	// Implementation using store.pool would go here.
	return "", nil // Stub
}

// GetByID is a stub implementation for getting a trip by ID.
// Deprecated: Use TripDB.GetTripByID instead.
func (r *TripRepo) GetByID(ctx context.Context, id string) (*types.Trip, error) {
	// Implementation using store.pool would go here.
	return nil, nil // Stub
}

// Update attempts to update a trip, but likely calls itself recursively or incorrectly.
// Deprecated: Use TripDB.UpdateTrip instead.
func (r *TripRepo) Update(ctx context.Context, id string, update *types.TripUpdate) (*types.Trip, error) {
	// This call appears incorrect as it delegates back to the interface it implements via the store.
	// return r.store.Trips.Update(ctx, id, update)
	return nil, fmt.Errorf("TripRepo.Update is deprecated and likely incorrectly implemented") // Indicate issue
}

// Delete is a stub implementation for deleting a trip.
// Deprecated: Use TripDB.DeleteTrip instead.
func (r *TripRepo) Delete(ctx context.Context, id string) error {
	// Implementation using store.pool would go here.
	return nil // Stub
}
