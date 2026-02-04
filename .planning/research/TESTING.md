# Go Testing Infrastructure Research

**Project:** NomadCrew Backend
**Researched:** 2026-02-04
**Go Version:** 1.24
**Overall Confidence:** HIGH

## Executive Summary

This research addresses repairing the broken test suite (22 packages failing) and establishes best practices for Go testing in 2025/2026. The primary issues are version mismatches (pgx/v4 imports when using v5), missing dependencies, and inconsistent mock patterns. The recommended approach combines testify for assertions, mockery v3 for generated mocks, pgxmock v4 for database testing, and miniredis for Redis testing.

## Critical Issues in Current Codebase

### 1. pgx Version Mismatch (BLOCKING)

**Problem:** Test files import `github.com/jackc/pgx/v4` but `go.mod` declares `github.com/jackc/pgx/v5`.

**Affected files:**
- `internal/store/postgres/user_store_mock_test.go` (line 19)
- `store/postgres/trip_store_pg_mock_test.go` (line 15)

**Solution:** Change all pgx imports to v5 and use `pgxmock/v4` (which supports pgx/v5).

### 2. Missing Test Dependencies (BLOCKING)

**Missing from go.mod:**
```go
// Required additions
github.com/DATA-DOG/go-sqlmock     // Used in existing tests (but should migrate to pgxmock)
github.com/pashagolub/pgxmock/v4   // Proper pgx/v5 mocking
github.com/alicebob/miniredis/v2   // Redis testing
```

### 3. Inconsistent Mock Patterns (TECHNICAL DEBT)

**Current state:**
- Manual mocks scattered across test files
- Duplicate mock definitions (e.g., `MockTripStore` in multiple locations)
- No centralized mock generation
- Interface mismatches between mocks and implementations

**Example of duplication:**
- `tests/mocks/mock_trip_store.go`
- `internal/store/postgres/user_store_mock_test.go` (defines inline mocks)
- `models/trip/service/trip_model_coordinator_test.go` (defines inline mocks)

---

## Recommended Testing Stack

### Core Tools

| Tool | Version | Purpose | Confidence |
|------|---------|---------|------------|
| testify | v1.10.0 | Assertions, suite, require | HIGH |
| mockery | v3.5+ | Mock generation | HIGH |
| pgxmock | v4.3+ | pgx/v5 database mocking | HIGH |
| miniredis | v2.x | Redis mocking | HIGH |
| testcontainers-go | v0.37+ | Integration tests | HIGH |

### Why These Tools

**testify (keep):** Already in use, well-established, provides `assert`, `require`, `mock`, and `suite` packages. 60% of Go developers use built-in testing; testify is the dominant third-party choice.

**mockery v3 (add):** Generates type-safe mocks from interfaces automatically. v3 provides:
- 5-10x faster than v2 for large codebases
- Template-based generation
- Single output file per package
- Support for multiple mock styles

**pgxmock v4 (add):** Native pgx/v5 support without requiring `database/sql`. The existing `sqlmock` tests are incorrect for pgx native interface.

**miniredis v2 (add):** In-process Redis server for unit tests. Compatible with go-redis/v9. Faster than mocks for most scenarios.

**testcontainers-go (keep):** Already in use for integration tests. Provides real database instances for true integration testing.

---

## Mock Strategy: Mockery vs Manual

### Recommendation: Mockery v3 for Interface Mocks

**Why generated mocks:**
1. **Type safety:** Compiler catches interface changes
2. **Consistency:** All mocks follow same pattern
3. **Maintenance:** Regenerate when interfaces change
4. **Speed:** v3 is orders of magnitude faster than manual maintenance

**When to use manual mocks:**
- Simple test-specific fakes (like `MockPublisher` in events package)
- Mocks with custom behavior beyond expectation matching
- Testing concurrent code with specific timing requirements

### Configuration

Create `.mockery.yaml` in project root:

```yaml
# .mockery.yaml
version: 3
packages:
  github.com/NomadCrew/nomad-crew-backend/internal/store:
    interfaces:
      TripStore:
        config:
          dir: "tests/mocks"
          outpkg: "mocks"
      UserStore:
        config:
          dir: "tests/mocks"
          outpkg: "mocks"
      TodoStore:
        config:
          dir: "tests/mocks"
          outpkg: "mocks"
      ChatStore:
        config:
          dir: "tests/mocks"
          outpkg: "mocks"
      PushTokenStore:
        config:
          dir: "tests/mocks"
          outpkg: "mocks"
  github.com/NomadCrew/nomad-crew-backend/types:
    interfaces:
      EventPublisher:
        config:
          dir: "tests/mocks"
          outpkg: "mocks"
      EmailService:
        config:
          dir: "tests/mocks"
          outpkg: "mocks"
  github.com/NomadCrew/nomad-crew-backend/middleware:
    interfaces:
      Validator:
        config:
          dir: "tests/mocks"
          outpkg: "mocks"
      UserResolver:
        config:
          dir: "tests/mocks"
          outpkg: "mocks"
```

