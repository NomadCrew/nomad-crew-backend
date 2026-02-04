---
phase: 26-critical-security-fixes
plan: 01
subsystem: security
status: complete
tags: [rate-limiting, redis, fail-closed, security, middleware]

dependency_graph:
  requires: []
  provides: [fail-closed-rate-limiter, in-memory-fallback]
  affects: [26-02]

tech_stack:
  added: []
  patterns: [fail-closed-security, graceful-degradation]

file_tracking:
  created: []
  modified:
    - middleware/rate_limit.go
    - router/router.go

decisions:
  - decision: in-memory-fallback-architecture
    rationale: Prevents fail-open vulnerability while maintaining availability
    alternatives: [fail-closed-reject-all, redis-cluster]
    chosen: in-memory-fallback
  - decision: x-ratelimit-mode-header
    rationale: Allows monitoring and debugging of fallback usage
    alternatives: [logging-only, no-indication]
    chosen: header-based-indication

metrics:
  duration: 3 minutes
  completed: 2026-02-04
---

# Phase 26 Plan 01: Fail-Closed Rate Limiter with In-Memory Fallback Summary

**One-liner:** Thread-safe in-memory rate limiter fallback that enforces limits even when Redis fails, closing SEC-01 fail-open vulnerability.

## What Was Built

Implemented a security-hardened rate limiting system that uses Redis as primary store but falls back to an in-memory rate limiter when Redis is unavailable. This closes the critical SEC-01 vulnerability where rate limiting failed open during Redis outages, allowing unlimited brute-force attacks.

### Key Components

1. **InMemoryRateLimiter struct** (`middleware/rate_limit.go`)
   - Thread-safe using sync.RWMutex
   - Sliding window rate limiting
   - Periodic cleanup of expired entries
   - Returns (allowed, remaining) for consistent API with Redis version

2. **AuthRateLimiterWithFallback middleware** (`middleware/rate_limit.go`)
   - Attempts Redis first (distributed rate limiting)
   - Falls back to in-memory limiter on Redis error
   - Enforces rate limits in both modes (fail-closed)
   - Adds X-RateLimit-Mode header to indicate fallback usage

3. **Router integration** (`router/router.go`)
   - Creates fallback limiter at startup
   - Switches from AuthRateLimiter to AuthRateLimiterWithFallback
   - Applied to `/users/onboard` endpoint (brute-force target)

## Technical Details

### In-Memory Fallback Architecture

```go
// On Redis error, fallback enforces limits
allowed, remaining := fallback.Allow(key)
if !allowed {
    // Return 429 with rate limit headers
    c.Header("X-RateLimit-Mode", "fallback")
    c.Error(apperrors.RateLimitExceeded(...))
    c.Abort()
    return
}
```

**Key characteristics:**
- Per-instance state (not distributed)
- Acceptable for short outages
- Prevents complete bypass during Redis failure
- O(1) Allow operation, O(n) periodic cleanup

### Rate Limit Headers

**Normal mode (Redis):**
```
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 7
X-RateLimit-Reset: 1770212400
Retry-After: 60
```

**Fallback mode (in-memory):**
```
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 5
X-RateLimit-Mode: fallback
Retry-After: 60
```

## Security Impact

**Before this change:**
- Redis outage → rate limiting disabled
- Attackers could brute-force during outage
- No protection during degraded state

**After this change:**
- Redis outage → fallback to in-memory limiter
- Rate limiting always enforced (fail-closed)
- Per-instance limits better than no limits
- Observable via X-RateLimit-Mode header

## Deviations from Plan

None - plan executed exactly as written.

## Task Summary

| Task | Description | Files | Commit |
|------|-------------|-------|--------|
| 1-2 | Add InMemoryRateLimiter struct and AuthRateLimiterWithFallback | middleware/rate_limit.go | f7f3923 |
| 3 | Enable fail-closed rate limiter in router | router/router.go | 5845708 |

## Verification Performed

1. ✅ `go build ./...` - Project compiles successfully
2. ✅ `go build ./middleware/...` - Middleware compiles successfully
3. ✅ `go build ./router/...` - Router compiles successfully
4. ✅ InMemoryRateLimiter struct exists with thread-safe Allow method
5. ✅ AuthRateLimiterWithFallback calls fallback.Allow on Redis error
6. ✅ Router creates fallbackLimiter and uses new middleware

## Integration Points

### Upstream Dependencies
- None (foundational security change)

### Downstream Impacts
- Phase 26-02: IP spoofing fix will strengthen rate limiting effectiveness
- Monitoring: X-RateLimit-Mode header can be tracked for Redis health

### Breaking Changes
- None (backward compatible)
- Old AuthRateLimiter still exists if needed

## Next Phase Readiness

**Blockers:** None

**Concerns:** None

**Recommendations:**
1. Add Prometheus metrics for fallback mode usage
2. Alert on sustained fallback mode (indicates Redis issues)
3. Consider distributed fallback (e.g., Redis Cluster) for production

## Decisions Made

### 1. In-Memory Fallback vs. Fail-Closed Reject All

**Decision:** Use in-memory fallback

**Context:** When Redis fails, we need to choose between:
- Reject all requests (guaranteed security, poor availability)
- In-memory fallback (good security, good availability)
- No rate limiting (poor security, good availability - previous behavior)

**Rationale:**
- In-memory limits are better than no limits
- Per-instance limits acceptable for auth endpoints (not high traffic)
- Maintains service availability during Redis outages
- Attackers would need to bypass multiple instances

**Trade-offs:**
- Per-instance limits less effective than distributed
- Memory growth if cleanup fails (bounded by cleanup interval)
- No cross-instance coordination

### 2. X-RateLimit-Mode Header

**Decision:** Add header to indicate fallback mode

**Context:** Need visibility into when fallback is active

**Rationale:**
- Enables monitoring of Redis health
- Helps debug rate limiting behavior
- Follows standard practice (e.g., CloudFlare uses similar headers)

**Alternatives considered:**
- Logging only: Less observable to clients and monitoring
- No indication: Hard to debug issues

## Commits

```
f7f3923 feat(26-01): add in-memory rate limiter fallback
5845708 feat(26-01): enable fail-closed rate limiter in router
```

## Files Modified

### middleware/rate_limit.go
**Lines changed:** +139

**Key additions:**
- InMemoryRateLimiter struct with sync.RWMutex
- NewInMemoryRateLimiter constructor
- Allow method with periodic cleanup
- AuthRateLimiterWithFallback middleware function
- X-RateLimit-Mode header support

**Preserved:**
- Original AuthRateLimiter (backward compatibility)
- getClientIP function
- WSRateLimiter unchanged

### router/router.go
**Lines changed:** +10, -2

**Key changes:**
- Create fallbackLimiter at startup
- Switch to AuthRateLimiterWithFallback
- Document security rationale in comments

**Unchanged:**
- Endpoint routing
- Other middleware
- Rate limiter application points

---

**Status:** ✅ Complete
**Duration:** 3 minutes
**Completed:** 2026-02-04
