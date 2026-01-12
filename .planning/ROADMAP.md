# Roadmap: NomadCrew Backend

## Completed Milestones

- [v1.0 Codebase Refactoring](milestones/v1.0-ROADMAP.md) (Phases 1-12) — SHIPPED 2026-01-12
- [v1.1 Infrastructure Migration](milestones/v1.1-ROADMAP.md) (Phases 13-19) — SHIPPED 2026-01-12

---

## 🚧 v1.2 Mobile Integration & Quality (In Progress)

**Milestone Goal:** Get the mobile app working end-to-end with the backend, optimize developer workflow, and establish testing practices.

**Phases:** 20-25 (6 phases)

---

### Phase 20: Windows DevX for Mobile Development

**Goal:** Streamline Windows development experience for building Android/iOS mobile apps with the NomadCrew frontend
**Depends on:** Previous milestone complete
**Research:** Likely (WSL2 performance tuning, Android toolchain optimization)
**Research topics:** WSL2 vs native Windows for React Native, Gradle build cache, Metro bundler tuning
**Plans:** TBD

Plans:
- [ ] 20-01: TBD (run /gsd:plan-phase 20 to break down)

**Scope:**
- Shell configuration (Git Bash .bashrc)
- Android development workflow optimization
- WSL2 integration for React Native (optional path)
- Environment variable cleanup
- Development scripts and aliases
- VS Code workspace configuration

**Success Criteria:**
- `npm run android` works reliably from Git Bash
- Emulator starts in < 30 seconds
- Hot reload works consistently
- Single command to start full development environment
- New developer can set up in < 1 hour with guide

---

### Phase 21: Auth Flow Integration

**Goal:** Verify Supabase auth works end-to-end with mobile app, fix any issues discovered
**Depends on:** Phase 20
**Research:** Unlikely (Supabase auth patterns established)
**Plans:** TBD

Plans:
- [ ] 21-01: TBD (run /gsd:plan-phase 21 to break down)

**Scope:**
- Test JWT token flow from mobile to backend
- Verify refresh token handling
- Test admin role detection with app_metadata
- Fix any auth-related bugs discovered

---

### Phase 22: API Gap Analysis

**Goal:** Identify missing endpoints needed for mobile app, implement what mobile needs
**Depends on:** Phase 21
**Research:** Unlikely (internal CRUD patterns)
**Plans:** TBD

Plans:
- [ ] 22-01: TBD (run /gsd:plan-phase 22 to break down)

**Scope:**
- Audit mobile app API calls vs backend endpoints
- Identify missing or incomplete endpoints
- Implement required endpoints following established patterns
- Update API documentation

---

### Phase 23: Real-time Features

**Goal:** Implement location sharing, chat/messaging, and push notifications
**Depends on:** Phase 22
**Research:** Likely (real-time architecture decision, push notification services)
**Research topics:** WebSocket vs SSE vs Supabase Realtime, FCM/APNs integration, location update frequency
**Plans:** TBD

Plans:
- [ ] 23-01: TBD (run /gsd:plan-phase 23 to break down)

**Scope:**
- Location sharing between trip members
- Real-time chat/messaging
- Push notification integration (FCM for Android, APNs for iOS)
- Real-time trip updates

---

### Phase 24: Bug Discovery & Fixes

**Goal:** Run the app end-to-end to find what's broken, fix discovered issues
**Depends on:** Phase 23
**Research:** Unlikely (debugging and fixing)
**Plans:** TBD

Plans:
- [ ] 24-01: TBD (run /gsd:plan-phase 24 to break down)

**Scope:**
- End-to-end app testing across all features
- Document discovered bugs
- Fix critical and high-priority issues
- Verify fixes in staging environment

---

### Phase 25: E2E Testing Setup

**Goal:** Research and implement automated testing for Go API and mobile integration
**Depends on:** Phase 24
**Research:** Likely (testing frameworks, CI integration)
**Research topics:** Go API testing patterns (testify, httptest), React Native E2E (Detox), CI/CD test integration
**Plans:** TBD

Plans:
- [ ] 25-01: TBD (run /gsd:plan-phase 25 to break down)

**Scope:**
- Set up Go API integration tests
- Configure test database and fixtures
- Explore mobile E2E testing options
- Integrate tests into CI/CD pipeline

---

## Progress

| Phase | Milestone | Plans | Status | Completed |
|-------|-----------|-------|--------|-----------|
| 1-12 | v1.0 | 16/16 | Complete | 2026-01-12 |
| 13-19 | v1.1 | 8/8 | Complete | 2026-01-12 |
| 20. Windows DevX | v1.2 | 0/? | Not started | - |
| 21. Auth Flow | v1.2 | 0/? | Not started | - |
| 22. API Gaps | v1.2 | 0/? | Not started | - |
| 23. Real-time | v1.2 | 0/? | Not started | - |
| 24. Bug Fixes | v1.2 | 0/? | Not started | - |
| 25. E2E Testing | v1.2 | 0/? | Not started | - |

**Total Phases Completed:** 19
**Current Production:** https://api.nomadcrew.uk (AWS EC2 + Coolify)

---

*Created: 2026-01-10*
*Last Updated: 2026-01-12 - v1.2 milestone created*
