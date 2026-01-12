# NomadCrew Backend

## Current State (v1.1)

**Production URL:** https://api.nomadcrew.uk
**Infrastructure:** AWS EC2 m8g.large (ARM Graviton4) + Coolify
**Database:** Neon PostgreSQL
**Cache:** Upstash Redis
**Monitoring:** Grafana Cloud (synthetic checks)

**Codebase:** 57,583 lines of Go 1.24 with clean layered architecture (Handler -> Service/Model -> Store)

## What This Is

A Go-based REST API with WebSocket support for trip coordination. Powers the NomadCrew mobile app for digital nomads to plan and coordinate group trips.

**Key domains:**
- Trip management with RBAC permissions
- User management with Supabase auth
- Location tracking with real-time sync
- Chat messaging via Supabase Realtime
- Push notifications via Expo
- Todo management within trips

## Core Value

Clean, maintainable code with reliable infrastructure - enabling fast feature development for the mobile app.

## Requirements

### Validated

- Trip CRUD operations with RBAC permissions — v1.0
- User management with Supabase auth integration — v1.0
- Admin role check via JWT app_metadata — v1.0
- Consistent error handling pattern (c.Error()) — v1.0
- Todo management within trips — v1.0
- Location tracking with real-time sync — v1.0
- Chat messaging via Supabase Realtime — v1.0
- Invitation system with email notifications — v1.0
- Push notifications via Expo — v1.0
- WebSocket hub for real-time events — v1.0
- SQLC-based type-safe database access — v1.0
- Rate limiting on auth endpoints — v1.0
- Swagger API documentation — v1.0
- AWS EC2 + Coolify infrastructure — v1.1
- HTTPS with Let's Encrypt SSL — v1.1
- GitHub Actions CI/CD — v1.1
- Synthetic monitoring with Discord alerts — v1.1

### Active

- [ ] Windows developer experience optimization (Phase 20)
- [ ] Mobile app integration testing

### Out of Scope

- New backend features — focus on frontend/mobile development
- Database schema changes — stable schema for now
- Performance optimization — not needed at current scale

## Context

**What shipped in v1.0 (Codebase Refactoring):**
- 12 phases, 16 plans over 3 days
- Fixed critical security issue (admin role hardcoded false)
- Established consistent patterns across all handlers
- Removed 660+ lines of deprecated code
- Verified permission architecture is correct

**What shipped in v1.1 (Infrastructure Migration):**
- 7 phases, 8 plans over 2 days
- Migrated from GCP Cloud Run to AWS EC2
- Set up Coolify for Heroku-like deployments
- Configured SSL, monitoring, and alerting
- Decommissioned GCP resources

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Domain-by-domain refactoring | Complete each domain before moving to next | Good |
| app_metadata for admin status | Server-only, secure from user modification | Good |
| c.Error() + c.Abort() pattern | Consistent error handling | Good |
| AWS over Oracle Cloud | OCI had no ARM capacity | Required |
| m8g.large instance | t4g.small couldn't handle Coolify | Required |
| Synthetic monitoring | Simple, no agent needed | Good |

## Constraints

- **No API changes:** External contracts must remain stable
- **No database changes:** Schema is stable
- **Maintain architecture:** Keep Handler -> Service -> Store pattern
- **Production stability:** Changes must not break api.nomadcrew.uk

---
*Last updated: 2026-01-12 after v1.1 milestone*
