package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/models"
	notificationSvc "github.com/NomadCrew/nomad-crew-backend/models/notification/service"
	"github.com/NomadCrew/nomad-crew-backend/store"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NotificationHandler handles HTTP requests related to notifications.
type NotificationHandler struct {
	notificationService notificationSvc.NotificationService
	logger              *zap.Logger
}

// NewNotificationHandler creates a new NotificationHandler.
func NewNotificationHandler(ns notificationSvc.NotificationService, logger *zap.Logger) *NotificationHandler {
	return &NotificationHandler{
		notificationService: ns,
		logger:              logger.Named("NotificationHandler"),
	}
}

// notificationResponse is the DTO sent to the frontend.
// It maps backend fields to the shape expected by the frontend Zod schema.
type notificationResponse struct {
	ID        string                 `json:"id"`
	Message   string                 `json:"message"`
	Read      bool                   `json:"read"`
	CreatedAt string                 `json:"createdAt"`
	Type      string                 `json:"type"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// toNotificationResponse converts a models.Notification to the frontend DTO shape.
func toNotificationResponse(n models.Notification) notificationResponse {
	// Parse metadata into a map
	var metadata map[string]interface{}
	if len(n.Metadata) > 0 {
		if err := json.Unmarshal(n.Metadata, &metadata); err != nil {
			metadata = make(map[string]interface{})
		}
	} else {
		metadata = make(map[string]interface{})
	}

	return notificationResponse{
		ID:        n.ID.String(),
		Message:   generateMessage(n.Type, metadata),
		Read:      n.IsRead,
		CreatedAt: n.CreatedAt.Format(time.RFC3339),
		Type:      n.Type,
		Metadata:  metadata,
	}
}

// generateMessage produces a human-readable message from the notification type and metadata.
func generateMessage(notifType string, metadata map[string]interface{}) string {
	tripName := getMetadataString(metadata, "tripName", "a trip")
	senderName := getMetadataString(metadata, "senderName", "Someone")

	switch notifType {
	case "TRIP_INVITATION", "TRIP_INVITATION_RECEIVED":
		inviterName := getMetadataString(metadata, "inviterName", "Someone")
		return fmt.Sprintf("%s invited you to %s", inviterName, tripName)
	case "CHAT_MESSAGE", "NEW_CHAT_MESSAGE":
		return fmt.Sprintf("%s sent a message", senderName)
	case "MEMBER_ADDED", "TRIP_MEMBER_JOINED", "MEMBERSHIP_CHANGE":
		memberName := getMetadataString(metadata, "addedUserName", getMetadataString(metadata, "memberName", "A new member"))
		return fmt.Sprintf("%s joined %s", memberName, tripName)
	case "TRIP_UPDATE", "TRIP_UPDATED":
		return fmt.Sprintf("%s has been updated", tripName)
	case "TODO_ASSIGNED", "TASK_ASSIGNED":
		assignerName := getMetadataString(metadata, "assignerName", "Someone")
		todoTitle := getMetadataString(metadata, "todoTitle", "a task")
		return fmt.Sprintf("%s assigned you: %s", assignerName, todoTitle)
	case "TODO_COMPLETED", "TASK_COMPLETED":
		completerName := getMetadataString(metadata, "completerName", "Someone")
		todoTitle := getMetadataString(metadata, "todoTitle", "a task")
		return fmt.Sprintf("%s completed: %s", completerName, todoTitle)
	case "TRIP_MEMBER_LEFT":
		memberName := getMetadataString(metadata, "memberName", "A member")
		return fmt.Sprintf("%s left %s", memberName, tripName)
	default:
		return "You have a new notification"
	}
}

// getMetadataString safely extracts a string value from metadata.
func getMetadataString(m map[string]interface{}, key, defaultValue string) string {
	if m == nil {
		return defaultValue
	}
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok && str != "" {
			return str
		}
	}
	return defaultValue
}

// toNotificationResponseList converts a slice of notifications to response DTOs.
func toNotificationResponseList(notifications []models.Notification) []notificationResponse {
	result := make([]notificationResponse, 0, len(notifications))
	for _, n := range notifications {
		result = append(result, toNotificationResponse(n))
	}
	return result
}

// GetNotificationsByUser godoc
// @Summary Get user notifications
// @Description Retrieves notifications for the authenticated user with pagination and filtering
// @Tags notifications
// @Produce json
// @Param limit query int false "Number of notifications to return (default 20, max 100)"
// @Param offset query int false "Offset for pagination (default 0)"
// @Param status query string false "Filter by status ('read' or 'unread')"
// @Success 200 {array} notificationResponse
// @Failure 400 {object} types.ErrorResponse "Invalid query parameters"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 500 {object} types.ErrorResponse "Internal Server Error"
// @Router /notifications [get]
// @Security BearerAuth
func (h *NotificationHandler) GetNotificationsByUser(c *gin.Context) {
	userIDStr := getUserIDFromContext(c)
	if userIDStr == "" {
		_ = c.Error(apperrors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		_ = c.Error(apperrors.InternalServerError("invalid user ID format"))
		return
	}

	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")
	statusStr := c.Query("status") // optional

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 20 // Default/fallback limit
		h.logger.Warn("Invalid limit query parameter, using default", zap.String("providedLimit", limitStr))
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0 // Default/fallback offset
		h.logger.Warn("Invalid offset query parameter, using default", zap.String("providedOffset", offsetStr))
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
			_ = c.Error(apperrors.ValidationFailed("invalid_status", "status must be 'read' or 'unread'"))
			return
		}
	}

	notifications, err := h.notificationService.GetNotifications(c.Request.Context(), userID, limit, offset, status)
	if err != nil {
		_ = c.Error(apperrors.InternalServerError("failed to retrieve notifications"))
		return
	}

	c.JSON(http.StatusOK, toNotificationResponseList(notifications))
}

// GetUnreadCount godoc
// @Summary Get unread notification count
// @Description Returns the count of unread notifications for the authenticated user
// @Tags notifications
// @Produce json
// @Success 200 {object} map[string]int64 "Returns unread count"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 500 {object} types.ErrorResponse "Internal Server Error"
// @Router /notifications/unread-count [get]
// @Security BearerAuth
func (h *NotificationHandler) GetUnreadCount(c *gin.Context) {
	userIDStr := getUserIDFromContext(c)
	if userIDStr == "" {
		_ = c.Error(apperrors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		_ = c.Error(apperrors.InternalServerError("invalid user ID format"))
		return
	}

	count, err := h.notificationService.GetUnreadNotificationCount(c.Request.Context(), userID)
	if err != nil {
		_ = c.Error(apperrors.InternalServerError("failed to get unread notification count"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"count": count})
}

// MarkNotificationAsRead godoc
// @Summary Mark a notification as read
// @Description Marks a specific notification as read for the authenticated user
// @Tags notifications
// @Accept json
// @Produce json
// @Param notificationId path string true "Notification ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} types.ErrorResponse "Invalid Notification ID"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 403 {object} types.ErrorResponse "Forbidden (Notification does not belong to user)"
// @Failure 404 {object} types.ErrorResponse "Notification Not Found"
// @Failure 500 {object} types.ErrorResponse "Internal Server Error"
// @Router /notifications/{notificationId}/read [patch]
// @Security BearerAuth
func (h *NotificationHandler) MarkNotificationAsRead(c *gin.Context) {
	userIDStr := getUserIDFromContext(c)
	if userIDStr == "" {
		_ = c.Error(apperrors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		_ = c.Error(apperrors.InternalServerError("invalid user ID format"))
		return
	}

	notificationIDStr := c.Param("notificationId")
	notificationID, err := uuid.Parse(notificationIDStr)
	if err != nil {
		_ = c.Error(apperrors.ValidationFailed("invalid_notification_id", "invalid notification ID format"))
		return
	}

	err = h.notificationService.MarkNotificationAsRead(c.Request.Context(), userID, notificationID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			_ = c.Error(apperrors.NotFound("notification", notificationIDStr))
		} else if errors.Is(err, store.ErrForbidden) {
			_ = c.Error(apperrors.Forbidden("not_authorized", "you are not authorized to update this notification"))
		} else {
			_ = c.Error(apperrors.InternalServerError("failed to mark notification as read"))
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
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 500 {object} types.ErrorResponse "Internal Server Error"
// @Router /notifications/read-all [patch]
// @Security BearerAuth
func (h *NotificationHandler) MarkAllNotificationsRead(c *gin.Context) {
	userIDStr := getUserIDFromContext(c)
	if userIDStr == "" {
		_ = c.Error(apperrors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		_ = c.Error(apperrors.InternalServerError("invalid user ID format"))
		return
	}

	affectedRows, err := h.notificationService.MarkAllNotificationsAsRead(c.Request.Context(), userID)
	if err != nil {
		_ = c.Error(apperrors.InternalServerError("failed to mark all notifications as read"))
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
// @Failure 400 {object} types.ErrorResponse "Invalid Notification ID"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 403 {object} types.ErrorResponse "Forbidden (Notification does not belong to user)"
// @Failure 404 {object} types.ErrorResponse "Notification Not Found"
// @Failure 500 {object} types.ErrorResponse "Internal Server Error"
// @Router /notifications/{notificationId} [delete]
// @Security BearerAuth
func (h *NotificationHandler) DeleteNotification(c *gin.Context) {
	userIDStr := getUserIDFromContext(c)
	if userIDStr == "" {
		_ = c.Error(apperrors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		_ = c.Error(apperrors.InternalServerError("invalid user ID format"))
		return
	}

	notificationIDStr := c.Param("notificationId")
	notificationID, err := uuid.Parse(notificationIDStr)
	if err != nil {
		_ = c.Error(apperrors.ValidationFailed("invalid_notification_id", "invalid notification ID format"))
		return
	}

	err = h.notificationService.DeleteNotification(c.Request.Context(), userID, notificationID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			_ = c.Error(apperrors.NotFound("notification", notificationIDStr))
		} else if errors.Is(err, store.ErrForbidden) {
			_ = c.Error(apperrors.Forbidden("not_authorized", "you are not authorized to delete this notification"))
		} else {
			_ = c.Error(apperrors.InternalServerError("failed to delete notification"))
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// DeleteAllNotifications godoc
// @Summary Delete all notifications
// @Description Deletes all notifications for the authenticated user
// @Tags notifications
// @Produce json
// @Success 200 {object} map[string]int64 "Returns the number of notifications deleted"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 500 {object} types.ErrorResponse "Internal Server Error"
// @Router /notifications [delete]
// @Security BearerAuth
func (h *NotificationHandler) DeleteAllNotifications(c *gin.Context) {
	userIDStr := getUserIDFromContext(c)
	if userIDStr == "" {
		_ = c.Error(apperrors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		_ = c.Error(apperrors.InternalServerError("invalid user ID format"))
		return
	}

	deletedCount, err := h.notificationService.DeleteAllNotifications(c.Request.Context(), userID)
	if err != nil {
		_ = c.Error(apperrors.InternalServerError("failed to delete all notifications"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted_count": deletedCount})
}
