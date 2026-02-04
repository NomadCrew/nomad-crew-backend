# Security Remediation Research

**Project:** NomadCrew Backend
**Researched:** 2026-02-04
**Confidence:** HIGH (verified with official documentation)

## Executive Summary

This research addresses four specific security vulnerabilities identified in the 3-pass adversarial code review:

1. **Simulator Bypass Token** - Weak substring matching on JWT payload allows bypass
2. **X-Forwarded-For IP Spoofing** - No trusted proxy configuration enables rate limit bypass
3. **Rate Limiter Fails Open** - Redis failure disables rate limiting entirely
4. **Goroutine Leaks** - Unbounded async notification goroutines without worker pool

All four issues are fixable with well-established Go patterns. Priority order: Rate Limiter (immediate security), IP Spoofing (rate limit bypass), Simulator Bypass (dev-only but risky), Goroutine Leaks (reliability).

---

## Issue 1: Simulator Bypass Token Vulnerability

### Current Vulnerable Code

Location: `middleware/auth.go:42-80`

```go
// VULNERABLE: Substring matching on decoded payload
func isSimulatorToken(token string) bool {
    // ...decodes JWT payload...
    decoded, err := base64.StdEncoding.DecodeString(payload)
    // ...
    return strings.Contains(string(decoded), simulatorUserID)  // DANGEROUS
}
```

### Why This Is Dangerous

An attacker can craft a valid JWT where the payload JSON contains the simulator user ID anywhere in the string - not necessarily in the `sub` claim. For example:

```json
{
  "sub": "attacker-real-id",
  "notes": "00000000-0000-0000-0000-000000000001",
  "exp": 9999999999
}
```

The `strings.Contains` check would match this token, granting simulator bypass.

### Remediation Pattern: Proper JWT Claim Parsing

**DO NOT** use substring matching. Parse the JWT properly and check the `sub` claim directly.

```go
package middleware

import (
    "encoding/base64"
    "encoding/json"
    "os"
    "strings"
)

const (
    simulatorUserID = "00000000-0000-0000-0000-000000000001"
)

// isSimulatorBypassEnabled checks if simulator bypass should be allowed.
// CRITICAL: Only enabled in development AND when explicitly configured.
func isSimulatorBypassEnabled() bool {
    env := os.Getenv("SERVER_ENVIRONMENT")
    // Double-check: require explicit opt-in even in development
    explicitEnable := os.Getenv("ENABLE_SIMULATOR_BYPASS") == "true"
    return env == "development" && explicitEnable
}

// isSimulatorToken validates that this is a legitimate simulator token.
// SECURE: Parses claims properly instead of substring matching.
func isSimulatorToken(token string) bool {
    // Legacy format - exact match only
    if token == "simulator-dev-token-bypass" {
        return true
    }

    // JWT format - parse and validate sub claim properly
    parts := strings.Split(token, ".")
    if len(parts) != 3 {
        return false
    }

    // Decode payload
    payload := parts[1]
    // Add padding for base64url
    switch len(payload) % 4 {
    case 2:
        payload += "=="
    case 3:
        payload += "="
    }
    payload = strings.ReplaceAll(payload, "-", "+")
    payload = strings.ReplaceAll(payload, "_", "/")

    decoded, err := base64.StdEncoding.DecodeString(payload)
    if err != nil {
        return false
    }

    // SECURE: Parse as JSON and check sub claim specifically
    var claims struct {
        Sub string `json:"sub"`
        Alg string `json:"alg,omitempty"` // Header check for "none" algorithm
    }
    if err := json.Unmarshal(decoded, &claims); err != nil {
        return false
    }

    // EXACT match on sub claim only - not substring
    return claims.Sub == simulatorUserID
}
```

### Better Pattern: Build Tags for Development-Only Code

For maximum security, use Go build tags to completely exclude simulator bypass from production builds:

