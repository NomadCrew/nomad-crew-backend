# Phase 27: Test Suite Repair - Research

**Researched:** 2026-02-04
**Domain:** Go testing, pgx v5 mocking, test compilation failures
**Confidence:** HIGH

## Summary

The test suite currently has **10 packages failing to compile** out of 52 total packages, with multiple categories of failures:

1. **Missing dependencies** (5 packages): pgx/v4 imports, go-sqlmock, redismock/v9 not in go.mod
2. **Interface mismatches** (4 packages): Mocks missing new interface methods
3. **Duplicate mock declarations** (1 package): handlers package declares MockUserService twice
4. **Type errors** (2 packages): Missing types, incorrect API usage
5. **Test logic errors** (1 package): internal/notification pagination assertion failure

**Primary recommendation:** Fix in dependency order - install missing packages first, update pgx/v4 to pgxmock, resolve interface mismatches, consolidate duplicate mocks, then fix type/logic errors.

## Current State Analysis

### Compilation Errors by Package

| Package | Error Type | Root Cause | Blocking |
|---------|-----------|------------|----------|
| `config` | Missing import | `github.com/jackc/pgx/v4/pgxpool` not in go.mod | YES |
| `db` | Missing import | `github.com/DATA-DOG/go-sqlmock` not in go.mod | YES |
| `internal/store/postgres` | Missing import | `github.com/DATA-DOG/go-sqlmock` not in go.mod | YES |
| `services` | Missing import | `github.com/go-redis/redismock/v9` not in go.mod | YES |
| `store/postgres` | Missing import | `github.com/DATA-DOG/go-sqlmock` not in go.mod | YES |
| `middleware` | Interface mismatch | `MockJWTValidator` missing `ValidateAndGetClaims()` | YES |
| `handlers` | Duplicate + type errors | `MockUserService` declared twice, missing types | YES |
| `internal/auth` | API change | `jwt.Parser.Parts()` method doesn't exist | YES |
| `models/user/service` | Interface mismatch | `MockUserStore` missing `GetUserByContactEmail()` | YES |
| `models/trip/service` | Multiple issues | Missing `mocks.UserStore`, wrong constructor args | YES |

### Non-Blocking Issues

| Package | Issue | Impact |
|---------|-------|--------|
| `internal/notification` | Test assertion failure | Pagination limit expects 10, gets 20 |
| `tests/integration` | Missing field | `router.Dependencies` has no `LocationHandler` field |

### Dependency Resolution Status

**Currently in go.mod:**
- `github.com/jackc/pgx/v5 v5.7.6` ✅
- `github.com/pashagolub/pgxmock` ❌ (needed for pgx/v5 mocking)
- `github.com/go-redis/redismock/v9` ❌ (needed for Redis mocking)
- `github.com/DATA-DOG/go-sqlmock` ❌ (wrong approach for pgx, not SQL)
- `github.com/stretchr/testify v1.10.0` ✅

**Files incorrectly importing pgx/v4:**
1. `config/database_utils_test.go:10` - imports `github.com/jackc/pgx/v4/pgxpool`
2. `db/db_client_test.go:11` - imports `github.com/jackc/pgx/v4/pgxpool`
3. `services/health_service_test.go:12` - imports `github.com/jackc/pgx/v4/pgxpool`

**Files correctly using pashagolub/pgxmock:**
- `services/health_service_test.go:13` - already imports `github.com/pashagolub/pgxmock` (but package not in go.mod)

## Standard Stack

### Core Testing Dependencies

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/pashagolub/pgxmock/v4` | Latest (v4.3+) | Mock pgx v5 driver | Official pgx mocking library, supports pgx v5, batch queries |
| `github.com/go-redis/redismock/v9` | Latest | Mock Redis v9 client | Official go-redis mocking library |
| `github.com/stretchr/testify` | v1.10.0 | Assertions & mocks | Already in go.mod, widely used |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/alicebob/miniredis/v2` | v2.33+ | In-memory Redis | Integration tests needing real Redis behavior |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `pashagolub/pgxmock` | `DATA-DOG/go-sqlmock` | sqlmock uses database/sql, incompatible with pgx driver interface |
| `go-redis/redismock` | `alicebob/miniredis` | miniredis is heavier (in-memory server) vs lightweight mock |
| Manual mocks | `mockgen` (gomock) | Code generation adds build complexity, manual mocks sufficient for this codebase |

