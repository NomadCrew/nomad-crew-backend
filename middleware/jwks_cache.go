package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/lestrrat-go/jwx/v2/jwk"
)

// JWKSCache is a thread-safe cache for JWKS keys.
type JWKSCache struct {
	keys        map[string]jwk.Key // kid -> key mapping
	expiresAt   time.Time
	mutex       sync.RWMutex
	jwksURL     string
	anonKey     string        // Supabase Anon Key for fetching JWKS
	ttl         time.Duration // Cache duration
	refreshLock sync.Mutex    // Lock specifically for refresh operation
	httpClient  *http.Client
}

var (
	jwksCacheInstance *JWKSCache
	jwksCacheOnce     sync.Once
)

// GetJWKSCache initializes and returns a singleton instance of the JWKS cache.
// Configuration parameters (URL, Anon Key, TTL) must be provided on first call.
func GetJWKSCache(jwksURL, anonKey string, ttl time.Duration) *JWKSCache {
	jwksCacheOnce.Do(func() {
		log := logger.GetLogger()
		log.Infow("Initializing JWKS Cache singleton",
			"url", jwksURL,
			"ttl", ttl,
			"anon_key_present", anonKey != "")

		jwksCacheInstance = &JWKSCache{
			keys:      make(map[string]jwk.Key),
			expiresAt: time.Now(), // Expire immediately to force initial fetch
			jwksURL:   jwksURL,
			anonKey:   anonKey,
			ttl:       ttl,
			httpClient: &http.Client{
				Timeout: 10 * time.Second, // Increased timeout slightly
			},
		}
		// Attempt an initial fetch to populate the cache on startup? Optional.
		// _, err := jwksCacheInstance.refreshCache("") // Refresh all keys initially
		// if err != nil {
		//  log.Errorw("Initial JWKS cache refresh failed", "error", err)
		// }
	})

	// Update config if it changed (e.g., hot reload scenario, though rare for singletons)
	jwksCacheInstance.mutex.Lock()
	if jwksCacheInstance.jwksURL != jwksURL || jwksCacheInstance.anonKey != anonKey {
		jwksCacheInstance.jwksURL = jwksURL
		jwksCacheInstance.anonKey = anonKey
		// Force refresh on config change might be desired
		jwksCacheInstance.expiresAt = time.Now()
	}
	jwksCacheInstance.mutex.Unlock()

	return jwksCacheInstance
}

// GetKey returns a key by its ID (kid), fetching/refreshing the JWKS if necessary.
func (c *JWKSCache) GetKey(kid string) (jwk.Key, error) {
	log := logger.GetLogger()

	// Check cache with read lock
	c.mutex.RLock()
	key, found := c.keys[kid]
	isExpired := time.Now().After(c.expiresAt)
	c.mutex.RUnlock()

	if found && !isExpired {
		log.Debugw("Using cached JWKS key", "kid", kid)
		return key, nil
	}

	// Cache miss or expired, need to refresh (acquires full lock)
	log.Infow("JWKS Cache miss or expired, attempting refresh", "kid", kid, "expired", isExpired)
	refreshedKey, err := c.refreshCache(kid)
	if err != nil {
		// If refresh failed, maybe return the stale key if found? Or just error.
		// Currently just errors out.
		log.Errorw("Failed to refresh JWKS cache", "kid", kid, "error", err)
		return nil, fmt.Errorf("failed to refresh JWKS cache for kid %s: %w", kid, err)
	}

	// Check if the specific key was found during refresh
	if refreshedKey == nil && kid != "" {
		log.Warnw("JWKS cache refreshed, but target key not found", "kid", kid)
		// Attempt to read cache again in case another concurrent refresh got it
		c.mutex.RLock()
		key, found = c.keys[kid]
		c.mutex.RUnlock()
		if !found {
			return nil, fmt.Errorf("key with kid '%s' not found in JWKS after refresh", kid)
		}
		// Fallthrough to return the key found by another goroutine's refresh
	} else if refreshedKey != nil {
		key = refreshedKey // Use the key found directly by this refresh
	}

	log.Debugw("Returning key after JWKS cache refresh", "kid", kid)
	return key, nil
}