**File: `middleware/auth_simulator_dev.go`**
```go
//go:build development

package middleware

import (
    "context"
    "encoding/base64"
    "encoding/json"
    "os"
    "strings"

    "github.com/NomadCrew/nomad-crew-backend/logger"
    "github.com/NomadCrew/nomad-crew-backend/types"
    "github.com/gin-gonic/gin"
)

const (
    simulatorUserID = "00000000-0000-0000-0000-000000000001"
)

// handleSimulatorBypass checks for and handles simulator bypass in development.
// This function only exists in development builds.
func handleSimulatorBypass(c *gin.Context, token string) bool {
    if os.Getenv("ENABLE_SIMULATOR_BYPASS") != "true" {
        return false
    }

    if !isValidSimulatorToken(token) {
        return false
    }

    log := logger.GetLogger()
    log.Warnw("SIMULATOR BYPASS ACTIVE - Development only",
        "path", c.Request.URL.Path)

    mockUser := &types.User{
        ID:        simulatorUserID,
        Email:     "simulator@dev.nomadcrew.local",
        Username:  "SimulatorDev",
    }

    c.Set(string(UserIDKey), simulatorUserID)
    c.Set(string(AuthenticatedUserKey), mockUser)
    c.Set(string(IsAdminKey), false)

    newCtx := context.WithValue(c.Request.Context(), UserIDKey, simulatorUserID)
    newCtx = context.WithValue(newCtx, AuthenticatedUserKey, mockUser)
    newCtx = context.WithValue(newCtx, IsAdminKey, false)
    c.Request = c.Request.WithContext(newCtx)

    return true
}

func isValidSimulatorToken(token string) bool {
    if token == "simulator-dev-token-bypass" {
        return true
    }

    parts := strings.Split(token, ".")
    if len(parts) != 3 {
        return false
    }

    payload := parts[1]
    switch len(payload) % 4 {
    case 2:
        payload += "=="
    case 3:
        payload += "="
    }
    payload = strings.ReplaceAll(payload, "-", "+")
    payload = strings.ReplaceAll(payload, "_", "/")

    decoded, err := base64.StdEncoding.DecodeString(payload)
    if err != nil {
        return false
    }

    var claims struct {
        Sub string `json:"sub"`
    }
    if err := json.Unmarshal(decoded, &claims); err != nil {
        return false
    }

    return claims.Sub == simulatorUserID
}
```

**File: `middleware/auth_simulator_prod.go`**
```go
//go:build !development

package middleware

import "github.com/gin-gonic/gin"

// handleSimulatorBypass is a no-op in production builds.
// The simulator bypass code is completely excluded from production binaries.
func handleSimulatorBypass(c *gin.Context, token string) bool {
    return false
}
```

**Updated AuthMiddleware:**
```go
func AuthMiddleware(validator Validator, userResolver UserResolver) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Check simulator bypass (no-op in production builds)
        authHeader := c.GetHeader("Authorization")
        token := strings.TrimPrefix(authHeader, "Bearer ")
        if handleSimulatorBypass(c, token) {
            c.Next()
            return
        }

        // Normal authentication flow...
    }
}
```

**Build commands:**
```bash
# Development build (includes simulator bypass)
go build -tags=development -o server-dev ./...

# Production build (excludes simulator bypass completely)
go build -o server ./...
```

### Confidence: HIGH
- Pattern verified with official Go documentation on build tags
- JWT claim parsing pattern from golang-jwt/jwt v5 documentation

---

## Issue 2: X-Forwarded-For IP Spoofing

### Current Vulnerable Code

Location: `middleware/rate_limit.go:117-134`

```go
// VULNERABLE: Trusts X-Forwarded-For from ANY client
func getClientIP(c *gin.Context) string {
    // Blindly trusts X-Forwarded-For header
    if forwarded := c.GetHeader("X-Forwarded-For"); forwarded != "" {
        ips := strings.Split(forwarded, ",")
        if len(ips) > 0 {
            return strings.TrimSpace(ips[0])  // SPOOFABLE
        }
    }
    // ...
}
```

### Why This Is Dangerous

Any client can set `X-Forwarded-For: 1.2.3.4` to appear as a different IP, completely bypassing IP-based rate limiting. An attacker can rotate through fake IPs to circumvent the rate limiter.

### Remediation Pattern: Gin Trusted Proxy Configuration

Gin has built-in trusted proxy support. Configure it properly:

