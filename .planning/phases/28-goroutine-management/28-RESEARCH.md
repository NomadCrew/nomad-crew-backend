# Phase 28: Goroutine Management - Research

**Researched:** 2026-02-04
**Domain:** Go concurrency, worker pools, graceful shutdown
**Confidence:** HIGH

## Summary

This phase addresses unbounded goroutine spawning in the notification service and other background tasks. The codebase currently uses fire-and-forget `go func()` patterns in 12+ locations across notification, user sync, trip sync, and membership sync operations. These goroutines are not tracked, have no backpressure mechanism, and are not awaited during shutdown.

The standard approach in Go for this problem is a **worker pool pattern** with bounded concurrency, a job queue, and graceful shutdown via `sync.WaitGroup` and context cancellation. The codebase already demonstrates this pattern excellently in `internal/events/redis_publisher.go` which uses `sync.WaitGroup` for goroutine tracking and proper shutdown sequencing.

**Primary recommendation:** Implement a generic `WorkerPool` in `services/notification_worker_pool.go` using the established patterns from `redis_publisher.go`, then refactor `NotificationFacadeService` and other services to submit tasks to this pool instead of spawning raw goroutines.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Standard library | Go 1.24 | `sync.WaitGroup`, `context`, channels | Zero dependencies, idiomatic Go, already used in codebase |
| `prometheus/client_golang` | 1.14.0 | Metrics for queue depth, worker count | Already in use for event metrics |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/alitto/pond/v2` | 2.x | Production worker pool library | If more features needed (subpools, result pools, auto-scaling) |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Custom worker pool | pond v2 | pond adds dependency but provides more features (subpools, result pools, dynamic resize). Custom is simpler, matches codebase patterns, zero new deps |
| Unbounded channel | Bounded channel with drop-oldest | Drop-oldest requires custom ring buffer; backpressure (blocking) is simpler and already used in `redis_publisher.go` |

**Recommendation:** Use custom implementation matching existing `redis_publisher.go` patterns. The codebase already has the pattern established; adding pond would introduce inconsistency and an unnecessary dependency.

**Installation:**
```bash
# No new dependencies required - use standard library
```

## Architecture Patterns

### Recommended Project Structure
```
services/
├── notification_facade_service.go    # Refactored to use worker pool
├── notification_worker_pool.go       # NEW: Generic worker pool
└── notification_worker_pool_test.go  # NEW: Worker pool tests
```

### Pattern 1: Worker Pool with Bounded Queue

**What:** Fixed number of workers pulling from a buffered channel, with WaitGroup tracking
**When to use:** Any background task that should be bounded and tracked for graceful shutdown

**Example:**
```go
// Source: Based on internal/events/redis_publisher.go pattern
type WorkerPool struct {
    jobQueue    chan Job
    wg          sync.WaitGroup
    ctx         context.Context
    cancel      context.CancelFunc
    logger      *zap.SugaredLogger
    metrics     *WorkerPoolMetrics
    maxWorkers  int
    queueSize   int
    mu          sync.Mutex
    running     bool
}

type Job struct {
    Ctx     context.Context
    Execute func(ctx context.Context) error
    Name    string
}

func NewWorkerPool(maxWorkers, queueSize int, logger *zap.SugaredLogger) *WorkerPool {
    ctx, cancel := context.WithCancel(context.Background())
    wp := &WorkerPool{
        jobQueue:   make(chan Job, queueSize),
        ctx:        ctx,
        cancel:     cancel,
        logger:     logger,
        maxWorkers: maxWorkers,
        queueSize:  queueSize,
        metrics:    newWorkerPoolMetrics(),
    }
    return wp
}

func (wp *WorkerPool) Start() {
    wp.mu.Lock()
    defer wp.mu.Unlock()
    if wp.running {
        return
    }
    wp.running = true

    for i := 0; i < wp.maxWorkers; i++ {
        wp.wg.Add(1)
        go wp.worker(i)
    }
}

func (wp *WorkerPool) worker(id int) {
    defer wp.wg.Done()
    for {
        select {
        case <-wp.ctx.Done():
            return
        case job, ok := <-wp.jobQueue:
            if !ok {
                return
            }
            wp.metrics.activeWorkers.Inc()
            start := time.Now()
            if err := job.Execute(job.Ctx); err != nil {
                wp.logger.Errorw("Job execution failed",
                    "job", job.Name,
                    "worker", id,
                    "error", err)
                wp.metrics.errorCount.Inc()
            }
            wp.metrics.jobDuration.Observe(time.Since(start).Seconds())
            wp.metrics.activeWorkers.Dec()
            wp.metrics.completedJobs.Inc()
        }
    }
}

