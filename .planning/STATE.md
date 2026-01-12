# Project State

## Milestones

- [v1.0 Codebase Refactoring](milestones/v1.0-ROADMAP.md) (Phases 1-12) — SHIPPED 2026-01-12
- [v1.1 Infrastructure Migration](milestones/v1.1-ROADMAP.md) (Phases 13-19) — SHIPPED 2026-01-12
- **v1.2 Mobile Integration & Quality** (Phases 20-25) — IN PROGRESS

## Current Position

Phase: 20 of 25 (Windows DevX for Mobile Development)
Plan: Not started
Status: Ready to plan
Last activity: 2026-01-12 - Milestone v1.2 created

Progress: ░░░░░░░░░░ 0%

## Progress

| Milestone | Phases | Status | Shipped |
|-----------|--------|--------|---------|
| v1.0 Codebase Refactoring | 1-12 | Complete | 2026-01-12 |
| v1.1 Infrastructure Migration | 13-19 | Complete | 2026-01-12 |
| v1.2 Mobile Integration & Quality | 20-25 | In Progress | - |

**Total Phases Completed:** 19 phases, 24 plans

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

## Roadmap Evolution

- v1.0 created: Codebase refactoring, 12 phases (Phase 1-12)
- v1.1 created: Infrastructure migration, 7 phases (Phase 13-19)
- v1.2 created: Mobile integration & quality, 6 phases (Phase 20-25)

## Context for Next Session

### v1.2 Mobile Integration & Quality

**Phases:**
1. Phase 20: Windows DevX for Mobile Development
2. Phase 21: Auth Flow Integration
3. Phase 22: API Gap Analysis
4. Phase 23: Real-time Features (research needed)
5. Phase 24: Bug Discovery & Fixes
6. Phase 25: E2E Testing Setup (research needed)

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

1. `/gsd:plan-phase 20` to plan Windows DevX
2. Or `/gsd:discuss-phase 20` to gather more context first

## Session Continuity

Last session: 2026-01-12
Stopped at: Milestone v1.2 initialization
Resume file: None

---

*Last updated: 2026-01-12*
