---
phase: 01-trip-domain-handler-refactoring
plan: 01
type: execute
---

<objective>
Extract helper functions for response mapping and standardize handler patterns in trip_handler.go.

Purpose: Make handlers pure HTTP glue by moving response composition logic into helper functions, establishing patterns for subsequent handler refactoring phases.
Output: Cleaner trip handlers with extracted helper functions, all tests passing.
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

**Source files:**
@handlers/trip_handler.go
@errors/errors.go
@types/types.go

**Codebase context:**
@.planning/codebase/ARCHITECTURE.md
@.planning/codebase/CONVENTIONS.md

**Constraints from project:**
- No API changes — external contracts must remain identical
- No service layer changes — that's Phase 2
- Tests must pass after each change
- Follow standard Go/Gin idioms

**Phase vision:**
- Handlers become pure HTTP glue
- Zero business logic in handlers
- Consistent patterns across all trip endpoints
</context>

<tasks>

<task type="auto">
  <name>Task 1: Extract destination response helper</name>
  <files>handlers/trip_handler.go</files>
  <action>
Extract the inline destination struct composition (lines 192-214 in CreateTripHandler) into a helper function.

Create a new function:
```go
// buildDestinationResponse builds the destination response object from a Trip
func buildDestinationResponse(trip *types.Trip) DestinationResponse {
    // Move the destination composition logic here
}
```

Also extract the DestinationResponse struct type (currently inline anonymous struct) to a named type at the top of the file near TripWithMembersAndInvitationsResponse.

The CreateTripHandler should then call `dest := buildDestinationResponse(createdTrip)` instead of the inline composition.

Do NOT change the JSON structure or field names — the API contract must remain identical.
  </action>
  <verify>go build ./handlers/... && go test ./handlers/... -v</verify>
  <done>
- DestinationResponse type exists as named struct
- buildDestinationResponse helper function exists
- CreateTripHandler uses the helper
- No changes to API response structure
- Tests pass
  </done>
</task>

<task type="auto">
  <name>Task 2: Extract trip response builder helper</name>
  <files>handlers/trip_handler.go</files>
  <action>
Extract the TripWithMembersAndInvitationsResponse construction (lines 216-230 in CreateTripHandler) into a helper function.

Create a new function:
```go
// buildTripWithMembersResponse builds the full trip response with members and invitations
func buildTripWithMembersResponse(
    trip *types.Trip,
    members []*types.TripMembership,
    invitations []*types.TripInvitation,
) TripWithMembersAndInvitationsResponse {
    dest := buildDestinationResponse(trip)
    return TripWithMembersAndInvitationsResponse{
        // ... field mapping
    }
}
```

Update CreateTripHandler to use this helper. The handler should now be:
1. Bind request
2. Validate user
3. Map request to domain type
4. Call service
5. Call response builder helper
6. Return JSON

This establishes the pattern: handlers do HTTP translation, helpers do mapping.
  </action>
  <verify>go build ./handlers/... && go test ./handlers/... -v</verify>
  <done>
- buildTripWithMembersResponse helper function exists
- CreateTripHandler uses the helper
- Handler is shorter and more focused
- No changes to API response structure
- Tests pass
  </done>
</task>

<task type="auto">
  <name>Task 3: Standardize getUserID helper</name>
  <files>handlers/trip_handler.go</files>
  <action>
Multiple handlers repeat the pattern of extracting userID from context:
```go
userID := c.GetString(string(middleware.UserIDKey))
if userID == "" {
    log.Errorw("No user ID found in context...")
    c.JSON(http.StatusUnauthorized, gin.H{"error": "No authenticated user"})
    return
}
```

This appears in CreateTripHandler (lines 125-131) and ListUserTripsHandler (lines 402-408).

Extract this into a helper function:
```go
// getUserIDFromContext extracts the authenticated user ID from the Gin context.
// Returns empty string if not found (caller should handle unauthorized response).
func getUserIDFromContext(c *gin.Context) string {
    return c.GetString(string(middleware.UserIDKey))
}
```

Update all handlers that get userID to use this helper. The unauthorized check pattern can remain inline since it needs to return from the handler, but the extraction is standardized.

This establishes consistent context key access across all handlers.
  </action>
  <verify>go build ./handlers/... && go test ./handlers/... -v</verify>
  <done>
- getUserIDFromContext helper exists
- All handlers use consistent pattern for userID extraction
- No behavior changes
- Tests pass
  </done>
</task>

</tasks>

<verification>
Before declaring plan complete:
- [ ] `go build ./...` succeeds without errors
- [ ] `go test ./handlers/...` passes all tests
- [ ] `go vet ./handlers/...` reports no issues
- [ ] CreateTripHandler is noticeably shorter (target: under 50 lines)
- [ ] Helper functions are pure (no side effects, just data mapping)
</verification>

<success_criteria>
- All tasks completed
- All verification checks pass
- No errors or warnings introduced
- Handler code is cleaner and more readable
- Patterns established for subsequent refactoring
</success_criteria>

<output>
After completion, create `.planning/phases/01-trip-domain-handler-refactoring/1-01-SUMMARY.md`
</output>
