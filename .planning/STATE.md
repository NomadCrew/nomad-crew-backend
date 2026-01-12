# Project State

## Milestones

- [v1.0 Codebase Refactoring](milestones/v1.0-ROADMAP.md) (Phases 1-12) — SHIPPED 2026-01-12
- [v1.1 Infrastructure Migration](milestones/v1.1-ROADMAP.md) (Phases 13-19) — SHIPPED 2026-01-12
- **v1.2 Developer Experience** (Phase 20+) — NOT STARTED

## Current Status

**Active Milestone:** v1.2 — Developer Experience
**Current Phase:** 20 (Not Started) - Windows DevX for Mobile Development
**Phase Status:** Ready to plan

## Progress

| Milestone | Phases | Status | Shipped |
|-----------|--------|--------|---------|
| v1.0 Codebase Refactoring | 1-12 | Complete | 2026-01-12 |
| v1.1 Infrastructure Migration | 13-19 | Complete | 2026-01-12 |
| v1.2 Developer Experience | 20+ | Not Started | - |

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

## Context for Next Session

### v1.0 and v1.1 Complete

Both milestones archived:
- `.planning/milestones/v1.0-ROADMAP.md` - Codebase refactoring details
- `.planning/milestones/v1.1-ROADMAP.md` - Infrastructure migration details
- `.planning/MILESTONES.md` - Summary of shipped milestones

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

1. `/gsd:discuss-milestone` to plan v1.2 scope
2. Or directly start Phase 20: Windows DevX

## Files Modified This Session

- `.planning/milestones/v1.0-ROADMAP.md` - Created (v1.0 archive)
- `.planning/milestones/v1.1-ROADMAP.md` - Created (v1.1 archive)
- `.planning/MILESTONES.md` - Created (milestone summary)
- `.planning/ROADMAP.md` - Updated (collapsed milestones)
- `.planning/PROJECT.md` - Updated (current state)
- `.planning/STATE.md` - Updated (this file)

---

*Last updated: 2026-01-12*