// refreshCache fetches the latest keys from the JWKS endpoint.
// It uses a separate refreshLock to prevent multiple concurrent fetches.
// The targetKid is used for logging/early exit if found, but all keys are cached.
func (c *JWKSCache) refreshCache(targetKid string) (jwk.Key, error) {
	log := logger.GetLogger()

	// Use a dedicated lock for the refresh operation to prevent stampede
	c.refreshLock.Lock()
	defer c.refreshLock.Unlock()

	// Double-check if cache is still expired after acquiring refresh lock
	// Another goroutine might have refreshed it while we waited for the lock.
	c.mutex.RLock()
	isExpired := time.Now().After(c.expiresAt)
	if !isExpired {
		// Cache was refreshed by another goroutine, try reading the target key again
		key, found := c.keys[targetKid]
		c.mutex.RUnlock()
		log.Infow("JWKS cache already refreshed by another goroutine", "kid", targetKid)
		if found {
			return key, nil
		}
		// If target key still not found, return nil (GetKey will handle the error msg)
		return nil, nil
	}
	c.mutex.RUnlock() // Release read lock before network call

	log.Infow("Refreshing JWKS cache",
		"url", c.jwksURL,
		"target_kid", targetKid)

	if c.jwksURL == "" || c.anonKey == "" {
		return nil, fmt.Errorf("JWKS URL or Anon Key is not configured in the cache")
	}

	req, err := http.NewRequest("GET", c.jwksURL, nil)
	if err != nil {
		log.Errorw("Failed to create JWKS request", "error", err, "url", c.jwksURL)
		return nil, fmt.Errorf("failed to create JWKS request: %w", err)
	}

	// Add required headers for Supabase JWKS endpoint
	req.Header.Set("apikey", c.anonKey)
	req.Header.Set("Authorization", "Bearer "+c.anonKey)

	log.Debugw("Sending JWKS request", "url", c.jwksURL)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Errorw("Failed to fetch JWKS", "error", err, "url", c.jwksURL)
		return nil, fmt.Errorf("failed to fetch JWKS from %s: %w", c.jwksURL, err)
	}
	defer resp.Body.Close()

	log.Infow("JWKS response received", "status", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("JWKS endpoint returned status %d", resp.StatusCode)
		log.Errorw("JWKS endpoint returned non-200 status",
			"status", resp.StatusCode, "url", c.jwksURL, "response", string(bodyBytes))
		return nil, fmt.Errorf("%s: %s", errMsg, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorw("Failed to read JWKS response body", "error", err)
		return nil, fmt.Errorf("failed to read JWKS response body: %w", err)
	}

	// Use jwk.Parse instead of custom struct
	keySet, err := jwk.Parse(bodyBytes)
	if err != nil {
		log.Errorw("Failed to parse JWKS response using jwk.Parse", "error", err)
		// Fallback: Try the simpler struct approach if jwk.Parse fails (less robust)
		var simpleJwksResp struct {
			Keys []json.RawMessage `json:"keys"` // Parse as raw first
		}
		if errJson := json.Unmarshal(bodyBytes, &simpleJwksResp); errJson != nil {
			log.Errorw("Failed to decode JWKS response even with simple struct", "error", errJson)
			return nil, fmt.Errorf("failed to decode JWKS response: %w", errJson) // Return the JSON error
		}
		log.Warnw("Parsed JWKS using simple struct fallback", "key_count", len(simpleJwksResp.Keys))
		// Manually parse each key if needed, or rely on jwk.LookupKeyID below if possible
		// This fallback is complex, prefer fixing jwk.Parse if possible.
		// For now, we'll just proceed and hope LookupKeyID works or let it fail.
		// A better fallback might try parsing individual keys.
		keySet, err = jwk.Parse(bodyBytes) // Try parsing again to populate keySet for LookupKeyID
		if err != nil {
			log.Errorw("Failed to parse JWKS response even after fallback attempt", "error", err)
			return nil, fmt.Errorf("failed to parse JWKS keys: %w", err)
		}

	}

	newKeys := make(map[string]jwk.Key)
	var foundTargetKey jwk.Key

	log.Infow("Processing JWKS keys", "key_count", keySet.Len())
	it := keySet.Keys(context.Background()) // Use iterator
	for it.Next(context.Background()) {
		pair := it.Pair()
		key := pair.Value.(jwk.Key)
		kid := key.KeyID()
		if kid == "" {
			log.Warnw("Found JWK without a 'kid', skipping")
			continue
		}
		newKeys[kid] = key
		log.Debugw("Cached JWK", "kid", kid, "alg", key.Algorithm())
		if kid == targetKid {
			foundTargetKey = key
		}
	}

	// Update cache under write lock
	c.mutex.Lock()
	c.keys = newKeys
	c.expiresAt = time.Now().Add(c.ttl)
	c.mutex.Unlock()

	log.Infow("JWKS cache refreshed successfully",
		"keys_cached", len(newKeys),
		"target_kid_found", foundTargetKey != nil,
		"cache_expires_at", c.expiresAt.Format(time.RFC3339))

	// Return the specific key if it was the target and found during this refresh
	return foundTargetKey, nil
}
