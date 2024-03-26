package models

import (
	"context"
	"os"
	"github.com/NomadCrew/nomad-crew-backend/user-service/logger"
	"github.com/golang-jwt/jwt"
	"github.com/jackc/pgx/v4/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           int64  `json:"id,omitempty"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	Password     string `json:"password,omitempty"`
	PasswordHash string `json:"-"`
}

func (u *User) SaveUser(ctx context.Context, pool *pgxpool.Pool) error {
	const query = "INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3) RETURNING id"
	err := pool.QueryRow(ctx, query, u.Username, u.Email, u.PasswordHash).Scan(&u.ID)
	return err
}

func (u *User) HashPassword(password string) error {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(bytes)
	return nil
}

func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}


func AuthenticateUser(ctx context.Context, pool *pgxpool.Pool, email, password string) (*User, error) {
	log := logger.GetLogger()
	var u User
	const query = "SELECT id, username, email, password_hash FROM users WHERE email = $1"
	row := pool.QueryRow(ctx, query, email)
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash)
	if err != nil {
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	if err != nil {
		log.Errorf("Error comparing password hash: %s", err)
		return nil, err
	}

	return &u, nil
}

func GenerateJWT(user User) (string, error) {
	jwtSecretKey := os.Getenv("JWT_SECRET_KEY")
	if jwtSecretKey == "" {
		jwtSecretKey = "default_secret"
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": user.Username,
		"email":    user.Email,
	})

	tokenString, err := token.SignedString([]byte(jwtSecretKey))
	return tokenString, err
}