**Installation:**
```bash
go get github.com/pashagolub/pgxmock/v4
go get github.com/go-redis/redismock/v9
```

## Architecture Patterns

### Recommended Mock Organization

```
tests/
├── mocks/              # Shared mocks (when needed by multiple packages)
│   └── README.md       # Documents which mocks are canonical
handlers/
├── user_handler_test.go
└── mocks_test.go       # Package-local mocks (MockUserService, etc.)
middleware/
├── auth_test.go
└── mocks_test.go       # Package-local mocks (MockJWTValidator, etc.)
models/user/service/
└── user_service_test.go  # Embed mocks in test file (small interfaces)
```

### Pattern 1: Package-Local Mocks for Single Use

**What:** Define mocks in the same package that uses them, in a separate `mocks_test.go` file.

**When to use:** Mock only used by tests in this package.

**Example:**
```go
// handlers/mocks_test.go
package handlers

import (
	"context"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/stretchr/testify/mock"
)

// MockUserService implements user service for handler tests
type MockUserService struct {
	mock.Mock
}

func (m *MockUserService) GetUserByID(ctx context.Context, id string) (*types.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}
// ... other methods
```

**Why:** Keeps mocks close to their usage, avoids import cycles, self-documenting.

### Pattern 2: Embedded Mocks for Small Interfaces

**What:** Define mocks directly in the test file when the interface is small (1-3 methods).

**When to use:** Interface is simple and only used in one test.

**Example:**
```go
// models/user/service/user_service_test.go
package service

type MockUserStore struct {
	mock.Mock
}

func (m *MockUserStore) GetUserByID(ctx context.Context, id string) (*types.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}
```

**Why:** Minimal overhead, no extra files, easy to update when interface changes.

### Pattern 3: pgxmock for Database Tests

**What:** Use `pgxmock.NewPool()` to create a mock pgxpool.Pool with expectations.

**When to use:** Testing code that queries PostgreSQL via pgx.

**Example:**
```go
// Source: https://github.com/pashagolub/pgxmock
func TestHealthService_CheckDatabase(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	// Set expectations
	mockDB.ExpectPing().WillReturnError(nil)
	mockDB.ExpectStat().WillReturn(&pgxpool.Stat{})
	mockDB.ExpectConfig().WillReturn(&pgxpool.Config{MaxConns: 10})

	service := NewHealthService(mockDB, nil, "1.0.0")
	result := service.checkDatabase(context.Background())

	assert.Equal(t, types.HealthStatusUp, result.Status)
	require.NoError(t, mockDB.ExpectationsWereMet())
}
```

### Pattern 4: redismock for Redis Tests

**What:** Use `redismock.NewClientMock()` to create a mock Redis client.

**When to use:** Testing code that uses Redis.

**Example:**
```go
// Source: https://github.com/go-redis/redismock
func TestRedisOperation(t *testing.T) {
	mockRedis, redisMock := redismock.NewClientMock()

	key := "test-key"
	value := "test-value"

	redisMock.ExpectGet(key).SetVal(value)
	redisMock.ExpectSet(key, value, 30*time.Minute).SetVal("OK")

	// Run code using mockRedis
	result, err := mockRedis.Get(context.Background(), key).Result()

	assert.NoError(t, err)
	assert.Equal(t, value, result)
	require.NoError(t, redisMock.ExpectationsWereMet())
}
```

### Anti-Patterns to Avoid

- **Using database/sql mocks with pgx:** sqlmock uses database/sql.DB, pgx uses its own driver interface. They're incompatible.
- **Global mock variables:** Creates test pollution, breaks parallel tests.
- **Mocks in production code:** Keep mocks in `*_test.go` files only.
- **Over-mocking:** If you find yourself mocking 10+ methods, consider integration tests with testcontainers.

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Mocking pgx v5 connections | Custom interface wrappers | `pashagolub/pgxmock/v4` | Handles Ping(), Query(), QueryRow(), Exec(), transactions, batches, prepared statements |
| Mocking Redis commands | Custom Redis mock | `go-redis/redismock/v9` | Supports all Redis commands, pipelines, transactions, pub/sub |
| Assertions in tests | Manual if/else checks | `stretchr/testify/assert` | Cleaner failures, better diffs, already in go.mod |
| In-memory Postgres | Docker Postgres in tests | `testcontainers-go` | Already in go.mod, provides real Postgres behavior |

