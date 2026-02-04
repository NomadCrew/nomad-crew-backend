# Dependency Migration Guide

**Project:** NomadCrew Backend
**Researched:** 2026-02-04
**Go Version:** 1.24.0

## Executive Summary

This document provides migration paths for four deprecated/outdated dependencies in the NomadCrew backend. The migrations range from trivial (coder/websocket - import path change only) to significant (lestrrat-go/jwx v3 - major API changes).

| Dependency | Current | Target | Effort | Risk |
|------------|---------|--------|--------|------|
| nhooyr.io/websocket | v1.8.17 | github.com/coder/websocket v1.8.x | **LOW** | Minimal |
| lestrrat-go/jwx | v2.0.21 | v2.1.6 (NOT v3) | **LOW** | Minimal |
| prometheus/client_golang | v1.14.0 | v1.21.x | **MEDIUM** | Low |
| redis/go-redis | v9.7.3 | v9.17.x | **LOW** | Low |

**Recommended Migration Order:**
1. coder/websocket (trivial, no dependencies)
2. go-redis (minor version bump, low risk)
3. prometheus/client_golang (moderate changes, test metrics)
4. lestrrat-go/jwx v2.1.6 (stay on v2, security fixes only)

---

## 1. WebSocket: nhooyr.io/websocket to github.com/coder/websocket

### Summary

**Status:** DROP-IN REPLACEMENT (import path change only)
**Effort:** 15 minutes
**Confidence:** HIGH

Coder acquired maintenance of nhooyr/websocket in 2024. The library is functionally identical - only the import path changes.

### Breaking Changes

**None.** The Coder team explicitly stated: "No breaking API changes are planned, except for updating the import path."

### Files Affected

| File | Changes Required |
|------|------------------|
| `go.mod` | Replace dependency |
| `internal/websocket/handler.go` | Update import |
| `internal/websocket/hub.go` | Update import |

### Migration Steps

**Step 1: Update go.mod**
```bash
# Remove old dependency
go mod edit -droprequire nhooyr.io/websocket

# Add new dependency
go get github.com/coder/websocket@latest
```

**Step 2: Update imports in all affected files**

Replace:
```go
import "nhooyr.io/websocket"
import "nhooyr.io/websocket/wsjson"
```

With:
```go
import "github.com/coder/websocket"
import "github.com/coder/websocket/wsjson"
```

**Step 3: Run tests**
```bash
go test ./internal/websocket/...
```

### Code Transformation Examples

**handler.go (lines 14-16):**
```go
// BEFORE
import (
    "nhooyr.io/websocket"
    "nhooyr.io/websocket/wsjson"
)

// AFTER
import (
    "github.com/coder/websocket"
    "github.com/coder/websocket/wsjson"
)
```

**hub.go (line 12):**
```go
// BEFORE
import "nhooyr.io/websocket"

// AFTER
import "github.com/coder/websocket"
```

### Test Strategy

1. Run existing WebSocket unit tests
2. Manual test: Connect via WebSocket, verify ping/pong
3. Manual test: Subscribe to trip events, verify message delivery
4. Test connection close scenarios

### Sources

