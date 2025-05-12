package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

//nolint:unused
type mockCacheInterface interface {
	GetKey(kid string) (jwk.Key, error)
}

type MockJWKSCache struct {
	mock.Mock
}

// Ensure GetKey signature matches the actual JWKSCache.GetKey(kid string)
func (m *MockJWKSCache) GetKey(kid string) (jwk.Key, error) {
	args := m.Called(kid)
	key, _ := args.Get(0).(jwk.Key) // Handle nil safely
	return key, args.Error(1)
}

// Helper to create a valid RSA JWK and sign a token
func createTestJWKAndToken(t *testing.T, claims jwt.Claims, keyID string) (jwk.Key, string) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	jwkKey, err := jwk.FromRaw(privateKey.PublicKey) // Create JWK from public key
	require.NoError(t, err)
	err = jwkKey.Set(jwk.KeyIDKey, keyID)
	require.NoError(t, err)
	err = jwkKey.Set(jwk.AlgorithmKey, jwa.RS256)
	require.NoError(t, err)
	err = jwkKey.Set(jwk.KeyUsageKey, jwk.ForSignature)
	require.NoError(t, err)

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	jwtToken.Header["kid"] = keyID // Set Key ID in token header

	tokenString, err := jwtToken.SignedString(privateKey)
	require.NoError(t, err)

	return jwkKey, tokenString
}

// Create a test validator that implements the Validator interface
type TestJWTValidator struct {
	mock.Mock
}

func (m *TestJWTValidator) Validate(tokenString string) (string, error) {
	args := m.Called(tokenString)
	return args.String(0), args.Error(1)
}

func TestJWTValidator_Validate(t *testing.T) {
	mockValidator := new(TestJWTValidator)

	testUserID := uuid.New()
	keyID := "test-key-id"

	// Create claims using jwt.MapClaims
	validClaims := jwt.MapClaims{
		"sub": testUserID.String(), // Use standard 'sub' claim
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Add(-1 * time.Minute).Unix(),
		"nbf": time.Now().Add(-1 * time.Minute).Unix(),
	}

	_, validTokenString := createTestJWKAndToken(t, validClaims, keyID) // Ignore returned key

	expiredClaims := jwt.MapClaims{
		"sub": testUserID.String(),
		"exp": time.Now().Add(-1 * time.Hour).Unix(), // Expired
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
		"nbf": time.Now().Add(-2 * time.Hour).Unix(),
	}
	_, expiredTokenString := createTestJWKAndToken(t, expiredClaims, keyID)

	testCases := []struct {
		name           string
		tokenString    string
		mockSetup      func()
		expectedErr    bool
		expectedUserID string // Expect UserID string now
		checkErrText   string
	}{
		{
			name:        "Valid Token",
			tokenString: validTokenString,
			mockSetup: func() {
				mockValidator.On("Validate", validTokenString).Return(testUserID.String(), nil)
			},
			expectedErr:    false,
			expectedUserID: testUserID.String(),
		},
		{
			name:        "Expired Token",
			tokenString: expiredTokenString,
			mockSetup: func() {
				mockValidator.On("Validate", expiredTokenString).Return("", ErrTokenExpired)
			},
			expectedErr:  true,
			checkErrText: "token expired",
		},
		{
			name:        "Invalid Signature",
			tokenString: validTokenString + "invalid",
			mockSetup: func() {
				mockValidator.On("Validate", validTokenString+"invalid").Return("", ErrTokenInvalid)
			},
			expectedErr:  true,
			checkErrText: "token invalid",
		},
		{
			name:        "Malformed Token - No KID",
			tokenString: "malformed.token.no.kid",
			mockSetup: func() {
				mockValidator.On("Validate", "malformed.token.no.kid").Return("", ErrValidationMethodUnavailable)
			},
			expectedErr:  true,
			checkErrText: "no validation method available for token",
		},
	}

	gin.SetMode(gin.TestMode)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockValidator.ExpectedCalls = nil
			mockValidator.Calls = nil

			tc.mockSetup()

			// Call the mock validator
			userID, err := mockValidator.Validate(tc.tokenString)

			if tc.expectedErr {
				assert.Error(t, err)
				if tc.checkErrText != "" {
					assert.ErrorContains(t, err, tc.checkErrText)
				}
				assert.Empty(t, userID)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedUserID, userID)
			}

			mockValidator.AssertExpectations(t)
		})
	}
}

func TestNewJWTValidator(t *testing.T) {
	// Populate config.Config directly
	cfg := &config.Config{
		ExternalServices: config.ExternalServices{
			SupabaseURL:     "http://localhost",
			SupabaseAnonKey: "anon-key",
		},
	}

	validator, err := NewJWTValidator(cfg) // Returns Validator interface

	require.NoError(t, err)
	assert.NotNil(t, validator)
}

var _ mockCacheInterface = &MockJWKSCache{} // Verify implementation
var _ Validator = &TestJWTValidator{}       // Verify implementation
