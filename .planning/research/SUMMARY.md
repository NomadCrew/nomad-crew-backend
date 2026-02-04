# v1.3 Security Remediation & Code Quality - Research Summary

**Project:** NomadCrew Backend v1.3
**Domain:** Security Remediation, Testing Infrastructure, DevX
**Researched:** 2026-02-04
**Confidence:** HIGH

## Executive Summary

This milestone addresses four security vulnerabilities identified in adversarial code review, a broken test suite (22 packages failing), outdated dependencies, and developer experience gaps. The remediation strategy is well-documented with production-proven Go patterns, making this a **low-risk, high-impact** effort.

The recommended approach is to execute security fixes first (rate limiter fails-open and IP spoofing are exploitable today), then repair the test suite (foundation for safe refactoring), migrate dependencies (low-effort, removes CVE exposure), and finally establish DevX tooling (force-multiplier for future work). All four research areas reached HIGH confidence using official Go documentation and established library patterns.

The critical risk is the rate limiter fail-open behavior combined with IP spoofing - together these allow unlimited brute-force attacks on authentication endpoints during any Redis outage. This must be the first fix deployed.

## Key Findings

### Security Vulnerabilities (SECURITY.md)

Four vulnerabilities require remediation, in priority order:

| Issue | Severity | Effort | Fix |
|-------|----------|--------|-----|
| Rate Limiter Fails Open | CRITICAL | Medium | In-memory fallback + fail-closed on Redis failure |
| X-Forwarded-For IP Spoofing | HIGH | Low | Gin SetTrustedProxies configuration |
| Simulator Bypass Token | MEDIUM | Low | Proper JWT claim parsing, build tags for prod |
| Goroutine Leaks | MEDIUM | Medium | Worker pool with bounded concurrency |

**Key insight:** Issues 1 and 2 are synergistic - IP spoofing bypasses rate limiting even when Redis is up. Both must be fixed together.

### Testing Infrastructure (TESTING.md)

The test suite has **blocking issues** preventing compilation:

- **pgx version mismatch:** Tests import `pgx/v4`, code uses `pgx/v5`
- **Missing dependencies:** `pgxmock/v4`, `miniredis/v2` not in go.mod
- **Duplicate mocks:** Same mock defined in 3+ locations

**Recommended stack:**
- **testify v1.10** - Already in use, keep
- **mockery v3** - Generate mocks from interfaces (5-10x faster than v2)
- **pgxmock v4** - Native pgx/v5 mocking (replaces broken sqlmock usage)
- **miniredis v2** - In-process Redis for unit tests

### Dependency Migrations (MIGRATIONS.md)

Four dependencies need updating:

| Package | Current | Target | Effort | Risk |
|---------|---------|--------|--------|------|
| nhooyr.io/websocket | v1.8.17 | coder/websocket v1.8.x | LOW (import path only) | Minimal |
| lestrrat-go/jwx | v2.0.21 | v2.1.6 (NOT v3) | LOW | Minimal |
| prometheus/client_golang | v1.14.0 | v1.21.1 | MEDIUM | Low |
| redis/go-redis | v9.7.3 | v9.17.x | LOW | Low |

**Critical decision:** Stay on jwx v2.1.6, do NOT migrate to v3. The v3 has extensive breaking API changes and requires Go 1.24+ with no security benefit over v2.1.6.

### Developer Experience (DEVX.md)

Key improvements for Windows-primary development:

- **Task (Taskfile)** over Makefile - YAML syntax, Windows-native, single binary
- **golangci-lint v2** - New config format, built-in `golangci-lint fmt` command
- **pre-commit** with tekwizely/pre-commit-golang hooks
- **Consolidated .env.example** - Current 4-file setup is confusing

**Total DevX setup time:** ~3.5 hours

## Recommended Execution Order

Based on research, the optimal execution order considers:
1. **Security exposure:** Fix active vulnerabilities first
2. **Dependencies:** Security fixes may need test validation
3. **Foundation:** Testing infrastructure enables safe refactoring
4. **Productivity:** DevX multiplies future velocity

### Phase 1: Critical Security Fixes

**Rationale:** Rate limiter and IP spoofing are exploitable today. Combined, they allow unlimited authentication brute-force.

**Delivers:**
- Fail-closed rate limiter with in-memory fallback
- Trusted proxy configuration
- 503 response on Redis unavailability (vs. silent allow)

**Effort:** 1-2 days

**Files affected:**
- `middleware/rate_limit.go`
- `main.go` (router setup)
- `config/config.go` (TrustedProxies field)

### Phase 2: Test Suite Repair

**Rationale:** Security fixes need test coverage. Cannot safely refactor goroutine leak without tests.

**Delivers:**
- All 22 packages compiling
- pgxmock replacing broken sqlmock usage
- Centralized mock generation via mockery v3
- CI pipeline running tests

**Effort:** 2-3 days

**Blocked by:** Phase 1 (security fixes should be deployed first)

**Files affected:**
- `go.mod` (add pgxmock/v4, miniredis/v2)
- All `*_test.go` files with pgx/v4 imports
- `.mockery.yaml` (new)
- `tests/mocks/` (regenerated)

### Phase 3: Goroutine Leak Fix

**Rationale:** Requires tests to validate. Lower severity than rate limiter.

**Delivers:**
- Worker pool for async notifications
- Bounded concurrency (default: 10 workers, 1000 queue)
- Graceful shutdown with drain

