package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
)

type EventServiceConfig struct {
    MaxConnPerTrip int
    MaxConnPerUser int
    HeartbeatInterval time.Duration // New: Interval for sending heartbeats
    EventBufferSize   int           // New: Buffer size for event channels
    CleanupInterval   time.Duration // New: Interval for cleaning up stale connections
}

type subscriber struct {
    channel      chan types.Event
    lastEventID  string
    lastActivity time.Time
    userID       string
}

type EventService struct {
    subscribers     map[string]map[string]*subscriber // tripID -> subscriberID -> subscriber
    userConnections map[string]int
    mu             sync.RWMutex
    config         EventServiceConfig
    done           chan struct{}
    wg             sync.WaitGroup
}

func NewEventService(config EventServiceConfig) *EventService {
    if config.HeartbeatInterval == 0 {
        config.HeartbeatInterval = 30 * time.Second
    }
    if config.EventBufferSize == 0 {
        config.EventBufferSize = 100
    }
    if config.CleanupInterval == 0 {
        config.CleanupInterval = 5 * time.Minute
    }

    es := &EventService{
        subscribers:     make(map[string]map[string]*subscriber),
        userConnections: make(map[string]int),
        config:         config,
        done:           make(chan struct{}),
    }

    es.wg.Add(2)
    go es.heartbeatLoop()
    go es.cleanupLoop()

    return es
}

func (s *EventService) heartbeatLoop() {
    defer s.wg.Done()
    ticker := time.NewTicker(s.config.HeartbeatInterval)
    defer ticker.Stop()

    for {
        select {
        case <-s.done:
            return
        case <-ticker.C:
            s.sendHeartbeats()
        }
    }
}

func (s *EventService) sendHeartbeats() {
    s.mu.RLock()
    defer s.mu.RUnlock()
	log := logger.GetLogger()

    now := time.Now()
    heartbeat := types.Event{
        Type:      "HEARTBEAT",
        Timestamp: now,
    }

    for tripID, subs := range s.subscribers {
        for _, sub := range subs {
            select {
            case sub.channel <- heartbeat:
                sub.lastActivity = now
            default:
                // Channel is blocked, will be handled by cleanup
                log.Warnf("Dropping heartbeat for trip %s due to blocked channel", tripID)
            }
        }
    }
}

func (s *EventService) cleanupLoop() {
    defer s.wg.Done()
    ticker := time.NewTicker(s.config.CleanupInterval)
    defer ticker.Stop()

    for {
        select {
        case <-s.done:
            return
        case <-ticker.C:
            s.cleanupStaleConnections()
        }
    }
}

func (s *EventService) cleanupStaleConnections() {
    s.mu.Lock()
    defer s.mu.Unlock()
	log := logger.GetLogger()

    staleThreshold := time.Now().Add(-2 * s.config.HeartbeatInterval)

    for tripID, subs := range s.subscribers {
        for subID, sub := range subs {
            if sub.lastActivity.Before(staleThreshold) {
                log.Infof("Cleaning up stale connection",
							"tripID", tripID,
							"userID", sub.userID,
						)
                close(sub.channel)
                delete(subs, subID)
                s.userConnections[sub.userID]--

                if s.userConnections[sub.userID] <= 0 {
                    delete(s.userConnections, sub.userID)
                }
            }
        }

        if len(subs) == 0 {
            delete(s.subscribers, tripID)
        }
    }
}

func (s *EventService) Subscribe(ctx context.Context, tripID string, userID string) (<-chan types.Event, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Validate connection limits
    if s.userConnections[userID] >= s.config.MaxConnPerUser {
        return nil, fmt.Errorf("max connections (%d) reached for user", s.config.MaxConnPerUser)
    }

    tripSubs, exists := s.subscribers[tripID]
    if !exists {
        tripSubs = make(map[string]*subscriber)
        s.subscribers[tripID] = tripSubs
    }

    if len(tripSubs) >= s.config.MaxConnPerTrip {
        return nil, fmt.Errorf("max connections (%d) reached for trip", s.config.MaxConnPerTrip)
    }

    // Create new subscriber
    subID := uuid.New().String()
    sub := &subscriber{
        channel:      make(chan types.Event, s.config.EventBufferSize),
        lastActivity: time.Now(),
        userID:       userID,
    }

    tripSubs[subID] = sub
    s.userConnections[userID]++

    // Handle cleanup on context cancellation
    go func() {
        <-ctx.Done()
        s.mu.Lock()
        defer s.mu.Unlock()

        if subs, exists := s.subscribers[tripID]; exists {
            if sub, exists := subs[subID]; exists {
                close(sub.channel)
                delete(subs, subID)
                s.userConnections[userID]--

                if s.userConnections[userID] <= 0 {
                    delete(s.userConnections, userID)
                }
                if len(subs) == 0 {
                    delete(s.subscribers, tripID)
                }
            }
        }
    }()

    return sub.channel, nil
}

func (s *EventService) Publish(ctx context.Context, tripID string, event types.Event) error {
    if event.ID == "" {
        event.ID = uuid.New().String()
    }
    if event.Timestamp.IsZero() {
        event.Timestamp = time.Now()
    }

    s.mu.RLock()
    defer s.mu.RUnlock()
	log := logger.GetLogger()

    tripSubs, exists := s.subscribers[tripID]
    if !exists {
        return nil
    }

    var wg sync.WaitGroup
    for _, sub := range tripSubs {
        wg.Add(1)
        go func(sub *subscriber) {
            defer wg.Done()
            select {
            case sub.channel <- event:
                sub.lastEventID = event.ID
                sub.lastActivity = time.Now()
            case <-ctx.Done():
                return
            default:
                log.Warnw("Dropping event due to blocked channel",
							"tripID", tripID,
							"eventType", string(event.Type),
							"userID", sub.userID,
						)
            }
        }(sub)
    }

    wg.Wait()
    return nil
}

func (s *EventService) Shutdown() {
    close(s.done)
    s.wg.Wait()

    s.mu.Lock()
    defer s.mu.Unlock()

    for _, subs := range s.subscribers {
        for _, sub := range subs {
            close(sub.channel)
        }
    }

    s.subscribers = make(map[string]map[string]*subscriber)
    s.userConnections = make(map[string]int)
}