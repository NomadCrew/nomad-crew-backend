---
phase: 05-location-domain-refactoring
plan: 02
subsystem: api
tags: [location, store, interface, deprecation, security]

# Dependency graph
requires:
  - phase: 05-01
    provides: Handler deprecation and security gap documentation
provides:
  - Deprecated duplicate LocationStore interface
  - Documented interface consolidation path for phase-12
affects: [phase-12-final-cleanup]

# Tech tracking
tech-stack:
  added: []
  patterns: []

key-files:
  created: []
  modified:
    - internal/store/interfaces.go

key-decisions:
  - "Keep deprecated interface rather than delete - safer for any unknown imports"

patterns-established:
  - "Use store.LocationStore (service-oriented) as canonical interface"

issues-created: []

# Metrics
duration: 8min
completed: 2026-01-11
---

# Plan 05-02 Summary: Store & Authorization Cleanup

**Marked duplicate LocationStore interface as deprecated - Phase 5 location domain refactoring complete**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-01-11
- **Completed:** 2026-01-11
- **Tasks:** 2/2 (Task 2 already done in Plan 05-01)
- **Files modified:** 1

## Accomplishments

- Added deprecation comment to `internal/store/interfaces.go` LocationStore interface
- Documented that `store.LocationStore` (store/location_store.go) is the canonical interface
- Added TODO for removal in Phase 12
- Verified internal handler is not routed anywhere

## Task Commits

1. **Task 1: Consolidate LocationStore interfaces** - `d1374aa` (docs)
2. **Task 2: Add authorization check to internal handler** - Already completed in Plan 05-01 (`fffea50`)

**Plan metadata:** (this commit)

## Files Modified

- `internal/store/interfaces.go` - Added deprecation comment to LocationStore interface

## Interface Analysis

| Location | Interface | Status |
|----------|-----------|--------|
| `store/location_store.go` | Service-oriented (UpdateLocation by userID, GetTripMemberLocations, GetUserRole) | **IN USE** - canonical |
| `internal/store/interfaces.go` | CRUD-oriented (CreateLocation, GetLocation, UpdateLocation by id, DeleteLocation, ListTripMemberLocations, BeginTx) | **DEPRECATED** - different signatures |

The two interfaces have incompatible signatures for `UpdateLocation`:
- `store.LocationStore.UpdateLocation(ctx, userID, update)` - service-oriented
- `internal/store.LocationStore.UpdateLocation(ctx, id, update)` - CRUD-oriented

The internal version is only used by the deprecated `internal/handlers.LocationHandler`, which is not routed.

## Decisions Made

- Keep deprecated interface rather than delete immediately to avoid breaking any unknown imports

## Deviations from Plan

### Task 2 Already Complete

Task 2 (add authorization check to internal handler) was already completed in Plan 05-01:
- `internal/handlers/location.go` already has full deprecation comments
- Security gap already documented
- No additional work needed

**Impact:** Plan completed faster than expected, no wasted effort.

## Issues Encountered

None - straightforward deprecation marking.

## Phase 5 Complete

Phase 5: Location Domain Refactoring is now complete.

**Summary of Phase 5:**
- Plan 05-01: Documented all unused handlers as deprecated, removed 107 lines of duplicate code
- Plan 05-02: Deprecated duplicate interface, verified no active security issues

**Cleanup path for Phase 12:**
- Delete `internal/handlers/location.go` (deprecated handler)
- Delete `LocationStore` from `internal/store/interfaces.go` (deprecated interface)

Ready to proceed to Phase 6: Notification Domain Refactoring.

---
*Phase: 05-location-domain-refactoring*
*Plan: 02*
*Completed: 2026-01-11*
