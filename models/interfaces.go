package models

import (
    "context"
    "github.com/NomadCrew/nomad-crew-backend/types"
)

type UserModelInterface interface {
    CreateUser(ctx context.Context, user *types.User) error
    GetUserByID(ctx context.Context, id int64) (*types.User, error)
    UpdateUser(ctx context.Context, user *types.User) error
    DeleteUser(ctx context.Context, id int64) error
    AuthenticateUser(ctx context.Context, email, password string) (*types.User, error)
}

type TripModelInterface interface {
    CreateTrip(ctx context.Context, trip *types.Trip) error
    GetTripByID(ctx context.Context, id int64) (*types.Trip, error)
    UpdateTrip(ctx context.Context, id int64, update *types.TripUpdate) error
    DeleteTrip(ctx context.Context, id int64) error
    ListUserTrips(ctx context.Context, userID int64) ([]*types.Trip, error)
    SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error)
}

var _ UserModelInterface = (*UserModel)(nil)
var _ TripModelInterface = (*TripModel)(nil)