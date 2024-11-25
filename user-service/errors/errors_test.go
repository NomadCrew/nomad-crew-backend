package errors

import (
    "fmt"
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
    err := New(ValidationError, "invalid input", "field required")
    assert.Equal(t, ValidationError, err.Type)
    assert.Equal(t, "invalid input", err.Message)
    assert.Equal(t, "field required", err.Detail)
    assert.Equal(t, 400, err.HTTPStatus)
}

func TestWrap(t *testing.T) {
    originalErr := fmt.Errorf("original error")
    wrappedErr := Wrap(originalErr, DatabaseError, "database operation failed")
    
    assert.Equal(t, DatabaseError, wrappedErr.Type)
    assert.Equal(t, "database operation failed", wrappedErr.Message)
    assert.Equal(t, originalErr.Error(), wrappedErr.Detail)
    assert.Equal(t, 500, wrappedErr.HTTPStatus)
    assert.Equal(t, originalErr, wrappedErr.Raw)
}

func TestNotFound(t *testing.T) {
    err := NotFound("User", 123)
    assert.Equal(t, NotFoundError, err.Type)
    assert.Equal(t, "User not found", err.Message)
    assert.Equal(t, "ID: 123", err.Detail)
    assert.Equal(t, 404, err.HTTPStatus)
}

func TestValidationFailed(t *testing.T) {
    err := ValidationFailed("Invalid email", "format not correct")
    assert.Equal(t, ValidationError, err.Type)
    assert.Equal(t, "Invalid email", err.Message)
    assert.Equal(t, "format not correct", err.Detail)
    assert.Equal(t, 400, err.HTTPStatus)
}

func TestAuthenticationFailed(t *testing.T) {
    err := AuthenticationFailed("Invalid credentials")
    assert.Equal(t, AuthError, err.Type)
    assert.Equal(t, "Invalid credentials", err.Message)
    assert.Equal(t, 401, err.HTTPStatus)
}

func TestNewDatabaseError(t *testing.T) {
    originalErr := fmt.Errorf("connection failed")
    err := NewDatabaseError(originalErr)
    assert.Equal(t, DatabaseError, err.Type)
    assert.Equal(t, "Database operation failed", err.Message)
    assert.Equal(t, originalErr.Error(), err.Detail)
    assert.Equal(t, 500, err.HTTPStatus)
    assert.Equal(t, originalErr, err.Raw)
}

func TestError_Error(t *testing.T) {
    tests := []struct {
        name     string
        err      *AppError
        expected string
    }{
        {
            name: "with detail",
            err: &AppError{
                Type:    ValidationError,
                Message: "invalid input",
                Detail:  "field required",
            },
            expected: "VALIDATION_ERROR: invalid input (field required)",
        },
        {
            name: "without detail",
            err: &AppError{
                Type:    AuthError,
                Message: "unauthorized",
            },
            expected: "AUTHENTICATION_ERROR: unauthorized",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            assert.Equal(t, tt.expected, tt.err.Error())
        })
    }
}