- [Coder Blog - A New Home for nhooyr/websocket](https://coder.com/blog/websocket)
- [GitHub - coder/websocket](https://github.com/coder/websocket)

---

## 2. JWX: lestrrat-go/jwx v2.0.21 to v2.1.6

### Summary

**Status:** MINOR VERSION UPDATE (stay on v2, avoid v3)
**Effort:** 30 minutes
**Confidence:** HIGH

**CRITICAL DECISION:** Do NOT migrate to v3. JWX v3 requires Go 1.24 and has extensive breaking API changes. The v2 branch is actively maintained (v2.1.6 released April 2025) and receives security fixes.

### Why Stay on v2

| Factor | v2 | v3 |
|--------|----|----|
| Go Version | 1.18+ | **1.24+** |
| API Compatibility | Current code works | **Extensive rewrites needed** |
| Maintenance Status | Active (v2.1.6 Apr 2025) | Active |
| Migration Effort | 30 minutes | **2-4 hours** |
| Risk | Minimal | Moderate |

### Breaking Changes (v2.0.21 to v2.1.6)

**None that affect current usage.** Changes are primarily:
- Bug fixes (JWE edge case in v2.1.6)
- Go 1.24 compatibility (v2.1.4)
- Test improvements

### Files Affected

| File | Changes Required |
|------|------------------|
| `go.mod` | Version bump only |
| `middleware/jwt_validator.go` | None |
| `middleware/jwks_cache.go` | None |

### Migration Steps

**Step 1: Update dependency**
```bash
go get github.com/lestrrat-go/jwx/v2@v2.1.6
go mod tidy
```

**Step 2: Verify compilation**
```bash
go build ./...
```

**Step 3: Run tests**
```bash
go test ./middleware/...
```

### Current Usage Analysis

The codebase uses JWX v2 for:
1. JWT parsing and validation (`jwt.Parse`)
2. JWKS caching and key retrieval (`jwk.Parse`, `jwk.Key`)
3. Algorithm constants (`jwa.HS256`)

All of these APIs are stable in v2.1.x.

### Test Strategy

1. Run existing JWT validation tests
2. Test HS256 validation path
3. Test JWKS validation path with real Supabase endpoint
4. Verify token expiration handling

### Why NOT v3 (Reference)

If v3 migration is ever considered, here are the breaking changes:

```go
// v2 API
token, err := jwt.Parse([]byte(tokenString),
    jwt.WithKey(jwa.HS256, secret), // jwa.HS256 is a constant
    jwt.WithValidate(true),
)
sub := token.Subject() // Returns string

// v3 API (breaking changes)
token, err := jwt.Parse([]byte(tokenString),
    jwt.WithKey(jwa.HS256(), secret), // jwa.HS256() is now a FUNCTION
    jwt.WithValidate(true),
)
sub, ok := token.Subject() // Returns (string, bool) tuple

// v3: Get() method signature changed
// v2: value, err := token.Get("claim")
// v3: err := token.Get("claim", &destination)
```

### Sources

- [JWX Releases](https://github.com/lestrrat-go/jwx/releases)
- [JWX v3 Changes](https://github.com/lestrrat-go/jwx/blob/develop/v3/Changes-v3.md)
- [JWX v2 Package](https://pkg.go.dev/github.com/lestrrat-go/jwx/v2)

---

## 3. Prometheus: client_golang v1.14.0 to v1.21.x

### Summary

**Status:** MODERATE CHANGES
**Effort:** 1-2 hours
**Confidence:** HIGH

### Breaking Changes

| Version | Change | Impact |
|---------|--------|--------|
| v1.17.0 | Min Go 1.19 | N/A (using 1.24) |
| v1.19.0 | Min Go 1.20 | N/A |
| v1.20.0 | Removed `go_memstat_lookups_total` | None (not used) |
| v1.21.0 | Min Go 1.22 | N/A |
| v1.21.0 | UTF-8 label names allowed | Low (may affect tests) |

**IMPORTANT:** Use v1.21.1 or later to avoid performance regression in v1.21.0.

### Files Affected

| File | Changes Required |
|------|------------------|
| `go.mod` | Version bump |
| `router/router.go` | None (uses promhttp.Handler()) |
| `internal/events/redis_publisher.go` | Review promauto usage |

### Migration Steps

**Step 1: Update dependencies**
```bash
go get github.com/prometheus/client_golang@v1.21.1
go get github.com/prometheus/client_model@latest
go get github.com/prometheus/common@latest
go mod tidy
```

**Step 2: Verify metric names (optional)**

If your tests assert on specific label name validation behavior, you may need to enable legacy validation:

```go
import "github.com/prometheus/common/model"

func init() {
    model.NameValidationScheme = model.LegacyValidation
}
```

**Step 3: Run tests**
```bash
go test ./internal/events/...
go test ./router/...
```

### Current Usage Analysis

The codebase uses Prometheus for:

1. **HTTP metrics endpoint** (`router/router.go`):
```go
r.GET("/metrics", gin.WrapH(promhttp.Handler()))
```

2. **Custom metrics** (`internal/events/redis_publisher.go`):
```go
promauto.With(defaultRegistry).NewHistogram(...)
promauto.With(defaultRegistry).NewCounterVec(...)
promauto.With(defaultRegistry).NewGauge(...)
```

Both usage patterns are stable across v1.14 to v1.21.

### Test Strategy

1. Run existing tests
2. Verify `/metrics` endpoint returns valid Prometheus format
3. Check that event metrics (publish latency, error count, etc.) work
4. If using grafana/dashboards, verify metric names haven't changed

### Potential Issues

**Label name validation change (v1.21.0):**
- v1.21+ allows UTF-8 characters in label names
- If tests expect certain label names to fail validation, they may now pass
- Solution: Set `model.NameValidationScheme = model.LegacyValidation` if needed

**Performance regression (v1.21.0):**
- v1.21.0 had a performance regression fixed in v1.21.1
- Always use v1.21.1 or later

### Sources

- [Prometheus client_golang CHANGELOG](https://github.com/prometheus/client_golang/blob/main/CHANGELOG.md)
- [Prometheus client_golang Releases](https://github.com/prometheus/client_golang/releases)

---

## 4. Redis: go-redis v9.7.3 to v9.17.x

### Summary

**Status:** MINOR VERSION UPDATE
**Effort:** 30-60 minutes
**Confidence:** MEDIUM

### Breaking Changes

| Version | Change | Impact |
|---------|--------|--------|
| v9.12+ | Default buffer size 4KB -> 32KB | Low (performance improvement) |
| v9.17+ | Typed errors (errors.As pattern) | Low (recommended migration) |
| All v9 | Min Go 1.21 | N/A (using 1.24) |

### Files Affected

| File | Changes Required |
|------|------------------|
| `go.mod` | Version bump |
| `services/rate_limit_service.go` | None (stable API) |
| `internal/events/redis_publisher.go` | None (stable API) |
| `middleware/rate_limit.go` | None (stable API) |
| `config/database.go` | Optional: review connection options |

### Migration Steps

**Step 1: Update dependency**
```bash
go get github.com/redis/go-redis/v9@v9.17.3
go mod tidy
```

**Step 2: Optional - Adjust buffer sizes if needed**

If you experience memory issues with high connection counts:
```go
client := redis.NewClient(&redis.Options{
    Addr:       redisAddr,
    ReadBuffer: 4096,   // Restore old default
    WriteBuffer: 4096,  // Restore old default
})
```

**Step 3: Optional - Migrate to typed errors**

Old pattern (still works):
```go
if err == redis.Nil {
    // key not found
}
```

New pattern (recommended):
```go
if errors.Is(err, redis.Nil) {
    // key not found
}
```

**Step 4: Run tests**
```bash
go test ./services/...
go test ./internal/events/...
go test ./middleware/...
```

### Current Usage Analysis

The codebase uses go-redis for:

1. **Rate limiting** (`services/rate_limit_service.go`):
```go
pipe := s.redis.Pipeline()
incr := pipe.Incr(ctx, rKey)
pipe.Expire(ctx, rKey, duration)
_, err := pipe.Exec(ctx)
```

2. **Pub/Sub** (`internal/events/redis_publisher.go`):
```go
pubsub := p.rdb.Subscribe(ctx, channel)
p.rdb.Publish(ctx, channel, data)
ch := pubsub.Channel()
```

3. **Health checks** (`services/health_service.go`):
```go
redis.Ping(ctx)
```

All of these APIs are stable across v9.7 to v9.17.

### Test Strategy

1. Run existing Redis tests (with testcontainers)
2. Test rate limiting functionality
3. Test pub/sub event delivery
4. Verify health checks work
5. Monitor memory usage if concerned about buffer size change

### Potential Issues

**Buffer size increase (v9.12+):**
- Default buffers increased from 4KB to 32KB
- May increase memory usage with many connections
- Solution: Explicitly set `ReadBuffer` and `WriteBuffer` if needed

**RESP3 protocol (v9.0+):**
- go-redis v9 uses RESP3 by default
- Some Redis Stack commands may need RESP2
- Solution: Set `Protocol: 2` in options if issues arise

### Sources

- [go-redis GitHub](https://github.com/redis/go-redis)
- [go-redis Releases](https://github.com/redis/go-redis/releases)
- [go-redis Documentation](https://pkg.go.dev/github.com/redis/go-redis/v9)

---

## Migration Execution Plan

### Phase 1: Low-Risk Migrations (Week 1)

**Day 1-2: coder/websocket**
1. Create feature branch: `chore/migrate-websocket`
2. Update imports (2 files)
3. Run tests
4. Manual WebSocket testing
5. PR and merge

**Day 3-4: go-redis v9.17**
1. Create feature branch: `chore/upgrade-go-redis`
2. Update dependency
3. Run tests
4. Monitor staging for 24 hours
5. PR and merge

### Phase 2: Moderate Migrations (Week 2)

**Day 1-2: prometheus/client_golang v1.21**
1. Create feature branch: `chore/upgrade-prometheus`
2. Update dependencies (prometheus + common + model)
3. Run tests
4. Verify `/metrics` endpoint
5. PR and merge

**Day 3: jwx v2.1.6**
1. Create feature branch: `chore/upgrade-jwx`
2. Update dependency
3. Run tests
4. Test auth flow end-to-end
5. PR and merge

### Rollback Strategy

All migrations can be rolled back by reverting the commit:
```bash
git revert <commit-hash>
go mod tidy
```

For critical issues in production:
1. Revert to previous Docker image
2. Then investigate and fix forward

---

## Confidence Assessment

| Migration | Confidence | Reason |
|-----------|------------|--------|
| coder/websocket | **HIGH** | Drop-in replacement, explicit compatibility guarantee |
| jwx v2.1.6 | **HIGH** | Same major version, bug fixes only |
| prometheus v1.21 | **HIGH** | Well-documented changes, stable API |
| go-redis v9.17 | **MEDIUM** | Many minor releases, test thoroughly |

---

## Future Considerations

### JWX v3 Migration (Defer to 2027+)

If/when the project requires JWX v3 features, here's the scope:

**Estimated effort:** 4-6 hours
**Key changes:**
- All `jwa.XXX` constants become `jwa.XXX()` functions
- All accessors return `(value, bool)` tuples
- `Get()` method signature changes
- JWKS Cache completely reworked

**Files requiring significant changes:**
- `middleware/jwt_validator.go` (150+ lines)
- `middleware/jwks_cache.go` (100+ lines)

### Prometheus v2 (Future)

Prometheus client_golang v2 is not yet released. Monitor for announcements.

### go-redis v10 (Future)

No v10 announced. Current v9 line is actively maintained.
