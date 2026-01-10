# Coding Conventions

**Analysis Date:** 2026-01-10

## Naming Patterns

**Files:**
- snake_case for all Go files (`trip_handler.go`, `user_service.go`)
- `*_test.go` suffix for test files (co-located with source)
- `*_handler.go` for HTTP handlers
- `*_service.go` for service implementations
- `*_store.go` for store implementations
- `interfaces.go` for interface definitions in a package

**Functions:**
- PascalCase for exported functions (`CreateTrip`, `GetUserByID`)
- camelCase for unexported functions (`validateInput`, `buildQuery`)
- `Handle*` prefix for HTTP handler methods (`HandleCreateTrip`)
- `New*` prefix for constructors (`NewTripHandler`, `NewUserService`)

**Variables:**
- camelCase for variables (`tripID`, `userEmail`)
- Short names for loop variables and receivers (`t` for trip, `u` for user)
- Descriptive names for important variables (`currentUser`, `tripMembers`)

**Types:**
- PascalCase for types (`Trip`, `User`, `CreateTripRequest`)
- `*Request`/`*Response` suffix for API DTOs
- `*Handler`, `*Service`, `*Store` suffix for component types
- Interface names describe capability (`TripModelInterface`, `Validator`)

**Constants:**
- PascalCase for exported constants (`StatusActive`, `RoleAdmin`)
- Grouped in const blocks by domain

## Code Style

**Formatting:**
- gofmt standard formatting
- Tabs for indentation
- No line length limit (but readable lines preferred)

**Imports:**
- Standard library first
- External packages second
- Internal packages last
- Blank line between groups
- Aliased imports for clarity (`trip_service "github.com/NomadCrew/.../trip/service"`)

**Linting:**
- Standard Go linting
- No explicit linter config file found
- Run: `go vet ./...`

## Import Organization

**Order:**
1. Standard library (`context`, `fmt`, `net/http`)
2. External packages (`github.com/gin-gonic/gin`)
3. Internal packages (`github.com/NomadCrew/nomad-crew-backend/...`)

**Grouping:**
- Blank line between import groups
- Related imports grouped together

**Path Aliases:**
- Full import paths used
- Aliases for disambiguation (`trip_service`, `userSvc`, `locationSvc`)

## Error Handling

**Patterns:**
- Return `error` as last return value
- Wrap errors with context: `fmt.Errorf("failed to create trip: %w", err)`
- Custom error types in `errors/errors.go`
- Check errors immediately after function calls

**Error Types:**
- `errors.ErrNotFound` for missing resources
- `errors.ErrForbidden` for authorization failures
- `errors.ErrValidation` for input validation failures
- Wrap database errors with domain context

**Logging:**
- Log errors with context before returning
- Use structured logging: `log.Errorw("failed to create trip", "error", err, "userID", userID)`

## Logging

**Framework:**
- Zap logger (`go.uber.org/zap`)
- Sugared logger for convenience (`*zap.SugaredLogger`)

**Patterns:**
- Structured logging with key-value pairs
- `log.Infow("message", "key", value)`
- `log.Errorw("message", "error", err, "context", ctx)`

**Levels:**
- `Debug` - Development details
- `Info` - Normal operations
- `Warn` - Recoverable issues
- `Error` - Failures requiring attention
- `Fatal` - Unrecoverable errors (app exits)

**When to Log:**
- Service method entry/exit for important operations
- External service calls
- Error conditions with context
- State transitions

## Comments

**When to Comment:**
- Package documentation at top of main file
- Exported functions (godoc style)
- Complex business logic explanations
- TODO markers for future work

**Godoc Style:**
```go
// NewTripHandler creates a new TripHandler with the given dependencies.
// It initializes the handler with the trip model, event service, and other required services.
func NewTripHandler(...) *TripHandler {
```

**TODO Comments:**
- Format: `// TODO: description`
- Used for deferred work (see `handlers/user_handler.go:260`)

## Function Design

**Size:**
- Keep functions focused on single responsibility
- Extract helper functions for complex logic
- Handler methods typically 20-50 lines

**Parameters:**
- Context first: `ctx context.Context`
- Max 4-5 parameters, use struct for more
- Pointer receivers for methods that modify state

**Return Values:**
- Return early for error cases
- Named returns rarely used
- Multiple returns: (result, error) pattern

## Module Design

**Exports:**
- Export only what's needed by other packages
- Keep implementation details private
- Use interfaces for dependencies

**Package Structure:**
- One package per directory
- `interfaces.go` for package interfaces
- Related functionality grouped together

**Dependency Injection:**
- Constructor functions receive dependencies
- Store structs with interface dependencies
- No global state

## Context Keys

**Pattern:**
- Custom type for context keys (`middleware/context_keys.go`)
- Type safety for context values

```go
type contextKey string
const UserIDKey contextKey = "userID"
```

## RBAC Pattern

**Permission Checks:**
- Middleware-based permission checking
- Permission matrix in `types/permission_matrix.go`
- Action + Resource + Role = Permission

```go
middleware.RequirePermission(deps.TripModel, types.ActionRead, types.ResourceTrip, nil)
```

---

*Convention analysis: 2026-01-10*
*Update when patterns change*