**Key insight:** pgx v5 uses its own driver interface (not database/sql), so standard SQL mocks like sqlmock don't work. pgxmock implements the pgx-specific interfaces (Pool, Conn, Batch, Tx).

## Common Pitfalls

### Pitfall 1: Using sqlmock with pgx v5

**What goes wrong:** Tests import `github.com/DATA-DOG/go-sqlmock` but pgx doesn't use database/sql.

**Why it happens:** sqlmock is popular for database/sql testing, but pgx bypasses database/sql for performance.

**How to avoid:**
- Use `pashagolub/pgxmock/v4` for pgx v5
- Remove sqlmock imports: `db/db_client_test.go`, `config/database_utils_test.go`

**Warning signs:**
```go
import "github.com/DATA-DOG/go-sqlmock"  // WRONG for pgx
import "github.com/jackc/pgx/v4"         // WRONG - should be v5
```

### Pitfall 2: Importing pgx/v4 when go.mod has v5

**What goes wrong:** Compilation fails with "no required module provides package github.com/jackc/pgx/v4/pgxpool".

**Why it happens:** Copy-paste from old code, or following outdated examples.

**How to avoid:**
- Search for `pgx/v4` imports: `grep -r "pgx/v4" --include="*.go" .`
- Replace with `pgx/v5` or remove (use pgxmock instead)

**Warning signs:**
```go
import "github.com/jackc/pgx/v4/pgxpool"  // WRONG
// Should be:
import "github.com/jackc/pgx/v5/pgxpool"  // Correct
```

### Pitfall 3: Mock Interface Drift

**What goes wrong:** Test compiles initially, then fails after interface adds new method (e.g., `Validator` adds `ValidateAndGetClaims()`).

**Why it happens:** Mock doesn't implement all interface methods.

**How to avoid:**
- Add interface assertion: `var _ Validator = (*MockJWTValidator)(nil)`
- Compiler will catch missing methods immediately
- Implement new methods even if test doesn't use them (can panic or return nil)

**Warning signs:**
```
error: *MockJWTValidator does not implement Validator (missing method ValidateAndGetClaims)
```

### Pitfall 4: Duplicate Mock Declarations

**What goes wrong:** Two test files in the same package define the same mock type.

**Why it happens:** Multiple developers adding tests, or copy-paste without checking existing mocks.

**How to avoid:**
- Consolidate package mocks in `mocks_test.go`
- Document canonical mock location in README or comments
- Grep for existing mocks before creating: `grep -rn "type Mock" --include="*_test.go" .`

**Warning signs:**
```
user_handler_test.go:20:6: MockUserService redeclared in this block
trip_handler_test.go:135:6: other declaration of MockUserService
```

### Pitfall 5: jwt.Parser API Changes

**What goes wrong:** `jwt.NewParser().Parts(token)` doesn't exist in jwt/v5.

**Why it happens:** API changed between jwt versions, old test code.

**How to avoid:**
- Use `strings.Split(token, ".")` directly (JWT parts are base64-encoded, separated by dots)
- Or use `jwt.ParseUnverified()` if you need parsed claims

**Warning signs:**
```
error: jwt.NewParser().Parts undefined (type *jwt.Parser has no field or method Parts)
```

## Code Examples

Verified patterns from official sources:

### pgxmock: Basic Query Expectation

```go
// Source: https://github.com/pashagolub/pgxmock
import (
	"context"
	"testing"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"
)

func TestDatabaseQuery(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	rows := mock.NewRows([]string{"id", "name"}).
		AddRow(1, "Alice").
		AddRow(2, "Bob")

	mock.ExpectQuery("SELECT id, name FROM users").
		WillReturnRows(rows)

	// Test your code that uses mock...

	require.NoError(t, mock.ExpectationsWereMet())
}
```

### pgxmock: Transaction Test

```go
// Source: https://github.com/pashagolub/pgxmock
func TestTransaction(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").
		WithArgs("Alice", "alice@example.com").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	// Test your code...

	require.NoError(t, mock.ExpectationsWereMet())
}
```

