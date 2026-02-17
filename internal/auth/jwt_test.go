package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateJWT(t *testing.T) {
	secretKey := "test-secret-key-that-is-long-enough-for-testing"

	tests := []struct {
		name           string
		userID         string
		email          string
		secretKey      string
		expiryDuration time.Duration
		wantErr        bool
	}{
		{
			name:           "Valid JWT generation",
			userID:         "user123",
			email:          "test@example.com",
			secretKey:      secretKey,
			expiryDuration: time.Hour,
			wantErr:        false,
		},
		{
			name:           "Empty user ID",
			userID:         "",
			email:          "test@example.com",
			secretKey:      secretKey,
			expiryDuration: time.Hour,
			wantErr:        false, // Should still generate token
		},
		{
			name:           "Empty email",
			userID:         "user123",
			email:          "",
			secretKey:      secretKey,
			expiryDuration: time.Hour,
			wantErr:        false, // Should still generate token
		},
		{
			name:           "Empty secret key",
			userID:         "user123",
			email:          "test@example.com",
			secretKey:      "",
			expiryDuration: time.Hour,
			wantErr:        true,
		},
		{
			name:           "Negative expiry duration",
			userID:         "user123",
			email:          "test@example.com",
			secretKey:      secretKey,
			expiryDuration: -time.Hour,
			wantErr:        false, // Should still generate token (expired)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := GenerateJWT(tt.userID, tt.email, tt.secretKey, tt.expiryDuration)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, token)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, token)

				// Verify token structure (JWT tokens are period-separated: header.payload.signature)
				parts := strings.Split(token, ".")
				assert.Len(t, parts, 3, "JWT should have 3 parts")
			}
		})
	}
}

func TestValidateAccessToken(t *testing.T) {
	secretKey := "test-secret-key-that-is-long-enough-for-testing"
	userID := "user123"
	email := "test@example.com"

	tests := []struct {
		name      string
		setupFunc func() string
		secretKey string
		wantErr   bool
		errorType string
	}{
		{
			name: "Valid token",
			setupFunc: func() string {
				token, _ := GenerateJWT(userID, email, secretKey, time.Hour)
				return token
			},
			secretKey: secretKey,
			wantErr:   false,
		},
		{
			name: "Expired token",
			setupFunc: func() string {
				token, _ := GenerateJWT(userID, email, secretKey, -time.Hour)
				return token
			},
			secretKey: secretKey,
			wantErr:   true,
			errorType: "token_expired",
		},
		{
			name: "Invalid signature",
			setupFunc: func() string {
				token, _ := GenerateJWT(userID, email, secretKey, time.Hour)
				return token
			},
			secretKey: "wrong-secret-key-that-is-long-enough",
			wantErr:   true,
			errorType: "invalid_signature",
		},
		{
			name: "Malformed token",
			setupFunc: func() string {
				return "not.a.token"
			},
			secretKey: secretKey,
			wantErr:   true,
			errorType: "malformed_token",
		},
		{
			name: "Empty token",
			setupFunc: func() string {
				return ""
			},
			secretKey: secretKey,
			wantErr:   true,
			errorType: "malformed_token",
		},
		{
			name: "Token with invalid claims",
			setupFunc: func() string {
				// Create a token without the required "sub" claim (no user ID)
				claims := jwt.MapClaims{
					"email": email,
					"exp":   time.Now().Add(time.Hour).Unix(),
				}
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
				tokenString, _ := token.SignedString([]byte(secretKey))
				return tokenString
			},
			secretKey: secretKey,
			wantErr:   true,
			errorType: "invalid_claims",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := tt.setupFunc()
			claims, err := ValidateAccessToken(token, tt.secretKey)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, claims)
				
				// Check specific error type
				if tt.errorType != "" {
					appErr, ok := err.(*errors.AppError)
					require.True(t, ok, "Error should be of type AppError")
					assert.Contains(t, appErr.Details, tt.errorType)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, claims)
				assert.Equal(t, userID, claims.UserID)
				assert.Equal(t, email, claims.Email)
			}
		})
	}
}

