# Roadmap: NomadCrew Backend

## Completed Milestones

- [v1.0 Codebase Refactoring](milestones/v1.0-ROADMAP.md) (Phases 1-12) — SHIPPED 2026-01-12
- [v1.1 Infrastructure Migration](milestones/v1.1-ROADMAP.md) (Phases 13-19) — SHIPPED 2026-01-12

---

## v1.2 Mobile Integration & Quality (Phases 20-25)

**Milestone Goal:** Get the mobile app working end-to-end with the backend, optimize developer workflow, and establish testing practices.

**Status:** In Progress

| Phase | Name | Status |
|-------|------|--------|
| 20 | Windows DevX for Mobile Development | Complete |
| 21 | Auth Flow Integration | Not started |
| 22 | API Gap Analysis | Not started |
| 23 | Real-time Features | Not started |
| 24 | Bug Discovery & Fixes | Not started |
| 25 | E2E Testing Setup | Not started |

See [v1.2 details in archive](milestones/v1.2-ROADMAP.md) for full phase specifications.

---

## v1.3 Security Remediation & Code Quality (Phases 26-31)

**Milestone Goal:** Eliminate security vulnerabilities, repair broken test suite, update dependencies, and establish developer tooling for sustainable development velocity.

**Phases:** 26-31 (6 phases)
**Requirements:** 20 total (5 SEC, 5 TEST, 4 DEP, 6 DEVX)

---

### Phase 26: Critical Security Fixes

**Goal:** Eliminate rate limiter fail-open vulnerability and IP spoofing that together allow unlimited brute-force attacks
**Depends on:** v1.2 complete (or can be inserted as priority)
**Requirements:** SEC-01, SEC-02
**Plans:** 2 plans

Plans:
- [x] 26-01-PLAN.md — Rate limiter fail-closed with in-memory fallback (SEC-01)
- [x] 26-02-PLAN.md — Trusted proxy configuration for IP spoofing prevention (SEC-02)

**Status:** Complete

**Success Criteria:**
1. Rate limiter returns 503 (not 200) when Redis is unavailable
2. In-memory fallback activates within 100ms of Redis failure
3. X-Forwarded-For header only trusted from configured proxy CIDRs
4. `gin.Context.ClientIP()` returns actual client IP, not spoofed value

**Files to modify:**
- `middleware/rate_limit.go`
- `router/router.go`
- `config/config.go` (TrustedProxies field)

**Effort:** 1-2 days

---

### Phase 27: Test Suite Repair

**Goal:** All packages compile and tests can run in CI
**Depends on:** Phase 26 (security fixes deployed first)
**Requirements:** TEST-01, TEST-02, TEST-03, TEST-04, TEST-05
**Plans:** 10 plans (original 4 + 6 gap closure)

Plans:
- [x] 27-01-PLAN.md — Install test dependencies and fix pgx v4 imports
- [x] 27-02-PLAN.md — Consolidate duplicate mocks and fix interface mismatches (handlers, middleware)
- [x] 27-03-PLAN.md — Fix interface mismatches and type errors (handlers, models)
- [x] 27-04-PLAN.md — Fix API changes, test logic, and remaining compilation errors
- [x] 27-05-PLAN.md — Gap closure: Fix config package ConnectionString references
- [x] 27-06-PLAN.md — Gap closure: Add types import to middleware tests
- [x] 27-07-PLAN.md — Gap closure: Consolidate trip service mocks
- [x] 27-08-PLAN.md — Gap closure: Fix services pgxmock API usage
- [x] 27-09-PLAN.md — Gap closure: Fix internal/store/postgres unused variables
- [x] 27-10-PLAN.md — Gap closure: Fix store/postgres type definitions

**Status:** Complete

**Success Criteria:**
1. `go test ./...` compiles all 22 packages without errors
2. pgxmock/v4 replaces broken pgx/v4 mock imports
3. Single source of truth for each mock interface (no duplicates)
4. CI workflow passes with current coverage threshold (30%)
5. `go test -race ./...` detects no data races

**Files to modify:**
- `go.mod` (add pgxmock/v4, redismock/v9)
- All `*_test.go` files with pgx/v4 imports
- Mock consolidation across packages

**Effort:** 2-3 days

---

### Phase 28: Goroutine Management

**Goal:** Background tasks use bounded concurrency and shutdown gracefully
**Depends on:** Phase 27 (tests needed for validation)
**Requirements:** SEC-03, SEC-04

**Success Criteria:**
1. Notification service uses worker pool with configurable max workers (default: 10)
2. Queue has bounded capacity (default: 1000) with drop-oldest or backpressure on full
3. `SIGTERM` triggers graceful drain with configurable timeout (default: 30s)
4. No goroutine leaks detectable via `runtime.NumGoroutine()` after shutdown
5. Metrics exposed for queue depth and worker utilization

