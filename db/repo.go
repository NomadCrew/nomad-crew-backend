package db

import (
    "context"
    "github.com/NomadCrew/nomad-crew-backend/types"
)

// TripRepo implements TripRepository
type TripRepo struct {
    store *Store
}

func (r *TripRepo) Create(ctx context.Context, trip *types.Trip) (string, error) {
    // Implementation using store.pool
    return "", nil
}

func (r *TripRepo) GetByID(ctx context.Context, id string) (*types.Trip, error) {
    // Implementation using store.pool
    return nil, nil
}

func (r *TripRepo) Update(ctx context.Context, id string, update *types.TripUpdate) error {
    // Implementation using store.pool
    return nil
}

func (r *TripRepo) Delete(ctx context.Context, id string) error {
    // Implementation using store.pool
    return nil
}