package handlers

import (
	"net/http"
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	locationService "github.com/NomadCrew/nomad-crew-backend/models/location/service"
	"github.com/NomadCrew/nomad-crew-backend/services"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// LocationHandler handles location-related API requests
type LocationHandler struct {
	locationService locationService.LocationManagementServiceInterface
	tripService     TripServiceInterface
	supabase        *services.SupabaseService
	logger          *zap.Logger
}

// NewLocationHandler returns a new LocationHandler initialized with the provided services and logger.
func NewLocationHandler(
	locService locationService.LocationManagementServiceInterface,
	tripService TripServiceInterface,
	supabase *services.SupabaseService,
	logger *zap.Logger,
) *LocationHandler {
	return &LocationHandler{
		locationService: locService,
		tripService:     tripService,
		supabase:        supabase,
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

// UpdateLocation handles PUT /api/v1/locations
func (h *LocationHandler) UpdateLocationSupabase(c *gin.Context) {
	userID := c.GetString(string(middleware.UserIDKey))

	var req struct {
		TripID           string        `json:"trip_id,omitempty"`
		Latitude         float64       `json:"latitude" binding:"required,min=-90,max=90"`
		Longitude        float64       `json:"longitude" binding:"required,min=-180,max=180"`
		Accuracy         float32       `json:"accuracy" binding:"required,min=0"`
		SharingEnabled   bool          `json:"sharing_enabled"`
		SharingExpiresIn time.Duration `json:"sharing_expires_in,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// If trip specified, verify membership
	if req.TripID != "" {
		member, err := h.tripService.GetTripMember(c.Request.Context(), req.TripID, userID)
		if err != nil || member == nil || member.DeletedAt != nil {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "You are not an active member of this trip",
			})
			return
		}
	}

	// Validate sharing duration
	if req.SharingExpiresIn > 24*time.Hour {
		req.SharingExpiresIn = 24 * time.Hour // Max 24 hours
	}

	err := h.supabase.UpdateLocation(
		c.Request.Context(),
		userID,
		services.LocationUpdate{
			TripID:           req.TripID,
			Latitude:         req.Latitude,
			Longitude:        req.Longitude,
			Accuracy:         req.Accuracy,
			SharingEnabled:   req.SharingEnabled,
			SharingExpiresIn: req.SharingExpiresIn,
		},
	)

	if err != nil {
		h.logger.Error("Failed to update location", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update location",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "updated",
	})
}

// GetTripMemberLocations handles GET /api/v1/trips/:tripID/locations
func (h *LocationHandler) GetTripMemberLocationsSupabase(c *gin.Context) {
	userID := c.GetString(string(middleware.UserIDKey))
	tripID := c.Param("tripID")

	// Verify membership
	member, err := h.tripService.GetTripMember(c.Request.Context(), tripID, userID)
	if err != nil || member == nil || member.DeletedAt != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "You are not an active member of this trip",
		})
		return
	}

	// Locations are filtered by RLS in Supabase
	locations, err := h.supabase.GetTripMemberLocations(
		c.Request.Context(),
		tripID,
	)

	if err != nil {
		h.logger.Error("Failed to fetch locations", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch locations",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"locations": locations,
	})
}
