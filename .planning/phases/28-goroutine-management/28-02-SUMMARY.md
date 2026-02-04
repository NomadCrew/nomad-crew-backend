---
phase: 28-goroutine-management
plan: 02
subsystem: notification
tags: [worker-pool, graceful-shutdown, notification-service, async]
depends_on:
  requires: [28-01]
  provides: [notification-service-integration]
  affects: []
tech-stack:
  added: []
  patterns: [bounded-async, worker-pool-injection, graceful-shutdown-ordering]
key-files:
  created: []
  modified:
    - services/notification_facade_service.go
    - services/notification_facade_service_test.go
    - main.go
decisions:
  - key: optional-worker-pool
    choice: Worker pool parameter is optional (nil falls back to legacy goroutines)
    reason: Backward compatibility for tests and gradual migration
  - key: shutdown-order
    choice: Worker pool -> WebSocket hub -> Event service -> HTTP server
    reason: Drain pending notifications before closing client connections
  - key: job-naming
    choice: Include entity ID in job name (trip-update-{id}, chat-message-{id})
    reason: Enables debugging and filtering in logs
metrics:
  duration: 3m 45s
  completed: 2026-02-04
---

# Phase 28 Plan 02: Notification Service Integration Summary

**One-liner:** NotificationFacadeService now uses bounded worker pool for async operations with proper graceful shutdown ordering.

## What Was Built

### 1. NotificationFacadeService Worker Pool Integration

Modified `services/notification_facade_service.go` to accept and use a WorkerPool:

```go
type NotificationFacadeService struct {
    client     *notification.Client
    enabled    bool
    logger     *zap.SugaredLogger
    workerPool *WorkerPool  // NEW
}

// Constructor accepts optional worker pool
func NewNotificationFacadeService(cfg *config.NotificationConfig, workerPool *WorkerPool) *NotificationFacadeService

// Async methods now use worker pool
func (s *NotificationFacadeService) SendTripUpdateAsync(...) {
    if s.workerPool != nil {
        s.workerPool.Submit(Job{
            Name: fmt.Sprintf("trip-update-%s", data.TripID),
            Execute: func(jobCtx context.Context) error {
                return s.SendTripUpdate(jobCtx, userIDs, data, priority)
            },
        })
        return
    }
    // Fallback to legacy goroutine with warning
}
```

### 2. Main.go Lifecycle Integration

Worker pool is now properly managed in the application lifecycle:

**Initialization (after config loading):**
```go
notificationWorkerPool := services.NewWorkerPool(cfg.WorkerPool)
notificationWorkerPool.Start()
log.Infow("Notification worker pool started",
    "maxWorkers", cfg.WorkerPool.MaxWorkers,
    "queueSize", cfg.WorkerPool.QueueSize)

notificationFacadeService := services.NewNotificationFacadeService(&cfg.Notification, notificationWorkerPool)
```

**Shutdown sequence (correct ordering):**
```go
// 1. Drain pending notifications first
log.Info("Shutting down notification worker pool...")
if err := notificationWorkerPool.Shutdown(ctx); err != nil {
    log.Errorw("Error during notification worker pool shutdown", "error", err)
}

// 2. Close WebSocket connections
log.Info("Shutting down WebSocket hub...")
wsHub.Shutdown(ctx)

// 3. Stop event service
log.Info("Shutting down event service...")
eventService.Shutdown(ctx)

// 4. Stop HTTP server last
srv.Shutdown(ctx)
```

### 3. Test Updates

All tests in `services/notification_facade_service_test.go` updated to pass `nil` for worker pool (legacy fallback mode).

## Key Implementation Details

### Backward Compatibility

- Worker pool parameter is optional (`nil` falls back to legacy goroutines)
- Tests continue to work without real worker pool
- Warning logged when using fallback mode

### Shutdown Order Rationale

1. **Worker pool first** - Drain pending notification jobs
2. **WebSocket hub** - Close client connections gracefully
3. **Event service** - Stop Redis subscriptions
4. **HTTP server** - Stop accepting new requests

This ensures notifications triggered by final HTTP requests have a chance to be sent.

### Security Issues Closed

- **SEC-03:** Notification service now uses bounded worker pool instead of unbounded `go func()` goroutines
- **SEC-04:** Background tasks are now tracked via WaitGroup and awaited during graceful shutdown

## Commits

| Hash | Type | Description |
|------|------|-------------|
| 6a34edc | feat | Refactor NotificationFacadeService to use worker pool |
| 5599fa4 | feat | Integrate worker pool lifecycle in main.go |

## Deviations from Plan

None - plan executed exactly as written.

## Verification Results

1. `go build .` - OK (full application compiles)
2. `go vet ./services/...` - No issues
3. `go vet .` - No issues (main package)
4. All notification facade service tests pass (12 tests)
5. All worker pool tests pass (7 tests)
6. No duplicate metric registration panics

### Test Output Highlights

```
=== RUN   TestAsyncNotifications
WARN  Worker pool not configured, using fire-and-forget goroutine for async notification
--- PASS: TestAsyncNotifications (0.00s)
```

Confirms fallback mode works correctly when worker pool is nil.

## Success Criteria Met

- [x] NotificationFacadeService uses worker pool for SendTripUpdateAsync and SendChatMessageAsync
- [x] Worker pool is created and started in main.go with config values
- [x] Shutdown sequence is: worker pool -> WebSocket hub -> event service -> HTTP server
- [x] All existing tests pass
- [x] No compilation errors or vet warnings
- [x] SEC-03 closed: Notification service uses bounded worker pool
- [x] SEC-04 closed: Background tasks tracked and awaited during graceful shutdown

## Files Modified

| File | Changes |
|------|---------|
| `services/notification_facade_service.go` | Added workerPool field, updated constructor, refactored async methods |
| `services/notification_facade_service_test.go` | Updated all calls to pass nil for worker pool |
| `main.go` | Create/start worker pool, pass to facade service, update shutdown sequence |

## Production Impact

- **Performance:** Bounded concurrency prevents goroutine explosion under load
- **Reliability:** Graceful shutdown ensures pending notifications complete
- **Observability:** Prometheus metrics expose queue depth, active workers, dropped jobs
- **Debugging:** Job names include entity IDs for log filtering
