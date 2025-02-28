package handlers

import (
	"net/http"
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/services"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
)

// LocationHandler handles location-related API requests
type LocationHandler struct {
	locationService *services.LocationService
}

// NewLocationHandler creates a new LocationHandler
func NewLocationHandler(locationService *services.LocationService) *LocationHandler {
	return &LocationHandler{
		locationService: locationService,
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
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, location)
}

// GetTripMemberLocationsHandler handles requests to get locations of all members in a trip
func (h *LocationHandler) GetTripMemberLocationsHandler(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")

	if tripID == "" {
		log.Errorw("Trip ID not provided")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Trip ID is required",
		})
		return
	}

	locations, err := h.locationService.GetTripMemberLocations(c.Request.Context(), tripID)
	if err != nil {
		log.Errorw("Failed to get trip member locations", "tripID", tripID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve member locations",
		})
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
