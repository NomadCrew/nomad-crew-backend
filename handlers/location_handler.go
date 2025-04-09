package handlers

import (
	"context"
	"net/http"
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	locationService "github.com/NomadCrew/nomad-crew-backend/models/location/service"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
)

// LocationHandler handles location-related API requests
type LocationHandler struct {
	locationService *locationService.ManagementService
}

// NewLocationHandler creates a new LocationHandler
func NewLocationHandler(locService *locationService.ManagementService) *LocationHandler {
	return &LocationHandler{
		locationService: locService,
	}
}

// UpdateLocationHandler handles requests to update a user's location
func (h *LocationHandler) UpdateLocationHandler(c *gin.Context) {
	log := logger.GetLogger()
	userID := c.GetString("user_id")

	if userID == "" {
		log.Errorw("User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	var locationUpdate types.LocationUpdate
	if err := c.ShouldBindJSON(&locationUpdate); err != nil {
		log.Errorw("Invalid location update request", "error", err)
		if err := c.Error(apperrors.ValidationFailed("invalid_request", err.Error())); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
		return
	}

	location, err := h.locationService.UpdateLocation(c.Request.Context(), userID, locationUpdate)
	if err != nil {
		log.Errorw("Failed to update location", "userID", userID, "error", err)
		if appErr, ok := err.(*apperrors.AppError); ok {
			c.JSON(appErr.HTTPStatus, gin.H{"error": appErr.Message, "details": appErr.Detail})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update location"})
		}
		return
	}

	c.JSON(http.StatusOK, location)
}

// SaveOfflineLocationsHandler handles requests to save offline location updates
func (h *LocationHandler) SaveOfflineLocationsHandler(c *gin.Context) {
	log := logger.GetLogger()
	userID := c.GetString("user_id")

	if userID == "" {
		log.Errorw("User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	// Get device ID from request
	deviceID := c.GetHeader("X-Device-ID")
	if deviceID == "" {
		deviceID = "unknown"
	}

	// Parse request body
	var request struct {
		Updates []types.LocationUpdate `json:"updates" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		log.Errorw("Invalid offline location request", "error", err)
		if err := c.Error(apperrors.ValidationFailed("invalid_request", err.Error())); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
		return
	}

	if len(request.Updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No location updates provided",
		})
		return
	}

	// Save the offline location updates
	err := h.locationService.SaveOfflineLocations(c.Request.Context(), userID, request.Updates, deviceID)
	if err != nil {
		log.Errorw("Failed to save offline locations", "userID", userID, "error", err)
		if appErr, ok := err.(*apperrors.AppError); ok {
			c.JSON(appErr.HTTPStatus, gin.H{"error": appErr.Message, "details": appErr.Detail})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save offline locations"})
		}
		return
	}

	// Trigger processing of offline locations in the background
	go func() {
		// Create a new context since the request context will be canceled
		ctx := context.Background()
		if err := h.locationService.ProcessOfflineLocations(ctx, userID); err != nil {
			log.Errorw("Failed to process offline locations", "userID", userID, "error", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "Offline locations saved successfully",
		"count":   len(request.Updates),
	})
}

// ProcessOfflineLocationsHandler handles requests to process offline location updates
func (h *LocationHandler) ProcessOfflineLocationsHandler(c *gin.Context) {
	log := logger.GetLogger()
	userID := c.GetString("user_id")

	if userID == "" {
		log.Errorw("User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	// Process the offline location updates
	err := h.locationService.ProcessOfflineLocations(c.Request.Context(), userID)
	if err != nil {
		log.Errorw("Failed to process offline locations", "userID", userID, "error", err)
		if appErr, ok := err.(*apperrors.AppError); ok {
			c.JSON(appErr.HTTPStatus, gin.H{"error": appErr.Message, "details": appErr.Detail})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process offline locations"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Offline locations processed successfully",
	})
}

// GetTripMemberLocationsHandler handles requests to get locations of all members in a trip
func (h *LocationHandler) GetTripMemberLocationsHandler(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")
	userID := c.GetString("user_id")

	if tripID == "" {
		log.Errorw("Trip ID not provided")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Trip ID is required",
		})
		return
	}

	if userID == "" {
		log.Errorw("User ID not found in context for GetTripMemberLocationsHandler")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	locations, err := h.locationService.GetTripMemberLocations(c.Request.Context(), tripID, userID)
	if err != nil {
		log.Errorw("Failed to get trip member locations", "tripID", tripID, "userID", userID, "error", err)
		if appErr, ok := err.(*apperrors.AppError); ok {
			c.JSON(appErr.HTTPStatus, gin.H{"error": appErr.Message, "details": appErr.Detail})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve member locations"})
		}
		return
	}

	// Filter out locations older than 24 hours
	var recentLocations []types.MemberLocation
	cutoff := time.Now().Add(-24 * time.Hour)

	for _, loc := range locations {
		if loc.Timestamp.After(cutoff) {
			recentLocations = append(recentLocations, loc)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"locations": recentLocations,
	})
}