**Generate mocks:**
```bash
go install github.com/vektra/mockery/v3@latest
mockery
```

---

## Database Testing Patterns

### Unit Tests: pgxmock

For testing store implementations without a real database:

```go
import (
    "context"
    "testing"

    "github.com/pashagolub/pgxmock/v4"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestTripStore_GetTrip(t *testing.T) {
    mock, err := pgxmock.NewPool()
    require.NoError(t, err)
    defer mock.Close()

    tripID := "test-trip-id"

    rows := pgxmock.NewRows([]string{"id", "name", "description"}).
        AddRow(tripID, "Test Trip", "Description")

    mock.ExpectQuery("SELECT .+ FROM trips WHERE id = \\$1").
        WithArgs(tripID).
        WillReturnRows(rows)

    store := NewTripStore(mock)
    trip, err := store.GetTrip(context.Background(), tripID)

    assert.NoError(t, err)
    assert.Equal(t, tripID, trip.ID)
    assert.NoError(t, mock.ExpectationsWereMet())
}
```

**Key differences from sqlmock:**
- Use `pgxmock.NewPool()` instead of `sqlmock.New()`
- Native pgx types (no `database/sql` wrapper)
- Direct support for pgx-specific features

### Integration Tests: testcontainers

For testing real database interactions:

```go
func setupTestDatabase(t *testing.T) (*pgxpool.Pool, func()) {
    if runtime.GOOS == "windows" {
        t.Skip("Skipping integration test on Windows")
    }

    ctx := context.Background()

    container, err := postgres.Run(ctx,
        "postgres:16-alpine",
        postgres.WithDatabase("testdb"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").
                WithOccurrence(2).
                WithStartupTimeout(5*time.Second),
        ),
    )
    require.NoError(t, err)

    connStr, err := container.ConnectionString(ctx, "sslmode=disable")
    require.NoError(t, err)

    pool, err := pgxpool.New(ctx, connStr)
    require.NoError(t, err)

    // Run migrations
    runMigrations(t, pool)

    return pool, func() {
        pool.Close()
        container.Terminate(ctx)
    }
}
```

---

## Redis Testing Patterns

### Unit Tests: miniredis

```go
import (
    "context"
    "testing"

    "github.com/alicebob/miniredis/v2"
    "github.com/redis/go-redis/v9"
    "github.com/stretchr/testify/require"
)

func TestRateLimiter(t *testing.T) {
    mr := miniredis.RunT(t) // Auto-cleanup on test end

    client := redis.NewClient(&redis.Options{
        Addr: mr.Addr(),
    })
    defer client.Close()

    limiter := NewRateLimiter(client)

    // Test rate limiting logic
    allowed, err := limiter.Allow(context.Background(), "user-123")
    require.NoError(t, err)
    require.True(t, allowed)

    // Fast-forward time if needed
    mr.FastForward(time.Minute)
}
```

**Why miniredis over go-redis/redismock:**
- Actual Redis implementation (catches more bugs)
- No expectation setup required
- Time manipulation support (`FastForward`)
- Cleaner test code

### Integration Tests: testcontainers

For Redis-specific features or cluster testing:

```go
container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
    ContainerRequest: testcontainers.ContainerRequest{
        Image:        "redis:7-alpine",
        ExposedPorts: []string{"6379/tcp"},
        WaitingFor:   wait.ForLog("Ready to accept connections"),
    },
    Started: true,
})
```

---

## Test Organization

### Recommended Directory Structure

```
nomad-crew-backend/
├── tests/
│   ├── mocks/                    # Generated mocks (mockery output)
│   │   ├── mock_trip_store.go
│   │   ├── mock_user_store.go
│   │   └── ...
│   ├── testutil/                 # Test utilities
│   │   ├── fixtures.go           # Test data factories
│   │   ├── db.go                 # Database test helpers
│   │   └── assertions.go         # Custom assertions
│   └── integration/              # Integration tests
│       ├── db_test.go
│       └── trip_test.go
├── *_test.go                     # Unit tests alongside source
└── .mockery.yaml                 # Mockery configuration
```

### Test File Naming