func (wp *WorkerPool) Submit(job Job) bool {
    select {
    case wp.jobQueue <- job:
        wp.metrics.queuedJobs.Inc()
        return true
    default:
        // Queue full - apply backpressure strategy
        wp.metrics.droppedJobs.Inc()
        wp.logger.Warnw("Job dropped - queue full", "job", job.Name)
        return false
    }
}

func (wp *WorkerPool) Shutdown(ctx context.Context) error {
    wp.cancel()           // Signal workers to stop accepting new jobs
    close(wp.jobQueue)    // Close channel so workers exit their loops

    done := make(chan struct{})
    go func() {
        wp.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        wp.logger.Info("Worker pool shutdown complete")
        return nil
    case <-ctx.Done():
        wp.logger.Warn("Worker pool shutdown timed out")
        return ctx.Err()
    }
}
```

### Pattern 2: Graceful Shutdown Sequencing

**What:** Ordered shutdown of services during SIGTERM
**When to use:** main.go shutdown sequence

**Example:**
```go
// Source: Current main.go pattern, enhanced
// Shutdown sequence (order matters):
// 1. Stop accepting new HTTP requests
// 2. Shutdown WebSocket hub (close connections)
// 3. Shutdown notification worker pool (drain queue)
// 4. Shutdown event service (stop Redis subscriptions)
// 5. Close database connections

ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// 1. Shutdown notification worker pool (new)
log.Info("Shutting down notification worker pool...")
if err := notificationWorkerPool.Shutdown(ctx); err != nil {
    log.Errorw("Error during notification worker pool shutdown", "error", err)
}

// 2. Existing WebSocket hub shutdown
log.Info("Shutting down WebSocket hub...")
if err := wsHub.Shutdown(ctx); err != nil {
    log.Errorw("Error during WebSocket hub shutdown", "error", err)
}

// 3. Event service shutdown
log.Info("Shutting down event service...")
if err := eventService.Shutdown(ctx); err != nil {
    log.Errorw("Error during event service shutdown", "error", err)
}

// 4. HTTP server shutdown
if err := srv.Shutdown(ctx); err != nil {
    log.Fatalw("Server forced to shutdown", "error", err)
}
```

### Pattern 3: Non-Blocking Submit with Backpressure

**What:** Submit that doesn't block caller, with configurable behavior when queue is full
**When to use:** When callers shouldn't wait for queue space

**Example:**
```go
// Option A: Drop-oldest (ring buffer style) - more complex
// Option B: Drop-newest (current job) - simpler, recommended

