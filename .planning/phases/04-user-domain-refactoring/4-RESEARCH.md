# Phase 4: User Domain Refactoring - Research

**Researched:** 2026-01-10
**Domain:** Supabase Admin Role Implementation & Go RBAC
**Confidence:** HIGH

<research_summary>
## Summary

Researched implementation options for system-wide admin role checking in a Go backend with Supabase authentication. The current implementation has hardcoded `isAdmin := false` at two locations in the user handler.

The recommended approach is to use **Supabase app_metadata with JWT claim extraction**. This leverages Supabase's built-in `app_metadata` field (which is server-only and cannot be modified by users) to store an `is_admin` boolean. The existing JWT validation middleware can be extended to extract this claim.

Key distinction: This research addresses **system-level admin** (can modify any user's profile) vs the existing **trip-level MemberRoleAdmin** (trip membership role). These are separate concepts and should remain so.

**Primary recommendation:** Store `is_admin` in Supabase `app_metadata`, extract it via existing `lestrrat-go/jwx` middleware, expose via context key. Simple, secure, no extra DB calls.
</research_summary>

<standard_stack>
## Standard Stack

### Already In Use (No Changes)
| Library | Version | Purpose | Status |
|---------|---------|---------|--------|
| github.com/lestrrat-go/jwx/v2 | v2.0.21 | JWT/JWKS validation | Already handles Supabase tokens |
| github.com/golang-jwt/jwt/v5 | v5.2.1 | JWT claims types | Used in internal auth |
| github.com/supabase-community/supabase-go | v0.0.4 | Supabase Admin API | Already available for setting app_metadata |

### No New Libraries Needed

The existing stack is sufficient. The implementation requires:
1. Extending JWT claim extraction in `middleware/jwt_validator.go`
2. Using existing Supabase Admin API to set `app_metadata.is_admin`
3. Adding context key for admin status in `middleware/auth.go`

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| app_metadata claim | Database lookup | DB lookup adds latency per request, but is simpler to implement |
| app_metadata claim | Custom Access Token Hook | Hook is more scalable but requires Supabase Pro tier or self-hosted |
| Manual claim extraction | Casbin RBAC library | Casbin is overkill for simple admin check; useful if complex permission matrix needed |
</standard_stack>

<architecture_patterns>
## Architecture Patterns

### Recommended: Claim-Based Admin Check

**Pattern:** Extract admin status from JWT `app_metadata` at middleware level, inject into request context.

```go
// In middleware/jwt_validator.go - extend extractClaims()
func extractClaims(token jwt.Token) (*types.UserClaims, error) {
    claims := &types.UserClaims{}
    // ... existing extraction ...

    // Extract app_metadata for admin status
    if appMetaVal, ok := token.Get("app_metadata"); ok {
        if appMeta, ok := appMetaVal.(map[string]interface{}); ok {
            if isAdmin, ok := appMeta["is_admin"].(bool); ok {
                claims.IsAdmin = isAdmin
            }
        }
    }

    return claims, nil
}
```

### Context Propagation Pattern

**Pattern:** Store admin status in Gin context alongside user ID.

```go
// In middleware - add new context key
const IsAdminKey contextKey = "isAdmin"

// In handler - retrieve admin status
func (h *UserHandler) UpdateUser(c *gin.Context) {
    isAdmin := c.GetBool(string(middleware.IsAdminKey))
    // No more hardcoded false!
}
```

### Setting Admin Status (Server-Side Only)

**Pattern:** Use Supabase Admin API to set app_metadata.

```go
// Using supabase-go admin API
adminClient := supabase.CreateClient(url, serviceRoleKey)
_, err := adminClient.Auth.Admin.UpdateUserById(userID, admin.UpdateUserParams{
    AppMetadata: map[string]interface{}{
        "is_admin": true,
    },
})
```

### Project Structure Impact

```
middleware/
├── jwt_validator.go    # Extend extractClaims() for app_metadata
├── auth.go             # Add IsAdminKey context propagation
types/
├── user.go             # Add IsAdmin to UserClaims struct
handlers/
├── user_handler.go     # Replace hardcoded false with context lookup
```

### Anti-Patterns to Avoid
- **Checking user_metadata for admin status:** User-editable, not secure
- **Database lookup on every request:** Adds latency, defeats JWT purpose
- **Hardcoding admin user IDs:** Current anti-pattern being fixed
- **Using trip-level MemberRoleAdmin for system admin:** These are different concepts
</architecture_patterns>

<dont_hand_roll>
## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JWT parsing | Custom parser | lestrrat-go/jwx (already in use) | Handles edge cases, key rotation, algorithm verification |
| Admin flag storage | Custom users table column | Supabase app_metadata | Already integrated with auth flow, server-only, included in JWT |
| Role checking | Complex permission system | Simple boolean check | Only need "is/isn't system admin" - don't over-engineer |
| Session refresh | Custom token refresh | Supabase client SDK | Handles token refresh, claim propagation automatically |

**Key insight:** The existing Supabase + Go JWT stack already supports everything needed. The implementation is about correctly using what's there, not adding new systems.
</dont_hand_roll>

<common_pitfalls>
## Common Pitfalls

### Pitfall 1: Using user_metadata Instead of app_metadata
**What goes wrong:** Admin users can be created by any authenticated user
**Why it happens:** `user_metadata` is user-editable via `updateUser()` API
**How to avoid:** Always use `app_metadata` for security-sensitive claims
**Warning signs:** Any frontend code that sets admin status

### Pitfall 2: Confusing Trip Admin vs System Admin
**What goes wrong:** Trip admins get system-wide privileges
**Why it happens:** Both use "admin" terminology
**How to avoid:** Keep clear naming: `IsAdmin` (system) vs `MemberRoleAdmin` (trip)
**Warning signs:** Using `types.MemberRoleAdmin` for system permission checks

### Pitfall 3: Not Handling Missing Claims
**What goes wrong:** Nil pointer panic or incorrect authorization
**Why it happens:** Not all users have app_metadata.is_admin set
**How to avoid:** Default to `false` when claim is missing
**Warning signs:** Errors on user login for users without explicit admin claim

### Pitfall 4: Forgetting Claim Refresh Limitation
**What goes wrong:** User granted admin doesn't see access until re-login
**Why it happens:** JWT claims are set at login time, not real-time
**How to avoid:** Document behavior, or force token refresh after admin grant
**Warning signs:** "It works after logout/login" bug reports

### Pitfall 5: Exposing Service Role Key
**What goes wrong:** Anyone can grant admin status
**Why it happens:** Service role key accidentally exposed to client
**How to avoid:** Service role key only in server-side env vars
**Warning signs:** SUPABASE_SERVICE_ROLE_KEY in frontend config
</common_pitfalls>

<code_examples>
## Code Examples

### Extending UserClaims Struct
```go
// Source: Existing types/user.go pattern
type UserClaims struct {
    UserID   string `json:"sub"`
    Email    string `json:"email"`
    Username string `json:"username"`
    IsAdmin  bool   `json:"is_admin"` // NEW: System-level admin flag
    jwt.RegisteredClaims
}
```

### Extracting app_metadata in JWT Validator
```go
// Source: Pattern from middleware/jwt_validator.go:405
// After existing user_metadata extraction, add:
if appMetaVal, ok := token.Get("app_metadata"); ok {
    if appMeta, ok := appMetaVal.(map[string]interface{}); ok {
        if isAdmin, ok := appMeta["is_admin"].(bool); ok {
            claims.IsAdmin = isAdmin
        }
    }
}
```

### Context Key Definition
```go
// Source: Pattern from middleware/auth.go
type contextKey string

const (
    UserIDKey    contextKey = "userID"
    IsAdminKey   contextKey = "isAdmin" // NEW
)
```

### Handler Usage (Replacing Hardcoded False)
```go
// BEFORE (current anti-pattern):
isAdmin := false // TODO: Implement admin check

// AFTER:
isAdmin := c.GetBool(string(middleware.IsAdminKey))
```

### Setting Admin via Supabase Admin API
```go
// Using existing supabase-go client with service role
// This should only be called from a secure admin endpoint or CLI tool
func SetUserAdminStatus(supabaseClient *supabase.Client, userID string, isAdmin bool) error {
    _, err := supabaseClient.Auth.Admin.UpdateUserById(userID, admin.UpdateUserParams{
        AppMetadata: map[string]interface{}{
            "is_admin": isAdmin,
        },
    })
    return err
}
```
</code_examples>

<sota_updates>
## State of the Art (2025-2026)

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Database role lookup per request | JWT claims from app_metadata | 2023+ | Better performance, no DB call |
| Custom session storage | Supabase built-in sessions | Standard | Simpler architecture |
| Manual JWKS fetching | lestrrat-go/jwx auto-refresh | Standard | Already implemented |

**New tools/patterns to consider:**
- **Supabase Custom Access Token Hooks:** Can automatically inject database roles into JWT at login time (requires Pro tier or self-hosted)
- **supabase-custom-claims repo:** Community SQL functions for claim management

**What this project already has:**
- JWKS caching and auto-refresh (middleware/jwks_cache.go)
- User metadata extraction (middleware/jwt_validator.go:405)
- Trip-level RBAC (types/permission_matrix.go)

**What's missing:**
- app_metadata extraction for system admin
- Context propagation of admin status
- Admin management endpoint/tooling
</sota_updates>

<implementation_options>
## Implementation Options

### Option A: app_metadata + JWT Claim (RECOMMENDED)
**Effort:** Low (2-3 hours)
**Approach:**
1. Extend `extractClaims()` in jwt_validator.go to read `app_metadata.is_admin`
2. Add `IsAdmin` to UserClaims struct
3. Add `IsAdminKey` context propagation
4. Replace hardcoded `false` in handlers with context lookup
5. Bootstrap first admin via Supabase dashboard

**Pros:**
- Uses existing infrastructure
- No new dependencies
- No extra DB calls
- Secure (app_metadata is server-only)

**Cons:**
- Admin status cached in JWT until token refresh
- Requires Supabase dashboard/API for admin management

### Option B: Database Column Lookup
**Effort:** Low-Medium (3-4 hours)
**Approach:**
1. Add `is_admin` column to users table
2. Query user record in middleware or handler
3. Cache result in request context

**Pros:**
- Real-time admin status
- Simple to understand
- Easy to manage via SQL

**Cons:**
- Extra DB query per request
- Duplicates data (Supabase auth + local DB)
- Must sync with Supabase user creation

### Option C: Custom Access Token Hook
**Effort:** Medium (4-6 hours)
**Approach:**
1. Create `user_roles` table in Supabase
2. Create PL/pgSQL function for custom_access_token_hook
3. Enable hook in Supabase dashboard
4. Extract role from JWT claim

**Pros:**
- Most scalable
- Database-driven role management
- Standard Supabase pattern

**Cons:**
- Requires Supabase Pro tier or self-hosted
- More complex setup
- Still has JWT refresh limitation

### Recommendation

**Use Option A** for this phase. It's the simplest path that solves the immediate security issue. The existing codebase already extracts `user_metadata`; extending to `app_metadata` is minimal work.

If complex role hierarchies are needed later, Option C can be implemented as an enhancement.
</implementation_options>

<open_questions>
## Open Questions

1. **Who can grant admin status?**
   - What we know: Requires Supabase service role key or dashboard access
   - What's unclear: Should there be an admin endpoint? CLI tool?
   - Recommendation: Bootstrap via dashboard, document process; admin endpoint is Phase 4+ scope

2. **How many admins expected?**
   - What we know: Currently zero (hardcoded false)
   - What's unclear: Production admin count, rotation policy
   - Recommendation: Simple boolean is sufficient unless >10 admin roles needed

3. **Trip membership check at line 620**
   - What we know: SearchUsers endpoint optionally takes tripId but doesn't verify membership
   - What's unclear: Is this a security issue or just missing feature?
   - Recommendation: Phase 4 should decide: either add check or remove parameter
</open_questions>

<sources>
## Sources

### Primary (HIGH confidence)
- [Supabase JWT Claims Reference](https://supabase.com/docs/guides/auth/jwt-fields) - Official field documentation
- [Supabase Custom Claims & RBAC](https://supabase.com/docs/guides/database/postgres/custom-claims-and-role-based-access-control-rbac) - Official RBAC guide
- /supabase/auth Context7 docs - Auth API, app_metadata patterns

### Secondary (MEDIUM confidence)
- [supabase-community/supabase-custom-claims](https://github.com/supabase-community/supabase-custom-claims) - Community SQL functions, verified against official docs
- [WorkOS JWT Guide for Go](https://workos.com/blog/how-to-handle-jwt-in-go) - Go JWT patterns
- [golang-jwt/jwt/v5 Package](https://pkg.go.dev/github.com/golang-jwt/jwt/v5) - Library docs

### Tertiary (Already verified by codebase review)
- NomadCrew codebase: `middleware/jwt_validator.go` - Already extracts user_metadata
- NomadCrew codebase: `types/membership.go` - Existing trip-level MemberRoleAdmin
- NomadCrew codebase: `types/permission_matrix.go` - Existing RBAC for resources
</sources>

<metadata>
## Metadata

**Research scope:**
- Core technology: Supabase Auth JWT claims, Go middleware patterns
- Ecosystem: app_metadata, custom claims, RBAC
- Patterns: Claim-based authorization, context propagation
- Pitfalls: user_metadata vs app_metadata, claim refresh

**Confidence breakdown:**
- Standard stack: HIGH - No new dependencies, uses existing libraries
- Architecture: HIGH - Extends existing patterns in codebase
- Implementation options: HIGH - All verified against Supabase docs
- Code examples: HIGH - Based on existing codebase patterns

**Research date:** 2026-01-10
**Valid until:** 2026-02-10 (30 days - Supabase auth is stable)
</metadata>

---

*Phase: 04-user-domain-refactoring*
*Research completed: 2026-01-10*
*Ready for planning: yes*
