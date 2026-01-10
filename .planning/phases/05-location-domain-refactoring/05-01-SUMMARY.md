---
phase: 05-location-domain-refactoring
plan: 01
subsystem: api
tags: [location, handler, deprecation, cleanup]

# Dependency graph
requires:
  - phase: 04-user-domain-refactoring
    provides: Established patterns for deprecation and handler cleanup
provides:
  - Documented dead code in location handlers
  - Reduced duplicate code surface
affects: [phase-12-final-cleanup]

# Tech tracking
tech-stack:
  added: []
  patterns: []

key-files:
  created: []
  modified:
    - handlers/location_handler.go
    - internal/handlers/location.go

key-decisions:
  - "Skip refactoring deprecated dead code - wasteful to add helper functions to code marked for removal"
  - "Remove duplicate Supabase methods immediately rather than just deprecating"

patterns-established:
  - "Use router.go as source of truth for which handlers are actually used"

issues-created: []

# Metrics
duration: 15min
completed: 2026-01-11
---

# Plan 05-01 Summary: Handler Consolidation

**Documented 3 unused location handlers as deprecated and removed duplicate Supabase methods - reduced dead code by ~107 lines**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-01-11
- **Completed:** 2026-01-11
- **Tasks:** 2/2
- **Files modified:** 2

## Accomplishments

- Analyzed router.go to identify which handlers are actually routed
- Found ALL LocationHandler methods are dead code (instantiated but not routed)
- Added deprecation comments to handlers/location_handler.go (type + 4 methods)
- Added deprecation comments to internal/handlers/location.go (type + 2 methods)
- Documented security gap in internal handler (no authorization check)
- Removed duplicate Supabase methods (107 lines of dead code)

## Task Commits

1. **Task 1: Audit and document handler usage** - `fffea50` (docs)
2. **Task 2: Remove duplicate Supabase methods** - `2ffd282` (refactor)

## Files Modified

- `handlers/location_handler.go` - Added deprecation comments, removed duplicate methods (~107 lines)
- `internal/handlers/location.go` - Added deprecation comments with security warnings

## Router Analysis Findings

| Handler | Location | Status |
|---------|----------|--------|
| `handlers.LocationHandler` | handlers/location_handler.go | Instantiated in main.go but NOT routed |
| `handlers.LocationHandlerSupabase` | handlers/location_handler_supabase.go | **IN USE** - all location routes |
| `internal/handlers.LocationHandler` | internal/handlers/location.go | NOT used anywhere |

## Decisions Made

- **Skip helper function refactoring:** Since all LocationHandler methods are deprecated and marked for removal in Phase 12, adding helper functions would be wasteful. Deviated from plan.
- **Remove duplicates immediately:** UpdateLocationSupabase and GetTripMemberLocationsSupabase were exact duplicates of LocationHandlerSupabase methods - removed rather than just deprecating.

## Deviations from Plan

- Task 2 skipped adding `bindJSONOrError` and `getUserIDOrError` helpers to deprecated code
- Instead focused on removing duplicate methods to reduce code surface

## Issues Encountered

- Build fails due to pre-existing untracked files (notification_service.go, chat_handler.go) with compilation errors
- Verified location files specifically compile correctly

## Security Gaps Documented

- `internal/handlers/location.go:GetTripMemberLocations` has NO trip membership verification
- This allows ANY authenticated user to query ANY trip's locations
- Documented in deprecation comment, handler not routed so no active vulnerability

## Next Step

Ready for Plan 05-02: Store & Authorization Cleanup

---
*Phase: 05-location-domain-refactoring*
*Plan: 01*
*Completed: 2026-01-11*
