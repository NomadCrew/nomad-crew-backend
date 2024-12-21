package models

import (
    "context"
    "os"
    "time"
    "github.com/golang-jwt/jwt"
    "golang.org/x/crypto/bcrypt"
    "github.com/google/uuid"
    "fmt"
    "github.com/NomadCrew/nomad-crew-backend/errors"
    "github.com/NomadCrew/nomad-crew-backend/types"
    "github.com/NomadCrew/nomad-crew-backend/internal/store"
    "github.com/NomadCrew/nomad-crew-backend/logger"
)

// UserModel provides high-level user operations
type UserModel struct {
    store store.UserStore
}

func NewUserModel(store store.UserStore) *UserModel {
    return &UserModel{store: store}
}

func (um *UserModel) CreateUser(ctx context.Context, user *types.User) error {
    return um.store.SaveUser(ctx, user)
}

func (um *UserModel) GetUserByID(ctx context.Context, id int64) (*types.User, error) {
    user, err := um.store.GetUserByID(ctx, id)
    if err != nil {
        return nil, errors.Wrap(err, errors.NotFoundError, "User not found")
    }
    return user, nil
}

func (um *UserModel) UpdateUser(ctx context.Context, user *types.User) error {
    return um.store.UpdateUser(ctx, user)
}

func (um *UserModel) DeleteUser(ctx context.Context, id int64) error {
    return um.store.DeleteUser(ctx, id)
}

func (um *UserModel) AuthenticateUser(ctx context.Context, email, password string) (*types.User, error) {
    user, err := um.store.AuthenticateUser(ctx, email)
    if err != nil {
        return nil, errors.AuthenticationFailed("Invalid credentials")
    }

    err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
    if err != nil {
        return nil, errors.AuthenticationFailed("Invalid credentials")
    }

    return user, nil
}

func GenerateJWT(user *types.User) (string, error) {
    log := logger.GetLogger()

    jwtSecretKey := os.Getenv("JWT_SECRET_KEY")
    if jwtSecretKey == "" {
        return "", errors.New(errors.ServerError, "JWT configuration error", "secret key not set")
    }

    // Generate a unique identifier for the token
    jti := uuid.New().String()

    // Define claims for the JWT
    claims := jwt.MapClaims{
        "user_id":  user.ID,                              // Custom claim for user ID
        "username": user.Username,                       // Custom claim for username
        "email":    user.Email,                          // Custom claim for email
        "exp":      time.Now().Add(24 * time.Hour).Unix(), // Expiration time (24 hours)
        "jti":      jti,                                 // Unique identifier for the token
        "sub":      fmt.Sprintf("%d", user.ID),          // Subject (user ID)
    }

    // Create the token with claims and sign it
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    tokenString, err := token.SignedString([]byte(jwtSecretKey))
    if err != nil {
        log.Errorw("Failed to generate JWT", "error", err)
        return "", errors.New(errors.ServerError, "Failed to generate token", err.Error())
    }

    return tokenString, nil
}

func GenerateRefreshToken(user *types.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":   user.ID,
		"email": user.Email,
		"exp":   time.Now().Add(30 * 24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	secretKey := []byte(os.Getenv("JWT_SECRET_KEY"))

	return token.SignedString(secretKey)
}