### redismock: Command Expectations

```go
// Source: https://github.com/go-redis/redismock
import (
	"context"
	"testing"
	"time"
	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/require"
)

func TestRedisCache(t *testing.T) {
	client, mock := redismock.NewClientMock()

	key := "user:123"
	value := `{"id":"123","name":"Alice"}`

	// Expect GET to miss (key doesn't exist)
	mock.ExpectGet(key).RedisNil()

	// Expect SET to succeed
	mock.ExpectSet(key, value, 30*time.Minute).SetVal("OK")

	// Test your code...

	require.NoError(t, mock.ExpectationsWereMet())
}
```

### Interface Assertion Pattern

```go
// Ensures mock implements interface at compile time
type MockUserService struct {
	mock.Mock
}

var _ UserServiceInterface = (*MockUserService)(nil)

// If interface adds new method, compiler will fail here immediately
```

### Fixing Duplicate Mocks

```go
// BEFORE (handlers/user_handler_test.go):
type MockUserService struct { ... }  // Line 20

// BEFORE (handlers/trip_handler_test.go):
type MockUserService struct { ... }  // Line 135

// AFTER (handlers/mocks_test.go):
package handlers

// MockUserService implements UserServiceInterface for handler tests
type MockUserService struct {
	mock.Mock
}

// Implement ALL methods from UserServiceInterface
func (m *MockUserService) GetUserByID(ctx context.Context, id string) (*types.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}
// ... all other methods

// DELETE MockUserService from user_handler_test.go and trip_handler_test.go
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `pgx/v4` | `pgx/v5` | Oct 2023 (pgx v5.0.0) | Connection pooling improvements, better performance |
| `DATA-DOG/go-sqlmock` | `pashagolub/pgxmock/v4` | N/A (never correct for pgx) | pgxmock supports pgx driver, sqlmock doesn't |
| `go-redis/redis/v8` | `redis/go-redis/v9` | Feb 2023 (v9.0.0) | Context support, improved API |
| `go-redis/redismock/v8` | `go-redis/redismock/v9` | Jun 2023 | Matches go-redis/v9 |
| Manual JWT parsing | `jwt.ParseUnverified()` | jwt/v5 (Jan 2024) | Cleaner API, removed `Parser.Parts()` |

**Deprecated/outdated:**
- `jackc/pgx/v4` → use `v5` (v4 end of support: December 2024)
- `jwt.Parser.Parts()` → use `strings.Split()` or `ParseUnverified()`
- `go-redis/redis/v8` → use `v9` (better context support)

## Open Questions

Things that couldn't be fully resolved:

1. **Should we use testcontainers for integration tests?**
   - What we know: testcontainers already in go.mod, provides real Postgres
   - What's unclear: Whether to keep pgxmock for unit tests AND testcontainers for integration
   - Recommendation: Yes - use pgxmock for fast unit tests, testcontainers for slower integration tests

2. **Should duplicate mocks be in tests/mocks/ or package-local?**
   - What we know: handlers has duplicate MockUserService used by user_handler_test and trip_handler_test
   - What's unclear: If other packages also need MockUserService
   - Recommendation: Keep in `handlers/mocks_test.go` unless other packages import handlers for testing

3. **Is the notification pagination test failure a real bug?**
   - What we know: Test expects limit=10, API returns limit=20
   - What's unclear: Is 20 the correct default, or is test assertion wrong?
   - Recommendation: Review API spec, update test if 20 is correct default

## Execution Order

Critical path (dependencies between fixes):

```
1. Install missing test dependencies
   ├─> go get github.com/pashagolub/pgxmock/v4
   ├─> go get github.com/go-redis/redismock/v9
   └─> go mod tidy

2. Fix pgx v4 imports (blocks 5 packages)
   ├─> config/database_utils_test.go: v4/pgxpool → v5/pgxpool
   ├─> db/db_client_test.go: Remove sqlmock, use pgxmock
   └─> services/health_service_test.go: v4/pgxpool → v5/pgxpool (already has pgxmock)

3. Consolidate duplicate mocks (handlers package)
   ├─> Create handlers/mocks_test.go with canonical MockUserService
   ├─> Delete MockUserService from user_handler_test.go
   └─> Delete MockUserService from trip_handler_test.go