func TestGenerateInvitationToken(t *testing.T) {
	secretKey := "test-secret-key-that-is-long-enough-for-testing"

	tests := []struct {
		name           string
		invitationID   string
		tripID         string
		inviteeEmail   string
		secretKey      string
		expiryDuration time.Duration
		wantErr        bool
	}{
		{
			name:           "Valid invitation token",
			invitationID:   "inv123",
			tripID:         "trip456",
			inviteeEmail:   "invitee@example.com",
			secretKey:      secretKey,
			expiryDuration: 24 * time.Hour,
			wantErr:        false,
		},
		{
			name:           "Empty invitation ID",
			invitationID:   "",
			tripID:         "trip456",
			inviteeEmail:   "invitee@example.com",
			secretKey:      secretKey,
			expiryDuration: 24 * time.Hour,
			wantErr:        false,
		},
		{
			name:           "Empty trip ID",
			invitationID:   "inv123",
			tripID:         "",
			inviteeEmail:   "invitee@example.com",
			secretKey:      secretKey,
			expiryDuration: 24 * time.Hour,
			wantErr:        false,
		},
		{
			name:           "Empty invitee email",
			invitationID:   "inv123",
			tripID:         "trip456",
			inviteeEmail:   "",
			secretKey:      secretKey,
			expiryDuration: 24 * time.Hour,
			wantErr:        false,
		},
		{
			name:           "Empty secret key",
			invitationID:   "inv123",
			tripID:         "trip456",
			inviteeEmail:   "invitee@example.com",
			secretKey:      "",
			expiryDuration: 24 * time.Hour,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := GenerateInvitationToken(tt.invitationID, tt.tripID, tt.inviteeEmail, tt.secretKey, tt.expiryDuration)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, token)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, token)

				// Verify token structure (JWT tokens are period-separated: header.payload.signature)
				parts := strings.Split(token, ".")
				assert.Len(t, parts, 3, "JWT should have 3 parts")
			}
		})
	}
}

func TestValidateInvitationToken(t *testing.T) {
	secretKey := "test-secret-key-that-is-long-enough-for-testing"
	invitationID := "inv123"
	tripID := "trip456"
	inviteeEmail := "invitee@example.com"

	tests := []struct {
		name      string
		setupFunc func() string
		secretKey string
		wantErr   bool
		errorType string
	}{
		{
			name: "Valid invitation token",
			setupFunc: func() string {
				token, _ := GenerateInvitationToken(invitationID, tripID, inviteeEmail, secretKey, 24*time.Hour)
				return token
			},
			secretKey: secretKey,
			wantErr:   false,
		},
		{
			name: "Expired invitation token",
			setupFunc: func() string {
				token, _ := GenerateInvitationToken(invitationID, tripID, inviteeEmail, secretKey, -time.Hour)
				return token
			},
			secretKey: secretKey,
			wantErr:   true,
			errorType: "token_expired",
		},
		{
			name: "Invalid signature",
			setupFunc: func() string {
				token, _ := GenerateInvitationToken(invitationID, tripID, inviteeEmail, secretKey, 24*time.Hour)
				return token
			},
			secretKey: "wrong-secret-key-that-is-long-enough",
			wantErr:   true,
			errorType: "invalid_signature",
		},
		{
			name: "Malformed token",
			setupFunc: func() string {
				return "not.a.valid.token"
			},
			secretKey: secretKey,
			wantErr:   true,
			errorType: "malformed_token",
		},
		{
			name: "Token with wrong claims type",
			setupFunc: func() string {
				// Create a regular JWT instead of invitation token
				token, _ := GenerateJWT("user123", "test@example.com", secretKey, time.Hour)
				return token
			},
			secretKey: secretKey,
			wantErr:   true,
			errorType: "invalid_claims",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := tt.setupFunc()
			claims, err := ValidateInvitationToken(token, tt.secretKey)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, claims)

				// Check specific error type
				if tt.errorType != "" {
					appErr, ok := err.(*errors.AppError)
					require.True(t, ok, "Error should be of type AppError")
					assert.Contains(t, appErr.Details, tt.errorType)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, claims)
				assert.Equal(t, invitationID, claims.InvitationID)
				assert.Equal(t, tripID, claims.TripID)
				assert.Equal(t, inviteeEmail, claims.InviteeEmail)
			}
		})
	}
}

