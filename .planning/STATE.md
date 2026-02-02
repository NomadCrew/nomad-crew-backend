# Project State

## Milestones

- [v1.0 Codebase Refactoring](milestones/v1.0-ROADMAP.md) (Phases 1-12) — SHIPPED 2026-01-12
- [v1.1 Infrastructure Migration](milestones/v1.1-ROADMAP.md) (Phases 13-19) — SHIPPED 2026-01-12
- **v1.2 Mobile Integration & Quality** (Phases 20-25) — IN PROGRESS

## Current Position

Phase: 21 of 25 (Auth Flow Integration)
Plan: Not started
Status: Ready to plan
Last activity: 2026-02-02 - Mobile testing on physical device, auth & push fixes

Progress: ██░░░░░░░░ 17% (1/6 phases)

## Progress

| Milestone | Phases | Status | Shipped |
|-----------|--------|--------|---------|
| v1.0 Codebase Refactoring | 1-12 | Complete | 2026-01-12 |
| v1.1 Infrastructure Migration | 13-19 | Complete | 2026-01-12 |
| v1.2 Mobile Integration & Quality | 20-25 | In Progress | - |

**Total Phases Completed:** 20 phases, 24 plans

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

1. `/gsd:plan-phase 21` to plan Auth Flow Integration
2. Or continue with frontend bug fixes first

## Session Continuity

Last session: 2026-02-02
Stopped at: Mobile testing complete, auth & push notifications working
Resume file: None

### Phase 20 Deliverables

- `~/.bashrc` - Global shell config with cd hook
- `/n/NomadCrew/.nomadrc` - NomadCrew dev commands
- `.nvmrc` - Node v20.19.0 auto-switching
- Commands: `android-dev`, `eas-install`, `fe-start`, `be-run`, `emu-start`
- Server discovery fix: `REACT_NATIVE_PACKAGER_HOSTNAME=10.0.2.2`

### 2026-02-02 Mobile Testing Session

**Issues Fixed:**
- Google Sign-In DEVELOPER_ERROR 10 → Updated SHA-1 fingerprint in Google Cloud Console (EAS keystore, not system debug keystore)
- Push notifications FIS_AUTH_ERROR → Enabled Firebase Installations API and FCM Registration API in Google Cloud Console
- Push token registration 500 → Ran `user_push_tokens` migration on Coolify PostgreSQL
- Theme/spacing initialization errors → Added missing exports to `spacing.ts`
- Notification module null errors → Added proper initialization in `notifications.ts`
- Project ID undefined → Fixed to use `Constants.expoConfig?.extra?.eas?.projectId`
- Google Places API key missing → Added fallback to check `EXPO_PUBLIC_GOOGLE_PLACES_API_KEY` in PlacesAutocomplete.tsx
- Google Places REQUEST_DENIED → Use unrestricted key for HTTP fetch (not Android-restricted key)
- WebSocket JSON parse error → Removed double JSON.parse in WebSocketManager (data already parsed by WebSocketConnection)

**Working Features (verified 2026-02-02):**
- Google Sign-In (Android)
- Push notification token registration
- Google Places Autocomplete
- WebSocket real-time connection

**Migrations Applied:**
- `user_profiles` table - ✅ Applied
- `user_push_tokens` table - ✅ Applied (2026-02-02)

**Documentation Created:**
- `docs/TROUBLESHOOTING.md` - Backend troubleshooting guide
- Updated `frontend/docs/DEVELOPMENT_SETUP.md` - Added Google Sign-In and Push setup

---

*Last updated: 2026-02-02*