**Effort:** 1 day

**Blocked by:** Phase 2 (need tests for validation)

**Files affected:**
- `services/notification_facade_service.go`
- `services/notification_worker_pool.go` (new)
- `main.go` (shutdown sequence)

### Phase 4: Simulator Bypass Hardening

**Rationale:** Dev-only but demonstrates dangerous pattern. Lower priority since it requires `SERVER_ENVIRONMENT=development`.

**Delivers:**
- Proper JWT claim parsing (no substring matching)
- Build tags excluding simulator from production binaries
- Explicit `ENABLE_SIMULATOR_BYPASS` opt-in

**Effort:** 0.5 days

**Files affected:**
- `middleware/auth.go`
- `middleware/auth_simulator_dev.go` (new, build-tagged)
- `middleware/auth_simulator_prod.go` (new, build-tagged)

### Phase 5: Dependency Migrations

**Rationale:** Low-risk housekeeping. Can be done after security is stable.

**Delivers:**
- Updated websocket (import path change)
- Updated jwx (v2.1.6, security fixes)
- Updated prometheus (v1.21.1)
- Updated go-redis (v9.17.x)

**Effort:** 1 day

**Order within phase:**
1. coder/websocket (trivial, no deps)
2. go-redis (minor bump)
3. prometheus (moderate, test metrics)
4. jwx v2.1.6 (security fixes)

### Phase 6: Developer Experience

**Rationale:** Multiplicative value, but not blocking. Can proceed in parallel with later phases.

**Delivers:**
- Taskfile.yml replacing Makefile needs
- golangci-lint v2 configuration
- pre-commit hooks
- Consolidated .env.example
- VS Code settings

**Effort:** 0.5 days

**Can run in parallel with:** Phases 4-5

## Dependencies and Risks

### Dependency Graph

```
Phase 1 (Security) --> Phase 2 (Tests) --> Phase 3 (Goroutine)
                                       --> Phase 4 (Simulator)
                   --> Phase 5 (Deps) [after Phase 1 deployed]
                   --> Phase 6 (DevX) [independent]
```

### Risk Matrix

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Rate limiter fix causes false positives | LOW | MEDIUM | Deploy with monitoring, adjustable thresholds |
| Test fixes introduce regressions | LOW | LOW | Staged rollout, existing behavior preserved |
| jwx update breaks auth | VERY LOW | HIGH | v2.1.6 is API-compatible, comprehensive testing |
| DevX tools cause friction | LOW | LOW | Optional adoption, fallback to direct commands |

### Critical Dependencies

1. **Redis availability** - Rate limiter fallback assumes Redis is usually available
2. **Docker for testcontainers** - Integration tests need Docker (skip on Windows CI)
3. **Python for pre-commit** - DevX phase requires Python installation

## Total Effort Estimate

| Phase | Effort | Cumulative |
|-------|--------|------------|
| 1. Critical Security | 1-2 days | 1-2 days |
| 2. Test Suite | 2-3 days | 3-5 days |
| 3. Goroutine Fix | 1 day | 4-6 days |
| 4. Simulator Hardening | 0.5 days | 4.5-6.5 days |
| 5. Dependencies | 1 day | 5.5-7.5 days |
| 6. DevX | 0.5 days | **6-8 days** |

**Recommended timeline:** 2 weeks with buffer for testing and deployment windows.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Security | HIGH | Patterns from official Gin docs, golang-jwt, errgroup |
| Testing | HIGH | Official pgxmock, mockery v3, miniredis documentation |
| Migrations | HIGH | Library changelogs, explicit compatibility statements |
| DevX | HIGH | Official Task, golangci-lint v2, pre-commit documentation |

**Overall confidence:** HIGH

### Gaps to Address

1. **Trusted proxy CIDR ranges** - Need to confirm Coolify/EC2 network topology for production TrustedProxies config
2. **Rate limit thresholds** - Current values (100/min) may need tuning based on production traffic
3. **Worker pool sizing** - Default 10 workers / 1000 queue may need adjustment

## Research Flags for Planning

**Phases needing validation during implementation:**
- Phase 1: Verify Coolify proxy configuration before deploying TrustedProxies
- Phase 2: May discover additional test file issues during migration

**Phases with standard patterns (no additional research needed):**
- Phase 3: Worker pool is textbook Go concurrency
- Phase 4: Build tags are well-documented
- Phase 5: All migrations have explicit upgrade guides
- Phase 6: All tools have comprehensive documentation

## Sources

### Primary (HIGH confidence)
- [Gin SetTrustedProxies](https://pkg.go.dev/github.com/gin-gonic/gin#Engine.SetTrustedProxies)
- [Gin Deployment Docs](https://gin-gonic.com/en/docs/deployment/)
- [errgroup Package](https://pkg.go.dev/golang.org/x/sync/errgroup)
- [pgxmock v4](https://github.com/pashagolub/pgxmock)
- [mockery v3](https://vektra.github.io/mockery/v3.6/)
- [Task Documentation](https://taskfile.dev/)
- [golangci-lint v2 Config](https://golangci-lint.run/docs/configuration/)

### Secondary (MEDIUM confidence)
- [Gin X-Forwarded-For Issue #2473](https://github.com/gin-gonic/gin/issues/2473)
- [tekwizely/pre-commit-golang](https://github.com/tekwizely/pre-commit-golang)

---
*Research completed: 2026-02-04*
*Ready for requirements: yes*