func TestMapJWTError(t *testing.T) {
	tests := []struct {
		name          string
		inputError    error
		expectedType  string
		expectedCode  string
	}{
		{
			name:          "Nil error",
			inputError:    nil,
			expectedType:  "",
			expectedCode:  "",
		},
		{
			name:          "Token expired error",
			inputError:    jwt.ErrTokenExpired,
			expectedType:  errors.AuthError,
			expectedCode:  "token_expired",
		},
		{
			name:          "Signature invalid error",
			inputError:    jwt.ErrSignatureInvalid,
			expectedType:  errors.AuthError,
			expectedCode:  "invalid_signature",
		},
		{
			name:          "Token malformed error",
			inputError:    jwt.ErrTokenMalformed,
			expectedType:  errors.AuthError,
			expectedCode:  "malformed_token",
		},
		{
			name:          "Generic error",
			inputError:    jwt.ErrTokenInvalidClaims,
			expectedType:  errors.AuthError,
			expectedCode:  "invalid_token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mapJWTError(tt.inputError)

			if tt.inputError == nil {
				assert.Nil(t, err)
			} else {
				require.NotNil(t, err)
				appErr, ok := err.(*errors.AppError)
				require.True(t, ok, "Error should be of type AppError")
				assert.Equal(t, tt.expectedType, appErr.Type)
				assert.Contains(t, appErr.Details, tt.expectedCode)
			}
		})
	}
}

func TestTokenEdgeCases(t *testing.T) {
	secretKey := "test-secret-key-that-is-long-enough-for-testing"

	t.Run("Token with zero expiry time", func(t *testing.T) {
		// Generate token with zero duration (should create expired token)
		token, err := GenerateJWT("user123", "test@example.com", secretKey, 0)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Validation should fail due to expiry
		claims, err := ValidateAccessToken(token, secretKey)
		assert.Error(t, err)
		assert.Nil(t, claims)
	})

	t.Run("Token with very long expiry", func(t *testing.T) {
		// Generate token with 100 year expiry
		token, err := GenerateJWT("user123", "test@example.com", secretKey, 100*365*24*time.Hour)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Should validate successfully
		claims, err := ValidateAccessToken(token, secretKey)
		require.NoError(t, err)
		assert.NotNil(t, claims)
	})

	t.Run("Token with special characters in claims", func(t *testing.T) {
		userID := "user!@#$%^&*()"
		email := "test+special@example.com"
		
		token, err := GenerateJWT(userID, email, secretKey, time.Hour)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		claims, err := ValidateAccessToken(token, secretKey)
		require.NoError(t, err)
		assert.Equal(t, userID, claims.UserID)
		assert.Equal(t, email, claims.Email)
	})

	t.Run("Very long secret key", func(t *testing.T) {
		longSecret := string(make([]byte, 1024))
		for range longSecret {
			longSecret = "a"
		}

		token, err := GenerateJWT("user123", "test@example.com", longSecret, time.Hour)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		claims, err := ValidateAccessToken(token, longSecret)
		require.NoError(t, err)
		assert.NotNil(t, claims)
	})
}

func TestInvitationTokenClaims(t *testing.T) {
	secretKey := "test-secret-key-that-is-long-enough-for-testing"

	t.Run("Verify all invitation claims are present", func(t *testing.T) {
		invitationID := "inv123"
		tripID := "trip456"
		inviteeEmail := "invitee@example.com"
		
		token, err := GenerateInvitationToken(invitationID, tripID, inviteeEmail, secretKey, 24*time.Hour)
		require.NoError(t, err)

		// Parse and verify all claims
		parsedToken, err := jwt.ParseWithClaims(token, &types.InvitationClaims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(secretKey), nil
		})
		require.NoError(t, err)

		claims, ok := parsedToken.Claims.(*types.InvitationClaims)
		require.True(t, ok)
		
		// Verify all fields
		assert.Equal(t, invitationID, claims.InvitationID)
		assert.Equal(t, tripID, claims.TripID)
		assert.Equal(t, inviteeEmail, claims.InviteeEmail)
		assert.Equal(t, "nomadcrew-backend-invitation", claims.Issuer)
		assert.Equal(t, invitationID, claims.Subject)
		assert.NotEmpty(t, claims.ID)
		assert.True(t, claims.ExpiresAt.After(time.Now()))
		assert.True(t, claims.IssuedAt.Before(time.Now().Add(time.Minute)))
		assert.True(t, claims.NotBefore.Before(time.Now().Add(time.Minute)))
	})
}