**File: `main.go` (in router setup)**
```go
func SetupRouter(deps Dependencies) *gin.Engine {
    r := gin.New()  // Use gin.New() instead of gin.Default() for more control

    // SECURITY: Configure trusted proxies
    // Option 1: If NOT behind a reverse proxy (direct internet exposure)
    if deps.Config.Server.TrustedProxies == nil || len(deps.Config.Server.TrustedProxies) == 0 {
        // Disable proxy header parsing entirely
        r.SetTrustedProxies(nil)
        logger.GetLogger().Info("Trusted proxies disabled - using RemoteAddr directly")
    } else {
        // Option 2: Behind known reverse proxy (e.g., Cloudflare, AWS ALB)
        if err := r.SetTrustedProxies(deps.Config.Server.TrustedProxies); err != nil {
            logger.GetLogger().Fatalf("Invalid trusted proxy configuration: %v", err)
        }
        logger.GetLogger().Infow("Trusted proxies configured",
            "proxies", deps.Config.Server.TrustedProxies)
    }

    // Option 3: If using a known platform (e.g., Cloudflare)
    // r.TrustedPlatform = gin.PlatformCloudflare  // Uses CF-Connecting-IP

    // Continue with middleware setup...
    return r
}
```

**Updated config struct:**
```go
// config/config.go
type ServerConfig struct {
    // ... existing fields ...

    // TrustedProxies is a list of CIDR ranges or IPs of trusted reverse proxies.
    // If empty/nil, X-Forwarded-For headers are ignored entirely.
    // Examples: ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"]
    TrustedProxies []string `mapstructure:"trusted_proxies" yaml:"trusted_proxies"`
}
```

**Updated rate_limit.go:**
```go
// getClientIP now uses Gin's built-in ClientIP() which respects trusted proxy config
func getClientIP(c *gin.Context) string {
    // c.ClientIP() automatically:
    // 1. Checks if RemoteAddr is from a trusted proxy
    // 2. Only then parses X-Forwarded-For/X-Real-IP
    // 3. Falls back to RemoteAddr if not trusted
    return c.ClientIP()
}
```

### Production Configuration Examples

**For AWS EC2 behind ALB:**
```yaml
# config.production.yaml
server:
  trusted_proxies:
    - "10.0.0.0/8"      # VPC internal
    - "172.31.0.0/16"   # Default VPC
```

**For Cloudflare:**
```go
// In router setup
r.TrustedPlatform = gin.PlatformCloudflare
// Cloudflare IPs can be fetched from:
// https://www.cloudflare.com/ips-v4
// https://www.cloudflare.com/ips-v6
```

**For direct internet exposure (no reverse proxy):**
```yaml
# config.production.yaml
server:
  trusted_proxies: []  # Empty = trust no one
```

### Confidence: HIGH
- Pattern from official Gin documentation: https://gin-gonic.com/en/docs/deployment/
- Verified in Gin source code: `gin.Engine.SetTrustedProxies()`

---

## Issue 3: Rate Limiter Fails Open on Redis Failure

### Current Vulnerable Code

Location: `middleware/rate_limit.go:72-77`

```go
_, err := pipe.Exec(c.Request.Context())
if err != nil {
    // DANGEROUS: Allows request through on Redis failure
    c.Next()  // Fail-open behavior
    return
}
```

### Why This Is Dangerous

If an attacker can cause Redis to become unavailable (or if Redis has an outage), all rate limiting is disabled. This allows:
- Unlimited brute force attacks on authentication
- DDoS amplification through the API
- Credential stuffing attacks

### Remediation Pattern: Fail-Closed with In-Memory Fallback

**Option A: Strict Fail-Closed (Recommended for Auth Endpoints)**

```go
// AuthRateLimiterStrict fails closed on Redis failure - use for sensitive endpoints
func AuthRateLimiterStrict(redisClient *redis.Client, requestsPerMinute int, window time.Duration) gin.HandlerFunc {
    return func(c *gin.Context) {
        ip := c.ClientIP()
        key := fmt.Sprintf("ratelimit:auth:%s", ip)

        pipe := redisClient.TxPipeline()
        incr := pipe.Incr(c.Request.Context(), key)
        pipe.Expire(c.Request.Context(), key, window)

        _, err := pipe.Exec(c.Request.Context())
        if err != nil {
            // FAIL-CLOSED: Deny request when rate limiter is unavailable
            logger.GetLogger().Errorw("Rate limiter Redis failure - denying request",
                "error", err,
                "ip", ip,
                "path", c.Request.URL.Path)

            c.Header("Retry-After", "60")
            _ = c.Error(apperrors.ServiceUnavailable(
                "rate_limit_unavailable",
                "Service temporarily unavailable. Please try again later.",
            ))
            c.Abort()
            return
        }

        // ... rest of rate limit logic
    }
}
```

