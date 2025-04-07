package handlers

import (
	"net/http"
	"strconv"

	"github.com/NomadCrew/nomad-crew-backend/service"
	"github.com/NomadCrew/nomad-crew-backend/store"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// NotificationHandler handles HTTP requests related to notifications.
type NotificationHandler struct {
	notificationService service.NotificationService
	logger              *zap.Logger
}

// NewNotificationHandler creates a new NotificationHandler.
func NewNotificationHandler(ns service.NotificationService, logger *zap.Logger) *NotificationHandler {
	return &NotificationHandler{
		notificationService: ns,
		logger:              logger.Named("NotificationHandler"),
	}
}

// GetNotificationsByUser godoc
// @Summary Get user notifications
// @Description Retrieves notifications for the authenticated user with pagination and filtering
// @Tags notifications
// @Produce json
// @Param limit query int false "Number of notifications to return (default 20, max 100)"
// @Param offset query int false "Offset for pagination (default 0)"
// @Param status query string false "Filter by status ('read' or 'unread')"
// @Success 200 {array} models.Notification
// @Failure 400 {object} ErrorResponse "Invalid query parameters"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal Server Error"
// @Router /notifications [get]
// @Security BearerAuth
func (h *NotificationHandler) GetNotificationsByUser(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		return // Error handled by getUserIDFromContext
	}

	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")
	statusStr := c.Query("status") // optional

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 20 // Default/fallback limit
		// Optionally log or return a 400 error for invalid limit
		h.logger.Warn("Invalid limit query parameter, using default", zap.String("providedLimit", limitStr))
		// ErrorResponse(c, http.StatusBadRequest, "Invalid limit parameter")
		// return
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0 // Default/fallback offset
		h.logger.Warn("Invalid offset query parameter, using default", zap.String("providedOffset", offsetStr))
		// ErrorResponse(c, http.StatusBadRequest, "Invalid offset parameter")
		// return
	}

	var status *bool
	if statusStr != "" {
		if statusStr == "read" {
			readStatus := true
			status = &readStatus
		} else if statusStr == "unread" {
			readStatus := false
			status = &readStatus
		} else {
			// ErrorResponse(c, http.StatusBadRequest, "Invalid status parameter. Use 'read' or 'unread'.")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status parameter. Use 'read' or 'unread'."})
			return
		}
	}

	notifications, err := h.notificationService.GetNotifications(c.Request.Context(), userID, limit, offset, status)
	if err != nil {
		h.logger.Error("Failed to get notifications", zap.String("userID", userID.String()), zap.Error(err))
		// ErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve notifications")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve notifications"})
		return
	}

	c.JSON(http.StatusOK, notifications)
}

