package middleware

import (
	"net/http"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
)

// RequireRole enforces role-based access control for a specific route
func RequireRole(tripModel models.TripModelInterface, requiredRole types.MemberRole) gin.HandlerFunc {
    return func(c *gin.Context) {
        log := logger.GetLogger()

        tripID := c.Param("id")
        userID := c.GetString("user_id")

        if tripID == "" || userID == "" {
            log.Warnw("Unauthorized: Missing trip or user ID", 
                "tripID", tripID, 
                "userID", userID,
            )
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error":   "Unauthorized",
                "message": "User ID or Trip ID missing in request",
            })
            return
        }        

        // Fetch user role
        role, err := tripModel.GetUserRole(c.Request.Context(), tripID, userID)
        if err != nil {
            log.Warnw("Failed to fetch user role",
                "tripID", tripID,
                "userID", userID,
                "error", err,
            )
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error":   "Unauthorized",
                "message": "Failed to retrieve user role",
            })
            return
        }

        // Check if role has sufficient permissions
        if !role.IsAuthorizedFor(requiredRole) {
            log.Warnw("Permission denied",
                "tripID", tripID,
                "userID", userID,
                "userRole", role,
                "requiredRole", requiredRole,
            )
            c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
                "error":   "Forbidden",
                "message": "Insufficient permissions",
            })
            return
        }

        // Store the verified role in context and proceed
        c.Set("user_role", role)
        c.Next()
    }
}