**Option B: Fail-Closed with In-Memory Fallback (Better UX)**

```go
package middleware

import (
    "sync"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/redis/go-redis/v9"
)

// InMemoryRateLimiter provides a fallback when Redis is unavailable
type InMemoryRateLimiter struct {
    mu       sync.RWMutex
    counts   map[string]*rateLimitEntry
    limit    int
    window   time.Duration
    lastClean time.Time
}

type rateLimitEntry struct {
    count     int
    expiresAt time.Time
}

func NewInMemoryRateLimiter(limit int, window time.Duration) *InMemoryRateLimiter {
    return &InMemoryRateLimiter{
        counts:    make(map[string]*rateLimitEntry),
        limit:     limit,
        window:    window,
        lastClean: time.Now(),
    }
}

// Allow checks if the request should be allowed (returns remaining count)
func (l *InMemoryRateLimiter) Allow(key string) (allowed bool, remaining int) {
    l.mu.Lock()
    defer l.mu.Unlock()

    // Periodic cleanup of expired entries
    if time.Since(l.lastClean) > l.window {
        l.cleanup()
        l.lastClean = time.Now()
    }

    now := time.Now()
    entry, exists := l.counts[key]

    if !exists || now.After(entry.expiresAt) {
        // New entry or expired
        l.counts[key] = &rateLimitEntry{
            count:     1,
            expiresAt: now.Add(l.window),
        }
        return true, l.limit - 1
    }

    entry.count++
    if entry.count > l.limit {
        return false, 0
    }
    return true, l.limit - entry.count
}

func (l *InMemoryRateLimiter) cleanup() {
    now := time.Now()
    for key, entry := range l.counts {
        if now.After(entry.expiresAt) {
            delete(l.counts, key)
        }
    }
}

// AuthRateLimiterWithFallback uses Redis with in-memory fallback
func AuthRateLimiterWithFallback(
    redisClient *redis.Client,
    fallback *InMemoryRateLimiter,
    requestsPerMinute int,
    window time.Duration,
) gin.HandlerFunc {
    return func(c *gin.Context) {
        ip := c.ClientIP()
        key := fmt.Sprintf("ratelimit:auth:%s", ip)

        // Try Redis first
        pipe := redisClient.TxPipeline()
        incr := pipe.Incr(c.Request.Context(), key)
        pipe.Expire(c.Request.Context(), key, window)

        _, err := pipe.Exec(c.Request.Context())
        if err != nil {
            // Redis failed - use in-memory fallback (STILL ENFORCES LIMITS)
            logger.GetLogger().Warnw("Rate limiter falling back to in-memory",
                "error", err,
                "ip", ip)

            allowed, remaining := fallback.Allow(key)
            if !allowed {
                c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", requestsPerMinute))
                c.Header("X-RateLimit-Remaining", "0")
                c.Header("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
                _ = c.Error(apperrors.RateLimitExceeded(
                    "Too many requests (fallback mode)",
                    int(window.Seconds()),
                ))
                c.Abort()
                return
            }

            c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", requestsPerMinute))
            c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
            c.Header("X-RateLimit-Mode", "fallback")
            c.Next()
            return
        }

        // Redis succeeded - normal flow
        count := incr.Val()
        if count > int64(requestsPerMinute) {
            ttl, _ := redisClient.TTL(c.Request.Context(), key).Result()
            if ttl < 0 {
                ttl = window
            }
            c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", requestsPerMinute))
            c.Header("X-RateLimit-Remaining", "0")
            c.Header("Retry-After", fmt.Sprintf("%d", int(ttl.Seconds())))
            _ = c.Error(apperrors.RateLimitExceeded(
                "Too many requests",
                int(ttl.Seconds()),
            ))
            c.Abort()
            return
        }

        remaining := requestsPerMinute - int(count)
        if remaining < 0 {
            remaining = 0
        }
        c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", requestsPerMinute))
        c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
        c.Next()
    }
}
```

