# Project State

## Milestones

- 🚧 **v1.0 — Codebase Refactoring** (Phases 1-12) - In Progress
- 🚧 **v1.1 — Infrastructure Migration to AWS** (Phases 13-19) - In Progress

## Current Status

**Active Milestone:** v1.1 — Infrastructure Migration to AWS
**Current Phase:** 15 (Complete)
**Phase Status:** All plans complete (15-01, 15-02)

## Progress

| Phase | Status | Started | Completed |
|-------|--------|---------|-----------|
| 1. Trip Domain Handler Refactoring | Complete | 2026-01-10 | 2026-01-10 |
| 2. Trip Domain Service/Model Refactoring | Complete | 2026-01-10 | 2026-01-10 |
| 3. Trip Domain Store Refactoring | Complete | 2026-01-10 | 2026-01-10 |
| 4. User Domain Refactoring | Complete | 2026-01-10 | 2026-01-11 |
| 5. Location Domain Refactoring | Complete | 2026-01-11 | 2026-01-11 |
| 6. Notification Domain Refactoring | In Progress | 2026-01-11 | — |
| 7. Todo Domain Refactoring | Complete | 2026-01-12 | 2026-01-12 |
| 8. Chat Domain Refactoring | Not Started | — | — |
| 9. Weather Service Refactoring | Not Started | — | — |
| 10. Middleware and Cross-Cutting Concerns | Not Started | — | — |
| 11. Event System and WebSocket Refactoring | Not Started | — | — |
| 12. Final Cleanup and Documentation | Not Started | — | — |
| **v1.1 — Infrastructure Migration** | | | |
| 13. AWS EC2 Setup | Complete | 2026-01-11 | 2026-01-11 |
| 14. Coolify Installation | Complete | 2026-01-11 | 2026-01-11 |
| 15. CI/CD Pipeline Migration | Complete | 2026-01-11 | 2026-01-12 |
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
| 2026-01-11 | Switch from OCI to AWS | Oracle Cloud Dubai region had no ARM capacity after 60+ retries |
| 2026-01-11 | Use t4g.small ARM Graviton | Best cost/performance for Go backend + Coolify (~$14/month) |

## Context for Next Session

### v1.0 Context (Active)
- Phases 1-5 complete: Trip Domain, User Domain, and Location Domain fully refactored
- Phase 6 Plan 1 complete: Notification interface consolidation
- **Phase 7 complete:** Todo handler standardized with established patterns (bindJSONOrError, getUserIDFromContext, c.Error())
- Established patterns: bindJSONOrError, getUserIDFromContext, Deprecated: prefix, IsAdminKey context
- **New pattern:** NotificationService (database) vs NotificationFacadeService (AWS facade)
- **Next:** Complete Phase 6 remaining plans, or proceed to Phase 8 (Chat Domain)
- Critical security issue in Phase 4 (admin check) is now RESOLVED
- Remaining critical: Phase 9 (weather permissions)
- Pre-existing test issues: user_handler_test.go missing SearchUsers on mock
- Pre-existing compilation issues: chat_handler.go (undefined methods on TripServiceInterface)

### v1.1 Context (Active)
- **Goal:** Migrate from Cloud Run ($24/month) to AWS EC2 (~$14/month)
- **Target Stack:** AWS EC2 ARM + Coolify + Neon + Upstash + Grafana Cloud
- **Phase 13 Complete:** AWS EC2 t4g.small deployed at 3.130.209.141
- **Phase 14 Complete:** Coolify v4.0.0-beta.460 installed, admin@nomadcrew.uk
- **Phase 15 Complete:** GitHub workflows migrated to Coolify webhook deployment
- **EC2 Instance Name:** sftp (renamed from default)
- **Coolify App:** nomad-crew-backend (GitHub App source, auto-deploy enabled)
- **Critical Note:** Keep existing Neon PostgreSQL and Upstash Redis - they work great
- **Next:** Phase 16 - Application Deployment (configure env vars, first deploy)
- **Coolify Dashboard:** http://3.130.209.141:8000
- **Before first deploy:** Add COOLIFY_WEBHOOK_URL and COOLIFY_WEBHOOK_SECRET to GitHub secrets

## Files Modified This Session

- `.github/workflows/deploy-coolify.yml` - NEW: Coolify webhook deployment workflow (Phase 15-02)
- `.github/SECRETS.md` - NEW: GitHub secrets documentation (Phase 15-02)
- `.github/workflows-archived/` - Cloud Run workflows archived (Phase 15-02)

---

*Last updated: 2026-01-12*
