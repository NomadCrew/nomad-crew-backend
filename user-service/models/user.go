package models

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
)

type User struct {
	ID       int64  `json:"id,omitempty"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

func (u *User) SaveUser(ctx context.Context, pool *pgxpool.Pool) error {
	const query = "INSERT INTO users (username, email) VALUES ($1, $2) RETURNING id"
	err := pool.QueryRow(ctx, query, u.Username, u.Email).Scan(&u.ID)
	return err
}

func GetUserByEmail(ctx context.Context, db *pgxpool.Pool, email string) (*User, error) {
	user := &User{}
	query := `SELECT id, username, email, password_hash FROM users WHERE email=$1`
	err := db.QueryRow(ctx, query, email).Scan(&user.ID, &user.Username, &user.Email)
	if err != nil {
		return nil, err
	}
	return user, nil
}
