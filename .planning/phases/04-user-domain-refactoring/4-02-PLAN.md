---
phase: 04-user-domain-refactoring
plan: 02
type: execute
---

<objective>
Resolve the incomplete trip membership check in SearchUsers and apply consistent patterns to user handler.

Purpose: Clean up incomplete code (tripId parameter accepted but not used) and standardize patterns.
Output: Either working trip membership check OR removed unused parameter with updated API docs.
</objective>

<execution_context>
~/.claude/get-shit-done/workflows/execute-phase.md
~/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md

# Prior plan summary:
@.planning/phases/04-user-domain-refactoring/4-01-SUMMARY.md

# Source files:
@handlers/user_handler.go
@types/user.go

**Issue from CONCERNS.md:**
- File: `handlers/user_handler.go:620`
- Problem: TODO to add trip membership check when TripStore available
- The `tripId` query parameter is documented in swagger but silently ignored
- This creates API inconsistency - parameter accepted but has no effect

**Established patterns from Phases 1-3:**
- Remove dead code and unused variables
- Keep APIs honest - don't accept parameters that do nothing
</context>

<tasks>

<task type="auto">
  <name>Task 1: Remove unused tripId parameter from SearchUsers</name>
  <files>handlers/user_handler.go</files>
  <action>
The tripId parameter is accepted but never used - the TripStore dependency was never added. Rather than leave incomplete code, remove it cleanly:

1. Remove the swagger annotation for tripId parameter (~line 578):
   - DELETE: `// @Param tripId query string false "Trip ID to check membership (marks users as isMember)"`

2. Remove the tripId query extraction (~line 598):
   - DELETE: `tripID := c.Query("tripId")`

3. Remove the TODO comment block (~lines 618-621):
   - DELETE the comment about trip membership check
   - DELETE: `_ = tripID // Silence unused variable warning`

4. Add a brief comment explaining the API returns user info without membership context:
   ```go
   // Return search results
   // Note: Trip membership info not included - use trip member endpoints for that
   ```

This is the clean approach: don't accept parameters that do nothing. If trip membership filtering is needed in the future, it can be added as a proper feature.

Do NOT change any other logic in SearchUsers.
  </action>
  <verify>go build ./handlers/... succeeds, grep shows no "tripID" in SearchUsers function</verify>
  <done>tripId parameter removed from swagger docs and handler, no dead code, API is honest</done>
</task>

<task type="auto">
  <name>Task 2: Verify user handler follows established patterns</name>
  <files>handlers/user_handler.go</files>
  <action>
Review the user handler for pattern consistency with Phase 1-3 changes:

1. Check error handling uses consistent pattern:
   - Should use `apperrors.AppError` type checks
   - Should not use strings.Contains for error type detection

2. Check logging patterns:
   - Remove any verbose success logs for routine read operations
   - Keep error logs with consistent format

3. Check for any remaining TODO comments that should be addressed:
   - If security-related, flag for fixing
   - If enhancement-related, document in ISSUES.md

Do minimal changes - only fix patterns that are clearly inconsistent with established Phase 1-3 patterns. Do NOT refactor working code without clear justification.
  </action>
  <verify>go build ./handlers/... succeeds</verify>
  <done>User handler follows established error handling and logging patterns</done>
</task>

</tasks>

<verification>
Before declaring plan complete:
- [ ] `go build ./handlers/...` succeeds
- [ ] `grep -n "tripID" handlers/user_handler.go` returns no results (or only in unrelated functions)
- [ ] No swagger annotation for tripId in SearchUsers
- [ ] No remaining TODO comments for admin check
</verification>

<success_criteria>

- All tasks completed
- All verification checks pass
- No dead code (unused tripId parameter removed)
- API documentation matches actual behavior
- Handler follows established patterns
- Phase 4 complete
</success_criteria>

<output>
After completion, create `.planning/phases/04-user-domain-refactoring/4-02-SUMMARY.md`:

# Plan 4-02 Summary: User Handler Cleanup

**[Substantive one-liner about what was accomplished]**

## Tasks Completed

### Task 1: [summary]
### Task 2: [summary]

## Verification

- [x] Build succeeds
- [x] Dead code removed
- [x] Patterns consistent

## Files Modified

- `handlers/user_handler.go` - Description

## Notes

[Any observations]

## Phase 4 Complete

User Domain Refactoring complete:
- Admin role check implemented (4-01)
- Handler cleanup completed (4-02)

Ready to proceed to Phase 5: Location Domain Refactoring.
</output>
