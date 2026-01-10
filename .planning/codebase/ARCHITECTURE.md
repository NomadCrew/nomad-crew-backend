# Architecture

**Analysis Date:** 2026-01-10

## Pattern Overview

**Overall:** Layered Monolithic REST API with Event-Driven Real-time Features

**Key Characteristics:**
- Clean layered architecture (Handler → Service/Model → Store)
- SQLC-generated type-safe database access
- Event-driven real-time updates via Redis pub/sub and WebSockets
- Supabase integration for auth and realtime sync
- RBAC-based authorization with permission matrix

## Layers

**Handler Layer:**
- Purpose: HTTP request/response handling, input validation, authentication
- Contains: Route handlers, request parsing, response formatting
- Location: `handlers/*.go`, `internal/handlers/*.go`
- Depends on: Service/Model layer, middleware
- Used by: Router

**Model/Service Layer:**
- Purpose: Business logic, domain operations, event publishing
- Contains: Trip management, user operations, notifications, location tracking
- Location: `models/*/service/*.go`, `services/*.go`
- Depends on: Store layer, event service
- Used by: Handler layer

**Store Layer:**
- Purpose: Data persistence, database operations
- Contains: SQLC-generated queries, store adapters
- Location: `internal/store/sqlcadapter/*.go`, `internal/sqlc/*.go`
- Depends on: Database (pgx pool)
- Used by: Model/Service layer

**Event Layer:**
- Purpose: Real-time event publishing and subscription
- Contains: Redis pub/sub, event routing, WebSocket hub
- Location: `internal/events/*.go`, `internal/websocket/*.go`
- Depends on: Redis client
- Used by: Model/Service layer, WebSocket handlers

**Middleware Layer:**
- Purpose: Cross-cutting concerns (auth, RBAC, rate limiting, CORS)
- Contains: JWT validation, permission checks, request ID, error handling
- Location: `middleware/*.go`
- Depends on: Config, JWT validator
- Used by: Router

## Data Flow

**HTTP Request Lifecycle:**

1. Request arrives at Gin router (`router/router.go`)
2. Global middleware executes (RequestID, SecurityHeaders, CORS)
3. Route-specific middleware (JWTAuth, RBAC permission check)
4. Handler validates input, calls service/model methods
5. Service/Model executes business logic, calls store
6. Store executes SQLC-generated queries against PostgreSQL
7. Events published to Redis for real-time subscribers
8. Response formatted and returned

**WebSocket Event Flow:**

1. Client connects to `/v1/ws` with JWT token
2. `WSJwtAuth` middleware validates token
3. WebSocket handler upgrades connection, registers with hub
4. Hub subscribes to Redis pub/sub for trip events
5. Events from services publish to Redis
6. Hub receives events, broadcasts to relevant connected clients
7. Client disconnection triggers cleanup

**State Management:**
- Stateless HTTP handlers - no session state
- PostgreSQL for persistent data (trips, users, todos, etc.)
- Redis for ephemeral state (rate limiting, event pub/sub)
- Supabase for realtime sync and auth state

## Key Abstractions

**Store Interface:**
- Purpose: Abstract database operations for testability
- Examples: `TripStore`, `UserStore`, `TodoStore`, `LocationStore`
- Location: `internal/store/interfaces.go`
- Pattern: Interface + SQLC adapter implementation

**Model:**
- Purpose: Domain logic coordination, event publishing
- Examples: `TripModel`, `TodoModel`
- Location: `models/trip/model.go`, `models/model.go`
- Pattern: Aggregate root with injected dependencies

**Service:**
- Purpose: Specific domain operations
- Examples: `LocationManagementService`, `TripMemberService`, `NotificationService`
- Location: `models/*/service/*.go`, `services/*.go`
- Pattern: Stateless service with store and event dependencies

**Handler:**
- Purpose: HTTP endpoint implementation
- Examples: `TripHandler`, `UserHandler`, `TodoHandler`
- Location: `handlers/*.go`
- Pattern: Struct with model/service dependencies, methods per endpoint

## Entry Points

**Main Entry:**
- Location: `main.go`
- Triggers: Application startup (`go run main.go`)
- Responsibilities: Initialize config, database, services, handlers, router; start HTTP server; graceful shutdown

**Router Setup:**
- Location: `router/router.go`
- Triggers: Called by main during startup
- Responsibilities: Configure middleware, define routes, wire handlers

**WebSocket Hub:**
- Location: `internal/websocket/hub.go`
- Triggers: Started by main, handles client connections
- Responsibilities: Manage WebSocket connections, broadcast events

## Error Handling

**Strategy:** Typed errors with error handler middleware

**Patterns:**
- Custom error types in `errors/errors.go` and `internal/errors/errors.go`
- Services return errors, handlers translate to HTTP responses
- Global error handler middleware catches panics
- Structured logging with Zap for error context

## Cross-Cutting Concerns

**Logging:**
- Zap logger initialized in `logger/logger.go`
- Structured logging with request context
- Log levels configurable via environment

**Validation:**
- Gin binding for request validation
- Custom validation in handlers before service calls
- RBAC permission checks via middleware

**Authentication:**
- Supabase JWT tokens validated via JWKS
- `AuthMiddleware` extracts user from token
- User context available via Gin context keys

**Authorization:**
- Permission matrix in `types/permission_matrix.go`
- RBAC middleware checks user role against required permission
- Ownership checks for user-specific resources

---

*Architecture analysis: 2026-01-10*
*Update when major patterns change*
