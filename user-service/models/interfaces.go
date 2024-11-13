package models

import (
    "context"
    "github.com/jackc/pgx/v4/pgxpool"
)

// UserStore defines database operations for users
type UserStore interface {
    GetPool() *pgxpool.Pool
    GetUserByID(ctx context.Context, id int64) (*User, error)
    SaveUser(ctx context.Context, user *User) error
    UpdateUser(ctx context.Context, user *User) error
    DeleteUser(ctx context.Context, id int64) error
    AuthenticateUser(ctx context.Context, email, password string) (*User, error)
}