func (wp *WorkerPool) SubmitAsync(job Job) {
    select {
    case wp.jobQueue <- job:
        wp.metrics.queuedJobs.Inc()
    default:
        // Queue full - drop this job (backpressure)
        wp.metrics.droppedJobs.Inc()
        wp.logger.Warnw("Job dropped due to backpressure",
            "job", job.Name,
            "queueSize", len(wp.jobQueue),
            "maxSize", wp.queueSize)
    }
}
```

### Anti-Patterns to Avoid
- **Fire-and-forget goroutines:** `go func() { ... }()` without tracking - causes goroutine leaks and prevents graceful shutdown
- **context.Background() in async work:** Loses parent context cancellation; use a timeout context for async work
- **Unbounded channels:** Can cause memory exhaustion under load spikes
- **Close before drain:** Closing the job channel before workers finish causes lost work

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Worker pool | Custom from scratch | Copy pattern from `redis_publisher.go` | Proven in codebase, handles edge cases |
| Graceful shutdown | Ad-hoc signal handling | `signal.NotifyContext` + WaitGroup | Standard pattern, handles SIGTERM/SIGINT |
| Queue with metrics | Plain channel | Channel + Prometheus gauges | Metrics already established in codebase |
| Context timeout | Manual time.After | `context.WithTimeout` | Propagates cancellation correctly |

**Key insight:** The codebase already has excellent patterns in `internal/events/redis_publisher.go` and `internal/websocket/hub.go`. These should be the template, not external libraries.

## Common Pitfalls

### Pitfall 1: Not Draining Queue Before Shutdown
**What goes wrong:** Jobs in queue are lost when shutdown happens
**Why it happens:** Channel is closed or context cancelled before workers process remaining jobs
**How to avoid:** Shutdown sequence: 1) stop accepting new jobs, 2) drain existing queue, 3) wait for workers
**Warning signs:** Jobs mysteriously "lost" during deployments/restarts

### Pitfall 2: WaitGroup Add/Done Mismatch
**What goes wrong:** `wg.Wait()` hangs forever or returns too early
**Why it happens:** Adding to WaitGroup in goroutine instead of before starting it
**How to avoid:** Always `wg.Add(1)` BEFORE `go func()`, and `defer wg.Done()` at start of goroutine
**Warning signs:** Deadlocks during shutdown, or shutdown completing with workers still running

### Pitfall 3: Using Original Context in Async Work
**What goes wrong:** Async work cancelled when HTTP request completes
**Why it happens:** Using `ctx` from HTTP handler which gets cancelled when response sent
**How to avoid:** Create new context with timeout: `context.WithTimeout(context.Background(), 30*time.Second)`
**Warning signs:** "context cancelled" errors in notification sending

### Pitfall 4: No Timeout on Shutdown Wait
**What goes wrong:** Shutdown hangs forever waiting for stuck workers
**Why it happens:** `wg.Wait()` without deadline
**How to avoid:** Use `context.WithTimeout` and select on both `wg.Wait()` completion and context deadline
**Warning signs:** Pod termination taking 30+ seconds, eventually killed by SIGKILL

### Pitfall 5: Metrics Not Thread-Safe
**What goes wrong:** Race conditions on metric counters
**Why it happens:** Using plain `int` counters without synchronization
**How to avoid:** Use `sync/atomic` or Prometheus counters (already thread-safe)
**Warning signs:** Incorrect metric values, race detector warnings

## Code Examples

Verified patterns from official sources and existing codebase:

### Existing Pattern: RedisPublisher Graceful Shutdown
```go
// Source: internal/events/redis_publisher.go (lines 364-386)
func (p *RedisPublisher) Shutdown(ctx context.Context) error {
    p.mu.Lock()
    localSubs := make(map[string]*subscription, len(p.subs))
    for k, v := range p.subs {
        localSubs[k] = v
    }
    p.subs = make(map[string]*subscription)
    p.mu.Unlock()

    p.log.Infow("Shutting down RedisPublisher, cancelling subscriptions...", "count", len(localSubs))

    for subKey, sub := range localSubs {
        p.log.Debugw("Cancelling context for subscription", "subKey", subKey)
        sub.cancelCtx()
    }

    p.log.Infow("Waiting for subscription goroutines to finish...")
    p.wg.Wait()  // KEY: Wait for all goroutines to complete
    p.log.Infow("All subscription goroutines finished. RedisPublisher shutdown complete.")

    return nil
}
```

### Existing Pattern: WebSocket Hub Shutdown
```go
// Source: internal/websocket/hub.go (lines 349-369)
func (h *Hub) Shutdown(ctx context.Context) error {
    h.shutdownOnce.Do(func() {
        close(h.shutdownCh)

        h.mu.Lock()
        connections := make([]*Connection, 0, len(h.connections))
        for _, conn := range h.connections {
            connections = append(connections, conn)
        }
        h.connections = make(map[string]*Connection)
        h.mu.Unlock()

        for _, conn := range connections {
            h.closeConnection(conn, "server shutdown")
        }
    })

    h.log.Info("WebSocket hub shutdown complete")
    return nil
}
```

### Current Problem: Fire-and-Forget Goroutines
```go
// Source: services/notification_facade_service.go (lines 92-106)
// PROBLEM: This goroutine is not tracked and cannot be awaited during shutdown
func (s *NotificationFacadeService) SendTripUpdateAsync(...) {
    if !s.enabled {
        return
    }

    go func() {
        asyncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        if err := s.SendTripUpdate(asyncCtx, userIDs, data, priority); err != nil {
            s.logger.Error("Async trip update notification failed", "error", err)
        }
    }()
}
```

### Metrics Pattern from Codebase
```go
// Source: internal/events/redis_publisher.go (lines 35-78)
type metrics struct {
    publishLatency    prometheus.Histogram
    subscribeLatency  prometheus.Histogram
    errorCount        *prometheus.CounterVec
    eventCount        *prometheus.CounterVec
    activeSubscribers prometheus.Gauge
}

