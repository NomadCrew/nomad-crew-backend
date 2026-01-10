---
phase: 01-trip-domain-handler-refactoring
plan: 02
type: execute
---

<objective>
Standardize error handling and validation patterns across all trip handlers.

Purpose: Ensure consistent error responses and validation patterns, completing the handler refactoring phase.
Output: All trip handlers follow identical patterns, phase ready for completion.
</objective>

<execution_context>
@~/.claude/get-shit-done/workflows/execute-phase.md
@~/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/01-trip-domain-handler-refactoring/1-CONTEXT.md
@.planning/phases/01-trip-domain-handler-refactoring/1-01-SUMMARY.md

**Source files:**
@handlers/trip_handler.go
@errors/errors.go

**Codebase context:**
@.planning/codebase/ARCHITECTURE.md
@.planning/codebase/CONVENTIONS.md

**Constraints from project:**
- No API changes — external contracts must remain identical
- No service layer changes — that's Phase 2
- Tests must pass after each change
- Follow standard Go/Gin idioms

**Established patterns from Plan 01:**
- buildDestinationResponse helper for destination mapping
- buildTripWithMembersResponse helper for full trip response
- getUserIDFromContext helper for context access
</context>

<tasks>

<task type="auto">
  <name>Task 1: Standardize request binding error handling</name>
  <files>handlers/trip_handler.go</files>
  <action>
Multiple handlers have inconsistent patterns for handling JSON binding errors:

CreateTripHandler (lines 116-122):
```go
if err := c.ShouldBindJSON(&req); err != nil {
    log.Errorw("Invalid request for CreateTripHandler", "error", err)
    if bindErr := c.Error(apperrors.ValidationFailed("invalid_request_payload", err.Error())); bindErr != nil {
        log.Errorw("Failed to set error in context for CreateTripHandler", "error", bindErr)
    }
    return
}
```

UpdateTripHandler (lines 295-301):
```go
if err := c.ShouldBindJSON(&update); err != nil {
    log.Errorw("Invalid update data", "error", err)
    if err := c.Error(apperrors.ValidationFailed("Invalid update data", err.Error())); err != nil {
        log.Errorw("Failed to set error in context", "error", err)
    }
    return
}
```

Create a helper function to standardize this:
```go
// bindJSONOrError binds JSON request body and sets validation error if binding fails.
// Returns true if binding succeeded, false if error was set (caller should return).
func bindJSONOrError(c *gin.Context, obj interface{}) bool {
    if err := c.ShouldBindJSON(obj); err != nil {
        log := logger.GetLogger()
        log.Errorw("Invalid request payload", "error", err)
        _ = c.Error(apperrors.ValidationFailed("invalid_request_payload", err.Error()))
        return false
    }
    return true
}
```

Update all handlers that bind JSON to use this helper:
- CreateTripHandler
- UpdateTripHandler
- UpdateTripStatusHandler
- SearchTripsHandler

Each handler should now use:
```go
if !bindJSONOrError(c, &req) {
    return
}
```

This makes the binding pattern consistent and removes duplicate error logging code.
  </action>
  <verify>go build ./handlers/... && go test ./handlers/... -v</verify>
  <done>
- bindJSONOrError helper exists
- All JSON-binding handlers use this helper consistently
- No duplicate binding error handling code
- Tests pass
  </done>
</task>

<task type="auto">
  <name>Task 2: Remove debug logging from handlers</name>
  <files>handlers/trip_handler.go</files>
  <action>
Several handlers contain debug logging that adds noise and doesn't follow production patterns:

CreateTripHandler:
- Line 113: `log.Infow("Received CreateTrip request")` — Remove, router logs requests
- Line 126: `log.Infow("[DEBUG] User ID from context", "userID", userID)` — Remove debug log
- Line 148: `log.Infow("[DEBUG] Trip to be created", "tripToCreate", tripToCreate)` — Remove debug log
- Line 168: `log.Infow("Successfully created trip", "trip", createdTrip)` — Keep but simplify to just tripID

ListUserTripsHandler:
- Line 410: `log.Infow("Listing trips for user", "supabaseUserID", supabaseUserID)` — Remove, unnecessary verbose logging

Keep meaningful error logs and significant operation logs (like "Failed to..." or warnings).
Remove debug-style logs that were added during development.

This follows the CONVENTIONS.md guidance: log errors with context, don't log routine operations.
  </action>
  <verify>go build ./handlers/... && go test ./handlers/... -v</verify>
  <done>
- [DEBUG] prefixed logs removed
- Routine "Received request" logs removed
- Error logging preserved
- Significant operation logs (like Pexels fetch success) preserved
- Tests pass
  </done>
</task>

<task type="auto">
  <name>Task 3: Final handler review and cleanup</name>
  <files>handlers/trip_handler.go</files>
  <action>
Review all handlers for remaining inconsistencies and clean up:

1. **Ensure all handlers follow the same structure:**
   - Extract params/userID
   - Bind request (if needed)
   - Call model/service
   - Handle error or return response

2. **Verify TriggerWeatherUpdateHandler** (lines 525-573):
   - This handler has good structure but verbose error handling
   - The c.Error pattern with double-logging can be simplified
   - Ensure it follows same patterns as other handlers

3. **Verify handleModelError is used consistently:**
   - All handlers should use handleModelError for model/service errors
   - No handler should manually construct error responses for model errors

4. **Remove any remaining inline anonymous structs:**
   - Check for any anonymous struct definitions that should be named types
   - The gin.H{"message": ...} responses are fine for simple success messages

5. **Ensure consistent use of http.Status constants:**
   - All handlers should use http.StatusOK, http.StatusCreated, etc.
   - No magic numbers for status codes

After this task, the trip_handler.go should be:
- Clean and consistent
- All handlers under 50 lines (except CreateTripHandler which may be ~60 due to mapping)
- Following established patterns
- Ready for use as the template for other handler refactoring phases
  </action>
  <verify>go build ./handlers/... && go test ./handlers/... -v && go vet ./handlers/...</verify>
  <done>
- All handlers follow consistent patterns
- No inline anonymous structs (except gin.H which is idiomatic)
- handleModelError used for all model errors
- Consistent http.Status constant usage
- go vet reports no issues
- Tests pass
- Phase 1 complete
  </done>
</task>

</tasks>

<verification>
Before declaring plan complete:
- [ ] `go build ./...` succeeds without errors
- [ ] `go test ./handlers/...` passes all tests
- [ ] `go vet ./handlers/...` reports no issues
- [ ] All handlers follow same structural pattern
- [ ] No [DEBUG] or verbose routine logs remain
- [ ] Helper functions are used consistently
</verification>

<success_criteria>
- All tasks completed
- All verification checks pass
- No errors or warnings introduced
- Trip handlers are clean, consistent, and readable
- Patterns established can be replicated to other domain handlers
- Phase 1: Trip Domain Handler Refactoring complete
</success_criteria>

<output>
After completion, create `.planning/phases/01-trip-domain-handler-refactoring/1-02-SUMMARY.md`

Mark Phase 1 complete in STATE.md.
</output>
