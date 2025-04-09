package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	// Removed locationSvc alias as interface is now in the same package
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	// Redis key prefixes
	offlineLocationQueuePrefix = "offline_location_queue:"
	offlineLocationLockPrefix  = "offline_location_lock:"

	// Lock duration for processing offline locations
	offlineLocationLockDuration = 5 * time.Minute

	// Maximum number of location updates to process in a single batch
	maxBatchSize = 100

	// Maximum age of location updates to process (older ones will be discarded)
	maxLocationAge = 24 * time.Hour
)

// OfflineLocationService handles offline location updates
type OfflineLocationService struct {
	redisClient *redis.Client
	// Use the interface from the refactored service package
	locationService OfflineLocationServiceInterface // Use interface from same package
}

// NewOfflineLocationService creates a new OfflineLocationService
// Accept the interface type, allow nil initially for dependency cycle resolution
func NewOfflineLocationService(redisClient *redis.Client, locService OfflineLocationServiceInterface) *OfflineLocationService { // Use interface from same package
	return &OfflineLocationService{
		redisClient:     redisClient,
		locationService: locService,
	}
}

// SetLocationService allows setting the location service after initialization
// to resolve circular dependencies.
func (s *OfflineLocationService) SetLocationService(locService OfflineLocationServiceInterface) { // Use interface from same package
	if s.locationService == nil && locService != nil {
		s.locationService = locService
		logger.GetLogger().Info("OfflineLocationService: Location service dependency injected.")
	} else if locService == nil {
		logger.GetLogger().Warn("OfflineLocationService: Attempted to inject nil location service.")
	} else {
		logger.GetLogger().Warn("OfflineLocationService: Location service dependency already set.")
	}
}

// SaveOfflineLocations saves a batch of location updates to the queue
func (s *OfflineLocationService) SaveOfflineLocations(ctx context.Context, userID string, updates []types.LocationUpdate, deviceID string) error {
	log := logger.GetLogger()

	if len(updates) == 0 {
		return nil
	}

	// Create offline location update object
	offlineUpdate := types.OfflineLocationUpdate{
		UserID:    userID,
		Updates:   updates,
		DeviceID:  deviceID,
		CreatedAt: time.Now(),
	}

	// Serialize to JSON
	data, err := json.Marshal(offlineUpdate)
	if err != nil {
		log.Errorw("Failed to marshal offline location update", "userID", userID, "error", err)
		return fmt.Errorf("failed to marshal offline location update: %w", err)
	}

	// Generate a unique key for this batch
	queueKey := fmt.Sprintf("%s%s:%s", offlineLocationQueuePrefix, userID, uuid.New().String())

	// Store in Redis with expiration (keep for 24 hours)
	err = s.redisClient.Set(ctx, queueKey, data, 24*time.Hour).Err()
	if err != nil {
		log.Errorw("Failed to save offline location updates to Redis", "userID", userID, "error", err)
		return fmt.Errorf("failed to save offline location updates: %w", err)
	}

	log.Infow("Saved offline location updates",
		"userID", userID,
		"count", len(updates),
		"deviceID", deviceID,
		"queueKey", queueKey,
	)

	return nil
}

// ProcessOfflineLocations processes all offline location updates for a user
func (s *OfflineLocationService) ProcessOfflineLocations(ctx context.Context, userID string) error {
	log := logger.GetLogger()

	// Check if locationService is set
	if s.locationService == nil {
		log.Errorw("Location service dependency not set in OfflineLocationService", "userID", userID)
		return fmt.Errorf("internal configuration error: location service not available")
	}

	// Try to acquire a lock to prevent concurrent processing
	lockKey := fmt.Sprintf("%s%s", offlineLocationLockPrefix, userID)
	lockValue := uuid.New().String()

	// Set lock with NX option (only if it doesn't exist)
	locked, err := s.redisClient.SetNX(ctx, lockKey, lockValue, offlineLocationLockDuration).Result()
	if err != nil {
		log.Errorw("Failed to check lock for offline location processing", "userID", userID, "error", err)
		return fmt.Errorf("failed to check lock: %w", err)
	}

	if !locked {
		log.Infow("Offline location processing already in progress", "userID", userID)
		return nil // Another process is already handling this user's updates
	}

	// Ensure lock is released when done
	defer func() {
		// Only delete the lock if it's still our lock
		// Use Lua script for atomic check-and-delete
		script := redis.NewScript(`
			if redis.call("get", KEYS[1]) == ARGV[1] then
				return redis.call("del", KEYS[1])
			else
				return 0
			end
		`)
		_, err := script.Run(ctx, s.redisClient, []string{lockKey}, lockValue).Result()
		if err != nil {
			log.Errorw("Failed to release offline location processing lock", "userID", userID, "error", err)
		}
	}()

	// Get all queue keys for this user
	pattern := fmt.Sprintf("%s%s:*", offlineLocationQueuePrefix, userID)
	keys, err := s.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		log.Errorw("Failed to get offline location queue keys", "userID", userID, "error", err)
		return fmt.Errorf("failed to get queue keys: %w", err)
	}

	if len(keys) == 0 {
		log.Infow("No offline location updates to process", "userID", userID)
		return nil
	}

	log.Infow("Processing offline location updates", "userID", userID, "batchCount", len(keys))

	// Process each batch
	for _, key := range keys {
		if err := s.processBatch(ctx, userID, key); err != nil {
			log.Errorw("Failed to process offline location batch",
				"userID", userID,
				"key", key,
				"error", err,
			)
			// Continue with other batches even if one fails
			continue
		}
	}

	return nil
}

