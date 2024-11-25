package models

import (
    "context"
    "github.com/NomadCrew/nomad-crew-backend/user-service/types"
)

type UserModelInterface interface {
    CreateUser(ctx context.Context, user *types.User) error
    GetUserByID(ctx context.Context, id int64) (*types.User, error)
    UpdateUser(ctx context.Context, user *types.User) error
    DeleteUser(ctx context.Context, id int64) error
    AuthenticateUser(ctx context.Context, email, password string) (*types.User, error)
}

var _ UserModelInterface = (*UserModel)(nil)