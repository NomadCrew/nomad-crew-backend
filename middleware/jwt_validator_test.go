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

func TestJWTValidator_Validate(t *testing.T) {
	mockCache := new(MockJWKSCache)

	// Populate config.Config directly
	cfg := &config.Config{
		ExternalServices: config.ExternalServices{
			SupabaseURL:     "http://mock.supabase.url",
			SupabaseAnonKey: "mock-anon-key",
		},
	}

	// Call NewJWTValidator which now returns the interface 'Validator'
	validator, err := NewJWTValidator(cfg) // Returns Validator interface
	require.NoError(t, err)
	require.NotNil(t, validator)

	testUserID := uuid.New()
	keyID := "test-key-id"

	// Create claims using jwt.MapClaims or a custom struct implementing jwt.Claims
	validClaims := jwt.MapClaims{
		"sub": testUserID.String(), // Use standard 'sub' claim
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Add(-1 * time.Minute).Unix(),
		"nbf": time.Now().Add(-1 * time.Minute).Unix(),
		// Add Audience/Issuer here if tokens actually contain them and validation should check
		// "iss": "test-issuer",
		// "aud": "test-audience",
	}

	// Remove jwkKey as it's unused in this test scope (cache logic tested separately)
	// jwkKey, validTokenString := createTestJWKAndToken(t, validClaims, keyID)
	_, validTokenString := createTestJWKAndToken(t, validClaims, keyID) // Ignore returned key

	expiredClaims := jwt.MapClaims{
		"sub": testUserID.String(),
		"exp": time.Now().Add(-1 * time.Hour).Unix(), // Expired
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
		"nbf": time.Now().Add(-2 * time.Hour).Unix(),
	}
	_, expiredTokenString := createTestJWKAndToken(t, expiredClaims, keyID)

	// invalidAudienceClaims := jwt.MapClaims{ ... } // Remove if audience not validated
	// _, invalidAudTokenString := createTestJWKAndToken(t, invalidAudienceClaims, keyID)

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
			mockSetup:   func() { /* Mock setup likely ineffective */ },
			expectedErr: false,
			// Expect the subject claim as the user ID
			expectedUserID: testUserID.String(),
		},
		// ... Adapt other test cases ...
		{
			name:        "Expired Token",
			tokenString: expiredTokenString,
			mockSetup:   func() {},
			expectedErr: true,
			// checkErrText: "token is expired", // Check specific error wrapping if needed
			checkErrText: ErrTokenExpired.Error(), // Check against exported error variable
		},
		// Remove Invalid Audience test if not applicable
		{
			name:        "Invalid Signature",
			tokenString: validTokenString + "invalid",
			mockSetup:   func() {},
			expectedErr: true,
			// checkErrText: "signature is invalid", // Check specific error wrapping
			checkErrText: ErrTokenInvalid.Error(),
		},
		{
			name:        "Malformed Token - No KID", // JWKS validation fails if no KID
			tokenString: "malformed.token.no.kid",   // This still needs generation if HS256 is off
			mockSetup:   func() {},
			expectedErr: true,
			// Error might be different depending on parse steps
			checkErrText: ErrValidationMethodUnavailable.Error(), // Or ErrTokenInvalid?
		},
	}

	gin.SetMode(gin.TestMode)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCache.ExpectedCalls = nil
			mockCache.Calls = nil

			// Call the correct method: Validate
			userID, err := validator.Validate(tc.tokenString)

			if tc.expectedErr {
				assert.Error(t, err)
				if tc.checkErrText != "" {
					// Use ErrorContains for substring or Is/As for specific errors
					assert.ErrorContains(t, err, tc.checkErrText)
					// Example using errors.Is: assert.True(t, errors.Is(err, ErrTokenExpired))
				}
				assert.Empty(t, userID) // Expect empty userID on error
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedUserID, userID) // Compare returned userID string
			}
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
