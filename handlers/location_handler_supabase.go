package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/internal/store"
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
	userStore       store.UserStore
	logger          *zap.SugaredLogger
}

// NewLocationHandlerSupabase creates a new instance of LocationHandlerSupabase
func NewLocationHandlerSupabase(
	tripService TripServiceInterface,
	supabaseService *services.SupabaseService,
	userStore store.UserStore,
) *LocationHandlerSupabase {
	return &LocationHandlerSupabase{
		tripService:     tripService,
		supabaseService: supabaseService,
		userStore:       userStore,
		logger:          logger.GetLogger(),
	}
}

// validateLocationData validates the incoming location update data
func (h *LocationHandlerSupabase) validateLocationData(c *gin.Context, locationUpdate *types.LocationUpdate) bool {
	if err := c.ShouldBindJSON(locationUpdate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid location data: " + err.Error(),
		})
		return false
	}

	// Validate coordinates
	if locationUpdate.Latitude < -90 || locationUpdate.Latitude > 90 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Latitude must be between -90 and 90",
		})
		return false
	}

	if locationUpdate.Longitude < -180 || locationUpdate.Longitude > 180 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Longitude must be between -180 and 180",
		})
		return false
	}

	return true
}

// validateTripAccess validates that the user has access to the trip and returns the trip ID and member data
func (h *LocationHandlerSupabase) validateTripAccess(c *gin.Context, userID string) (string, interface{}, bool) {
	tripID := c.Param("id")
	if tripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Trip ID is required",
		})
		return "", nil, false
	}

	// Verify user is a member of the trip
	member, err := h.tripService.GetTripMember(c.Request.Context(), tripID, userID)
	if err != nil || member == nil || member.DeletedAt != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "You are not an active member of this trip",
		})
		return "", nil, false
	}

	return tripID, member, true
}

// checkTripExists verifies that the trip exists in Supabase and attempts auto-sync if needed
func (h *LocationHandlerSupabase) checkTripExists(c *gin.Context, tripID, userID string, member interface{}) bool {
	tripExists, err := h.supabaseService.CheckTripExists(c.Request.Context(), tripID)
	if err != nil {
		h.logger.Errorw("Failed to check trip existence in Supabase", "error", err, "tripID", tripID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to verify trip status",
		})
		return false
	}

	// If trip doesn't exist in Supabase, attempt immediate sync
	if !tripExists {
		h.logger.Warnw("Trip not found in Supabase, attempting immediate sync",
			"tripID", tripID, "userID", userID)

		// Fetch trip data for sync
		trip, err := h.tripService.GetTripForSync(c.Request.Context(), tripID)
		if err != nil {
			h.logger.Errorw("Failed to fetch trip data for sync", "error", err, "tripID", tripID)
			c.JSON(http.StatusConflict, gin.H{
				"error": "Trip synchronization required. Please retry in a moment.",
				"code":  "TRIP_SYNC_REQUIRED",
			})
			return false
		}

		// Get the user's Supabase ID for the foreign key reference
		var createdBySupabaseID string
		var shouldSync bool

		if trip.CreatedBy != nil && *trip.CreatedBy != "" {
			// Get user data from local store to get the Supabase ID
			if user, err := h.userStore.GetUserByID(c.Request.Context(), *trip.CreatedBy); err == nil && user != nil && user.SupabaseID != "" {
				createdBySupabaseID = user.SupabaseID
				shouldSync = true

				// Also sync user to Supabase to ensure they exist
				userSyncData := services.UserSyncData{
					ID:       user.SupabaseID,
					Email:    user.Email,
					Username: user.Username,
				}

				if err := h.supabaseService.SyncUser(c.Request.Context(), userSyncData); err != nil {
					h.logger.Errorw("Failed to sync user to Supabase before trip sync", "error", err, "userID", *trip.CreatedBy, "supabaseID", user.SupabaseID)
					// Continue with trip sync even if user sync fails
				} else {
					h.logger.Infow("Successfully synced user to Supabase before trip sync", "userID", *trip.CreatedBy, "supabaseID", user.SupabaseID)
				}
			} else {
				h.logger.Errorw("Failed to get user data for trip creator", "error", err, "userID", *trip.CreatedBy, "tripID", tripID)
				c.JSON(http.StatusConflict, gin.H{
					"error": "User data required for trip synchronization. Please ensure user is properly synced.",
					"code":  "USER_SYNC_REQUIRED",
				})
				return false
			}
		} else {
			h.logger.Warnw("Trip has no creator specified, cannot sync to Supabase", "tripID", tripID)
			c.JSON(http.StatusConflict, gin.H{
				"error": "Trip synchronization failed: no creator specified.",
				"code":  "INVALID_TRIP_DATA",
			})
			return false
		}

		if !shouldSync {
			h.logger.Errorw("Cannot sync trip: missing required user data", "tripID", tripID)
			c.JSON(http.StatusConflict, gin.H{
				"error": "Trip synchronization failed: missing user data.",
				"code":  "USER_DATA_MISSING",
			})
			return false
		}

		// Convert Trip to TripSyncData with the correct Supabase user ID
		tripSyncData := services.TripSyncData{
			ID:                   trip.ID,
			Name:                 trip.Name,
			CreatedBy:            createdBySupabaseID, // Use Supabase ID instead of internal ID
			StartDate:            trip.StartDate,
			EndDate:              trip.EndDate,
			DestinationLatitude:  trip.DestinationLatitude,
			DestinationLongitude: trip.DestinationLongitude,
		}

		// Attempt to sync the trip immediately using the fetched trip data
		err = h.supabaseService.SyncTripImmediate(c.Request.Context(), tripSyncData)
		if err != nil {
			h.logger.Errorw("Failed to sync trip to Supabase", "error", err, "tripID", tripID)
			c.JSON(http.StatusConflict, gin.H{
				"error": "Trip synchronization required. Please retry in a moment.",
				"code":  "TRIP_SYNC_REQUIRED",
			})
			return false
		}

		h.logger.Infow("Trip successfully synced to Supabase", "tripID", tripID)
	}

	h.logger.Debugw("Trip existence verified in Supabase", "tripID", tripID)
	return true
}

