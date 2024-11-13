package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/user-service/db"
	"github.com/NomadCrew/nomad-crew-backend/user-service/logger"
	"github.com/golang-jwt/jwt"
	"github.com/jackc/pgx/v4/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
    ID             int64     `json:"id"`
    Username       string    `json:"username"`
    Email          string    `json:"email"`
    PasswordHash   string    `json:"-"` // Never sent to client
    FirstName      string    `json:"first_name"`
    LastName       string    `json:"last_name"`
    ProfilePicture string    `json:"profile_picture"`
    PhoneNumber    string    `json:"phone_number"`
    Address        string    `json:"address"`
    CreatedAt      time.Time `json:"created_at,omitempty"`
    UpdatedAt      time.Time `json:"updated_at,omitempty"`
    DeletedAt      *time.Time `json:"deleted_at,omitempty"`
}

func (u *User) SaveUser(ctx context.Context, userDB *db.UserDB) error {
    const query = `
        INSERT INTO users (
            username, email, password_hash, first_name, last_name,
            profile_picture, phone_number, address
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING id`

    err := userDB.pool.QueryRow(
        ctx, 
        query,
        u.Username,
        u.Email,
        u.PasswordHash,
        u.FirstName,
        u.LastName,
        u.ProfilePicture,
        u.PhoneNumber,
        u.Address,
    ).Scan(&u.ID)

    if err!= nil {
        log.Errorw("Failed to save user",
            "username", u.Username,
            "error", err)
        return fmt.Errorf("failed to save user: %w", err)
    }

    // Insert metadata
    const metadataQuery = `
        INSERT INTO metadata (table_name, record_id, created_at, updated_at)
        VALUES ($1, $2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`
    
    _, err = pool.Exec(ctx, metadataQuery, "users", u.ID)
    if err != nil {
        log.Errorw("Failed to save user metadata",
            "userId", u.ID,
            "error", err)
        return fmt.Errorf("failed to save user metadata: %w", err)
    }

    return nil
}

// GetUserByID retrieves a user by ID
func GetUserByID(ctx context.Context, userDB *db.UserDB, id int64) (*User, error) {
    const query = `SELECT id, username, email, first_name, last_name, profile_picture, phone_number, address FROM users WHERE id = $1`
    var user User
    err := userDB.pool.QueryRow(ctx, query, id).Scan(
        &user.ID,
        &user.Username,
        &user.Email,
        &user.FirstName,
        &user.LastName,
        &user.ProfilePicture,
        &user.PhoneNumber,
        &user.Address,
    )
    if err!= nil {
        if err == sql.ErrNoRows {
            return nil, errors.New("user not found")
        }
        return nil, err
    }
    return &user, nil
}

func (u *User) UpdateUser(ctx context.Context, userDB *db.UserDB) error {
    log := logger.GetLogger()
    
    const query = `
        UPDATE users
        SET username = $1, email = $2, first_name = $3, last_name = $4,
            profile_picture = $5, phone_number = $6, address = $7
        WHERE id = $8`

        result, err := userDB.pool.Exec(ctx, query,
            u.Username,
            u.Email,
            u.FirstName,
            u.LastName,
            u.ProfilePicture,
            u.PhoneNumber,
            u.Address,
            u.ID,
        )    

    if err != nil {
        log.Errorw("Failed to update user",
            "userId", u.ID,
            "error", err)
        return fmt.Errorf("failed to update user: %w", err)
    }

    if result.RowsAffected() == 0 {
        return fmt.Errorf("user not found")
    }

    // Update metadata
    const metadataQuery = `
        UPDATE metadata
        SET updated_at = CURRENT_TIMESTAMP
        WHERE table_name = 'users' AND record_id = $1`
    
    _, err = pool.Exec(ctx, metadataQuery, u.ID)
    if err != nil {
        log.Errorw("Failed to update user metadata",
            "userId", u.ID,
            "error", err)
        return fmt.Errorf("failed to update user metadata: %w", err)
    }

    return nil
}

func (u *User) DeleteUser(ctx context.Context, userDB *db.UserDB) error {
    log := logger.GetLogger()
    
    // Soft delete using metadata table
    const query = `
        UPDATE metadata
        SET deleted_at = CURRENT_TIMESTAMP
        WHERE table_name = 'users' AND record_id = $1`

        result, err := userDB.pool.Exec(ctx, query, u.ID)
    if err != nil {
        log.Errorw("Failed to delete user",
            "userId", u.ID,
            "error", err)
        return fmt.Errorf("failed to delete user: %w", err)
    }

    if result.RowsAffected() == 0 {
        return fmt.Errorf("user not found")
    }

    return nil
}

func AuthenticateUser(ctx context.Context, pool *pgxpool.Pool, email, password string) (*User, error) {
    log := logger.GetLogger()
    
    const query = `
        SELECT u.id, u.username, u.email, u.password_hash, u.first_name, 
               u.last_name, u.profile_picture, u.phone_number, u.address
        FROM users u
        LEFT JOIN metadata m ON m.table_name = 'users' AND m.record_id = u.id
        WHERE u.email = $1 AND m.deleted_at IS NULL`

    var user User
    err := pool.QueryRow(ctx, query, email).Scan(
        &user.ID,
        &user.Username,
        &user.Email,
        &user.PasswordHash,
        &user.FirstName,
        &user.LastName,
        &user.ProfilePicture,
        &user.PhoneNumber,
        &user.Address,
    )

    if err != nil {
        if err == sql.ErrNoRows {
            log.Errorw("User not found", "email", email)
            return nil, fmt.Errorf("invalid credentials")
        }
        log.Errorw("Failed to authenticate user",
            "email", email,
            "error", err)
        return nil, fmt.Errorf("authentication failed: %w", err)
    }

    err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
    if err != nil {
        log.Errorw("Invalid password", "userId", user.ID)
        return nil, fmt.Errorf("invalid credentials")
    }

    return &user, nil
}

func (u *User) GenerateJWT() (string, error) {
    log := logger.GetLogger()
    
    jwtSecretKey := os.Getenv("JWT_SECRET_KEY")
    if jwtSecretKey == "" {
        log.Fatal("JWT secret key is not set")
        return "", fmt.Errorf("JWT secret key is not set")
    }

    claims := jwt.MapClaims{
        "user_id":   u.ID,
        "username": u.Username,
        "email":    u.Email,
        "exp":      time.Now().Add(24 * time.Hour).Unix(),
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    tokenString, err := token.SignedString([]byte(jwtSecretKey))
    if err != nil {
        log.Errorw("Failed to generate JWT",
            "userId", u.ID,
            "error", err)
        return "", fmt.Errorf("failed to generate token: %w", err)
    }

    return tokenString, nil
}