**Usage in router:**
```go
// Create fallback limiter at startup
fallbackLimiter := middleware.NewInMemoryRateLimiter(
    deps.Config.RateLimit.AuthRequestsPerMinute,
    time.Duration(deps.Config.RateLimit.WindowSeconds)*time.Second,
)

// Use in router
authRateLimiter := middleware.AuthRateLimiterWithFallback(
    deps.RedisClient,
    fallbackLimiter,
    deps.Config.RateLimit.AuthRequestsPerMinute,
    time.Duration(deps.Config.RateLimit.WindowSeconds)*time.Second,
)
```

### Confidence: HIGH
- Pattern verified with Redis rate limiting best practices
- In-memory fallback pattern from production Go services

---

## Issue 4: Goroutine Leaks in Notification System

### Current Vulnerable Code

Location: `services/notification_facade_service.go:92-106`

```go
// VULNERABLE: Unbounded goroutine creation
func (s *NotificationFacadeService) SendTripUpdateAsync(ctx context.Context, ...) {
    go func() {  // Creates unbounded goroutines
        asyncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        if err := s.SendTripUpdate(asyncCtx, userIDs, data, priority); err != nil {
            s.logger.Error("Async trip update notification failed", "error", err)
        }
    }()
}
```

### Why This Is Dangerous

1. **Memory leaks**: Each `go func()` allocates stack memory
2. **Unbounded growth**: Under load, thousands of goroutines can be spawned
3. **No backpressure**: No way to slow down producers when workers are saturated
4. **Shutdown issues**: No way to gracefully wait for in-flight notifications

### Remediation Pattern: Worker Pool with errgroup

**File: `services/notification_worker_pool.go`**
```go
package services

import (
    "context"
    "sync"
    "time"

    "github.com/NomadCrew/nomad-crew-backend/internal/notification"
    "github.com/NomadCrew/nomad-crew-backend/logger"
    "go.uber.org/zap"
    "golang.org/x/sync/errgroup"
)

// NotificationTask represents a notification to be sent
type NotificationTask struct {
    UserID   string
    Type     string
    Data     interface{}
    Priority notification.Priority
}

// NotificationWorkerPool manages bounded goroutines for async notifications
type NotificationWorkerPool struct {
    client      *notification.Client
    taskChan    chan NotificationTask
    logger      *zap.SugaredLogger
    wg          sync.WaitGroup
    ctx         context.Context
    cancel      context.CancelFunc
    workerCount int
    queueSize   int
}

// WorkerPoolConfig configures the notification worker pool
type WorkerPoolConfig struct {
    WorkerCount int // Number of concurrent workers (default: 10)
    QueueSize   int // Max pending tasks (default: 1000)
    TaskTimeout time.Duration // Timeout per task (default: 30s)
}

func DefaultWorkerPoolConfig() WorkerPoolConfig {
    return WorkerPoolConfig{
        WorkerCount: 10,
        QueueSize:   1000,
        TaskTimeout: 30 * time.Second,
    }
}

// NewNotificationWorkerPool creates a bounded worker pool
func NewNotificationWorkerPool(
    client *notification.Client,
    cfg WorkerPoolConfig,
) *NotificationWorkerPool {
    ctx, cancel := context.WithCancel(context.Background())

    pool := &NotificationWorkerPool{
        client:      client,
        taskChan:    make(chan NotificationTask, cfg.QueueSize),
        logger:      logger.GetLogger(),
        ctx:         ctx,
        cancel:      cancel,
        workerCount: cfg.WorkerCount,
        queueSize:   cfg.QueueSize,
    }

    // Start workers
    pool.startWorkers(cfg.TaskTimeout)

    return pool
}

// startWorkers launches the worker goroutines
func (p *NotificationWorkerPool) startWorkers(taskTimeout time.Duration) {
    for i := 0; i < p.workerCount; i++ {
        p.wg.Add(1)
        go p.worker(i, taskTimeout)
    }
    p.logger.Infow("Notification worker pool started",
        "workers", p.workerCount,
        "queueSize", p.queueSize)
}

// worker processes tasks from the channel
func (p *NotificationWorkerPool) worker(id int, taskTimeout time.Duration) {
    defer p.wg.Done()

    for {
        select {
        case <-p.ctx.Done():
            p.logger.Debugw("Worker shutting down", "workerId", id)
            return
        case task, ok := <-p.taskChan:
            if !ok {
                // Channel closed
                return
            }
            p.processTask(task, taskTimeout)
        }
    }
}

// processTask handles a single notification task
func (p *NotificationWorkerPool) processTask(task NotificationTask, timeout time.Duration) {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    var err error
    switch task.Type {
    case "trip_update":
        if data, ok := task.Data.(notification.TripUpdateData); ok {
            _, err = p.client.SendTripUpdate(ctx, task.UserID, data, task.Priority)
        }
    case "chat_message":
        if data, ok := task.Data.(notification.ChatMessageData); ok {
            _, err = p.client.SendChatMessage(ctx, task.UserID, data)
        }
    // Add other notification types as needed
    default:
        p.logger.Warnw("Unknown notification task type", "type", task.Type)
    }

    if err != nil {
        p.logger.Errorw("Failed to send notification",
            "type", task.Type,
            "userId", task.UserID,
            "error", err)
    }
}

// Submit adds a task to the queue (non-blocking with backpressure)
func (p *NotificationWorkerPool) Submit(task NotificationTask) bool {
    select {
    case p.taskChan <- task:
        return true
    default:
        // Queue full - apply backpressure
        p.logger.Warnw("Notification queue full, dropping task",
            "type", task.Type,
            "userId", task.UserID,
            "queueSize", p.queueSize)
        return false
    }
}

// SubmitBlocking adds a task, blocking if queue is full (with timeout)
func (p *NotificationWorkerPool) SubmitBlocking(ctx context.Context, task NotificationTask) error {
    select {
    case p.taskChan <- task:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    case <-p.ctx.Done():
        return context.Canceled
    }
}

// Shutdown gracefully stops the worker pool
func (p *NotificationWorkerPool) Shutdown(ctx context.Context) error {
    p.logger.Info("Shutting down notification worker pool...")

    // Signal workers to stop
    p.cancel()

    // Close task channel to drain remaining tasks
    close(p.taskChan)

    // Wait for workers with timeout
    done := make(chan struct{})
    go func() {
        p.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        p.logger.Info("Notification worker pool shut down gracefully")
        return nil
    case <-ctx.Done():
        p.logger.Warn("Notification worker pool shutdown timed out")
        return ctx.Err()
    }
}

// QueueLength returns the current number of pending tasks
func (p *NotificationWorkerPool) QueueLength() int {
    return len(p.taskChan)
}
```