// processLocationUpdate processes the location update data and sets defaults
func (h *LocationHandlerSupabase) processLocationUpdate(c *gin.Context, locationUpdate *types.LocationUpdate, defaultPrivacy types.LocationPrivacy) (*services.LocationUpdate, bool) {
	// Set default values for optional fields
	privacy := string(defaultPrivacy)
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
			return nil, false
		}
		sharingExpiresIn = time.Duration(secs) * time.Second
		if sharingExpiresIn > 24*time.Hour {
			sharingExpiresIn = 24 * time.Hour
		}
	}

	return &services.LocationUpdate{
		Latitude:         locationUpdate.Latitude,
		Longitude:        locationUpdate.Longitude,
		Accuracy:         float32(locationUpdate.Accuracy),
		SharingEnabled:   isSharingEnabled,
		SharingExpiresIn: sharingExpiresIn,
		Privacy:          privacy,
	}, true
}

// updateLocationInSupabase handles the Supabase location update
func (h *LocationHandlerSupabase) updateLocationInSupabase(c *gin.Context, userID, tripID string, locationUpdate *services.LocationUpdate, isCreate bool) bool {
	locationUpdate.TripID = tripID

	err := h.supabaseService.UpdateLocation(c.Request.Context(), userID, *locationUpdate)
	if err != nil {
		action := "update"
		if isCreate {
			action = "create"
		}
		h.logger.Errorw("Failed to "+action+" location in Supabase", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to " + action + " location",
		})
		return false
	}

	return true
}

// generateLocationResponse creates a standardized location response
func (h *LocationHandlerSupabase) generateLocationResponse(userID, tripID string, originalLocation *types.LocationUpdate, privacy string) gin.H {
	now := time.Now()
	idSuffix := tripID
	if tripID == "" {
		idSuffix = "global"
	}

	response := gin.H{
		"id":            userID + "_" + idSuffix + "_" + now.Format("20060102150405"),
		"user_id":       userID,
		"latitude":      originalLocation.Latitude,
		"longitude":     originalLocation.Longitude,
		"accuracy":      originalLocation.Accuracy,
		"timestamp":     now.Format(time.RFC3339),
		"privacy_level": privacy,
		"created_at":    now.Format(time.RFC3339),
		"updated_at":    now.Format(time.RFC3339),
	}

	if tripID != "" {
		response["trip_id"] = tripID
	}

	return response
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

	tripID, member, ok := h.validateTripAccess(c, userID)
	if !ok {
		return
	}

	if !h.checkTripExists(c, tripID, userID, member) {
		return
	}

	var locationUpdate types.LocationUpdate
	if !h.validateLocationData(c, &locationUpdate) {
		return
	}

	processedLocation, ok := h.processLocationUpdate(c, &locationUpdate, types.LocationPrivacyApproximate)
	if !ok {
		return
	}

	if !h.updateLocationInSupabase(c, userID, tripID, processedLocation, false) {
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

	_, _, ok := h.validateTripAccess(c, userID)
	if !ok {
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

	tripID, member, ok := h.validateTripAccess(c, userID)
	if !ok {
		return
	}

	if !h.checkTripExists(c, tripID, userID, member) {
		return
	}

	var locationUpdate types.LocationUpdate
	if !h.validateLocationData(c, &locationUpdate) {
		return
	}

	processedLocation, ok := h.processLocationUpdate(c, &locationUpdate, types.LocationPrivacyPrecise)
	if !ok {
		return
	}

	if !h.updateLocationInSupabase(c, userID, tripID, processedLocation, true) {
		return
	}

	response := h.generateLocationResponse(userID, tripID, &locationUpdate, processedLocation.Privacy)
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
	if !h.validateLocationData(c, &locationUpdate) {
		return
	}

	processedLocation, ok := h.processLocationUpdate(c, &locationUpdate, types.LocationPrivacyPrecise)
	if !ok {
		return
	}

	// For legacy endpoint, we update location globally (no specific trip)
	if !h.updateLocationInSupabase(c, userID, "", processedLocation, false) {
		return
	}

	response := h.generateLocationResponse(userID, "", &locationUpdate, processedLocation.Privacy)
	c.JSON(http.StatusOK, response)
}
