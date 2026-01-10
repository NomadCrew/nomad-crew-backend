---
phase: 03-trip-domain-store-refactoring
plan: 01
type: execute
---

<objective>
Clean up trip store interface and implementation, ensure consistent patterns across all store methods.

Purpose: Complete the trip domain refactoring by standardizing the data access layer.
Output: Cleaned TripStore interface and implementation with consistent error handling and deprecation documentation.
</objective>

<execution_context>
~/.claude/get-shit-done/workflows/execute-phase.md
~/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md

# Prior phase summaries (establishes patterns):
@.planning/phases/01-trip-domain-handler-refactoring/1-02-SUMMARY.md
@.planning/phases/02-trip-domain-service-model-refactoring/2-01-SUMMARY.md

# Source files:
@internal/store/interfaces.go
@internal/store/sqlcadapter/trip_store.go
@internal/store/sqlcadapter/converters.go

**Established patterns from Phases 1-2:**
- Use Go-standard `Deprecated:` prefix for documentation
- Consistent error handling with `fmt.Errorf` wrapping
- Remove debug/verbose logging, keep meaningful error logs
- Functions should be clearly documented

**Constraining decisions:**
- Phase 1: Established `handleModelError` and helper function patterns
- Phase 2: Used `Deprecated:` prefix convention for deprecated methods
</context>

<tasks>

<task type="auto">
  <name>Task 1: Clean up TripStore interface deprecation documentation</name>
  <files>internal/store/interfaces.go</files>
  <action>
Update the TripStore interface to properly document deprecated methods:
1. Add `Deprecated:` comment prefix to `Commit()` and `Rollback()` methods in the interface
2. Document that `BeginTx()` should be used instead, and transaction methods called on the returned `DatabaseTransaction`
3. Review if any other methods in TripStore interface are unused or should be documented

Do NOT remove the deprecated methods (would break interface compatibility). Just document them clearly following Go conventions.
  </action>
  <verify>go build ./internal/store/... succeeds</verify>
  <done>TripStore interface has clear deprecation documentation on Commit() and Rollback() methods</done>
</task>

<task type="auto">
  <name>Task 2: Standardize trip_store.go error handling and logging</name>
  <files>internal/store/sqlcadapter/trip_store.go</files>
  <action>
Review and standardize patterns in trip_store.go:

1. **Review logging consistency:**
   - Ensure error logs use consistent format: `log.Errorw("Failed to [action]", "key", value, "error", err)`
   - Remove any success logs that are too verbose (routine operations don't need logging)
   - Keep meaningful success logs only for significant operations (CreateTrip, SoftDeleteTrip are fine)

2. **Review error wrapping:**
   - All errors should use `fmt.Errorf("failed to [action]: %w", err)` pattern
   - Ensure `apperrors.NotFound()` is used consistently for not-found cases

3. **Improve deprecated method documentation:**
   - Update `Commit()` and `Rollback()` methods to use `Deprecated:` prefix (Go convention)
   - Current warnings are good but documentation should match interface

4. **Remove any verbose/debug logs** that aren't needed in production

Do NOT change any business logic or method signatures.
  </action>
  <verify>go build ./internal/store/sqlcadapter/... succeeds</verify>
  <done>trip_store.go has consistent error handling, logging patterns, and proper deprecation documentation</done>
</task>

<task type="auto">
  <name>Task 3: Verify build and run existing tests</name>
  <files>internal/store/sqlcadapter/trip_store.go, internal/store/interfaces.go</files>
  <action>
1. Run `go build ./...` to ensure no compilation issues
2. Run `go test ./internal/store/...` to verify store tests pass
3. Run `go test ./models/trip/...` to ensure trip domain still works with store changes
4. Document any pre-existing test issues (don't fix unrelated issues)
  </action>
  <verify>go build ./... succeeds, go test ./internal/store/... and go test ./models/trip/... pass (or document pre-existing failures)</verify>
  <done>All builds succeed, tests pass or pre-existing failures documented</done>
</task>

</tasks>

<verification>
Before declaring phase complete:
- [ ] `go build ./...` succeeds without errors
- [ ] `go test ./internal/store/...` passes
- [ ] `go test ./models/trip/...` passes
- [ ] TripStore interface has proper deprecation documentation
- [ ] trip_store.go follows consistent error/logging patterns
</verification>

<success_criteria>

- All tasks completed
- All verification checks pass
- No new errors or warnings introduced
- Deprecation documentation follows Go conventions (`Deprecated:` prefix)
- Error handling is consistent across all store methods
- Phase 3 complete
</success_criteria>

<output>
After completion, create `.planning/phases/03-trip-domain-store-refactoring/3-01-SUMMARY.md`:

# Plan 3-01 Summary: Trip Domain Store Cleanup

**[Substantive one-liner about what was accomplished]**

## Tasks Completed

### Task 1: [summary]
### Task 2: [summary]
### Task 3: [summary]

## Verification

- [x] Build succeeds
- [x] Tests pass
- [x] Documentation updated

## Files Modified

- `internal/store/interfaces.go` - Description
- `internal/store/sqlcadapter/trip_store.go` - Description

## Notes

[Any observations about the store layer quality]

## Phase 3 Complete

Ready to proceed to Phase 4: User Domain Refactoring.
</output>
