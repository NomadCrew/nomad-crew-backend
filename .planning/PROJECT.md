# NomadCrew Backend Refactoring

## What This Is

A comprehensive refactoring effort for the NomadCrew backend API - a Go-based REST API with WebSocket support for trip coordination. The goal is to reduce complexity, remove duplication, and improve architecture across all layers and domains while maintaining existing functionality.

## Core Value

Clean, maintainable code that is easy to understand, extend, and debug - making future development faster and reducing bugs.

## Requirements

### Validated

<!-- Existing functionality that must be preserved -->

- ✓ Trip CRUD operations with RBAC permissions — existing
- ✓ User management with Supabase auth integration — existing
- ✓ Todo management within trips — existing
- ✓ Location tracking with real-time sync — existing
- ✓ Chat messaging via Supabase Realtime — existing
- ✓ Invitation system with email notifications — existing
- ✓ Push notifications via Expo — existing
- ✓ WebSocket hub for real-time events — existing
- ✓ SQLC-based type-safe database access — existing
- ✓ Rate limiting on auth endpoints — existing
- ✓ Swagger API documentation — existing

### Active

<!-- Refactoring goals for this effort -->

- [ ] Reduce handler complexity across all domains
- [ ] Implement proper admin role checks (currently hardcoded false)
- [ ] Add missing permission checks in weather and user services
- [ ] Remove code duplication across handlers and services
- [ ] Improve layer separation (handler → service → store)
- [ ] Clean up TODO/FIXME technical debt
- [ ] Ensure consistent error handling patterns
- [ ] Improve interface definitions for better testability

### Out of Scope

- New features — pure refactoring, no functional changes
- Database schema changes — no migrations
- API contract changes — all endpoints remain the same
- Dependency upgrades — keep current versions
- Performance optimization — focus on code quality first

## Context

**Codebase state:** Brownfield Go 1.24 backend with layered architecture (Handler → Service/Model → Store). Uses Gin for HTTP, pgx/SQLC for database, Redis for events/caching, Supabase for auth.

**Key domains:**
- Trip (primary domain) - `models/trip/`, `handlers/trip_handler.go`
- User - `models/user/`, `handlers/user_handler.go`
- Location - `models/location/`, `handlers/location_handler.go`
- Notifications - `models/notification/`, `handlers/notification_handler.go`
- Chat - `handlers/chat_handler_supabase.go`
- Todo - `models/model.go`, `handlers/todo_handler.go`

**Known tech debt:**
- Admin check hardcoded to `false` in `handlers/user_handler.go:260,343`
- Missing trip membership check in `handlers/user_handler.go:620`
- TODO comments for permission checks in weather service
- Incomplete integration tests

**Codebase mapping:** `.planning/codebase/` contains 7 documents with detailed analysis.

## Constraints

- **Tests must pass**: All existing tests must continue to pass after refactoring
- **No API changes**: External API contracts (endpoints, request/response formats) must remain unchanged
- **No database changes**: No migrations or schema modifications
- **Preserve architecture**: Keep the layered architecture pattern (Handler → Service → Store)

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Domain-by-domain approach | Allows complete refactoring of each domain before moving to next | — Pending |
| All layers equally | No layer is clean enough to skip | — Pending |
| Tests as safety net | Existing tests validate refactoring correctness | — Pending |

---
*Last updated: 2026-01-10 after initialization*
