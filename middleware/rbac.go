package middleware

import (
	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
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
		userID := c.GetString(string(UserIDKey))

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
			_ = c.Error(apperrors.ValidationFailed("missing_trip_id", "Trip ID is required"))
			c.Abort()
			return
		}

		if userID == "" {
			_ = c.Error(apperrors.Unauthorized("missing_user_id", "Authentication required"))
			c.Abort()
			return
		}

		// Fetch user role
		role, err := tripModel.GetUserRole(c.Request.Context(), tripID, userID)
		if err != nil {
			_ = c.Error(apperrors.Forbidden("not_trip_member", "You are not a member of this trip"))
			c.Abort()
			return
		}

		// Check if role has sufficient permissions
		if !role.IsAuthorizedFor(requiredRole) {
			_ = c.Error(apperrors.Forbidden("insufficient_permissions", "You don't have permission to perform this action"))
			c.Abort()
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
		userID := c.GetString(string(UserIDKey))

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
			_ = c.Error(apperrors.ValidationFailed("missing_trip_id", "Trip ID is required"))
			c.Abort()
			return
		}

		if userID == "" {
			_ = c.Error(apperrors.Unauthorized("missing_user_id", "Authentication required"))
			c.Abort()
			return
		}

		// Fetch user role in this trip
		role, err := tripModel.GetUserRole(c.Request.Context(), tripID, userID)
		if err != nil {
			_ = c.Error(apperrors.Forbidden("not_trip_member", "You are not a member of this trip"))
			c.Abort()
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
			_ = c.Error(apperrors.Forbidden("insufficient_permissions", "You don't have permission to perform this action"))
			c.Abort()
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
		userID := c.GetString(string(UserIDKey))

		if tripID == "" || userID == "" {
			_ = c.Error(apperrors.ValidationFailed("missing_parameters", "Trip ID and authentication required"))
			c.Abort()
			return
		}

		role, err := tripModel.GetUserRole(c.Request.Context(), tripID, userID)
		if err != nil {
			log.Debugw("User is not a trip member",
				"tripID", tripID,
				"userID", userID,
				"error", err,
			)
			_ = c.Error(apperrors.Forbidden("not_trip_member", "You are not a member of this trip"))
			c.Abort()
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
