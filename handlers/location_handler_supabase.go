package handlers

import (
	"net/http"
	"strconv"
	"time"

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
}

// NewLocationHandlerSupabase creates a new instance of LocationHandlerSupabase
func NewLocationHandlerSupabase(
	tripService TripServiceInterface,
	supabaseService *services.SupabaseService,
) *LocationHandlerSupabase {
	return &LocationHandlerSupabase{
		tripService:     tripService,
		supabaseService: supabaseService,
		logger:          logger.GetLogger(),
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
	userID := c.GetString(string(middleware.InternalUserIDKey))
	tripID := c.Param("id")

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
		secs := *locationUpdate.SharingExpiresIn
		if secs <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "sharingExpiresIn must be > 0"})
			return
		}
		sharingExpiresIn = time.Duration(secs) * time.Second
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
}

// GetTripMemberLocations handles requests to get locations of trip members
// @Summary Get trip member locations
// @Description Retrieves the latest locations of all trip members
// @Tags locations
// @Produce json
// @Param tripId path string true "Trip ID"
// @Success 200 {object} LocationsResponse
// @Failure 400 {object} types.ErrorResponse
// @Failure 401 {object} types.ErrorResponse
// @Failure 403 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/trips/{tripId}/locations [get]
func (h *LocationHandlerSupabase) GetTripMemberLocations(c *gin.Context) {
	userID := c.GetString(string(middleware.InternalUserIDKey))
	tripID := c.Param("id")

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

	// Parse pagination parameters
	limit := 50 // default
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	offset := 0 // default
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	// For the Supabase implementation, return proper structure with pagination
	// The actual location data will be retrieved by the client directly from Supabase
	// But we need to return the expected structure to prevent frontend crashes
	response := gin.H{
		"locations": []types.MemberLocation{}, // Empty array of locations
		"pagination": gin.H{
			"has_more": false,  // No more pages since we're returning empty
			"total":    0,      // Total count is 0
			"limit":    limit,  // Echo back the limit parameter
			"offset":   offset, // Echo back the offset parameter
		},
	}

	c.JSON(http.StatusOK, response)
}

// CreateLocation handles location creation requests via POST
// @Summary Create/Post user's location
// @Description Posts the current user's location for a specific trip
// @Tags locations
// @Accept json
// @Produce json
// @Param tripId path string true "Trip ID"
// @Param location body types.LocationUpdate true "Location data"
// @Success 201 {object} types.LocationResponse
// @Failure 400 {object} types.ErrorResponse
// @Failure 401 {object} types.ErrorResponse
// @Failure 403 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/trips/{tripId}/locations [post]
func (h *LocationHandlerSupabase) CreateLocation(c *gin.Context) {
	userID := c.GetString(string(middleware.InternalUserIDKey))
	tripID := c.Param("id")

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

	// Set default values for optional fields
	privacy := string(types.LocationPrivacyPrecise) // Default to precise for frontend compatibility
	if locationUpdate.Privacy != nil {
		privacy = string(*locationUpdate.Privacy)
	}

	isSharingEnabled := true // default to sharing
	if locationUpdate.IsSharingEnabled != nil {
		isSharingEnabled = *locationUpdate.IsSharingEnabled
	}

	var sharingExpiresIn time.Duration
	if locationUpdate.SharingExpiresIn != nil {
		secs := *locationUpdate.SharingExpiresIn
		if secs <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "sharingExpiresIn must be > 0"})
			return
		}
		sharingExpiresIn = time.Duration(secs) * time.Second
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
		h.logger.Errorw("Failed to create location in Supabase", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create location",
		})
		return
	}

	// Return a response structure that matches frontend expectations
	now := time.Now()
	response := gin.H{
		"id":            userID + "_" + tripID + "_" + now.Format("20060102150405"), // Generate a simple ID
		"trip_id":       tripID,
		"user_id":       userID,
		"latitude":      locationUpdate.Latitude,
		"longitude":     locationUpdate.Longitude,
		"accuracy":      locationUpdate.Accuracy,
		"timestamp":     now.Format(time.RFC3339),
		"privacy_level": privacy,
		"created_at":    now.Format(time.RFC3339),
		"updated_at":    now.Format(time.RFC3339),
	}

	c.JSON(http.StatusCreated, response)
}

// LegacyUpdateLocation handles the legacy global location update endpoint
// @Summary Update user's location (legacy endpoint)
// @Description Updates the current user's location globally (legacy compatibility)
// @Tags locations
// @Accept json
// @Produce json
// @Param location body types.LocationUpdate true "Location data"
// @Success 200 {object} types.LocationResponse
// @Failure 400 {object} types.ErrorResponse
// @Failure 401 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/location/update [post]
func (h *LocationHandlerSupabase) LegacyUpdateLocation(c *gin.Context) {
	userID := c.GetString(string(middleware.InternalUserIDKey))

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

	// Set default values for optional fields
	privacy := string(types.LocationPrivacyPrecise) // Default to precise for frontend compatibility
	if locationUpdate.Privacy != nil {
		privacy = string(*locationUpdate.Privacy)
	}

	isSharingEnabled := true // default to sharing
	if locationUpdate.IsSharingEnabled != nil {
		isSharingEnabled = *locationUpdate.IsSharingEnabled
	}

	var sharingExpiresIn time.Duration
	if locationUpdate.SharingExpiresIn != nil {
		secs := *locationUpdate.SharingExpiresIn
		if secs <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "sharingExpiresIn must be > 0"})
			return
		}
		sharingExpiresIn = time.Duration(secs) * time.Second
		if sharingExpiresIn > 24*time.Hour {
			sharingExpiresIn = 24 * time.Hour
		}
	}

	// For legacy endpoint, we update location globally (no specific trip)
	// This is for backward compatibility - location gets updated for all user's active trips
	err := h.supabaseService.UpdateLocation(c.Request.Context(), userID, services.LocationUpdate{
		TripID:           "", // Empty trip ID for global update
		Latitude:         locationUpdate.Latitude,
		Longitude:        locationUpdate.Longitude,
		Accuracy:         float32(locationUpdate.Accuracy),
		SharingEnabled:   isSharingEnabled,
		SharingExpiresIn: sharingExpiresIn,
		Privacy:          privacy,
	})

	if err != nil {
		h.logger.Errorw("Failed to update location in Supabase (legacy)", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update location",
		})
		return
	}

	// Return a response structure that matches frontend expectations
	now := time.Now()
	response := gin.H{
		"id":            userID + "_global_" + now.Format("20060102150405"), // Generate a simple ID
		"user_id":       userID,
		"latitude":      locationUpdate.Latitude,
		"longitude":     locationUpdate.Longitude,
		"accuracy":      locationUpdate.Accuracy,
		"timestamp":     now.Format(time.RFC3339),
		"privacy_level": privacy,
		"created_at":    now.Format(time.RFC3339),
		"updated_at":    now.Format(time.RFC3339),
	}

	c.JSON(http.StatusOK, response)
}
