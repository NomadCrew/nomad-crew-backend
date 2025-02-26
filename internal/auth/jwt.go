package auth

import (
	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/golang-jwt/jwt/v5"
)

func ValidateInvitationToken(tokenString, secret string) (*types.InvitationClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &types.InvitationClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})

	if err != nil || !token.Valid {
		return nil, errors.Unauthorized("invalid_token", "Invalid invitation")
	}

	claims, ok := token.Claims.(*types.InvitationClaims)
	if !ok {
		return nil, errors.Unauthorized("invalid_claims", "Invalid token structure")
	}

	return claims, nil
}
