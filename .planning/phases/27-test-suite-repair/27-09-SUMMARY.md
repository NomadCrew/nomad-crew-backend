---
phase: 27
plan: 09
subsystem: testing
tags: [store-tests, postgres, unused-variables, type-fixes]
dependency-graph:
  requires: [27-03, 27-04]
  provides: [internal/store/postgres test compilation]
  affects: [27-10]
tech-stack:
  added: []
  patterns: [blank-identifiers, testing.TB]
key-files:
  created: []
  modified:
    - internal/store/postgres/chat_store_mock_test.go
    - internal/store/postgres/location_mock_test.go
    - internal/store/postgres/todo_mock_test.go
    - internal/store/postgres/user_store_mock_test.go
decisions:
  - Use blank identifiers for unused db/ctx variables in stub tests
  - Update struct field references to match current types
  - Use testing.TB interface for benchmark compatibility
metrics:
  duration: ~12 minutes
  completed: 2026-02-04
---

# Phase 27 Plan 09: Fix Internal Store Postgres Test Compilation Summary

Fixed internal/store/postgres package test compilation by addressing unused variables and type mismatches.

## What Was Done

### Task 1: Fix unused variables and type errors in internal/store/postgres tests

**Files modified:**
- `internal/store/postgres/chat_store_mock_test.go`
- `internal/store/postgres/location_mock_test.go`
- `internal/store/postgres/todo_mock_test.go`
- `internal/store/postgres/user_store_mock_test.go`

**Issues fixed:**

1. **Unused variable errors** - Tests declared `db` and `ctx` variables that were never used (stub tests)
   - Changed `db, mock, cleanup := setupMockDB(t)` to `_, mock, cleanup := setupMockDB(t)`
   - Changed `ctx := context.Background()` to `_ = context.Background()`

2. **types.User field mismatches** - Test helpers used non-existent fields
   - `DisplayName` -> `FirstName` + `LastName`
   - `Bio` -> removed (not in struct)
   - `AvatarURL` -> `ProfilePictureURL`
   - `LastSeen` -> `LastSeenAt`
   - `SupabaseID` -> removed (not in struct)

3. **types.MemberLocation structure** - Test helper used wrong field structure
   - Updated to use embedded `Location` struct with `UserName` and `UserRole` fields

4. **types.TodoStatus constants** - Tests used non-existent constants
   - `TodoStatusPending` -> `TodoStatusIncomplete`
   - `TodoStatusInProgress` -> removed (only 2 states)
   - `TodoStatusCompleted` -> `TodoStatusComplete`

5. **types.LocationUpdate.Timestamp** - Test used wrong type
   - Changed from `time.Time` to `int64` (Unix milliseconds)

6. **types.SupabaseUser.UserMetadata** - Test used wrong type
   - Changed from `map[string]interface{}` to `types.UserMetadata` struct

7. **Benchmark function signatures** - `setupMockDB` expected `*testing.T`
   - Changed `setupMockDB(t *testing.T)` to `setupMockDB(t testing.TB)` to support both `*testing.T` and `*testing.B`

8. **Unused imports** - Removed unused imports
   - Removed `database/sql`, `encoding/json` (from most files)
   - Removed `github.com/stretchr/testify/assert`, `github.com/stretchr/testify/require` (where unused)
   - Removed `github.com/jackc/pgx/v5`, `github.com/jackc/pgx/v5/pgxpool`, `fmt` from user_store_mock_test.go

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] types.PaginationParams has no Cursor field**
- **Found during:** Task 1
- **Issue:** Tests used `Cursor` field that doesn't exist in `types.PaginationParams`
- **Fix:** Changed cursor pagination test to offset pagination
- **Files modified:** `internal/store/postgres/chat_store_mock_test.go`
- **Commit:** 49e0475

**2. [Rule 3 - Blocking] types.MemberLocation struct mismatch**
- **Found during:** Task 1
- **Issue:** Test helper created `types.MemberLocation` with wrong field structure
- **Fix:** Updated to use embedded `Location` struct
- **Files modified:** `internal/store/postgres/location_mock_test.go`
- **Commit:** 49e0475

**3. [Rule 3 - Blocking] types.TodoStatus constants don't exist**
- **Found during:** Task 1
- **Issue:** Tests used `TodoStatusPending`, `TodoStatusInProgress`, `TodoStatusCompleted` which don't exist
- **Fix:** Changed to `TodoStatusIncomplete` and `TodoStatusComplete`
- **Files modified:** `internal/store/postgres/todo_mock_test.go`
- **Commit:** 49e0475

**4. [Rule 3 - Blocking] types.User struct fields changed**
- **Found during:** Task 1
- **Issue:** Tests used old field names (`DisplayName`, `Bio`, `AvatarURL`, etc.)
- **Fix:** Updated to current field names (`FirstName`, `LastName`, `ProfilePictureURL`, etc.)
- **Files modified:** `internal/store/postgres/user_store_mock_test.go`
- **Commit:** 49e0475

**5. [Rule 3 - Blocking] types.SupabaseUser.UserMetadata type mismatch**
- **Found during:** Task 1
- **Issue:** Tests used `map[string]interface{}` but actual type is `types.UserMetadata` struct
- **Fix:** Changed to use `types.UserMetadata` struct
- **Files modified:** `internal/store/postgres/user_store_mock_test.go`
- **Commit:** 49e0475

## Verification Results

```bash
# Package compiles successfully
$ go test -c ./internal/store/postgres/...
# (exits 0, no errors)

# Tests pass
$ go test -v ./internal/store/postgres/...
PASS
ok  	github.com/NomadCrew/nomad-crew-backend/internal/store/postgres	1.656s
```

## Commits

| Hash | Type | Description |
|------|------|-------------|
| 49e0475 | fix | Fix internal/store/postgres test compilation |

## Notes for Future Sessions

1. **These are stub tests** - The test functions set up mock expectations but don't actually call store methods. They're placeholders for future implementation.

2. **Type definitions may drift** - The test files reference types from `github.com/NomadCrew/nomad-crew-backend/types`. If those types change, tests may break again.

3. **UserStore not implemented** - The tests reference a `UserStore` type that doesn't exist in this package. Tests were updated to avoid direct references.
