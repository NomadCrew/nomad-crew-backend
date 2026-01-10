---
phase: 02-trip-domain-service-model-refactoring
plan: 01
type: execute
---

<objective>
Clean up trip service layer: remove debug logging, address TODOs, and improve consistency.

Purpose: Complete service layer cleanup following established patterns from Phase 1.
Output: Clean, consistent trip service code ready for store refactoring in Phase 3.
</objective>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/02-trip-domain-service-model-refactoring/2-CONTEXT.md

**Source files:**
@models/trip/service/trip_service.go
@models/trip/service/member_service.go
@models/trip/service/trip_model_coordinator.go
@models/trip/validation/membership.go

**Constraints:**
- No API changes
- No store layer changes (Phase 3)
- Tests must pass after each change
</context>

<tasks>

<task type="auto">
  <name>Task 1: Remove debug logging from trip_service.go</name>
  <files>models/trip/service/trip_service.go</files>
  <action>
Remove the debug logging in CreateTrip method (lines 46-52):

```go
logger.GetLogger().Infow("[DEBUG] trip.CreatedBy in service before DB call", "type", fmt.Sprintf("%T", trip.CreatedBy), "value", func() string {
    if trip.CreatedBy != nil {
        return *trip.CreatedBy
    } else {
        return "<nil>"
    }
}())
```

This debug log adds unnecessary noise and shouldn't be in production code.

Also check for any other [DEBUG] prefixed logs in the file and remove them.
  </action>
  <verify>go build ./models/trip/... && go test ./models/trip/... -v -short</verify>
  <done>
- Debug log removed from CreateTrip
- No other [DEBUG] logs remain in trip_service.go
- Build and tests pass
  </done>
</task>

<task type="auto">
  <name>Task 2: Address membership validation TODO</name>
  <files>models/trip/validation/membership.go</files>
  <action>
The ValidateMembershipStatus function has a TODO comment and commented-out code:

```go
func ValidateMembershipStatus(oldStatus, newStatus types.MembershipStatus) error {
    // if oldStatus == types.MembershipStatusInvited && newStatus != types.MembershipStatusActive { // MembershipStatusInvited removed
    // ... commented code ...
    // TODO: Add any other relevant membership status transition validation if needed.
    // For now, with only ACTIVE/INACTIVE, direct transitions are usually allowed based on auth.
    return nil
}
```

Options:
1. If ACTIVE/INACTIVE are the only statuses and no validation is needed, simplify the function to just return nil with a clear comment explaining the design decision.
2. Remove the commented-out code that references the removed MembershipStatusInvited.

Update to:
```go
// ValidateMembershipStatus validates membership status transitions.
// With only ACTIVE and INACTIVE statuses, transitions are controlled by authorization
// rather than state machine validation.
func ValidateMembershipStatus(oldStatus, newStatus types.MembershipStatus) error {
    // Status transitions are authorized at the handler/service level.
    // No additional validation needed for ACTIVE <-> INACTIVE transitions.
    return nil
}
```
  </action>
  <verify>go build ./models/trip/... && go test ./models/trip/validation/... -v</verify>
  <done>
- Commented-out code removed
- TODO addressed with clear documentation
- Function purpose clearly documented
- Tests pass
  </done>
</task>

<task type="auto">
  <name>Task 3: Clean up deprecated chat methods in coordinator</name>
  <files>models/trip/service/trip_model_coordinator.go</files>
  <action>
The TripModelCoordinator has deprecated chat methods that return empty/no-op:

```go
// ListMessages is deprecated - chat functionality has moved to models/chat/service
func (c *TripModelCoordinator) ListMessages(...) ([]*types.ChatMessage, error) {
    return []*types.ChatMessage{}, nil
}

// UpdateLastReadMessage is deprecated - chat functionality has moved to models/chat/service
func (c *TripModelCoordinator) UpdateLastReadMessage(...) error {
    return nil
}
```

Also, ChatService field is always nil:
```go
ChatService       TripChatServiceInterface
var chatServiceInstance TripChatServiceInterface = nil
```

Options:
1. Remove these methods entirely if nothing depends on them
2. Keep them but add clearer deprecation notices

First, check if these methods are part of the TripModelInterface:
- If they ARE in the interface, they must stay (but can have clearer comments)
- If they are NOT in the interface, they can be removed

For now, keep them with improved deprecation documentation since they may be part of backward compatibility.
  </action>
  <verify>go build ./models/trip/... && go test ./models/trip/... -v -short</verify>
  <done>
- Deprecated methods documented clearly
- ChatService field handling documented
- Build passes
  </done>
</task>

</tasks>

<verification>
Before declaring plan complete:
- [ ] `go build ./models/trip/...` succeeds
- [ ] `go test ./models/trip/...` passes (or runs with -short)
- [ ] No [DEBUG] logs remain in service files
- [ ] TODOs addressed or documented
- [ ] Deprecated code clearly marked
</verification>

<success_criteria>
- All tasks completed
- All verification checks pass
- Service layer is clean and consistent
- Ready to proceed to Phase 3: Trip Domain Store Refactoring
</success_criteria>

<output>
After completion, create `.planning/phases/02-trip-domain-service-model-refactoring/2-01-SUMMARY.md`
Update STATE.md to mark Phase 2 complete.
</output>
