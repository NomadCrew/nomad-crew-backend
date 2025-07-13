package middleware

import (
	"net/http"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	tripinterfaces "github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
)

// RequireRole enforces role-based access control for a specific route
func RequireRole(tripModel tripinterfaces.TripModelInterface, requiredRole types.MemberRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		log := logger.GetLogger()

		tripID := c.Param("id")
		userID := c.GetString("user_id")

		// More detailed logging of all context values
		log.Debugw("RBAC Check Initiated",
			"tripID", tripID,
			"userID", userID,
			"path", c.Request.URL.Path,
			"method", c.Request.Method,
			"headers", c.Request.Header,
			"contextKeys", c.Keys, // Log all keys in the context
		)

		if tripID == "" {
			log.Warnw("Unauthorized: Missing trip ID", "tripID", tripID)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "User ID or Trip ID missing in request",
			})
			return
		}

		if userID == "" {
			log.Warnw("Unauthorized: Missing user ID",
				"tripID", tripID,
				"userID", userID,
				"authHeader", c.GetHeader("Authorization"), // Log the auth header to see if it's present
				"contextKeys", c.Keys, // Log context keys again
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
			log.Warnw("Failed to get user role",
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
				"message": "User does not have access to this resource",
			})
			return
		}

		// Store the verified role in context and proceed
		c.Set("user_role", role)
		c.Next()
	}
}