**Files to modify:**
- `services/notification_facade_service.go`
- `services/notification_worker_pool.go` (new)
- `main.go` (shutdown sequence)

**Effort:** 1 day

---

### Phase 29: Simulator Bypass Hardening

**Goal:** Simulator bypass cannot be exploited through token manipulation
**Depends on:** Phase 27 (tests needed)
**Requirements:** SEC-05

**Success Criteria:**
1. JWT `sub` claim checked with exact equality, not substring match
2. Simulator bypass requires both `SERVER_ENVIRONMENT=development` AND `ENABLE_SIMULATOR_BYPASS=true`
3. Production binary excludes simulator code via build tags
4. Auth middleware logs warning when simulator bypass is used

**Files to modify:**
- `middleware/auth.go`
- `middleware/auth_simulator_dev.go` (new, build-tagged)
- `middleware/auth_simulator_prod.go` (new, build-tagged)

**Effort:** 0.5 days

---

### Phase 30: Dependency Migrations

**Goal:** All dependencies updated to current stable versions
**Depends on:** Phase 26 (security stable before dep changes)
**Requirements:** DEP-01, DEP-02, DEP-03, DEP-04

**Success Criteria:**
1. `nhooyr.io/websocket` replaced with `github.com/coder/websocket` (import path only)
2. `lestrrat-go/jwx` upgraded to v2.1.6 (NOT v3)
3. `prometheus/client_golang` upgraded to v1.21.1
4. `go-redis/v9` upgraded to v9.17.x
5. All existing tests pass after upgrades

**Migration order:**
1. coder/websocket (trivial, no deps)
2. go-redis (minor bump)
3. prometheus (moderate, test metrics)
4. jwx v2.1.6 (security fixes)

**Effort:** 1 day

---

### Phase 31: Developer Experience

**Goal:** Consistent, documented development workflow with automated quality checks
**Depends on:** None (can run in parallel with Phases 28-30)
**Requirements:** DEVX-01, DEVX-02, DEVX-03, DEVX-04, DEVX-05, DEVX-06

**Success Criteria:**
1. `task build`, `task test`, `task lint`, `task docker` all work on Windows
2. `golangci-lint run` passes with v2 configuration
3. `pre-commit run --all-files` passes (go-fmt, go-imports, golangci-lint)
4. Single `.env.example` documents all environment variables with descriptions
5. VS Code Go extension auto-configured via `.vscode/settings.json`

**Files to create:**
- `Taskfile.yml`
- `.golangci.yml`
- `.pre-commit-config.yaml`
- `.editorconfig`
- `.env.example` (consolidated)
- `.vscode/settings.json`
- `.vscode/extensions.json`

**Effort:** 0.5 days

---

## Dependencies Graph

```
Phase 26 (Critical Security)
    |
    v
Phase 27 (Test Suite Repair)
    |
    +---> Phase 28 (Goroutine Management)
    |
    +---> Phase 29 (Simulator Bypass)
    |
Phase 26 --> Phase 30 (Dependencies) [after security deployed]

Phase 31 (DevX) [independent, can run in parallel]
```

---

## Progress

| Phase | Milestone | Plans | Status | Completed |
|-------|-----------|-------|--------|-----------|
| 1-12 | v1.0 | 16/16 | Complete | 2026-01-12 |
| 13-19 | v1.1 | 8/8 | Complete | 2026-01-12 |
| 20. Windows DevX | v1.2 | - | Complete | 2026-01-12 |
| 21. Auth Flow | v1.2 | 0/? | Not started | - |
| 22. API Gaps | v1.2 | 0/? | Not started | - |
| 23. Real-time | v1.2 | 0/? | Not started | - |
| 24. Bug Fixes | v1.2 | 0/? | Not started | - |
| 25. E2E Testing | v1.2 | 0/? | Not started | - |
| 26. Critical Security | v1.3 | 2/2 | Complete | 2026-02-04 |
| 27. Test Suite Repair | v1.3 | 10/10 | Complete | 2026-02-04 |
| 28. Goroutine Management | v1.3 | 0/? | Not started | - |
| 29. Simulator Bypass | v1.3 | 0/? | Not started | - |
| 30. Dependency Migrations | v1.3 | 0/? | Not started | - |
| 31. Developer Experience | v1.3 | 0/? | Not started | - |

**Total Phases Completed:** 22
**Current Production:** https://api.nomadcrew.uk (AWS EC2 + Coolify)

---

*Created: 2026-01-10*
*Last Updated: 2026-02-04 - Phase 27 complete (all packages compile)*
