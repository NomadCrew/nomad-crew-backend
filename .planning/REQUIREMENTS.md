# Requirements: NomadCrew Backend v1.3

**Defined:** 2026-02-04
**Core Value:** Clean, maintainable code with reliable infrastructure - enabling fast feature development for the mobile app.

## v1.3 Requirements

Requirements for Security Remediation & Code Quality milestone. Each maps to roadmap phases.

### Security

- [x] **SEC-01**: Rate limiter fails closed with in-memory fallback when Redis is unavailable
- [x] **SEC-02**: Gin router configured with trusted proxies to prevent X-Forwarded-For spoofing
- [x] **SEC-03**: Notification service uses bounded worker pool instead of unbounded goroutines
- [x] **SEC-04**: Background tasks tracked and awaited during graceful shutdown
- [ ] **SEC-05**: Simulator bypass uses proper JWT claim parsing (check `sub` exactly, not substring)

### Testing

- [x] **TEST-01**: All 22 packages compile without errors (fix pgx v4/v5 import mismatch)
- [x] **TEST-02**: Missing test dependencies installed (pgxmock v4, miniredis v2)
- [x] **TEST-03**: Interface mismatches between mocks and implementations resolved
- [x] **TEST-04**: Duplicate mock declarations consolidated
- [x] **TEST-05**: CI test workflow passes with current coverage threshold

### Dependencies

- [ ] **DEP-01**: nhooyr.io/websocket migrated to github.com/coder/websocket
- [ ] **DEP-02**: lestrrat-go/jwx upgraded from v2.0.21 to v2.1.6 (stay on v2.x branch)
- [ ] **DEP-03**: prometheus/client_golang upgraded from v1.14.0 to v1.21.1
- [ ] **DEP-04**: go-redis/v9 upgraded from v9.7.3 to v9.17.x

### Developer Experience

- [ ] **DEVX-01**: Taskfile.yml added with common development tasks (build, test, lint, docker)
- [ ] **DEVX-02**: .golangci.yml v2 configuration added with appropriate linters
- [ ] **DEVX-03**: Pre-commit hooks configured (go-fmt, go-imports, golangci-lint)
- [ ] **DEVX-04**: .editorconfig added for consistent formatting
- [ ] **DEVX-05**: .env.example files consolidated into single documented file
- [ ] **DEVX-06**: VS Code settings added (.vscode/settings.json, extensions.json)

## Future Requirements

Deferred to later milestones. Tracked but not in current roadmap.

### Testing Enhancements

- **TEST-F01**: Set up mockery v3 for automated mock generation
- **TEST-F02**: Increase CI coverage threshold from 30% to 50%
- **TEST-F03**: Add integration test coverage for all handlers

### Security Enhancements

- **SEC-F01**: Add JWT blacklist capability for emergency token revocation
- **SEC-F02**: Add Content-Security-Policy header to responses
- **SEC-F03**: Add progressive rate limit lockout (longer bans after repeated violations)

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Remove simulator bypass | Essential for local iOS/Android development |
| JWX v3 migration | API breaking changes too extensive; v2.1.6 is actively maintained |
| New API endpoints | Focus is remediation, not features |
| Database schema changes | Schema is stable per project constraints |
| Performance optimization | Not needed at current scale |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| SEC-01 | Phase 26 | Complete |
| SEC-02 | Phase 26 | Complete |
| SEC-03 | Phase 28 | Complete |
| SEC-04 | Phase 28 | Complete |
| SEC-05 | Phase 29 | Pending |
| TEST-01 | Phase 27 | Complete |
| TEST-02 | Phase 27 | Complete |
| TEST-03 | Phase 27 | Complete |
| TEST-04 | Phase 27 | Complete |
| TEST-05 | Phase 27 | Complete |
| DEP-01 | Phase 30 | Pending |
| DEP-02 | Phase 30 | Pending |
| DEP-03 | Phase 30 | Pending |
| DEP-04 | Phase 30 | Pending |
| DEVX-01 | Phase 31 | Pending |
| DEVX-02 | Phase 31 | Pending |
| DEVX-03 | Phase 31 | Pending |
| DEVX-04 | Phase 31 | Pending |
| DEVX-05 | Phase 31 | Pending |
| DEVX-06 | Phase 31 | Pending |

**Coverage:**
- v1.3 requirements: 20 total
- Mapped to phases: 20
- Unmapped: 0

---
*Requirements defined: 2026-02-04*
*Last updated: 2026-02-04 - SEC-03, SEC-04 complete (Phase 28)*
