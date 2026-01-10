---
phase: 04-user-domain-refactoring
plan: 01
type: execute
---

<objective>
Implement system-wide admin role check by extracting `app_metadata.is_admin` from JWT claims and propagating via Gin context.

Purpose: Fix the CRITICAL security issue where admin role check is hardcoded to `false`, blocking admin operations.
Output: Working admin role extraction from JWT, context propagation, and updated user handler using real admin status.
</objective>

<execution_context>
~/.claude/get-shit-done/workflows/execute-phase.md
~/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md

# Research (contains recommended approach and code examples):
@.planning/phases/04-user-domain-refactoring/4-RESEARCH.md

# Prior phase summaries (establishes patterns):
@.planning/phases/03-trip-domain-store-refactoring/3-01-SUMMARY.md

# Source files to modify:
@types/user.go
@middleware/jwt_validator.go
@middleware/auth.go
@handlers/user_handler.go

**Established patterns from Phases 1-3:**
- Use Go-standard `Deprecated:` prefix for documentation
- Consistent error handling with `fmt.Errorf` wrapping
- Context key pattern from `middleware.UserIDKey`

**Research findings (from 4-RESEARCH.md):**
- Use `app_metadata.is_admin` from JWT (server-only, secure)
- Do NOT use `user_metadata` (user-editable, insecure)
- Keep clear distinction: `IsAdmin` (system) vs `MemberRoleAdmin` (trip-level)
- Default to `false` when claim is missing
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add IsAdmin to UserClaims and extract from JWT</name>
  <files>types/user.go, middleware/jwt_validator.go</files>
  <action>
1. In `types/user.go`, add `IsAdmin bool` field to `UserClaims` struct (if exists) or create a new claims type.

2. In `middleware/jwt_validator.go`, extend the `extractClaims()` function to extract `app_metadata.is_admin`:
   - After existing `user_metadata` extraction (~line 405)
   - Add extraction of `app_metadata` map
   - Extract `is_admin` boolean, default to `false` if missing
   - Pattern:
     ```go
     if appMetaVal, ok := token.Get("app_metadata"); ok {
         if appMeta, ok := appMetaVal.(map[string]interface{}); ok {
             if isAdmin, ok := appMeta["is_admin"].(bool); ok {
                 claims.IsAdmin = isAdmin
             }
         }
     }
     ```

3. Update the debug log at line 416 to include admin status.

Do NOT modify any other JWT validation logic. Only add the app_metadata extraction.
  </action>
  <verify>go build ./types/... ./middleware/... succeeds</verify>
  <done>UserClaims has IsAdmin field, extractClaims() extracts from app_metadata, defaults to false</done>
</task>

<task type="auto">
  <name>Task 2: Add IsAdminKey context propagation</name>
  <files>middleware/auth.go</files>
  <action>
1. Add new context key for admin status following existing UserIDKey pattern:
   ```go
   const IsAdminKey contextKey = "isAdmin"
   ```

2. In the JWT validation middleware function that sets UserIDKey, also set IsAdminKey:
   - After setting `c.Set(string(UserIDKey), claims.UserID)` or similar
   - Add `c.Set(string(IsAdminKey), claims.IsAdmin)`

3. Search the file for where context is populated from claims and ensure IsAdmin is propagated.

Do NOT change any existing context key patterns - follow the established pattern exactly.
  </action>
  <verify>go build ./middleware/... succeeds, grep confirms IsAdminKey is defined and used</verify>
  <done>IsAdminKey context key defined, admin status set in context alongside user ID</done>
</task>

<task type="auto">
  <name>Task 3: Update user_handler.go to use context admin check</name>
  <files>handlers/user_handler.go</files>
  <action>
1. Replace hardcoded admin check at line ~260:
   - BEFORE: `isAdmin := false // TODO: Implement admin check`
   - AFTER: `isAdmin := c.GetBool(string(middleware.IsAdminKey))`

2. Replace hardcoded admin check at line ~343:
   - Same pattern as above

3. Add import for middleware package if not already present.

4. Remove the TODO comments since they're now resolved.

Do NOT change any business logic in UpdateUser or UpdateUserPreferences - only replace the hardcoded false with context lookup.
  </action>
  <verify>go build ./handlers/... succeeds, grep shows no more "isAdmin := false" in user_handler.go</verify>
  <done>Both admin checks use context lookup, no hardcoded false, no TODO comments for admin check</done>
</task>

</tasks>

<verification>
Before declaring plan complete:
- [ ] `go build ./types/... ./middleware/... ./handlers/...` succeeds
- [ ] `grep -n "isAdmin := false" handlers/user_handler.go` returns no results
- [ ] `grep -n "IsAdminKey" middleware/auth.go` confirms context key exists
- [ ] `grep -n "app_metadata" middleware/jwt_validator.go` confirms extraction exists
</verification>

<success_criteria>

- All tasks completed
- All verification checks pass
- No new errors or warnings introduced
- Admin status extracted from JWT app_metadata
- Admin status propagated via Gin context
- User handler uses context-based admin check (no more hardcoded false)
</success_criteria>

<output>
After completion, create `.planning/phases/04-user-domain-refactoring/4-01-SUMMARY.md`:

# Plan 4-01 Summary: Admin Role Implementation

**[Substantive one-liner about what was accomplished]**

## Tasks Completed

### Task 1: [summary]
### Task 2: [summary]
### Task 3: [summary]

## Verification

- [x] Build succeeds
- [x] Admin extraction implemented
- [x] Context propagation working
- [x] Handler using context lookup

## Files Modified

- `types/user.go` - Description
- `middleware/jwt_validator.go` - Description
- `middleware/auth.go` - Description
- `handlers/user_handler.go` - Description

## Notes

[Any observations about the implementation]

## Admin Bootstrapping

To grant admin status to a user:
1. Use Supabase dashboard or Admin API
2. Set `app_metadata.is_admin = true` for the user
3. User must re-login to get updated JWT claims

## Next Steps

Ready for Plan 4-02: Trip membership check and handler cleanup.
</output>