4. Fix interface mismatches
   ├─> middleware/auth_test.go: Add ValidateAndGetClaims() to MockJWTValidator
   ├─> models/user/service/user_service_test.go: Add GetUserByContactEmail() to MockUserStore
   └─> handlers/trip_handler_test.go: Add AddMember() to MockTripModel

5. Fix type errors
   ├─> handlers/trip_handler_test.go: Add missing types.WeatherForecast, types.UserUpdate
   ├─> internal/auth/jwt_test.go: Replace jwt.Parser.Parts() with strings.Split()
   └─> models/trip/service/trip_service_notification_test.go: Fix constructor args

6. Fix test logic errors
   ├─> internal/notification/client_test.go: Update pagination assertion
   └─> tests/integration/invitation_integration_test.go: Remove LocationHandler field

7. Verify all packages compile
   └─> go test ./...
```

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| pgxmock API incompatible with existing code | LOW | HIGH | pgxmock v4 designed for pgx v5, well-documented |
| New dependencies break existing builds | LOW | MEDIUM | Both libraries are mature, stable APIs |
| Interface changes require extensive mock updates | MEDIUM | HIGH | Use interface assertions to catch at compile time |
| Tests fail after fixes (logic errors) | MEDIUM | LOW | Fix compilation first, then address test failures |
| CI environment missing new dependencies | LOW | LOW | go.mod update triggers automatic install in CI |

**Mitigation strategy:**
1. Install dependencies first (lowest risk)
2. Fix compilation errors (deterministic)
3. Run tests package-by-package to isolate failures
4. Update CI workflow if needed (already uses `go mod download`)

## Verification Commands

After each fix, run these commands to verify:

```bash
# 1. Check dependencies installed
go list -m github.com/pashagolub/pgxmock/v4
go list -m github.com/go-redis/redismock/v9

# 2. Find remaining pgx/v4 imports
grep -r "pgx/v4" --include="*.go" .

# 3. Check for duplicate mock declarations
grep -rn "type Mock" --include="*_test.go" . | sort

# 4. Compile all packages (should show 0 failures)
go test -c ./... 2>&1 | grep -E "FAIL|setup failed" | wc -l

# 5. Run tests (find logic errors)
go test ./... -v

# 6. Run with race detector
go test -race ./...

# 7. Check coverage
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep total
```

**Success criteria:**
- `go test -c ./...` produces 0 "FAIL" or "setup failed" messages
- `go test ./...` runs (may have test failures, but should compile)
- No pgx/v4 imports remain
- No duplicate mock declarations
- All interface assertions compile

## Sources

### Primary (HIGH confidence)
- [pashagolub/pgxmock GitHub](https://github.com/pashagolub/pgxmock) - Official pgx mocking library
- [pashagolub/pgxmock v4 announcement](https://pashagolub.github.io/blog/2024/pgxmock-v4/) - v4 features and migration guide
- [go-redis/redismock GitHub](https://github.com/go-redis/redismock) - Official Redis mocking library
- [pgxmock package docs](https://pkg.go.dev/github.com/pashagolub/pgxmock) - API reference
- [jackc/pgx v5 docs](https://pkg.go.dev/github.com/jackc/pgx/v5) - pgx v5 driver documentation

### Secondary (MEDIUM confidence)
- [Testing in Go When You Have a Redis Dependency](https://www.razvanh.com/blog/testing-golang-redis-dependency) - Redis testing patterns
- [5 Mocking Techniques for Go](https://www.myhatchpad.com/insight/mocking-techniques-for-go/) - Go mocking best practices
- [Testing Patterns in Go](https://www.eric-fritz.com/articles/testing-patterns-in-go/) - Testing architecture patterns

### Tertiary (LOW confidence)
- WebSearch results for "golang test duplicate mock declarations" - General patterns, needs validation

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Official libraries, actively maintained, designed for pgx v5 / go-redis v9
- Architecture: HIGH - Patterns verified from official examples and documentation
- Pitfalls: HIGH - Directly observed in codebase compilation errors
- Execution order: HIGH - Dependency analysis from go.mod and import statements

**Research date:** 2026-02-04
**Valid until:** 2026-03-04 (30 days - stable ecosystem, but test libraries update frequently)
