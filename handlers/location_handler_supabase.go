package handlers

import (
	"net/http"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/services"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// LocationHandlerSupabase handles location-related HTTP requests with Supabase Realtime integration
type LocationHandlerSupabase struct {
	tripService     TripServiceInterface
	supabaseService *services.SupabaseService
	logger          *zap.SugaredLogger
	featureFlags    config.FeatureFlags
}

// NewLocationHandlerSupabase creates a new instance of LocationHandlerSupabase
func NewLocationHandlerSupabase(
	tripService TripServiceInterface,
	supabaseService *services.SupabaseService,
	featureFlags config.FeatureFlags,
) *LocationHandlerSupabase {
	return &LocationHandlerSupabase{
		tripService:     tripService,
		supabaseService: supabaseService,
		logger:          logger.GetLogger(),
		featureFlags:    featureFlags,
	}
}

// UpdateLocation handles location update requests
// @Summary Update user's location
// @Description Updates the current user's location for a specific trip
// @Tags locations
// @Accept json
// @Produce json
// @Param tripId path string true "Trip ID"
// @Param location body types.LocationUpdate true "Location update data"
// @Success 200 {object} types.Response
// @Failure 400 {object} types.ErrorResponse
// @Failure 401 {object} types.ErrorResponse
// @Failure 403 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/trips/{tripId}/locations [put]
func (h *LocationHandlerSupabase) UpdateLocation(c *gin.Context) {
	tripID := c.Param("tripId")
	userID := c.GetString(string(middleware.UserIDKey))

	if tripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Trip ID is required",
		})
		return
	}

	// Verify user is a member of the trip
	member, err := h.tripService.GetTripMember(c.Request.Context(), tripID, userID)
	if err != nil || member == nil || member.DeletedAt != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "You are not an active member of this trip",
		})
		return
	}

	var locationUpdate types.LocationUpdate
	if err := c.ShouldBindJSON(&locationUpdate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid location data: " + err.Error(),
		})
		return
	}

	// Validate coordinates
	if locationUpdate.Latitude < -90 || locationUpdate.Latitude > 90 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Latitude must be between -90 and 90",
		})
		return
	}

	if locationUpdate.Longitude < -180 || locationUpdate.Longitude > 180 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Longitude must be between -180 and 180",
		})
		return
	}

	// Check if we should use Supabase
	if h.featureFlags.EnableSupabaseRealtime && h.supabaseService != nil {
		// Set default values for optional fields
		privacy := string(types.LocationPrivacyApproximate)
		if locationUpdate.Privacy != nil {
			privacy = string(*locationUpdate.Privacy)
		}

		isSharingEnabled := true // default to sharing
		if locationUpdate.IsSharingEnabled != nil {
			isSharingEnabled = *locationUpdate.IsSharingEnabled
		}

		var sharingExpiresIn time.Duration
		if locationUpdate.SharingExpiresIn != nil {
			sharingExpiresIn = time.Duration(*locationUpdate.SharingExpiresIn) * time.Second // Convert seconds to Duration
			// Cap at 24 hours for safety
			if sharingExpiresIn > 24*time.Hour {
				sharingExpiresIn = 24 * time.Hour
			}
		}

		// Send to Supabase
		err = h.supabaseService.UpdateLocation(c.Request.Context(), userID, services.LocationUpdate{
			TripID:           tripID,
			Latitude:         locationUpdate.Latitude,
			Longitude:        locationUpdate.Longitude,
			Accuracy:         float32(locationUpdate.Accuracy),
			SharingEnabled:   isSharingEnabled,
			SharingExpiresIn: sharingExpiresIn,
			Privacy:          privacy,
		})

		if err != nil {
			h.logger.Errorw("Failed to update location in Supabase", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to update location",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "Location updated via Supabase",
		})
		return
	}

	// If Supabase is not enabled, respond with an error
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error": "Location updates via Supabase are not enabled",
	})
}

// GetTripMemberLocations handles requests to get locations of trip members
// @Summary Get trip member locations
// @Description Retrieves the latest locations of all trip members
// @Tags locations
// @Produce json
// @Param tripId path string true "Trip ID"
// @Success 200 {array} types.MemberLocation
// @Failure 400 {object} types.ErrorResponse
// @Failure 401 {object} types.ErrorResponse
// @Failure 403 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/trips/{tripId}/locations [get]
func (h *LocationHandlerSupabase) GetTripMemberLocations(c *gin.Context) {
	tripID := c.Param("tripId")
	userID := c.GetString(string(middleware.UserIDKey))

	if tripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Trip ID is required",
		})
		return
	}

	// Verify user is a member of the trip
	member, err := h.tripService.GetTripMember(c.Request.Context(), tripID, userID)
	if err != nil || member == nil || member.DeletedAt != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "You are not an active member of this trip",
		})
		return
	}

	// For the Supabase implementation, we simply return an empty array
	// The actual location data will be retrieved by the client directly from Supabase
	c.JSON(http.StatusOK, []types.MemberLocation{})
}