| Pattern | Purpose | Example |
|---------|---------|---------|
| `*_test.go` | Unit tests (same package) | `trip_service_test.go` |
| `*_internal_test.go` | Internal unit tests | `cache_internal_test.go` |
| `tests/integration/*_test.go` | Integration tests | `trip_integration_test.go` |

### Build Tags for Test Types

```go
//go:build integration
// +build integration

package integration

// Integration tests here
```

Run selectively:
```bash
# Unit tests only (default)
go test ./...

# Integration tests only
go test -tags=integration ./tests/integration/...

# All tests
go test -tags=integration ./...
```

---

## Interface Design for Testability

### Principle: Consumer-Defined Interfaces

Define small interfaces where they are used, not where implemented:

```go
// BAD: Large interface in store package
type Store interface {
    CreateTrip(ctx context.Context, trip types.Trip) (string, error)
    GetTrip(ctx context.Context, id string) (*types.Trip, error)
    UpdateTrip(ctx context.Context, id string, update types.TripUpdate) (*types.Trip, error)
    DeleteTrip(ctx context.Context, id string) error
    ListTrips(ctx context.Context, userID string) ([]*types.Trip, error)
    // ... 20 more methods
}

// GOOD: Small interfaces at consumer
// In trip service:
type TripReader interface {
    GetTrip(ctx context.Context, id string) (*types.Trip, error)
}

type TripWriter interface {
    CreateTrip(ctx context.Context, trip types.Trip) (string, error)
    UpdateTrip(ctx context.Context, id string, update types.TripUpdate) (*types.Trip, error)
}

// Service uses only what it needs
type TripService struct {
    reader TripReader
    writer TripWriter
}
```

**Benefits:**
- Smaller mocks (implement only needed methods)
- Clear dependencies
- Easier testing
- Follows Interface Segregation Principle

### Current Codebase Issue

The `internal/store/interfaces.go` defines large interfaces (`TripStore` with 20+ methods). Consider:

1. **Short-term:** Keep existing interfaces, generate mocks with mockery
2. **Long-term:** Refactor to consumer-defined interfaces as services are modified

---

## CI/CD Configuration

### Recommended GitHub Actions Workflow

```yaml
name: Test Suite

on:
  push:
    branches: [main, develop, feature/*]
  pull_request:
    branches: [main, develop]

env:
  GO_VERSION: '1.24'

jobs:
  unit-tests:
    name: Unit Tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true  # Built-in caching

      - name: Generate Mocks
        run: |
          go install github.com/vektra/mockery/v3@latest
          mockery

      - name: Run Unit Tests
        run: go test -v -race -coverprofile=coverage.out ./...

      - name: Upload Coverage
        uses: codecov/codecov-action@v4
        with:
          file: ./coverage.out

  integration-tests:
    name: Integration Tests
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16-alpine
        env:
          POSTGRES_USER: test
          POSTGRES_PASSWORD: test
          POSTGRES_DB: testdb
        ports:
          - 5432:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

      redis:
        image: redis:7-alpine
        ports:
          - 6379:6379
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true

      - name: Run Migrations
        env:
          DATABASE_URL: postgresql://test:test@localhost:5432/testdb?sslmode=disable
        run: |
          go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
          migrate -path db/migrations -database "$DATABASE_URL" up

      - name: Run Integration Tests
        env:
          DATABASE_URL: postgresql://test:test@localhost:5432/testdb?sslmode=disable
          REDIS_URL: redis://localhost:6379
        run: go test -v -tags=integration ./tests/integration/...

  lint:
    name: Lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true

      - uses: golangci/golangci-lint-action@v4
        with:
          version: latest
          args: --timeout=5m
```

### Key Improvements Over Current Workflow

1. **Separate unit and integration tests** - Faster feedback
2. **Mock generation in CI** - Ensures mocks are current
3. **Built-in Go caching** - Faster builds
4. **Postgres 16** - Match production version
5. **Updated action versions** - v4/v5 of checkout and setup-go

---

## Go 1.24 Testing Features to Adopt

### 1. T.Context() for Resource Cleanup

```go
func TestWithContext(t *testing.T) {
    ctx := t.Context() // Canceled when test completes

    // Resources tied to ctx auto-cleanup
    pool, err := pgxpool.New(ctx, connStr)
    require.NoError(t, err)
    // No defer pool.Close() needed - canceled with context
}
```

### 2. B.Loop() for Benchmarks

```go
func BenchmarkTripCreate(b *testing.B) {
    // Old way (error-prone)
    // for i := 0; i < b.N; i++ { ... }

    // New way (Go 1.24+)
    for b.Loop() {
        store.CreateTrip(ctx, trip)
    }
}
```

