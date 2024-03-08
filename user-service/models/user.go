package models

import (
	"context"
	"database/sql"
	"os"

	"github.com/golang-jwt/jwt"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
}

var logger = logrus.New()

func (u *User) SaveUser(ctx context.Context, pool *pgxpool.Pool) error {
	const query = "INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3) RETURNING id"
	err := pool.QueryRow(ctx, query, u.Username, u.Email, u.PasswordHash).Scan(&u.ID)
	return err
}

func AuthenticateUser(ctx context.Context, pool *pgxpool.Pool, email, password string) (*User, error) {
	var u User
	const query = "SELECT id, username, email, password_hash FROM users WHERE email = $1"
	row := pool.QueryRow(ctx, query, email)
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	if err != nil {
		return nil, err
	}

	return &u, nil
}

func generateJWT(user User) (string, error) {
	jwtSecretKey := os.Getenv("JWT_SECRET_KEY")
	if jwtSecretKey == "" {
		logger.Fatal("JWT secret key is not set")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": user.Username,
		"email":    user.Email,
	})
	tokenString, err := token.SignedString([]byte(jwtSecretKey))
	return tokenString, err
}
