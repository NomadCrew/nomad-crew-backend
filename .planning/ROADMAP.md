# Roadmap: NomadCrew Backend Refactoring

## Current Milestone: v1.0 — Codebase Refactoring

### Overview

Domain-by-domain refactoring of the NomadCrew backend API to reduce complexity, remove duplication, and improve architecture while maintaining existing functionality.

**Approach:** Refactor each domain completely before moving to the next
**Constraints:** No API changes, no database changes, tests must pass

---

## Phase 1: Trip Domain Handler Refactoring
**Status:** Not Started
**Research Required:** No

**Goal:** Reduce complexity in trip_handler.go, extract helper functions, improve error handling

**Scope:**
- `handlers/trip_handler.go` - Main trip HTTP handlers
- `handlers/trip_handler_test.go` - Trip handler tests

**Key Tasks:**
- Extract repeated validation logic into helper functions
- Standardize error response formatting
- Break large handler methods into smaller focused functions
- Ensure consistent context key usage
- Update tests to match refactored code

**Success Criteria:**
- All existing tests pass
- No individual handler method exceeds 50 lines
- Consistent error handling pattern across all trip endpoints

**Dependencies:** None

---

## Phase 2: Trip Domain Service/Model Refactoring
**Status:** Not Started
**Research Required:** No

**Goal:** Simplify trip service layer, remove duplication, improve separation of concerns

**Scope:**
- `models/trip/model.go` - Trip aggregate root
- `models/trip/service/trip_model_coordinator.go` - Trip business logic
- `models/trip/service/trip_member_service.go` - Membership operations
- `models/trip/command/*.go` - CQRS commands
- `models/trip/validation/*.go` - Validation logic

**Key Tasks:**
- Review model vs coordinator responsibilities
- Remove any duplicate business logic
- Ensure clear separation between commands and queries
- Standardize validation error messages
- Fix membership status validation TODO (line 39)

**Success Criteria:**
- All existing tests pass
- Clear single responsibility for each service method
- No duplicate business logic between model and coordinator

**Dependencies:** Phase 1 (handler refactoring may reveal service issues)

---

## Phase 3: Trip Domain Store Refactoring
**Status:** Not Started
**Research Required:** No

**Goal:** Clean up trip store adapter, ensure consistent query patterns

**Scope:**
- `internal/store/sqlcadapter/trip_store.go` - Trip store implementation
- `internal/store/interfaces.go` - Store interface definitions (trip section)
- `store/postgres/trip_store_pg_mock_test.go` - Store tests

**Key Tasks:**
- Review store interface for unnecessary methods
- Ensure consistent error wrapping
- Verify all SQLC queries are properly used
- Remove any manual query building (use SQLC)

**Success Criteria:**
- All existing tests pass
- Consistent error handling in all store methods
- Interface matches actual usage needs

**Dependencies:** Phase 2

---

## Phase 4: User Domain Refactoring
**Status:** Not Started
**Research Required:** Yes (admin role implementation options)

**Goal:** Fix admin check implementation, add missing permission checks, reduce handler complexity

**Scope:**
- `handlers/user_handler.go` - User HTTP handlers (260, 343, 620 line issues)
- `models/user/service/user_service.go` - User business logic
- `internal/store/sqlcadapter/user_store.go` - User store implementation
- `services/supabase_service.go` - Supabase integration

**Key Tasks:**
- **CRITICAL:** Implement proper admin role check (currently hardcoded false at lines 260, 343)
- **CRITICAL:** Add trip membership check at line 620 (or document why not needed)
- Extract repeated validation patterns
- Standardize error responses
- Review Supabase service integration

**Success Criteria:**
- Admin role check properly implemented
- Trip membership verification in place
- All existing tests pass
- Security issues resolved

**Dependencies:** Phase 3 (may need TripStore for membership check)

---

## Phase 5: Location Domain Refactoring
**Status:** Not Started
**Research Required:** No

**Goal:** Simplify location tracking handlers and service, ensure proper authorization

**Scope:**
- `handlers/location_handler.go` - Location HTTP handlers
- `internal/handlers/location_ws_handler.go` - WebSocket location handler
- `models/location/service/location_management_service.go` - Location service
- `internal/store/sqlcadapter/location_store.go` - Location store

**Key Tasks:**
- Review location update authorization (trip membership)
- Standardize HTTP and WebSocket handler patterns
- Remove any duplicate logic between handlers
- Ensure consistent error handling

**Success Criteria:**
- All existing tests pass
- Consistent authorization checks
- Clear separation between HTTP and WebSocket concerns

**Dependencies:** Phase 4 (user domain provides auth patterns)

---

## Phase 6: Notification Domain Refactoring
**Status:** Not Started
**Research Required:** No

**Goal:** Clean up notification handlers and service, improve push notification patterns

**Scope:**
- `handlers/notification_handler.go` - Notification HTTP handlers
- `models/notification/notification_service.go` - Notification business logic
- `services/push_service.go` - Push notification service
- `internal/notification/client.go` - Expo push client (batch TODO at line 106)

**Key Tasks:**
- Review notification permission checks
- Standardize push notification error handling
- Consider batch notification implementation (or document as out of scope)
- Remove duplicate notification logic

