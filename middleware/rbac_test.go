package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type tripModelInterface interface {
    GetUserRole(ctx context.Context, tripID string, userID string) (types.MemberRole, error)
}

type MockTripModel struct {
    mock.Mock
}

func (m *MockTripModel) GetUserRole(ctx context.Context, tripID string, userID string) (types.MemberRole, error) {
    args := m.Called(ctx, tripID, userID)
    return args.Get(0).(types.MemberRole), args.Error(1)
}

func TestRequireRole(t *testing.T) {
    // Setup
    gin.SetMode(gin.TestMode)

    tests := []struct {
        name           string
        tripID        string
        userID        string
        requiredRole  types.MemberRole
        returnedRole  types.MemberRole
        returnedError error
        expectedCode  int
        contextRole   string // Role that should be set in context
    }{
        {
            name:          "Owner can access owner resources",
            tripID:        "trip-123",
            userID:        "user-123",
            requiredRole:  types.MemberRoleOwner,
            returnedRole:  types.MemberRoleOwner,
            returnedError: nil,
            expectedCode:  http.StatusOK,
            contextRole:   string(types.MemberRoleOwner),
        },
        {
            name:          "Owner can access member resources",
            tripID:        "trip-123",
            userID:        "user-123",
            requiredRole:  types.MemberRoleMember,
            returnedRole:  types.MemberRoleOwner,
            returnedError: nil,
            expectedCode:  http.StatusOK,
            contextRole:   string(types.MemberRoleOwner),
        },
        {
            name:          "Member cannot access owner resources",
            tripID:        "trip-123",
            userID:        "user-123",
            requiredRole:  types.MemberRoleOwner,
            returnedRole:  types.MemberRoleMember,
            returnedError: nil,
            expectedCode:  http.StatusForbidden,
            contextRole:   "",
        },
        {
            name:          "Member can access member resources",
            tripID:        "trip-123",
            userID:        "user-123",
            requiredRole:  types.MemberRoleMember,
            returnedRole:  types.MemberRoleMember,
            returnedError: nil,
            expectedCode:  http.StatusOK,
            contextRole:   string(types.MemberRoleMember),
        },
        {
            name:          "Missing trip ID",
            tripID:        "",
            userID:        "user-123",
            requiredRole:  types.MemberRoleMember,
            returnedRole:  types.MemberRoleNone,
            returnedError: nil,
            expectedCode:  http.StatusUnauthorized,
            contextRole:   "",
        },
        {
            name:          "Missing user ID",
            tripID:        "trip-123",
            userID:        "",
            requiredRole:  types.MemberRoleMember,
            returnedRole:  types.MemberRoleNone,
            returnedError: nil,
            expectedCode:  http.StatusUnauthorized,
            contextRole:   "",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup mock
            mockTripModel := new(MockTripModel)
            if tt.tripID != "" && tt.userID != "" {
                mockTripModel.On("GetUserRole", mock.Anything, tt.tripID, tt.userID).
                    Return(tt.returnedRole, tt.returnedError)
            }

            // Setup router and middleware
            router := gin.New()
            router.Use(func(c *gin.Context) {
                c.Set("user_id", tt.userID)
            })

            // Add test endpoint with middleware
            router.GET("/:id", RequireRole(mockTripModel, tt.requiredRole), func(c *gin.Context) {
                role, exists := c.Get("user_role")
                if exists {
                    assert.Equal(t, tt.contextRole, role)
                } else {
                    assert.Empty(t, tt.contextRole)
                }
                c.Status(http.StatusOK)
            })

            // Create request
            w := httptest.NewRecorder()
            req, _ := http.NewRequest("GET", "/"+tt.tripID, nil)
            router.ServeHTTP(w, req)

            // Assert
            assert.Equal(t, tt.expectedCode, w.Code)
            mockTripModel.AssertExpectations(t)
        })
    }
}