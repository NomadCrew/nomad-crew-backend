# Project State

## Milestones

- ✅ **v1.0 — Codebase Refactoring** (Phases 1-12) - COMPLETE
- 🚧 **v1.1 — Infrastructure Migration to AWS** (Phases 13-19) - In Progress

## Current Status

**Active Milestone:** v1.1 — Infrastructure Migration
**Current Phase:** 16 (Complete), 17 next
**Phase Status:** v1.0 milestone COMPLETE! All 12 phases done.

## Progress

| Phase | Status | Started | Completed |
|-------|--------|---------|-----------|
| 1. Trip Domain Handler Refactoring | Complete | 2026-01-10 | 2026-01-10 |
| 2. Trip Domain Service/Model Refactoring | Complete | 2026-01-10 | 2026-01-10 |
| 3. Trip Domain Store Refactoring | Complete | 2026-01-10 | 2026-01-10 |
| 4. User Domain Refactoring | Complete | 2026-01-10 | 2026-01-11 |
| 5. Location Domain Refactoring | Complete | 2026-01-11 | 2026-01-11 |
| 6. Notification Domain Refactoring | Complete | 2026-01-11 | 2026-01-12 |
| 7. Todo Domain Refactoring | Complete | 2026-01-12 | 2026-01-12 |
| 8. Chat Domain Refactoring | Complete | 2026-01-12 | 2026-01-12 |
| 9. Weather Service Refactoring | Complete | 2026-01-12 | 2026-01-12 |
| 10. Middleware and Cross-Cutting Concerns | Complete | 2026-01-12 | 2026-01-12 |
| 11. Event System and WebSocket Refactoring | Complete | 2026-01-12 | 2026-01-12 |
| 12. Final Cleanup and Documentation | Complete | 2026-01-12 | 2026-01-12 |
| **v1.1 — Infrastructure Migration** | | | |
| 13. AWS EC2 Setup | Complete | 2026-01-11 | 2026-01-11 |
| 14. Coolify Installation | Complete | 2026-01-11 | 2026-01-11 |
| 15. CI/CD Pipeline Migration | Complete | 2026-01-11 | 2026-01-12 |
| 16. Application Deployment | Complete | 2026-01-12 | 2026-01-12 |
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

### v1.0 Context (COMPLETE)
- **All 12 phases complete:** Codebase fully refactored
- **Phase 12 complete:** Final cleanup
  - Removed deprecated handlers (ChatHandler, LocationHandler, internal/handlers/location.go)
  - Removed deprecated store interfaces (LocationStore, NotificationStore)
  - Updated remaining TODOs to NOTEs
  - 660+ lines of dead code removed
- Established patterns: bindJSONOrError, getUserIDFromContext, c.Error(), Deprecated: prefix, IsAdminKey context, PublishEventWithContext
- **Architecture pattern:** NotificationService (database) vs NotificationFacadeService (AWS facade)
- All critical security issues RESOLVED (Phase 4 admin check, Phase 9 weather verified)
- Pre-existing test issues: user_handler_test.go missing SearchUsers on mock

### v1.1 Context (Active)
- **Goal:** Migrate from Cloud Run ($24/month) to AWS EC2 (~$14/month)
- **Target Stack:** AWS EC2 ARM + Coolify + Neon + Upstash + Grafana Cloud
- **Phase 13 Complete:** AWS EC2 t4g.small deployed at 3.130.209.141
- **Phase 14 Complete:** Coolify v4.0.0-beta.460 installed, admin@nomadcrew.uk
- **Phase 15 Complete:** GitHub workflows migrated to Coolify webhook deployment
- **Phase 16 Complete:** Application deployed and running healthy
- **Application URL:** http://3.130.209.141:8081
- **Coolify Dashboard:** http://3.130.209.141:8000
- **Coolify App:** nomad-crew-backend (GitHub App source, auto-deploy enabled)
- **Port Mapping:** 8081:8080 (avoids conflict with Coolify on 8080)
- **Next:** Phase 17 - Domain & SSL Configuration (Cloudflare DNS, Let's Encrypt)
- **No GitHub secrets needed** - Coolify GitHub App handles deployment automatically

## Files Modified This Session

- `infrastructure/aws/main.tf` - Added port 8081 to security group (Phase 16-01)
- `handlers/chat_handler.go` - DELETED (Phase 12-01)
- `handlers/location_handler.go` - DELETED (Phase 12-01)
- `internal/handlers/location.go` - DELETED (Phase 12-01)
- `internal/handlers/location_test.go` - DELETED (Phase 12-01)
- `internal/store/interfaces.go` - Removed deprecated interfaces (Phase 12-01)
- `main.go` - Removed unused locationHandler code (Phase 12-01)
- `router/router.go` - Removed LocationHandler from Dependencies (Phase 12-01)
- `logger/logger.go` - TODO → NOTE (Phase 12-01)
- `internal/errors/errors.go` - TODO → NOTE (Phase 12-01)

---

*Last updated: 2026-01-12*
