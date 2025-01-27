package services

import (
	"context"
	"encoding/json"
	"time"
	
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/redis/go-redis/v9"
)

const eventChannelPrefix = "trip_events:"

type RedisEventService struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisEventService(client *redis.Client) *RedisEventService {
	return &RedisEventService{
		client: client,
		ttl:    2 * time.Hour,
	}
}

func (s *RedisEventService) Publish(ctx context.Context, tripID string, event types.Event) error {
	event.Timestamp = time.Now()
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	
	return s.client.Publish(ctx, eventChannelPrefix+tripID, payload).Err()
}

func (s *RedisEventService) Subscribe(ctx context.Context, tripID string) (<-chan types.Event, error) {
	pubsub := s.client.Subscribe(ctx, eventChannelPrefix+tripID)
	
	eventChan := make(chan types.Event, 100)
	go func() {
		defer pubsub.Close()
		
		for {
			select {
			case msg := <-pubsub.Channel():
				var event types.Event
				if err := json.Unmarshal([]byte(msg.Payload), &event); err == nil {
					eventChan <- event
				}
			case <-ctx.Done():
				close(eventChan)
				return
			}
		}
	}()
	
	return eventChan, nil
}

func (s *RedisEventService) Unsubscribe(ctx context.Context, tripID string, ch <-chan types.Event) {
	// Redis automatically cleans up subscriptions when connection closes
} 