### 3. T.Chdir() for Working Directory

```go
func TestMigrations(t *testing.T) {
    t.Chdir("../db/migrations") // Automatically restored after test
    // Test migration files
}
```

---

## Anti-Patterns to Avoid

### 1. pgx/v4 and v5 Mixing

**Current issue:** Test files import v4, production uses v5.

**Fix:** Standardize on v5 everywhere:
```go
// WRONG
import "github.com/jackc/pgx/v4"

// RIGHT
import "github.com/jackc/pgx/v5"
```

### 2. sqlmock with pgx Native Interface

**Current issue:** Tests use `DATA-DOG/go-sqlmock` but code uses pgx native.

**Fix:** Use `pgxmock` for pgx native interface:
```go
// WRONG (with pgx native code)
db, mock, _ := sqlmock.New()

// RIGHT
mock, _ := pgxmock.NewPool()
```

### 3. Duplicate Mock Definitions

**Current issue:** Same mock defined in multiple files.

**Fix:** Generate once with mockery, import everywhere:
```go
// WRONG - inline mock in test file
type MockTripStore struct { mock.Mock }
func (m *MockTripStore) GetTrip(...) { ... }

// RIGHT - import generated mock
import "github.com/NomadCrew/nomad-crew-backend/tests/mocks"

mockStore := mocks.NewMockTripStore(t)
```

### 4. Testing Implementation Instead of Behavior

**Current issue:** Some tests verify internal implementation details.

**Fix:** Test observable behavior:
```go
// WRONG - testing internal state
assert.Equal(t, 3, store.callCount)

// RIGHT - testing behavior
trip, err := service.GetTrip(ctx, id)
assert.NoError(t, err)
assert.Equal(t, expectedTrip.Name, trip.Name)
```

### 5. Missing Interface Compliance Checks

**Good pattern already in codebase:**
```go
// Ensure mock implements interface at compile time
var _ Validator = (*MockJWTValidator)(nil)
```

**Enforce this for all mocks.**

---

## Migration Plan

### Phase 1: Fix Blocking Issues (Immediate)

1. Add missing dependencies to go.mod:
   ```bash
   go get github.com/pashagolub/pgxmock/v4
   go get github.com/alicebob/miniredis/v2
   ```

2. Fix pgx version imports (v4 -> v5) in all test files

3. Remove sqlmock usage, replace with pgxmock

### Phase 2: Centralize Mocks (Week 1)

1. Install mockery v3
2. Create `.mockery.yaml` configuration
3. Generate mocks for all interfaces
4. Update tests to use generated mocks
5. Remove duplicate manual mock definitions

### Phase 3: Standardize Patterns (Week 2)

1. Create `tests/testutil/` package with:
   - Test fixtures/factories
   - Database helpers
   - Custom assertions
2. Refactor integration tests to use testcontainers modules
3. Add build tags for test separation

### Phase 4: CI/CD Updates (Week 2)

1. Update GitHub Actions workflow
2. Add mock generation step
3. Separate unit and integration test jobs
4. Add coverage gates

---

## Verification Commands

```bash
# Verify all tests compile
go test -c ./...

# Run unit tests with race detection
go test -v -race ./...

# Run unit tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run integration tests
go test -v -tags=integration ./tests/integration/...

# Regenerate mocks
mockery

# Lint
golangci-lint run
```

---

## Sources

### Official Documentation
- [Go 1.24 Release Notes](https://go.dev/doc/go1.24) - Testing package improvements
- [testcontainers-go](https://golang.testcontainers.org/) - Integration testing
- [pgxmock](https://github.com/pashagolub/pgxmock) - pgx/v5 mocking
- [mockery](https://vektra.github.io/mockery/v3.6/) - Mock generation

### Best Practices
- [Mockery v3 Announcement](https://topofmind.dev/blog/2025/04/08/announcing-mockery-v3/) - v3 features
- [GitHub Actions Go Setup](https://github.com/actions/setup-go) - CI caching
- [Testcontainers Best Practices](https://www.docker.com/blog/testcontainers-best-practices/) - Docker testing
- [Interface Segregation in Go](https://rednafi.com/go/interface-segregation/) - Small interfaces

### Community Guides
- [Testing in Go with Redis](https://www.razvanh.com/blog/testing-golang-redis-dependency) - miniredis usage
- [Unit Tests in Go Structure](https://www.glukhov.org/post/2025/11/unit-tests-in-go/) - Test organization
- [JetBrains Mock Testing Guide](https://www.jetbrains.com/guide/go/tutorials/mock_testing_with_go/) - Mock patterns