// Singleton pattern to avoid double registration
var (
    metricsInstance *metrics
    metricsOnce     sync.Once
)

func newMetrics() *metrics {
    metricsOnce.Do(func() {
        metricsInstance = &metrics{
            // ... metric registration
        }
    })
    return metricsInstance
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `go func(){}()` fire-and-forget | Worker pool with WaitGroup | Standard practice | Enables graceful shutdown |
| Unbounded goroutines | Bounded worker pool | Standard practice | Prevents resource exhaustion |
| `os.Exit(0)` on signal | `signal.NotifyContext` + drain | Go 1.16+ | Clean shutdown |
| Manual channel close | `sync.Once` for close | Standard practice | Prevents double-close panic |

**Deprecated/outdated:**
- Using `time.After` in select without cancel leads to goroutine leaks
- `runtime.Goexit()` for shutdown - use context cancellation instead

## Open Questions

Things that couldn't be fully resolved:

1. **Drop-oldest vs backpressure strategy**
   - What we know: Both are valid; backpressure is simpler
   - What's unclear: Product requirements for notification delivery guarantees
   - Recommendation: Use backpressure (reject new jobs when full) with logging; can switch to drop-oldest later if needed

2. **Optimal worker count and queue size**
   - What we know: Phase spec says default 10 workers, 1000 queue size
   - What's unclear: Whether these are based on profiling or estimates
   - Recommendation: Make configurable via environment variables, start with spec defaults

3. **Should user/trip sync goroutines also use the pool?**
   - What we know: There are 10+ other `go func()` patterns in user_service.go, trip_service.go, member_service.go
   - What's unclear: Whether scope is limited to notification_facade_service.go
   - Recommendation: Phase scope says notification service; other services can use same pool in future phase

## Sources

### Primary (HIGH confidence)
- `internal/events/redis_publisher.go` - Existing WaitGroup + graceful shutdown pattern
- `internal/websocket/hub.go` - Existing shutdown sequencing pattern
- `internal/events/router.go` - Existing Prometheus metrics pattern
- `main.go` - Existing shutdown sequence (lines 313-338)

### Secondary (MEDIUM confidence)
- [Go by Example: Worker Pools](https://gobyexample.com/worker-pools) - Canonical pattern reference
- [Go Optimization Guide: Worker Pools](https://goperf.dev/01-common-patterns/worker-pool/) - Performance considerations
- [Implementing Graceful Shutdown in Go](https://www.rudderstack.com/blog/implementing-graceful-shutdown-in-go/) - Signal handling patterns
- [Graceful Shutdown Patterns in Go - Medium](https://rafalroppel.medium.com/graceful-shutdown-in-go-explained-signals-contexts-and-the-correct-shutdown-sequence-f24fd9ef8fac) - Kubernetes shutdown sequence

### Tertiary (LOW confidence)
- [pond v2 library](https://github.com/alitto/pond) - Alternative if more features needed later

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Using existing codebase patterns, no new dependencies
- Architecture: HIGH - Directly modeled on redis_publisher.go which is proven
- Pitfalls: HIGH - Well-documented in Go community, observed in codebase

**Research date:** 2026-02-04
**Valid until:** 2026-03-04 (30 days - patterns are stable)

## Appendix: Files with Unbounded Goroutines

For reference, here are all locations spawning untracked goroutines:

| File | Lines | Pattern | Priority |
|------|-------|---------|----------|
| `services/notification_facade_service.go` | 97, 153 | `SendTripUpdateAsync`, `SendChatMessageAsync` | HIGH - Phase scope |
| `models/notification/service/notification_service.go` | 97, 140 | Event publishing, push notification | MEDIUM |
| `models/user/service/user_service.go` | 320, 412, 836 | Supabase sync | LOW |
| `models/trip/service/trip_service.go` | 91, 304, 408 | Supabase sync | LOW |
| `models/trip/service/member_service.go` | 53, 113, 161 | Membership sync | LOW |
| `handlers/invitation_handler.go` | Various | Email sending | LOW |

**Recommendation:** Focus Phase 28 on `notification_facade_service.go` as specified, but design the worker pool to be reusable for future phases.
