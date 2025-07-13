package services

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiterInterface defines the contract for rate limiting operations.
type RateLimiterInterface interface {
	CheckLimit(ctx context.Context, key string, limit int, duration time.Duration) (bool, time.Duration, error)
}

// RateLimitService provides rate limiting functionality using Redis.
// It implements the RateLimiterInterface.
type RateLimitService struct {
	redis     *redis.Client
	keyPrefix string
}

func NewRateLimitService(redis *redis.Client) *RateLimitService {
	return &RateLimitService{
		redis:     redis,
		keyPrefix: "rate_limit:",
	}
}

func (s *RateLimitService) GetRedisClient() *redis.Client {
	return s.redis
}

func (s *RateLimitService) CheckLimit(ctx context.Context, key string, limit int, duration time.Duration) (bool, time.Duration, error) {
	rKey := s.keyPrefix + key

	pipe := s.redis.Pipeline()
	incr := pipe.Incr(ctx, rKey)
	pipe.Expire(ctx, rKey, duration)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, err
	}

	count := incr.Val()
	if count > int64(limit) {
		ttl, err := s.redis.TTL(ctx, rKey).Result()
		if err != nil {
			return false, 0, err
		}
		return false, ttl, nil
	}

	return true, 0, nil
}
