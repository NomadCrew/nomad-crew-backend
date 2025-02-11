package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/redis/go-redis/v9"
)

// RedisEventService implements the EventPublisher interface using Redis Pub/Sub.
type RedisEventService struct {
	redisClient *redis.Client
}

// NewRedisEventService returns a new instance of RedisEventService.
func NewRedisEventService(redisClient *redis.Client) *RedisEventService {
	logger.GetLogger().Infow("Initializing Redis event service",
		"redisAddress", redisClient.Options().Addr,
		"dbNumber", redisClient.Options().DB)
	return &RedisEventService{
		redisClient: redisClient,
	}
}

// Publish serializes the event and publishes it on a Redis channel.
// The channel is named "trip:{tripID}".
func (r *RedisEventService) Publish(ctx context.Context, tripID string, event types.Event) error {
	// Ensure the event has an ID and timestamp.
	if event.ID == "" {
		event.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	channel := fmt.Sprintf("trip:%s", tripID)
	logger.GetLogger().Debugw("Publishing event to Redis",
		"channel", channel,
		"eventType", event.Type,
		"payloadSize", len(data))
	return r.redisClient.Publish(ctx, channel, data).Err()
}

// Subscribe subscribes to the Redis channel for the given trip.
// It returns a Go channel on which events will be sent.
func (r *RedisEventService) Subscribe(ctx context.Context, tripID string, userID string) (<-chan types.Event, error) {
	channelName := fmt.Sprintf("trip:%s", tripID)
	logger.GetLogger().Debugw("Subscribing to Redis channel",
		"channel", channelName,
		"userID", userID)

	pubsub := r.redisClient.Subscribe(ctx, channelName)

	// Create a channel for our events.
	eventChan := make(chan types.Event)

	// Start a goroutine to forward Redis messages to eventChan.
	go func() {
		defer close(eventChan)
		ch := pubsub.Channel()
		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var event types.Event
				if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
					logger.GetLogger().Errorf("failed to unmarshal event: %v", err)
					continue
				}
				eventChan <- event
			case <-ctx.Done():
				_ = pubsub.Close()
				return
			}
		}
	}()

	return eventChan, nil
}
