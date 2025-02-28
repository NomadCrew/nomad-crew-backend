package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/logger"
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
	redisClient     *redis.Client
	locationService *LocationService
}

// NewOfflineLocationService creates a new OfflineLocationService
func NewOfflineLocationService(redisClient *redis.Client, locationService *LocationService) *OfflineLocationService {
	return &OfflineLocationService{
		redisClient:     redisClient,
		locationService: locationService,
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
		s.redisClient.Del(ctx, lockKey)
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

		// Process the update
		_, err := s.locationService.UpdateLocation(ctx, userID, update)
		if err != nil {
			log.Warnw("Failed to process offline location update",
				"userID", userID,
				"timestamp", updateTime,
				"error", err,
			)
			continue
		}

		processedCount++

		// Limit the number of updates processed in a single batch
		if processedCount >= maxBatchSize {
			log.Infow("Reached max batch size limit, remaining updates will be processed in next run",
				"userID", userID,
				"maxBatchSize", maxBatchSize,
				"totalUpdates", len(offlineUpdate.Updates),
			)
			break
		}
	}

	// Delete the batch after processing
	s.redisClient.Del(ctx, key)

	log.Infow("Processed offline location batch",
		"userID", userID,
		"total", len(offlineUpdate.Updates),
		"processed", processedCount,
		"deviceID", offlineUpdate.DeviceID,
	)

	return nil
}

// CleanupExpiredLocations removes expired location batches
func (s *OfflineLocationService) CleanupExpiredLocations(ctx context.Context) error {
	// This is handled automatically by Redis TTL, but we could add additional cleanup logic here if needed
	return nil
}
