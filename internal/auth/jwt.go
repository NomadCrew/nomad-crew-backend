package auth

import (
	stderrors "errors"
	"fmt"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Common JWT error types — kept for backwards compatibility with any callers
var (
	ErrTokenExpired     = fmt.Errorf("token is expired")
	ErrTokenInvalid     = fmt.Errorf("token is invalid")
	ErrTokenMalformed   = fmt.Errorf("token is malformed")
	ErrSignatureInvalid = fmt.Errorf("token signature is invalid")
)

// ValidateInvitationToken validates a JWT for trip invitations
func ValidateInvitationToken(tokenString, secret string) (*types.InvitationClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &types.InvitationClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})

	if err != nil {
		return nil, mapJWTError(err)
	}

	if !token.Valid {
		return nil, errors.Unauthorized("invalid_token", "Invalid invitation token")
	}

	claims, ok := token.Claims.(*types.InvitationClaims)
	if !ok {
		return nil, errors.Unauthorized("invalid_claims", "Invalid token structure")
	}

	// Verify invitation-specific claims
	if claims.InvitationID == "" {
		return nil, errors.Unauthorized("invalid_claims", "Token is not a valid invitation token")
	}

	// Verify expiration explicitly
	if !claims.ExpiresAt.IsZero() && time.Now().After(claims.ExpiresAt.Time) {
		return nil, errors.Unauthorized("token_expired", "Invitation has expired")
	}

	return claims, nil
}

// ValidateAccessToken validates a standard JWT access token
func ValidateAccessToken(tokenString, secret string) (*types.JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &types.JWTClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})

	if err != nil {
		return nil, mapJWTError(err)
	}

	if !token.Valid {
		return nil, errors.Unauthorized("invalid_token", "Invalid access token")
	}

	claims, ok := token.Claims.(*types.JWTClaims)
	if !ok {
		return nil, errors.Unauthorized("invalid_claims", "Invalid token structure")
	}

	if claims.UserID == "" {
		return nil, errors.Unauthorized("invalid_claims", "Token missing required user ID claim")
	}

	return claims, nil
}

// mapJWTError maps JWT library errors to application errors
func mapJWTError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case stderrors.Is(err, jwt.ErrTokenExpired):
		return errors.Unauthorized("token_expired", "Token has expired")
	case stderrors.Is(err, jwt.ErrSignatureInvalid):
		return errors.Unauthorized("invalid_signature", "Token signature is invalid")
	case stderrors.Is(err, jwt.ErrTokenMalformed):
		return errors.Unauthorized("malformed_token", "Token is malformed")
	default:
		return errors.Unauthorized("invalid_token", fmt.Sprintf("Token validation failed: %v", err))
	}
}

// GenerateJWT creates a new standard JWT access token.
func GenerateJWT(userID string, email string, secretKey string, expiryDuration time.Duration) (string, error) {
	if secretKey == "" {
		return "", fmt.Errorf("secret key must not be empty")
	}
	expirationTime := time.Now().Add(expiryDuration)
	claims := &types.JWTClaims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "nomadcrew-backend", // Optional: configure issuer
			Subject:   userID,
			ID:        uuid.NewString(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", errors.InternalServerError("Failed to sign token: " + err.Error())
	}
	return tokenString, nil
}

// GenerateInvitationToken creates a new JWT specifically for trip invitations.
func GenerateInvitationToken(invitationID string, tripID string, inviteeEmail string, secretKey string, expiryDuration time.Duration) (string, error) {
	if secretKey == "" {
		return "", fmt.Errorf("secret key must not be empty")
	}
	expirationTime := time.Now().Add(expiryDuration)
	claims := &types.InvitationClaims{
		InvitationID: invitationID,
		TripID:       tripID,
		InviteeEmail: inviteeEmail,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "nomadcrew-backend-invitation",
			Subject:   invitationID,
			ID:        uuid.NewString(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", errors.InternalServerError("Failed to sign invitation token: " + err.Error())
	}
	return tokenString, nil
}
