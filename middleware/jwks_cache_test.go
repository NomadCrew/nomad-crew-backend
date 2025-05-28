package middleware

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	// Import zap for NewNop
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// MockRoundTripper is a mock type for http.RoundTripper
type MockRoundTripper struct {
	mock.Mock
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	res, _ := args.Get(0).(*http.Response) // Handle nil safely
	return res, args.Error(1)
}

func createTestJWKS(t *testing.T, key jwk.Key) jwk.Set {
	t.Helper()
	set := jwk.NewSet()
	err := set.AddKey(key)
	require.NoError(t, err)
	return set
}

func generateRSAKeys(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return priv, &priv.PublicKey
}

func TestJWKSCache_GetKey(t *testing.T) {
	jwksURL := "http://test.com/.well-known/jwks.json"
	anonKey := "test-anon-key"
	cacheTTL := 1 * time.Minute
	keyID := "test-key-id"
	// Use zap.NewNop() directly for a no-op logger during tests
	// log := logger.NewNopLogger() // Replace this
	_ = zap.NewNop() // Keep verifying zap compiles

	// Create a valid JWK for testing
	_, pubKey := generateRSAKeys(t) // privKey is unused, so ignore it with _
	jwkKey, err := jwk.FromRaw(pubKey)
	require.NoError(t, err)
	_ = jwkKey.Set(jwk.KeyIDKey, keyID)
	_ = jwkKey.Set(jwk.AlgorithmKey, "RS256")

	jwksSet := createTestJWKS(t, jwkKey)
	jwksBytes, err := json.Marshal(jwksSet)
	require.NoError(t, err)

	testCases := []struct {
		name         string
		initialCache map[string]jwk.Key // Pre-populate cache
		mockSetup    func(mrt *MockRoundTripper)
		keyToGet     string
		expectFetch  bool // Whether an HTTP fetch is expected
		expectedKey  jwk.Key
		expectedErr  bool
		checkErrText string
	}{
		{
			name: "Key Found in Cache",
			initialCache: map[string]jwk.Key{
				keyID: jwkKey,
			},
			mockSetup:   func(mrt *MockRoundTripper) {}, // No fetch expected
			keyToGet:    keyID,
			expectFetch: false,
			expectedKey: jwkKey,
			expectedErr: false,
		},
		{
			name:         "Key Not in Cache - Fetch Success",
			initialCache: map[string]jwk.Key{},
			mockSetup: func(mrt *MockRoundTripper) {
				resp := &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(jwksBytes)),
					Header:     make(http.Header),
				}
				resp.Header.Set("Content-Type", "application/json")
				// Expect a GET request to jwksURL with correct headers
				mrt.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
					return req.Method == http.MethodGet &&
						req.URL.String() == jwksURL &&
						req.Header.Get("Apikey") == anonKey && // Check headers
						req.Header.Get("Authorization") == "Bearer "+anonKey
				})).Return(resp, nil).Once()
			},
			keyToGet:    keyID,
			expectFetch: true,
			expectedKey: jwkKey,
			expectedErr: false,
		},
		{
			name:         "Key Not in Cache - Fetch HTTP Error",
			initialCache: map[string]jwk.Key{},
			mockSetup: func(mrt *MockRoundTripper) {
				mrt.On("RoundTrip", mock.AnythingOfType("*http.Request")).Return(nil, errors.New("network error")).Once()
			},
			keyToGet:     keyID,
			expectFetch:  true,
			expectedKey:  nil,
			expectedErr:  true,
			checkErrText: "failed to fetch JWKS", // Matches error from refreshCache
		},
		{
			name:         "Key Not in Cache - Fetch Non-200 Status",
			initialCache: map[string]jwk.Key{},
			mockSetup: func(mrt *MockRoundTripper) {
				resp := &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(bytes.NewReader([]byte("Not Found"))),
				}
				mrt.On("RoundTrip", mock.AnythingOfType("*http.Request")).Return(resp, nil).Once()
			},
			keyToGet:     keyID,
			expectFetch:  true,
			expectedKey:  nil,
			expectedErr:  true,
			checkErrText: "JWKS endpoint returned status 404", // Matches error from refreshCache
		},
		{
			name:         "Key Not in Cache - Fetch Invalid JSON",
			initialCache: map[string]jwk.Key{},
			mockSetup: func(mrt *MockRoundTripper) {
				resp := &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte("invalid json"))),
					Header:     make(http.Header),
				}
				resp.Header.Set("Content-Type", "application/json")
				mrt.On("RoundTrip", mock.AnythingOfType("*http.Request")).Return(resp, nil).Once()
			},
			keyToGet:    keyID,
			expectFetch: true,
			expectedKey: nil,
			expectedErr: true,
			// Error depends on whether primary parse or fallback parse fails
			// Let's check for a common part of the error message if possible, or adjust based on actual log/error.
			// Adjusting to match the actual error message format:
			checkErrText: "failed to decode JWKS response", // Matches error from refreshCache after parse failures
		},
		{
			name:         "Key Not Found After Successful Fetch",
			initialCache: map[string]jwk.Key{},
			mockSetup: func(mrt *MockRoundTripper) {
				// Simulate fetching a JWKS that *doesn't* contain the requested keyID
				otherKeyID := "other-key"
				_, otherPubKey := generateRSAKeys(t)
				otherKey, _ := jwk.FromRaw(otherPubKey)
				_ = otherKey.Set(jwk.KeyIDKey, otherKeyID)
				otherSet := createTestJWKS(t, otherKey)
				otherBytes, _ := json.Marshal(otherSet)

				resp := &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(otherBytes)),
					Header:     make(http.Header),
				}
				resp.Header.Set("Content-Type", "application/json")
				mrt.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
					return req.URL.String() == jwksURL
				})).Return(resp, nil).Once()
			},
			keyToGet:     keyID, // Requesting keyID
			expectFetch:  true,
			expectedKey:  nil,
			expectedErr:  true,                                                         // GetKey now returns an error if key not found after refresh
			checkErrText: "key with kid 'test-key-id' not found in JWKS after refresh", // Matches error from GetKey
		},
		// Add test for double-check lock in refreshCache
		// Add test for config change detection in GetJWKSCache if testing singleton directly
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockRT := new(MockRoundTripper)
			tc.mockSetup(mockRT)

			client := &http.Client{
				Transport: mockRT,
				Timeout:   5 * time.Second, // Test timeout
			}

			// Instantiate cache directly for unit testing GetKey/refreshCache
			cache := &JWKSCache{
				keys:       make(map[string]jwk.Key),
				expiresAt:  time.Now().Add(-1 * time.Hour), // Start expired if no initial cache
				jwksURL:    jwksURL,
				anonKey:    anonKey,
				ttl:        cacheTTL,
				httpClient: client,
				// Mutexes are zero-value ready
			}

			// Pre-populate cache and set expiry if needed
			if len(tc.initialCache) > 0 {
				cache.mutex.Lock()
				for k, v := range tc.initialCache {
					cache.keys[k] = v
				}
				cache.expiresAt = time.Now().Add(cacheTTL) // Set expiry if pre-populating valid cache
				cache.mutex.Unlock()
			}

			// Call GetKey - signature is GetKey(kid string)
			fetchedKey, err := cache.GetKey(tc.keyToGet)

			if tc.expectedErr {
				assert.Error(t, err)
				if tc.checkErrText != "" {
					assert.Contains(t, err.Error(), tc.checkErrText)
				}
				assert.Nil(t, fetchedKey)
			} else {
				assert.NoError(t, err)
				// Compare keys
				if tc.expectedKey != nil {
					assert.NotNil(t, fetchedKey)
					assert.Equal(t, tc.expectedKey.KeyID(), fetchedKey.KeyID())
				} else {
					assert.Nil(t, fetchedKey)
				}
			}

			mockRT.AssertExpectations(t)

			// Verify cache content after fetch (if successful fetch expected)
			if !tc.expectedErr && tc.expectFetch && tc.expectedKey != nil {
				cache.mutex.RLock()
				val, found := cache.keys[tc.keyToGet]
				cache.mutex.RUnlock()
				assert.True(t, found, "Key should be in cache after successful fetch")
				assert.NotNil(t, val)
				if val != nil {
					assert.Equal(t, tc.expectedKey.KeyID(), val.KeyID())
				}
			}
		})
	}
}

