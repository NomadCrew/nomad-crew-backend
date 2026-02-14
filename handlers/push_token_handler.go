package handlers

import (
	"net/http"

	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/internal/utils"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// PushTokenHandler handles HTTP requests related to push notification tokens.
type PushTokenHandler struct {
	pushTokenStore store.PushTokenStore
	logger         *zap.Logger
}

// NewPushTokenHandler creates a new PushTokenHandler.
func NewPushTokenHandler(pts store.PushTokenStore, logger *zap.Logger) *PushTokenHandler {
	return &PushTokenHandler{
		pushTokenStore: pts,
		logger:         logger.Named("PushTokenHandler"),
	}
}

// RegisterPushToken godoc
// @Summary Register a push notification token
// @Description Registers or updates a push notification token for the authenticated user's device
// @Tags push-tokens
// @Accept json
// @Produce json
// @Param body body types.RegisterPushTokenRequest true "Push token registration request"
// @Success 200 {object} types.PushToken "Token registered successfully"
// @Failure 400 {object} types.ErrorResponse "Invalid request body"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 500 {object} types.ErrorResponse "Internal Server Error"
// @Router /users/push-token [post]
// @Security BearerAuth
func (h *PushTokenHandler) RegisterPushToken(c *gin.Context) {
	userID, err := utils.GetUserIDFromContext(c.Request.Context())
	if err != nil {
		h.logger.Warn("Unauthorized attempt to register push token", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req types.RegisterPushTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid push token registration request",
			zap.String("userID", userID),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// Validate device type
	if req.DeviceType != "ios" && req.DeviceType != "android" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device type. Must be 'ios' or 'android'"})
		return
	}

	pushToken, err := h.pushTokenStore.RegisterToken(c.Request.Context(), userID, req.Token, req.DeviceType)
	if err != nil {
		h.logger.Error("Failed to register push token",
			zap.String("userID", userID),
			zap.String("deviceType", req.DeviceType),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register push token"})
		return
	}

	h.logger.Info("Successfully registered push token",
		zap.String("userID", userID),
		zap.String("deviceType", req.DeviceType))

	c.JSON(http.StatusOK, pushToken)
}

// DeregisterPushToken godoc
// @Summary Deregister a push notification token
// @Description Deactivates a push notification token for the authenticated user (typically on logout)
// @Tags push-tokens
// @Accept json
// @Produce json
// @Param body body types.DeregisterPushTokenRequest true "Push token deregistration request"
// @Success 204 "Token deregistered successfully"
// @Failure 400 {object} types.ErrorResponse "Invalid request body"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 500 {object} types.ErrorResponse "Internal Server Error"
// @Router /users/push-token [delete]
// @Security BearerAuth
func (h *PushTokenHandler) DeregisterPushToken(c *gin.Context) {
	userID, err := utils.GetUserIDFromContext(c.Request.Context())
	if err != nil {
		h.logger.Warn("Unauthorized attempt to deregister push token", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req types.DeregisterPushTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid push token deregistration request",
			zap.String("userID", userID),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	err = h.pushTokenStore.DeactivateToken(c.Request.Context(), userID, req.Token)
	if err != nil {
		h.logger.Error("Failed to deregister push token",
			zap.String("userID", userID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deregister push token"})
		return
	}

	h.logger.Info("Successfully deregistered push token", zap.String("userID", userID))

	c.Status(http.StatusNoContent)
}

// DeregisterAllPushTokens godoc
// @Summary Deregister all push notification tokens
// @Description Deactivates all push notification tokens for the authenticated user (e.g., on account logout from all devices)
// @Tags push-tokens
// @Produce json
// @Success 204 "All tokens deregistered successfully"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 500 {object} types.ErrorResponse "Internal Server Error"
// @Router /users/push-tokens [delete]
// @Security BearerAuth
func (h *PushTokenHandler) DeregisterAllPushTokens(c *gin.Context) {
	userID, err := utils.GetUserIDFromContext(c.Request.Context())
	if err != nil {
		h.logger.Warn("Unauthorized attempt to deregister all push tokens", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	err = h.pushTokenStore.DeactivateAllUserTokens(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to deregister all push tokens",
			zap.String("userID", userID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deregister all push tokens"})
		return
	}

	h.logger.Info("Successfully deregistered all push tokens", zap.String("userID", userID))

	c.Status(http.StatusNoContent)
}
