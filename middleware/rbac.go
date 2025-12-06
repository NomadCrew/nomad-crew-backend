package middleware

import (
	"net/http"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	tripinterfaces "github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
)

// Context keys for RBAC
const (
	ContextKeyUserRole       = "user_role"
	ContextKeyResourceOwner  = "resource_owner_id"
	ContextKeyIsResourceOwner = "is_resource_owner"
)

// OwnerIDExtractor is a function that extracts the owner ID of a resource from the request context.
// It returns the owner's user ID, or empty string if not applicable.
type OwnerIDExtractor func(c *gin.Context) string

// RequireRole enforces role-based access control for a specific route.
// This is the legacy function - prefer RequirePermission for new code.
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
				"contextKeys", c.Keys,                      // Log context keys again
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
		c.Set(ContextKeyUserRole, role)
		c.Next()
	}
}

// RequirePermission enforces permission-based access control using the permission matrix.
// It supports both role-based and ownership-based permissions.
//
// Parameters:
//   - tripModel: Interface to fetch user roles
//   - action: The action being performed (e.g., ActionUpdate, ActionDelete)
//   - resource: The resource type (e.g., ResourceTodo, ResourceChat)
//   - getOwnerID: Optional function to extract resource owner ID. Pass nil if not needed.
//
// Example usage:
//
//	// Simple role check (no ownership)
//	router.DELETE("/trips/:id", RequirePermission(tripModel, types.ActionDelete, types.ResourceTrip, nil))
//
//	// With ownership check (for todos, chats, etc.)
//	router.PUT("/trips/:id/todos/:todoID", RequirePermission(tripModel, types.ActionUpdate, types.ResourceTodo, getTodoOwner))
func RequirePermission(
	tripModel tripinterfaces.TripModelInterface,
	action types.Action,
	resource types.Resource,
	getOwnerID OwnerIDExtractor,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		log := logger.GetLogger()

		tripID := c.Param("id")
		userID := c.GetString("user_id")

		log.Debugw("Permission check initiated",
			"tripID", tripID,
			"userID", userID,
			"action", action,
			"resource", resource,
			"path", c.Request.URL.Path,
			"method", c.Request.Method,
		)

		// Validate required context
		if tripID == "" {
			log.Warnw("Permission denied: Missing trip ID",
				"action", action,
				"resource", resource,
			)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error":   "BadRequest",
				"message": "Trip ID is required",
			})
			return
		}

		if userID == "" {
			log.Warnw("Permission denied: Missing user ID",
				"tripID", tripID,
				"action", action,
				"resource", resource,
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "Authentication required",
			})
			return
		}

		// Fetch user role in this trip
		role, err := tripModel.GetUserRole(c.Request.Context(), tripID, userID)
		if err != nil {
			log.Warnw("Permission denied: Failed to get user role",
				"tripID", tripID,
				"userID", userID,
				"error", err,
			)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "Forbidden",
				"message": "You are not a member of this trip",
			})
			return
		}

		// Determine if ownership check is needed
		var isOwner bool
		var ownerID string

		if getOwnerID != nil && types.RequiresOwnership(resource, action) {
			ownerID = getOwnerID(c)
			isOwner = ownerID != "" && ownerID == userID
			c.Set(ContextKeyResourceOwner, ownerID)
			c.Set(ContextKeyIsResourceOwner, isOwner)
		}

		// Check permission using the permission matrix
		allowed := types.CanPerformWithOwnership(role, action, resource, isOwner)

		if !allowed {
			minRole := types.GetMinRoleForAction(resource, action)
			minRoleStr := "N/A"
			if minRole != nil {
				minRoleStr = string(*minRole)
			}

			log.Warnw("Permission denied",
				"tripID", tripID,
				"userID", userID,
				"userRole", role,
				"action", action,
				"resource", resource,
				"isOwner", isOwner,
				"requiredRole", minRoleStr,
			)

			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":         "Forbidden",
				"message":       "You don't have permission to perform this action",
				"action":        action,
				"resource":      resource,
				"your_role":     role,
				"required_role": minRoleStr,
			})
			return
		}

		// Store role in context for downstream handlers
		c.Set(ContextKeyUserRole, role)

		log.Debugw("Permission granted",
			"tripID", tripID,
			"userID", userID,
			"role", role,
			"action", action,
			"resource", resource,
			"isOwner", isOwner,
		)

		c.Next()
	}
}

// RequireTripMembership is a lightweight middleware that only checks if the user is a trip member.
// Use this when you need to verify membership but don't need specific permission checks.
func RequireTripMembership(tripModel tripinterfaces.TripModelInterface) gin.HandlerFunc {
	return func(c *gin.Context) {
		log := logger.GetLogger()

		tripID := c.Param("id")
		userID := c.GetString("user_id")

		if tripID == "" || userID == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error":   "BadRequest",
				"message": "Trip ID and authentication required",
			})
			return
		}

		role, err := tripModel.GetUserRole(c.Request.Context(), tripID, userID)
		if err != nil {
			log.Debugw("User is not a trip member",
				"tripID", tripID,
				"userID", userID,
				"error", err,
			)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "Forbidden",
				"message": "You are not a member of this trip",
			})
			return
		}

		c.Set(ContextKeyUserRole, role)
		c.Next()
	}
}

// GetUserRole retrieves the user's role from the request context.
// Returns the role and true if found, or empty role and false if not set.
func GetUserRole(c *gin.Context) (types.MemberRole, bool) {
	role, exists := c.Get(ContextKeyUserRole)
	if !exists {
		return "", false
	}
	memberRole, ok := role.(types.MemberRole)
	return memberRole, ok
}

// IsResourceOwner checks if the current user owns the resource being accessed.
// This should be called after RequirePermission middleware with an OwnerIDExtractor.
func IsResourceOwner(c *gin.Context) bool {
	isOwner, exists := c.Get(ContextKeyIsResourceOwner)
	if !exists {
		return false
	}
	return isOwner.(bool)
}
