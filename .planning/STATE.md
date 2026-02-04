# Project State

## Milestones

- [v1.0 Codebase Refactoring](milestones/v1.0-ROADMAP.md) (Phases 1-12) — SHIPPED 2026-01-12
- [v1.1 Infrastructure Migration](milestones/v1.1-ROADMAP.md) (Phases 13-19) — SHIPPED 2026-01-12
- v1.2 Mobile Integration & Quality (Phases 20-25) — IN PROGRESS (paused)
- **v1.3 Security Remediation & Code Quality** (Phases 26-31) — ACTIVE

## Current Position

Phase: 26 of 31 (Critical Security Fixes)
Plan: Not started
Status: Ready to plan
Last activity: 2026-02-04 - v1.3 milestone roadmap created

Progress: [==========----------] 0% (0/6 v1.3 phases)

## Progress

| Milestone | Phases | Status | Shipped |
|-----------|--------|--------|---------|
| v1.0 Codebase Refactoring | 1-12 | Complete | 2026-01-12 |
| v1.1 Infrastructure Migration | 13-19 | Complete | 2026-01-12 |
| v1.2 Mobile Integration & Quality | 20-25 | In Progress (paused) | - |
| v1.3 Security Remediation & Code Quality | 26-31 | Active | - |

**Total Phases Completed:** 20 phases, 24 plans

## v1.3 Phase Summary

| Phase | Name | Requirements | Status |
|-------|------|--------------|--------|
| 26 | Critical Security Fixes | SEC-01, SEC-02 | Not started |
| 27 | Test Suite Repair | TEST-01 to TEST-05 | Not started |
| 28 | Goroutine Management | SEC-03, SEC-04 | Not started |
| 29 | Simulator Bypass Hardening | SEC-05 | Not started |
| 30 | Dependency Migrations | DEP-01 to DEP-04 | Not started |
| 31 | Developer Experience | DEVX-01 to DEVX-06 | Not started |

## Production Status

| Resource | Status | URL |
|----------|--------|-----|
| API | Healthy | https://api.nomadcrew.uk |
| Coolify | Running | http://3.130.209.141:8000 |
| Grafana | Monitoring | nomadcrew5.grafana.net |

## Blockers

None currently.

## Decisions Made

| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-01-10 | Domain-by-domain refactoring | Complete each domain before moving to next |
| 2026-01-10 | app_metadata for admin | Server-only, secure |
| 2026-01-11 | Switch from OCI to AWS | Oracle Cloud had no ARM capacity |
| 2026-01-11 | t4g.small ARM Graviton | Best cost/performance for Go + Coolify |
| 2026-01-12 | Upgrade to m8g.large | t4g.small couldn't handle Coolify load |
| 2026-02-04 | Stay on jwx v2.1.6 | v3 has breaking API changes, no security benefit |
| 2026-02-04 | Fix rate limiter + IP spoofing together | Synergistic vulnerabilities, both must be fixed |
| 2026-02-04 | Test suite before goroutine fix | Need tests to validate concurrency changes |

## v1.3 Research Summary

**Key findings from adversarial code review:**

1. **Critical (Phase 26):** Rate limiter fails-open + IP spoofing = unlimited brute-force
2. **High (Phase 27):** Test suite broken (pgx v4/v5 mismatch, missing deps)
3. **Medium (Phase 28):** Unbounded goroutines in notification service
4. **Medium (Phase 29):** Simulator bypass uses substring matching for JWT
5. **Low (Phase 30):** Four dependencies need updates
6. **Low (Phase 31):** Missing developer tooling

**Total effort estimate:** 6-8 days

## Context for Next Session

### v1.3 Security Remediation & Code Quality

**Priority order:**
1. Phase 26: Critical Security Fixes (exploitable today)
2. Phase 27: Test Suite Repair (foundation for safe changes)
3. Phase 28: Goroutine Management (requires tests)
4. Phase 29: Simulator Bypass Hardening
5. Phase 30: Dependency Migrations
6. Phase 31: Developer Experience (can run in parallel)

### Production Info

- **API:** https://api.nomadcrew.uk (routes at /v1/... not /api/v1/...)
- **EC2:** m8g.large (4 vCPU, 16 GB Graviton4) at 3.130.209.141
- **SSL:** Let's Encrypt R12, expires Apr 12, 2026
- **Cost:** ~$163/month AWS EC2

### Established Patterns

- `getUserIDFromContext()` for user ID extraction
- `bindJSONOrError()` for request binding
- `c.Error()` + `c.Abort()` for error handling
- `IsAdminKey` context for admin status
- `events.PublishEventWithContext()` for event publishing

### Next Steps

1. `/gsd:plan-phase 26` to plan Critical Security Fixes
2. Phase 26 addresses rate limiter fail-open and IP spoofing

## Session Continuity

Last session: 2026-02-04
Stopped at: v1.3 roadmap created, ready to plan Phase 26
Resume file: None

### Research Documents

- `.planning/research/SUMMARY.md` - Overall research summary
- `.planning/research/SECURITY.md` - Security vulnerability details
- `.planning/research/TESTING.md` - Test infrastructure analysis
- `.planning/research/MIGRATIONS.md` - Dependency upgrade guides
- `.planning/research/DEVX.md` - Developer experience tooling

### Files to Modify in Phase 26

- `middleware/rate_limit.go` - Add in-memory fallback, fail-closed
- `main.go` - Configure SetTrustedProxies
- `config/config.go` - Add TrustedProxies configuration

---

*Last updated: 2026-02-04*
