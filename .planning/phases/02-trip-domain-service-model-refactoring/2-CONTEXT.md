# Phase 2: Trip Domain Service/Model Refactoring - Context

**Gathered:** 2026-01-10
**Status:** Ready for planning

## Current State Analysis

The trip service layer is already well-structured with good separation:
- **TripModel** - Thin wrapper delegating to coordinator (clean)
- **TripModelCoordinator** - Facade over services (clean)
- **TripManagementService** - Core trip CRUD operations
- **TripMemberService** - Member operations with event publishing
- **InvitationService** - Invitation handling

## Issues Identified

1. **Debug logging in trip_service.go** (lines 46-52) - Debug log should be removed
2. **TODO in membership.go** (line 39) - Status transition validation is commented out
3. **Deprecated chat methods** in coordinator - Should be removed or documented
4. **Inconsistent error handling** - Some methods wrap errors, some don't

## Out of Scope

- Store layer changes (Phase 3)
- Handler changes (Phase 1 complete)
- New functionality

## Approach

Focus on cleanup rather than major refactoring - the architecture is sound.
