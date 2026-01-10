# Project State

## Milestones

- 🚧 **v1.0 — Codebase Refactoring** (Phases 1-12) - In Progress
- 📋 **v1.1 — Infrastructure Migration to Oracle Cloud** (Phases 13-19) - Planned

## Current Status

**Active Milestone:** v1.1 — Infrastructure Migration to Oracle Cloud
**Current Phase:** 13 (Not Started)
**Phase Status:** Ready to plan Phase 13 - Oracle Cloud Setup

## Progress

| Phase | Status | Started | Completed |
|-------|--------|---------|-----------|
| 1. Trip Domain Handler Refactoring | Complete | 2026-01-10 | 2026-01-10 |
| 2. Trip Domain Service/Model Refactoring | Complete | 2026-01-10 | 2026-01-10 |
| 3. Trip Domain Store Refactoring | Complete | 2026-01-10 | 2026-01-10 |
| 4. User Domain Refactoring | Not Started | — | — |
| 5. Location Domain Refactoring | Not Started | — | — |
| 6. Notification Domain Refactoring | Not Started | — | — |
| 7. Todo Domain Refactoring | Not Started | — | — |
| 8. Chat Domain Refactoring | Not Started | — | — |
| 9. Weather Service Refactoring | Not Started | — | — |
| 10. Middleware and Cross-Cutting Concerns | Not Started | — | — |
| 11. Event System and WebSocket Refactoring | Not Started | — | — |
| 12. Final Cleanup and Documentation | Not Started | — | — |
| **v1.1 — Infrastructure Migration** | | | |
| 13. Oracle Cloud Setup | Not Started | — | — |
| 14. Coolify Installation | Not Started | — | — |
| 15. CI/CD Pipeline Migration | Not Started | — | — |
| 16. Application Deployment | Not Started | — | — |
| 17. Domain & SSL Config | Not Started | — | — |
| 18. Monitoring Setup | Not Started | — | — |
| 19. Cloud Run Decommissioning | Not Started | — | — |

## Blockers

None currently.

## Decisions Made

| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-01-10 | Domain-by-domain approach | Allows complete refactoring of each domain before moving to next |
| 2026-01-10 | All layers equally | No layer is clean enough to skip |
| 2026-01-10 | Tests as safety net | Existing tests validate refactoring correctness |

## Context for Next Session

### v1.0 Context (Paused)
- Phases 1-3 complete: Trip Domain fully refactored (Handler, Service/Model, Store)
- Established patterns: bindJSONOrError, getUserIDFromContext, Deprecated: prefix convention
- Next phase: Phase 4 - User Domain Refactoring
- Phase 4 (User Domain) requires research on admin role implementation
- Critical security issues in Phase 4 (admin check) and Phase 9 (weather permissions)
- Pre-existing test issues: user_handler_test.go missing SearchUsers on mock
- Untracked files with compilation issues: notification_service.go, chat_handler.go (blocks full test suite)

### v1.1 Context (Active)
- **Goal:** Migrate from Cloud Run ($24/month) to Oracle Cloud Always Free ($0/month)
- **Target Stack:** Oracle Cloud ARM + Coolify + Neon + Upstash + Grafana Cloud
- **Phase 13 Research Topics:** OCI account setup, ARM instance provisioning, Always Free limits
- **Critical Note:** Keep existing Neon PostgreSQL and Upstash Redis - they work great
- **Risk:** Oracle Cloud "out of capacity" errors - may need to try different regions

## Files Modified This Session

- `handlers/trip_handler.go` - Refactored (Phase 1)
- `models/trip/service/trip_service.go` - Cleaned up (Phase 2)
- `models/trip/validation/membership.go` - Cleaned up (Phase 2)
- `models/trip/service/trip_model_coordinator.go` - Improved deprecation docs (Phase 2)
- `internal/store/interfaces.go` - Added Deprecated: prefix to Commit/Rollback (Phase 3)
- `internal/store/sqlcadapter/trip_store.go` - Removed verbose logs, updated deprecation docs (Phase 3)

---

*Last updated: 2026-01-10*
