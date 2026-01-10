# Testing Patterns

**Analysis Date:** 2026-01-10

## Test Framework

**Runner:**
- Go standard testing (`go test`)
- No external test runner

**Assertion Library:**
- testify v1.10.0 (`github.com/stretchr/testify`)
- `assert` for soft assertions
- `require` for fatal assertions
- `mock` for mock generation

**Run Commands:**
```bash
go test ./...                           # Run all tests
go test ./... -v                        # Verbose output
go test ./handlers/...                  # Test specific package
go test -cover ./...                    # With coverage
go test -coverprofile=coverage.out ./... # Generate coverage report
go tool cover -html=coverage.out        # View coverage in browser
```

## Test File Organization

**Location:**
- `*_test.go` co-located with source files
- Integration tests in `tests/integration/`
- Mocks in `internal/store/mocks/` and `tests/mocks/`

**Naming:**
- `{file}_test.go` for unit tests
- `{feature}_integration_test.go` for integration tests
- `{mock_name}.go` for mock implementations

**Structure:**
```
handlers/
  trip_handler.go
  trip_handler_test.go
models/trip/service/
  trip_model_coordinator.go
  trip_service_test.go
tests/integration/
  trip_test.go
  invitation_integration_test.go
```

## Test Structure

**Suite Organization:**
```go
func TestTripHandler_CreateTrip(t *testing.T) {
    // Setup
    mockStore := mocks.NewTripStore(t)
    handler := NewTripHandler(mockStore, ...)

    t.Run("success case", func(t *testing.T) {
        // Arrange
        mockStore.On("CreateTrip", mock.Anything, mock.Anything).Return(trip, nil)

        // Act
        result, err := handler.CreateTrip(ctx, input)

        // Assert
        require.NoError(t, err)
        assert.Equal(t, expected, result)
    })

    t.Run("validation error", func(t *testing.T) {
        // Test error case
    })
}
```

**Patterns:**
- Table-driven tests for multiple scenarios
- `t.Run()` for subtests
- `require` for preconditions, `assert` for checks
- Arrange/Act/Assert structure

## Mocking

**Framework:**
- testify/mock for interface mocking
- mockery for mock generation (some mocks in `internal/store/mocks/`)

**Patterns:**
```go
// Setup mock
mockStore := mocks.NewTripStore(t)

// Configure expectations
mockStore.On("GetTrip", mock.Anything, tripID).Return(trip, nil)
mockStore.On("CreateTrip", mock.Anything, mock.MatchedBy(func(t *models.Trip) bool {
    return t.Name == "Test Trip"
})).Return(nil)

// Verify calls
mockStore.AssertExpectations(t)
mockStore.AssertCalled(t, "GetTrip", mock.Anything, tripID)
```

**What to Mock:**
- Database stores (`internal/store/mocks/`)
- External services (Supabase, Redis)
- Event publishers (`internal/events/mock_publisher.go`)
- HTTP clients

**What NOT to Mock:**
- Pure functions
- Value objects
- Simple utilities

## Fixtures and Factories

**Test Data:**
```go
// Helper functions in tests
func createTestTrip(overrides ...func(*models.Trip)) *models.Trip {
    trip := &models.Trip{
        ID:        uuid.New().String(),
        Name:      "Test Trip",
        Status:    "planning",
        CreatedAt: time.Now(),
    }
    for _, override := range overrides {
        override(trip)
    }
    return trip
}

// Usage
trip := createTestTrip(func(t *models.Trip) {
    t.Name = "Custom Name"
})
```

**Location:**
- Factory functions defined in test files
- Shared test utilities in `tests/testutil/`
- Integration test fixtures inline

## Coverage

**Requirements:**
- No enforced coverage target
- Coverage report generated: `coverage.out`, `coverage.html`
- Focus on critical paths

**Configuration:**
- Standard Go coverage tooling
- HTML report for visual inspection

**View Coverage:**
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
# Open coverage.html in browser
```

## Test Types

**Unit Tests:**
- Co-located with source (`*_test.go`)
- Mock all dependencies
- Fast execution (<1s per test)
- Examples: `handlers/trip_handler_test.go`, `middleware/auth_test.go`

**Integration Tests:**
- Located in `tests/integration/`
- Use testcontainers for real PostgreSQL
- Test multiple components together
- Examples: `tests/integration/invitation_integration_test.go`

**Mock Tests:**
- Use generated mocks from testify/mockery
- Located alongside unit tests
- Examples: `internal/store/postgres/user_store_mock_test.go`

## Common Patterns

**Async Testing:**
```go
func TestAsyncOperation(t *testing.T) {
    done := make(chan struct{})
    go func() {
        // async operation
        close(done)
    }()
    select {
    case <-done:
        // success
    case <-time.After(5 * time.Second):
        t.Fatal("timeout waiting for async operation")
    }
}
```

**Error Testing:**
```go
func TestErrorCase(t *testing.T) {
    _, err := service.DoSomething(invalidInput)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "expected error message")
}
```

**Context Testing:**
```go
func TestWithContext(t *testing.T) {
    ctx := context.Background()
    ctx = context.WithValue(ctx, middleware.UserIDKey, "user-123")

    result, err := handler.HandleRequest(ctx, request)
    require.NoError(t, err)
}
```

**HTTP Handler Testing:**
```go
func TestHTTPHandler(t *testing.T) {
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = httptest.NewRequest("POST", "/trips", body)
    c.Set(string(middleware.UserIDKey), "user-123")

    handler.CreateTripHandler(c)

    assert.Equal(t, http.StatusCreated, w.Code)
}
```

**Testcontainers (Integration):**
```go
func TestWithDatabase(t *testing.T) {
    ctx := context.Background()
    container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: testcontainers.ContainerRequest{
            Image:        "postgres:latest",
            ExposedPorts: []string{"5432/tcp"},
            // ...
        },
        Started: true,
    })
    require.NoError(t, err)
    defer container.Terminate(ctx)

    // Run tests against real database
}
```

---

*Testing analysis: 2026-01-10*
*Update when test patterns change*
