---
phase: 27-test-suite-repair
plan: 01
subsystem: testing
tags: [dependencies, pgx, redis, mocking, test-infrastructure]
dependencies:
  requires: []
  provides: [test-mock-libraries, pgx-v5-compatibility]
  affects: [27-02, 27-03]
tech-stack:
  added: [pgxmock/v4, redismock/v9]
  patterns: [test-mocking, dependency-injection]
key-files:
  created: []
  modified:
    - go.mod
    - go.sum
    - config/database_utils_test.go
    - db/db_client_test.go
    - services/health_service_test.go
decisions:
  - title: pgxmock/v4 for pgx v5
    rationale: pgxmock v4 is the official mock library for jackc/pgx/v5
  - title: Remove sqlmock
    rationale: go-sqlmock incompatible with pgx - must use pgxmock
metrics:
  duration: 5m25s
  completed: 2026-02-04
---

# Phase 27 Plan 01: Install Test Dependencies Summary

**One-liner:** Installed pgxmock/v4 and redismock/v9, fixed pgx v4/v5 import mismatches in config, db, and services test files.

## What Was Done

### Dependencies Added
- **github.com/pashagolub/pgxmock/v4 v4.9.0** - pgx v5 compatible mock library
- **github.com/go-redis/redismock/v9 v9.2.0** - Redis v9 compatible mock library

### Import Fixes
**config/database_utils_test.go:**
- Changed: `jackc/pgx/v4/pgxpool` → `jackc/pgx/v5/pgxpool`

**db/db_client_test.go:**
- Removed: `DATA-DOG/go-sqlmock` (incompatible with pgx)
- Changed: `jackc/pgx/v4/pgxpool` → `jackc/pgx/v5/pgxpool`
- Added: `pashagolub/pgxmock/v4`
- Replaced sqlmock wrapper with pgxmock.NewPool()

**services/health_service_test.go:**
- Changed: `jackc/pgx/v4/pgxpool` → `jackc/pgx/v5/pgxpool`
- Changed: `pashagolub/pgxmock` → `pashagolub/pgxmock/v4`

### Verification Results
✅ pgxmock/v4 and redismock/v9 present in go.mod
✅ Zero pgx/v4 imports in config/, db/, services/ test files
✅ Zero "no required module" errors in these packages

## Commits

| Commit | Type | Description | Files |
|--------|------|-------------|-------|
| e83aeda | chore | Add test mock dependencies (pgxmock/v4, redismock/v9) | go.mod, go.sum |
| 3296d73 | fix | Replace pgx v4 imports with v5 in 3 test files | config/, db/, services/ test files |
| b9dcb63 | fix | Remove unused database/sql import | db/db_client_test.go |

## Deviations from Plan

**None** - Plan executed exactly as written. No auto-fixes or architectural changes required.

## Decisions Made

### 1. Use pgxmock/v4 (not pgxmock v1)
**Context:** The codebase uses jackc/pgx/v5 for database access. Tests need matching mock library.

**Decision:** Install pgxmock/v4 v4.9.0

**Rationale:**
- pgxmock v4 is the official mock for pgx v5 (version numbers don't align)
- pgxmock v1.8.0 is for older pgx versions (v3/v4)
- Ensures API compatibility with production pgxpool.Pool interface

### 2. Remove sqlmock from db tests
**Context:** db/db_client_test.go used DATA-DOG/go-sqlmock with wrapper types

**Decision:** Replace with pgxmock.NewPool() pattern

**Rationale:**
- sqlmock is database/sql-based, incompatible with pgx driver
- pgxmock provides native pgx interface mocking
- Simpler code - no wrapper needed

### 3. Keep internal/store/postgres v4 imports for Plan 03
**Context:** 5 files in internal/store/postgres still use pgx/v4 and sqlmock

**Decision:** Leave them unchanged in Plan 01

**Rationale:**
- Plan 01 focuses on config/db/services packages
- Plan 03 handles internal/store/postgres migration
- Keeps this plan atomic and small

## Known Limitations

### Remaining Compilation Errors

**config package:**
- `ConnectionString` field missing from DatabaseConfig
- `GetConnectionString()` method not found
- These are type errors (Plan 02 fixes)

**services package:**
- `mockPgxPool` type mismatch with `*pgxpool.Pool`
- `ExpectStat()` and `ExpectConfig()` don't exist in pgxmock v4 API
- These are interface errors (Plan 02 fixes)

**Still using pgx/v4:**
- internal/store/postgres/user_store_mock_test.go
- internal/store/postgres/chat_store_mock_test.go
- internal/store/postgres/location_mock_test.go
- internal/store/postgres/todo_mock_test.go
- store/postgres/trip_store_pg_mock_test.go
- (Plan 03 handles these)

### Success Metrics

**Dependency resolution:**
✅ 0 "no required module provides package" errors (was: many)

**Import correctness:**
✅ 0 pgx/v4 imports in config/, db/, services/ (was: 3 files)

**Compilation progress:**
- config: ❌ Compiles but type errors remain (Plan 02)
- db: ✅ Compiles successfully
- services: ❌ Compiles but interface errors remain (Plan 02)

## Next Phase Readiness

### Blockers for Plan 02
**None** - All dependencies installed and imports fixed.

Plan 02 can proceed to fix:
- DatabaseConfig type mismatches (config package)
- pgxmock API usage (services package)
- Test helper functions

### Technical Debt Created
**None** - This plan only adds dependencies and fixes imports. No shortcuts taken.

### Follow-up Required
**Plan 03:** Migrate internal/store/postgres and store/postgres test files from sqlmock to pgxmock/v4 (same pattern as db/db_client_test.go).

## Architecture Impact

### Pattern Established
**pgxmock v4 as standard mock library:**
- All pgx v5 code should use pgxmock/v4 for testing
- Pattern: `mock, err := pgxmock.NewPool()`
- No wrapper types needed

### Dependency Graph
```
Production code:
  jackc/pgx/v5 (database driver)
    ↓
  jackc/pgx/v5/pgxpool (connection pool)

Test code:
  pashagolub/pgxmock/v4 (mock implementation)
    ↓
  implements: pgxpool.Pool interface
```

### Redis Pattern (for future reference)
```
Production: redis/go-redis/v9
Test: go-redis/redismock/v9

Usage:
  mockClient, mockRedis := redismock.NewClientMock()
  mockRedis.ExpectPing().SetVal("PONG")
```

## Lessons Learned

1. **pgxmock version confusion:** pgxmock v4 is for pgx v5 (not v4) - version numbers don't match
2. **sqlmock incompatibility:** Always use driver-specific mocks (pgxmock for pgx, not generic sqlmock)
3. **go mod tidy behavior:** Adds dependencies from imports even if they're wrong versions - fix imports first, then tidy

---

**Plan 01 Status:** ✅ Complete
**Unblocks:** Plan 02 (Test Interface & Type Fixes)
**Duration:** 5 minutes 25 seconds