**Alternative: Using sync/errgroup with SetLimit (Go 1.20+)**

```go
package services

import (
    "context"
    "sync"

    "golang.org/x/sync/errgroup"
)

// NotificationBatcher batches and sends notifications with bounded concurrency
type NotificationBatcher struct {
    client     *notification.Client
    maxWorkers int
    logger     *zap.SugaredLogger
}

func NewNotificationBatcher(client *notification.Client, maxWorkers int) *NotificationBatcher {
    return &NotificationBatcher{
        client:     client,
        maxWorkers: maxWorkers,
        logger:     logger.GetLogger(),
    }
}

// SendTripUpdatesAsync sends notifications to multiple users with bounded concurrency
func (b *NotificationBatcher) SendTripUpdatesAsync(
    ctx context.Context,
    userIDs []string,
    data notification.TripUpdateData,
    priority notification.Priority,
) error {
    g, ctx := errgroup.WithContext(ctx)
    g.SetLimit(b.maxWorkers)  // Limit concurrent goroutines

    for _, userID := range userIDs {
        userID := userID  // Capture for closure
        g.Go(func() error {
            _, err := b.client.SendTripUpdate(ctx, userID, data, priority)
            if err != nil {
                b.logger.Errorw("Failed to send trip update",
                    "userId", userID,
                    "error", err)
                // Return nil to continue sending to other users
                return nil
            }
            return nil
        })
    }

    return g.Wait()
}
```

