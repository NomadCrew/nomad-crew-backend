package db

import (
	"context"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

// UserRepo implements UserRepository
type UserRepo struct {
	store *Store
}

// TripRepo implements TripRepository
type TripRepo struct {
	store *Store
}

func (r *UserRepo) Create(ctx context.Context, user *types.User) error {
	// Implementation using store.pool
	return nil
}

func (r *UserRepo) GetByID(ctx context.Context, id string) (*types.User, error) {
	// Implementation using store.pool
	return nil, nil
}

func (r *UserRepo) Update(ctx context.Context, user *types.User) error {
	// Implementation using store.pool
	return nil
}

func (r *UserRepo) Delete(ctx context.Context, id string) error {
	// Implementation using store.pool
	return nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*types.User, error) {
	// Implementation using store.pool
	return nil, nil
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
