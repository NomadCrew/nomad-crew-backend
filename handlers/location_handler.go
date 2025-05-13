package handlers

import (
	"net/http"
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	locationService "github.com/NomadCrew/nomad-crew-backend/models/location/service"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// LocationHandler handles location-related API requests
type LocationHandler struct {
	locationService locationService.LocationManagementServiceInterface
	logger          *zap.Logger
}

// NewLocationHandler creates a new LocationHandler
func NewLocationHandler(locService locationService.LocationManagementServiceInterface, logger *zap.Logger) *LocationHandler {
	return &LocationHandler{
		locationService: locService,
		logger:          logger,
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
// @Failure 401 {object} docs.ErrorResponse "Unauthorized - User not logged in"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Router /locations [put]
// @Security BearerAuth
// UpdateLocationHandler handles requests to update a user's location
func (h *LocationHandler) UpdateLocationHandler(c *gin.Context) {
	log := logger.GetLogger()

	// Get userID from context
	userID := c.GetString(string(middleware.UserIDKey))
	if userID == "" {
		log.Warn("UpdateLocationHandler: User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID missing from context"})
		return
	}

	// Get trip ID from URL
	tripID := c.Param("id")
	if tripID == "" {
		log.Warn("UpdateLocationHandler: Trip ID missing from URL parameters")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trip ID is required"})
		return
	}

	// Parse request body
	var locationUpdate types.LocationUpdate
	if err := c.ShouldBindJSON(&locationUpdate); err != nil {
		log.Warnw("UpdateLocationHandler: Invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format: " + err.Error()})
		return
	}

	// Update location using the existing interface method signature
	location, err := h.locationService.UpdateLocation(c.Request.Context(), userID, locationUpdate)
	if err != nil {
		log.Errorw("UpdateLocationHandler: Failed to update location", "error", err)
		// Handle different error types
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update location: " + err.Error()})
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
// @Success 200 {object} docs.MemberLocationListResponse "List of member locations"
// @Failure 400 {object} docs.ErrorResponse "Bad request - Invalid or missing trip ID"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} docs.ErrorResponse "Forbidden - User is not a member of this trip"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Router /trips/{id}/locations [get]
// @Security BearerAuth
// GetTripMemberLocationsHandler handles requests to get locations of all members in a trip
func (h *LocationHandler) GetTripMemberLocationsHandler(c *gin.Context) {
	log := logger.GetLogger()

	// Get userID from context
	userID := c.GetString(string(middleware.UserIDKey))
	if userID == "" {
		log.Warn("GetTripMemberLocationsHandler: User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID missing from context"})
		return
	}

	// Get trip ID from URL
	tripID := c.Param("id")
	if tripID == "" {
		log.Warn("GetTripMemberLocationsHandler: Trip ID missing from URL parameters")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trip ID is required"})
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