// MarkNotificationAsRead godoc
// @Summary Mark a notification as read
// @Description Marks a specific notification as read for the authenticated user
// @Tags notifications
// @Accept json
// @Produce json
// @Param notificationId path string true "Notification ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} ErrorResponse "Invalid Notification ID"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden (Notification does not belong to user)"
// @Failure 404 {object} ErrorResponse "Notification Not Found"
// @Failure 500 {object} ErrorResponse "Internal Server Error"
// @Router /notifications/{notificationId}/read [patch]
// @Security BearerAuth
func (h *NotificationHandler) MarkNotificationAsRead(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		return
	}

	notificationIDStr := c.Param("notificationId")
	notificationID, err := uuid.Parse(notificationIDStr)
	if err != nil {
		// ErrorResponse(c, http.StatusBadRequest, "Invalid notification ID format")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID format"})
		return
	}

	err = h.notificationService.MarkNotificationAsRead(c.Request.Context(), userID, notificationID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// ErrorResponse(c, http.StatusNotFound, "Notification not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "Notification not found"})
		} else if errors.Is(err, store.ErrForbidden) {
			// ErrorResponse(c, http.StatusForbidden, "You are not authorized to update this notification")
			c.JSON(http.StatusForbidden, gin.H{"error": "You are not authorized to update this notification"})
		} else {
			h.logger.Error("Failed to mark notification as read",
				zap.String("userID", userID.String()),
				zap.String("notificationID", notificationIDStr),
				zap.Error(err))
			// ErrorResponse(c, http.StatusInternalServerError, "Failed to mark notification as read")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark notification as read"})
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// MarkAllNotificationsRead godoc
// @Summary Mark all notifications as read
// @Description Marks all unread notifications as read for the authenticated user
// @Tags notifications
// @Accept json
// @Produce json
// @Success 200 {object} map[string]int64 "Returns the number of notifications marked as read"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal Server Error"
// @Router /notifications/read-all [patch]
// @Security BearerAuth
func (h *NotificationHandler) MarkAllNotificationsRead(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		return
	}

	// Optional: Add request body validation if needed in the future, e.g., expecting {"status": "read"}
	// For now, PATCH to the collection endpoint implies marking all as read.

	affectedRows, err := h.notificationService.MarkAllNotificationsAsRead(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to mark all notifications as read", zap.String("userID", userID.String()), zap.Error(err))
		// ErrorResponse(c, http.StatusInternalServerError, "Failed to mark all notifications as read")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark all notifications as read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"marked_as_read_count": affectedRows})
}

// DeleteNotification godoc
// @Summary Delete a notification
// @Description Deletes a specific notification for the authenticated user
// @Tags notifications
// @Produce json
// @Param notificationId path string true "Notification ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} ErrorResponse "Invalid Notification ID"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden (Notification does not belong to user)"
// @Failure 404 {object} ErrorResponse "Notification Not Found"
// @Failure 500 {object} ErrorResponse "Internal Server Error"
// @Router /notifications/{notificationId} [delete]
// @Security BearerAuth
func (h *NotificationHandler) DeleteNotification(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		return // Error handled by getUserIDFromContext
	}

	notificationIDStr := c.Param("notificationId")
	notificationID, err := uuid.Parse(notificationIDStr)
	if err != nil {
		// ErrorResponse(c, http.StatusBadRequest, "Invalid notification ID format")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID format"})
		return
	}

	h.logger.Info("Attempting to delete notification", zap.String("userID", userID.String()), zap.String("notificationID", notificationIDStr))

	err = h.notificationService.DeleteNotification(c.Request.Context(), userID, notificationID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.logger.Warn("DeleteNotification: Notification not found", zap.String("notificationID", notificationIDStr), zap.String("userID", userID.String()))
			// ErrorResponse(c, http.StatusNotFound, "Notification not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "Notification not found"})
		} else if errors.Is(err, store.ErrForbidden) {
			h.logger.Warn("DeleteNotification: Forbidden", zap.String("notificationID", notificationIDStr), zap.String("userID", userID.String()))
			// ErrorResponse(c, http.StatusForbidden, "You are not authorized to delete this notification")
			c.JSON(http.StatusForbidden, gin.H{"error": "You are not authorized to delete this notification"})
		} else {
			h.logger.Error("Failed to delete notification",
				zap.String("userID", userID.String()),
				zap.String("notificationID", notificationIDStr),
				zap.Error(err))
			// ErrorResponse(c, http.StatusInternalServerError, "Failed to delete notification")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete notification"})
		}
		return
	}

	h.logger.Info("Successfully deleted notification", zap.String("notificationID", notificationIDStr), zap.String("userID", userID.String()))
	c.Status(http.StatusNoContent)
}

// getUserIDFromContext extracts the user ID safely from the Gin context.
// Assumes middleware.AuthMiddleware() sets the user ID using the key "user_id".
func getUserIDFromContext(c *gin.Context) (uuid.UUID, error) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		// ErrorResponse(c, http.StatusUnauthorized, "User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return uuid.Nil, errors.New("user ID not found in context")
	}

	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		// ErrorResponse(c, http.StatusInternalServerError, "User ID in context is of invalid type")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User ID in context is of invalid type"})
		return uuid.Nil, errors.New("user ID in context is of invalid type")
	}

	return userID, nil
}
