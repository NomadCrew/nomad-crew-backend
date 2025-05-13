package handlers

import (
	"net/http"
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	locationService "github.com/NomadCrew/nomad-crew-backend/models/location/service"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// LocationHandler handles location-related API requests
type LocationHandler struct {
	locationService locationService.LocationManagementServiceInterface
}

// NewLocationHandler creates a new LocationHandler
func NewLocationHandler(locService locationService.LocationManagementServiceInterface) *LocationHandler {
	return &LocationHandler{
		locationService: locService,
	}
}

// UpdateLocationHandler godoc
// @Summary Update user location
// @Description Updates the current user's location
// @Tags location
// @Accept json
// @Produce json
// @Param request body types.LocationUpdate true "Location update data"
// @Success 200 {object} types.Location "Updated location"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid location data"
// @Failure 401 {object} map[string]string "Unauthorized - User not logged in"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /locations [put]
// @Security BearerAuth
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
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			c.JSON(appErr.GetHTTPStatus(), gin.H{"error": appErr.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, location)
}

// GetTripMemberLocationsHandler godoc
// @Summary Get trip member locations
// @Description Retrieves the current locations of all members in a trip
// @Tags location
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Success 200 {array} types.MemberLocation "List of member locations"
// @Failure 400 {object} map[string]string "Bad request - Invalid or missing trip ID"
// @Failure 401 {object} map[string]string "Unauthorized - User not logged in"
// @Failure 403 {object} map[string]string "Forbidden - User is not a member of this trip"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /trips/{id}/locations [get]
// @Security BearerAuth
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
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			c.JSON(appErr.GetHTTPStatus(), gin.H{"error": appErr.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
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