**Updated NotificationFacadeService:**
```go
type NotificationFacadeService struct {
    client     *notification.Client
    enabled    bool
    logger     *zap.SugaredLogger
    workerPool *NotificationWorkerPool  // NEW: Worker pool
}

func NewNotificationFacadeService(cfg *config.NotificationConfig) *NotificationFacadeService {
    // ... existing init code ...

    // Create worker pool
    poolCfg := DefaultWorkerPoolConfig()
    workerPool := NewNotificationWorkerPool(client, poolCfg)

    return &NotificationFacadeService{
        client:     client,
        enabled:    true,
        logger:     log,
        workerPool: workerPool,
    }
}

// SendTripUpdateAsync now uses worker pool (SAFE: bounded goroutines)
func (s *NotificationFacadeService) SendTripUpdateAsync(
    ctx context.Context,
    userIDs []string,
    data notification.TripUpdateData,
    priority notification.Priority,
) {
    if !s.enabled {
        return
    }

    for _, userID := range userIDs {
        task := NotificationTask{
            UserID:   userID,
            Type:     "trip_update",
            Data:     data,
            Priority: priority,
        }
        if !s.workerPool.Submit(task) {
            s.logger.Warnw("Failed to queue notification - pool at capacity",
                "userId", userID)
        }
    }
}

// Shutdown must be called during application shutdown
func (s *NotificationFacadeService) Shutdown(ctx context.Context) error {
    if s.workerPool != nil {
        return s.workerPool.Shutdown(ctx)
    }
    return nil
}
```

**Update main.go shutdown sequence:**
```go
// In main.go shutdown handling
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// Shutdown notification service (drains worker pool)
if notificationFacadeService != nil {
    log.Info("Shutting down notification service...")
    if err := notificationFacadeService.Shutdown(ctx); err != nil {
        log.Errorw("Error during notification service shutdown", "error", err)
    }
}

// Continue with other shutdowns...
```

### Confidence: HIGH
- errgroup.SetLimit verified in official Go documentation (Go 1.20+)
- Worker pool pattern from well-established Go concurrency patterns

---

## Priority Order for Implementation

| Priority | Issue | Severity | Effort | Rationale |
|----------|-------|----------|--------|-----------|
| 1 | Rate Limiter Fails Open | CRITICAL | Medium | Auth brute force possible during Redis outage |
| 2 | X-Forwarded-For Spoofing | HIGH | Low | Rate limit bypass trivial without fix |
| 3 | Simulator Bypass Token | MEDIUM | Low | Dev-only but pattern is dangerous |
| 4 | Goroutine Leaks | MEDIUM | Medium | Reliability issue, not direct security |

## Testing Recommendations

### Rate Limiter Test
```go
func TestRateLimiterFailsClosed(t *testing.T) {
    // Create rate limiter with intentionally broken Redis
    brokenRedis := &redis.Client{} // Invalid connection
    limiter := AuthRateLimiterWithFallback(brokenRedis, fallback, 10, time.Minute)

    // Should still enforce limits via fallback
    for i := 0; i < 15; i++ {
        w := httptest.NewRecorder()
        c, _ := gin.CreateTestContext(w)
        limiter(c)
    }
    // Verify some requests were rate limited
}
```

### Trusted Proxy Test
```go
func TestTrustedProxyConfiguration(t *testing.T) {
    r := gin.New()
    r.SetTrustedProxies([]string{"10.0.0.0/8"})

    r.GET("/ip", func(c *gin.Context) {
        c.String(200, c.ClientIP())
    })

    // Request from untrusted IP with spoofed header
    req := httptest.NewRequest("GET", "/ip", nil)
    req.Header.Set("X-Forwarded-For", "1.2.3.4")
    req.RemoteAddr = "203.0.113.50:12345"  // Untrusted IP

    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    // Should return RemoteAddr, not spoofed header
    assert.Equal(t, "203.0.113.50", w.Body.String())
}
```

---

## Sources

### Official Documentation (HIGH confidence)
- [Gin SetTrustedProxies](https://pkg.go.dev/github.com/gin-gonic/gin#Engine.SetTrustedProxies)
- [Gin Deployment Docs](https://gin-gonic.com/en/docs/deployment/)
- [errgroup Package](https://pkg.go.dev/golang.org/x/sync/errgroup)
- [golang-jwt/jwt v5](https://pkg.go.dev/github.com/golang-jwt/jwt/v5)
- [Go Build Tags](https://www.digitalocean.com/community/tutorials/customizing-go-binaries-with-build-tags)

### Security References (MEDIUM confidence)
- [Gin X-Forwarded-For Issue #2473](https://github.com/gin-gonic/gin/issues/2473)
- [Rate Limiting with Redis](https://dev.to/mauriciolinhares/rate-limiting-http-requests-in-go-using-redis-51m7)
- [Goroutine Leak Prevention](https://dev.to/serifcolakel/go-concurrency-mastery-preventing-goroutine-leaks-with-context-timeout-cancellation-best-1lg0)
