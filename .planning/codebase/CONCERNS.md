# Codebase Concerns

**Analysis Date:** 2026-01-10

## Tech Debt

**Admin Check Not Implemented:**
- Issue: Admin role check is hardcoded to `false`
- Files: `handlers/user_handler.go:260`, `handlers/user_handler.go:343`
- Why: Placeholder during development
- Impact: Admin-only operations accessible to all authenticated users OR blocked entirely
- Fix approach: Implement proper admin role check using user metadata or separate admin flag

**Trip Membership Check Missing in User Handler:**
- Issue: TODO to add trip membership check when TripStore available
- File: `handlers/user_handler.go:620`
- Why: Handler doesn't have TripStore dependency
- Impact: Potential unauthorized access to user data
- Fix approach: Add TripStore dependency to UserHandler or create separate endpoint

**CloudWatch Paths Not Configured:**
- Issue: Logger has TODO for CloudWatch configuration
- File: `logger/logger.go:47`
- Why: Not needed during initial development
- Impact: No centralized log aggregation in production
- Fix approach: Configure CloudWatch paths or add alternative log aggregation

## Known Bugs

**None detected from TODO/FIXME scan**

## Security Considerations

**Hardcoded Admin Check:**
- Risk: All admin operations either fail or succeed for everyone
- Files: `handlers/user_handler.go:260`, `handlers/user_handler.go:343`
- Current mitigation: None (returns `false` always)
- Recommendations:
  - Add `is_admin` field to user model
  - Or use Supabase user metadata for role
  - Implement proper RBAC for admin operations

**Permission Checks in Weather Service:**
- Risk: Weather operations may not verify user has access to trip
- Files: `models/weather/service/weather_service.go:61`, `models/weather/service/weather_service.go:85`
- Current mitigation: TODO comments indicate awareness
- Recommendations: Add trip membership verification before processing weather requests

**Error Messages May Leak Information:**
- Risk: Detailed error messages could expose internal structure
- Current mitigation: Custom error types in `errors/errors.go`
- Recommendations: Ensure production errors are sanitized

## Performance Bottlenecks

**None detected from static analysis**

Potential areas to monitor:
- Database query performance (N+1 queries)
- Redis pub/sub throughput under load
- WebSocket connection scaling

## Fragile Areas

**Integration Tests Incomplete:**
- File: `tests/integration/trip_test.go:12`
- Why fragile: TODO indicates tests need implementation
- Impact: Trip functionality may have untested edge cases
- Recommendations: Implement comprehensive integration tests for trip operations

**Invitation Integration Tests:**
- File: `tests/integration/invitation_integration_test.go:220`, `tests/integration/invitation_integration_test.go:434`
- Why fragile: TODOs for migration dependency and additional test cases
- Impact: Invitation flow may have untested scenarios
- Recommendations: Complete test coverage for invitation lifecycle

## Scaling Limits

**WebSocket Hub:**
- Current capacity: Unknown (no load testing documented)
- Limit: Single server WebSocket connections
- Symptoms at limit: Connection refused, message delays
- Scaling path: Consider Redis-backed WebSocket scaling or Supabase Realtime

**Database Connections:**
- Current capacity: Neon serverless limits
- Configuration: `config/database.go` with pool settings
- Scaling path: Adjust pool size based on load

## Dependencies at Risk

**supabase-community/supabase-go:**
- Version: v0.0.4
- Risk: Community-maintained, not official Supabase SDK
- Impact: May lag behind Supabase API changes
- Migration plan: Monitor for official Go SDK, or ensure API compatibility

## Missing Critical Features

**Batch Push Notifications:**
- File: `internal/notification/client.go:106`
- Problem: Batch endpoint not implemented
- Current workaround: Individual notifications
- Impact: Inefficient for bulk notifications
- Fix complexity: Medium (implement Expo batch API)

**Membership Status Validation:**
- File: `models/trip/validation/membership.go:39`
- Problem: TODO for additional status transition validation
- Impact: Invalid membership state transitions may be allowed
- Fix complexity: Low (add validation rules)

## Test Coverage Gaps

**Trip Domain:**
- What's not tested: Supabase Realtime integration tests
- File: `tests/integration/trip_test.go:12`
- Risk: Real-time sync issues undetected
- Priority: High
- Difficulty: Requires Supabase test environment setup

**User Handler Admin Operations:**
- What's not tested: Admin role verification
- Risk: Authorization bypass undetected
- Priority: High (security)
- Difficulty: Low once admin check is implemented

**Weather Service Permissions:**
- What's not tested: Trip access verification for weather operations
- Risk: Unauthorized weather data access
- Priority: Medium
- Difficulty: Low

## Documentation Gaps

**API Documentation:**
- Swagger is generated (`docs/docs.go`)
- Available at `/swagger/*`
- Status: Present but completeness unknown

**Deployment Guide:**
- File: `docs/DEPLOYMENT_GUIDE.md` (untracked)
- Status: May need updates for current infrastructure

---

*Concerns audit: 2026-01-10*
*Update as issues are fixed or new ones discovered*