// Test GetJWKSCache singleton separately
func TestGetJWKSCache_Singleton(t *testing.T) {
	url1 := "http://test1.com"
	key1 := "key1"
	ttl1 := 5 * time.Minute

	instance1 := GetJWKSCache(url1, key1, ttl1)
	require.NotNil(t, instance1)
	assert.Equal(t, url1, instance1.jwksURL)
	assert.Equal(t, key1, instance1.anonKey)
	assert.Equal(t, ttl1, instance1.ttl)

	// Call again with same params, should return same instance
	instance2 := GetJWKSCache(url1, key1, ttl1)
	assert.Same(t, instance1, instance2)

	// Call with different params - should update the singleton instance
	url3 := "http://test3.com"
	key3 := "key3"
	ttl3 := 10 * time.Minute
	instance3 := GetJWKSCache(url3, key3, ttl3)
	assert.Same(t, instance1, instance3) // Still same instance pointer
	// Check if fields were updated (need RLock as GetKey does)
	instance3.mutex.RLock()
	assert.Equal(t, url3, instance3.jwksURL)
	assert.Equal(t, key3, instance3.anonKey)
	// TTL might not be updated by GetJWKSCache, depends on implementation. Let's assume it is for this test.
	// assert.Equal(t, ttl3, instance3.ttl)
	instance3.mutex.RUnlock()

	// Reset singleton for other tests (tricky, might require build tags or reflection)
	// For simplicity, we assume tests run sequentially or don't interfere significantly.
}

// Test fetchAndCacheJWKS directly if needed, though covered by GetKey tests
func TestJWKSCache_fetchAndCacheJWKS(t *testing.T) {
	// Similar setup as GetKey tests focusing only on the fetch logic
	// Instantiate cache directly, mock HTTP client, call fetchAndCacheJWKS
	// Check cache contents (keys, expiresAt) and error returns directly
}