// processBatch processes a single batch of offline location updates
func (s *OfflineLocationService) processBatch(ctx context.Context, userID string, key string) error {
	log := logger.GetLogger()

	// Check if locationService is set (double check)
	if s.locationService == nil {
		return fmt.Errorf("internal configuration error: location service not available during batch processing")
	}

	// Get the batch data
	data, err := s.redisClient.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			// Key was deleted by another process, just skip
			return nil
		}
		return fmt.Errorf("failed to get batch data: %w", err)
	}

	// Parse the batch
	var offlineUpdate types.OfflineLocationUpdate
	if err := json.Unmarshal(data, &offlineUpdate); err != nil {
		// Delete invalid data
		s.redisClient.Del(ctx, key)
		return fmt.Errorf("failed to unmarshal batch data: %w", err)
	}

	// Verify user ID matches (security check)
	if offlineUpdate.UserID != userID {
		s.redisClient.Del(ctx, key)
		return fmt.Errorf("user ID mismatch in offline location batch")
	}

	// Check if batch is too old
	if time.Since(offlineUpdate.CreatedAt) > maxLocationAge {
		log.Infow("Discarding old offline location batch",
			"userID", userID,
			"age", time.Since(offlineUpdate.CreatedAt),
		)
		s.redisClient.Del(ctx, key)
		return nil
	}

	// Process each location update in the batch
	processedCount := 0
	for _, update := range offlineUpdate.Updates {
		// Skip updates that are too old
		updateTime := time.UnixMilli(update.Timestamp)
		if time.Since(updateTime) > maxLocationAge {
			continue
		}

		// Process the update using the interface method
		// Note: The UpdateLocation method itself is part of the locationSvc.ManagementService,
		// not the locationSvc.OfflineLocationServiceInterface. We need the concrete type here or a wider interface.
		// Assuming locationService holds the concrete *locationSvc.ManagementService instance
		// This dependency needs careful handling. Let's assume for now it calls the correct method.
		// TODO: Revisit this dependency structure if issues arise.
		if locSvcImpl, ok := s.locationService.(*ManagementService); ok {
			_, err := locSvcImpl.UpdateLocation(ctx, userID, update)
			if err != nil {
				log.Warnw("Failed to process offline location update",
					"userID", userID,
					"timestamp", updateTime,
					"error", err,
				)
				continue
			}
		} else {
			log.Errorw("OfflineLocationService: locationService is not of expected type *locationSvc.ManagementService", "type", fmt.Sprintf("%T", s.locationService))
			// Skip this update as we cannot call the required method
			continue
		}
		processedCount++
	}

	log.Infow("Processed offline location batch",
		"userID", userID,
		"key", key,
		"processedCount", processedCount,
		"totalInBatch", len(offlineUpdate.Updates),
	)

	// Delete the batch key after successful processing
	if err := s.redisClient.Del(ctx, key).Err(); err != nil {
		log.Warnw("Failed to delete processed offline location batch key", "key", key, "error", err)
		// Log but don't return error, processing was successful
	}

	return nil
}

// CleanupExpiredLocations (Optional) - could be run periodically
// to remove very old batches that might not have been processed
// due to errors or long offline periods.
func (s *OfflineLocationService) CleanupExpiredLocations(ctx context.Context) error {
	log := logger.GetLogger()
	// Consider a more efficient way than scanning all keys if needed
	pattern := fmt.Sprintf("%s*", offlineLocationQueuePrefix)
	keys, err := s.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		log.Errorw("Failed to get keys for cleanup", "error", err)
		return err
	}

	// In a real-world scenario, you might want to process these in smaller chunks
	// or use SCAN for large numbers of keys.
	for _, key := range keys {
		data, err := s.redisClient.Get(ctx, key).Bytes()
		if err != nil {
			continue // Skip if key disappeared or error occurred
		}
		var offlineUpdate types.OfflineLocationUpdate
		if err := json.Unmarshal(data, &offlineUpdate); err == nil {
			if time.Since(offlineUpdate.CreatedAt) > (maxLocationAge + 24*time.Hour) { // Example: Remove if older than max age + 1 day
				log.Infow("Cleaning up very old offline location batch", "key", key)
				s.redisClient.Del(ctx, key)
			}
		}
	}
	return nil
}