**Success Criteria:**
- All existing tests pass
- Consistent notification delivery patterns
- Clear error handling for failed deliveries

**Dependencies:** Phase 5

---

## Phase 7: Todo Domain Refactoring
**Status:** Not Started
**Research Required:** No

**Goal:** Simplify todo handler and model, ensure proper trip authorization

**Scope:**
- `handlers/todo_handler.go` - Todo HTTP handlers
- `models/model.go` - Todo model (legacy location)
- `internal/store/sqlcadapter/todo_store.go` - Todo store

**Key Tasks:**
- Review todo authorization (trip membership)
- Consider moving todo model to proper domain directory
- Standardize CRUD patterns
- Ensure consistent error responses

**Success Criteria:**
- All existing tests pass
- Todo model in appropriate location
- Consistent authorization pattern

**Dependencies:** Phase 6

---

## Phase 8: Chat Domain Refactoring
**Status:** Not Started
**Research Required:** No

**Goal:** Clean up Supabase chat integration, ensure proper patterns

**Scope:**
- `handlers/chat_handler_supabase.go` - Chat HTTP handlers
- `handlers/chat_handler.go` - Legacy chat handler (if exists)
- `models/chat/service/chat_service.go` - Chat service

**Key Tasks:**
- Review Supabase realtime integration patterns
- Remove any legacy chat code if superseded
- Ensure consistent error handling
- Verify authorization for chat operations

**Success Criteria:**
- All existing tests pass
- Clean Supabase integration
- No duplicate chat implementations

**Dependencies:** Phase 7

---

## Phase 9: Weather Service Refactoring
**Status:** Not Started
**Research Required:** No

**Goal:** Add missing permission checks, clean up weather service

**Scope:**
- `models/weather/service/weather_service.go` - Weather service (TODO at lines 61, 85)

**Key Tasks:**
- **CRITICAL:** Add trip membership verification before weather operations
- Review weather data caching patterns
- Standardize error handling

**Success Criteria:**
- Permission checks implemented
- All existing tests pass
- Security issues resolved

**Dependencies:** Phase 8

---

## Phase 10: Middleware and Cross-Cutting Concerns
**Status:** Not Started
**Research Required:** No

**Goal:** Standardize middleware patterns, improve RBAC consistency

**Scope:**
- `middleware/auth.go` - Authentication middleware
- `middleware/rbac.go` - Authorization middleware
- `middleware/rate_limit.go` - Rate limiting
- `middleware/jwt_validator.go` - JWT validation
- `types/permission_matrix.go` - Permission definitions

**Key Tasks:**
- Review permission matrix completeness
- Ensure consistent middleware error responses
- Standardize context key usage across all middleware
- Review rate limiting configuration

**Success Criteria:**
- All existing tests pass
- Consistent middleware patterns
- Complete permission matrix coverage

**Dependencies:** Phases 1-9 (may discover middleware issues during domain refactoring)

---

## Phase 11: Event System and WebSocket Refactoring
**Status:** Not Started
**Research Required:** No

**Goal:** Clean up event publishing patterns, improve WebSocket hub

**Scope:**
- `internal/events/service.go` - Event service
- `internal/events/redis_publisher.go` - Redis pub/sub
- `internal/websocket/hub.go` - WebSocket connection manager
- `internal/websocket/handler.go` - WebSocket endpoint

**Key Tasks:**
- Review event type consistency
- Standardize event publishing patterns across all domains
- Ensure proper WebSocket cleanup on disconnect
- Review Redis pub/sub error handling

**Success Criteria:**
- All existing tests pass
- Consistent event publishing across domains
- Proper connection lifecycle management

**Dependencies:** Phase 10

---

## Phase 12: Final Cleanup and Documentation
**Status:** Not Started
**Research Required:** No

**Goal:** Remove remaining TODO/FIXME comments, ensure consistency across codebase

**Scope:**
- All files with remaining TODO/FIXME comments
- `logger/logger.go` - CloudWatch TODO at line 47
- Integration tests in `tests/integration/`

**Key Tasks:**
- Address or document remaining TODO comments
- Ensure consistent error types usage
- Review integration test coverage
- Verify all refactoring goals met
- Final test run and validation

**Success Criteria:**
- All tests pass
- No critical TODO/FIXME remaining
- Codebase meets refactoring goals from PROJECT.md

**Dependencies:** Phases 1-11

---

## Summary

| Phase | Domain | Critical Issues | Research |
|-------|--------|-----------------|----------|
| 1 | Trip Handlers | None | No |
| 2 | Trip Service/Model | Membership validation | No |
| 3 | Trip Store | None | No |
| 4 | User Domain | Admin check, membership check | Yes |
| 5 | Location Domain | None | No |
| 6 | Notification Domain | Batch notifications | No |
| 7 | Todo Domain | None | No |
| 8 | Chat Domain | None | No |
| 9 | Weather Service | Permission checks | No |
| 10 | Middleware | None | No |
| 11 | Events/WebSocket | None | No |
| 12 | Final Cleanup | TODO comments | No |

**Total Phases:** 12
**Phases with Critical Security Issues:** 2 (Phase 4, Phase 9)
**Phases Requiring Research:** 1 (Phase 4)

---

*Created: 2026-01-10*
*Approach: Domain-by-domain*
*Depth: Comprehensive*
