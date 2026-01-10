# Project State

## Milestones

- 🚧 **v1.0 — Codebase Refactoring** (Phases 1-12) - In Progress
- 📋 **v1.1 — Infrastructure Migration to Oracle Cloud** (Phases 13-19) - Planned

## Current Status

**Active Milestone:** v1.0 — Codebase Refactoring
**Current Phase:** 4 (Complete)
**Phase Status:** Phase 4 complete, ready for Phase 5

## Progress

| Phase | Status | Started | Completed |
|-------|--------|---------|-----------|
| 1. Trip Domain Handler Refactoring | Complete | 2026-01-10 | 2026-01-10 |
| 2. Trip Domain Service/Model Refactoring | Complete | 2026-01-10 | 2026-01-10 |
| 3. Trip Domain Store Refactoring | Complete | 2026-01-10 | 2026-01-10 |
| 4. User Domain Refactoring | Complete | 2026-01-10 | 2026-01-11 |
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
| 2026-01-10 | Use app_metadata for admin status | Server-only, secure - cannot be modified by users |
| 2026-01-10 | ValidateAndGetClaims in AuthMiddleware | Full claim access including IsAdmin |
| 2026-01-11 | Remove unused tripId from SearchUsers | Don't accept parameters that do nothing |

## Context for Next Session

### v1.0 Context (Active)
- Phases 1-4 complete: Trip Domain and User Domain fully refactored
- Established patterns: bindJSONOrError, getUserIDFromContext, Deprecated: prefix, IsAdminKey context
- **Next:** Phase 5 - Location Domain Refactoring
- Critical security issue in Phase 4 (admin check) is now RESOLVED
- Remaining critical: Phase 9 (weather permissions)
- Pre-existing test issues: user_handler_test.go missing SearchUsers on mock
- Untracked files with compilation issues: notification_service.go, chat_handler.go (blocks full test suite)

### v1.1 Context (Planned)
- **Goal:** Migrate from Cloud Run ($24/month) to Oracle Cloud Always Free ($0/month)
- **Target Stack:** Oracle Cloud ARM + Coolify + Neon + Upstash + Grafana Cloud
- **Phase 13 Research Topics:** OCI account setup, ARM instance provisioning, Always Free limits
- **Critical Note:** Keep existing Neon PostgreSQL and Upstash Redis - they work great
- **Risk:** Oracle Cloud "out of capacity" errors - may need to try different regions

## Files Modified This Session

- `handlers/user_handler.go` - Removed unused tripId parameter from SearchUsers (Phase 4-02)

---

*Last updated: 2026-01-11*
