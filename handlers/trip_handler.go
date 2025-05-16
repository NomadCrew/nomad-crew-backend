package handlers

import (
	"net/http"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/supabase-community/supabase-go"
)

// TripHandler handles HTTP requests related to trips and exposes the trip functionality.
type TripHandler struct {
	tripModel      interfaces.TripModelInterface
	eventService   types.EventPublisher
	supabaseClient *supabase.Client
	serverConfig   *config.ServerConfig
	weatherService types.WeatherServiceInterface
}

// NewTripHandler creates a new TripHandler with the given dependencies.
func NewTripHandler(
	tripModel interfaces.TripModelInterface,
	eventService types.EventPublisher,
	supabaseClient *supabase.Client,
	serverConfig *config.ServerConfig,
	weatherService types.WeatherServiceInterface,
) *TripHandler {
	return &TripHandler{
		tripModel:      tripModel,
		eventService:   eventService,
		supabaseClient: supabaseClient,
		serverConfig:   serverConfig,
		weatherService: weatherService,
	}
}

type UpdateTripStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

// CreateTripRequest represents the request body for creating a trip
type CreateTripRequest struct {
	Name                 string           `json:"name" binding:"required"`
	Description          string           `json:"description,omitempty"`
	DestinationPlaceID   *string          `json:"destinationPlaceId,omitempty"`
	DestinationAddress   *string          `json:"destinationAddress,omitempty"`
	DestinationName      *string          `json:"destinationName,omitempty"`
	DestinationLatitude  float64          `json:"destinationLatitude" binding:"required"`
	DestinationLongitude float64          `json:"destinationLongitude" binding:"required"`
	StartDate            time.Time        `json:"startDate" binding:"required"`
	EndDate              time.Time        `json:"endDate" binding:"required"`
	Status               types.TripStatus `json:"status,omitempty"` // Made omitempty, default will be PLANNING
	BackgroundImageURL   string           `json:"backgroundImageUrl,omitempty"`
}

// CreateTripHandler godoc
// @Summary Create a new trip
// @Description Creates a new trip with the given details
// @Tags trips
// @Accept json
// @Produce json
// @Param request body docs.CreateTripRequest true "Trip creation details"
// @Success 201 {object} docs.TripResponse "Successfully created trip details"
// @Failure 400 {object} docs.ErrorResponse "Bad request - Invalid input data"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized - User not logged in"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Router /trips [post]
// @Security BearerAuth
func (h *TripHandler) CreateTripHandler(c *gin.Context) {
	var req CreateTripRequest // Use the redefined CreateTripRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log := logger.GetLogger()
		log.Errorw("Invalid request for CreateTripHandler", "error", err)
		if bindErr := c.Error(apperrors.ValidationFailed("invalid_request_payload", err.Error())); bindErr != nil {
			log.Errorw("Failed to set error in context for CreateTripHandler", "error", bindErr)
		}
		return
	}

	userIDStr := c.GetString(string(middleware.UserIDKey))

	// Map CreateTripRequest to types.Trip
	tripToCreate := types.Trip{
		Name:                 req.Name,
		Description:          req.Description,
		DestinationPlaceID:   req.DestinationPlaceID,
		DestinationAddress:   req.DestinationAddress,
		DestinationName:      req.DestinationName,
		DestinationLatitude:  req.DestinationLatitude,
		DestinationLongitude: req.DestinationLongitude,
		StartDate:            req.StartDate,
		EndDate:              req.EndDate,
		Status:               req.Status, // Will be 'PLANNING' by default if empty due to omitempty and DB default
		BackgroundImageURL:   req.BackgroundImageURL,
		CreatedBy:            &userIDStr, // Correctly assign pointer
	}

	if tripToCreate.Status == "" { // Explicitly set to PLANNING if not provided by request
		tripToCreate.Status = types.TripStatusPlanning
	}

	createdTrip, err := h.tripModel.CreateTrip(c.Request.Context(), &tripToCreate)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	c.JSON(http.StatusCreated, createdTrip)
}

// GetTripHandler godoc
// @Summary Get trip details
// @Description Retrieves detailed information about a specific trip
// @Tags trips
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Success 200 {object} docs.TripResponse "Trip details"
// @Failure 400 {object} docs.ErrorResponse "Bad request - Invalid trip ID"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} docs.ErrorResponse "Forbidden - User not a member of this trip"
// @Failure 404 {object} docs.ErrorResponse "Not found - Trip not found"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Router /trips/{id} [get]
// @Security BearerAuth
func (h *TripHandler) GetTripHandler(c *gin.Context) {
	tripID := c.Param("id")
	userID := c.GetString(string(middleware.UserIDKey))

	trip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID, userID)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	c.JSON(http.StatusOK, trip)
}

// UpdateTripHandler godoc
// @Summary Update trip details
// @Description Updates specified fields of an existing trip. All fields in the request body are optional.
// @Tags trips
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Param request body docs.TripUpdateRequest true "Fields to update"
// @Success 200 {object} docs.TripResponse "Successfully updated trip details"
// @Failure 400 {object} docs.ErrorResponse "Bad request - Invalid trip ID or update data"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} docs.ErrorResponse "Forbidden - User not authorized to update this trip"
// @Failure 404 {object} docs.ErrorResponse "Not found - Trip not found"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Router /trips/{id} [put]
// @Security BearerAuth
func (h *TripHandler) UpdateTripHandler(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")
	userID := c.GetString(string(middleware.UserIDKey))

	var update types.TripUpdate
	if err := c.ShouldBindJSON(&update); err != nil {
		log.Errorw("Invalid update data", "error", err)
		if err := c.Error(apperrors.ValidationFailed("Invalid update data", err.Error())); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
		return
	}

	updatedTrip, err := h.tripModel.UpdateTrip(c.Request.Context(), tripID, userID, &update)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	c.JSON(http.StatusOK, updatedTrip)
}

// UpdateTripStatusHandler godoc
// @Summary Update trip status
// @Description Updates the status of a specific trip (e.g., PLANNING, ACTIVE, COMPLETED, CANCELLED).
// @Tags trips
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Param request body docs.UpdateTripStatusRequest true "New status for the trip"
// @Success 200 {object} docs.TripStatusUpdateResponse "Successfully updated trip status"
// @Failure 400 {object} docs.ErrorResponse "Bad request - Invalid trip ID or status value"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} docs.ErrorResponse "Forbidden - User not authorized to update this trip's status, or invalid status transition"
// @Failure 404 {object} docs.ErrorResponse "Not found - Trip not found"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Router /trips/{id}/status [patch]
// @Security BearerAuth
// UpdateTripStatusHandler updates trip status using the facade
func (h *TripHandler) UpdateTripStatusHandler(c *gin.Context) {
	log := logger.GetLogger()

	tripID := c.Param("id")

	var req UpdateTripStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw("Invalid status update request", "error", err)
		if err := c.Error(apperrors.ValidationFailed("invalid_request", err.Error())); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
		return
	}

	newStatus := types.TripStatus(req.Status)

	err := h.tripModel.UpdateTripStatus(c.Request.Context(), tripID, newStatus)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	updatedTrip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID, c.GetString(string(middleware.UserIDKey)))
	if err != nil {
		log.Errorw("Failed to fetch updated trip after status change", "tripID", tripID, "error", err)
		c.JSON(http.StatusOK, gin.H{"message": "Trip status updated successfully"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Trip status updated successfully",
		"data":    updatedTrip,
	})
}

func (h *TripHandler) handleModelError(c *gin.Context, err error) {
	log := logger.GetLogger()

	var response types.ErrorResponse
	var statusCode int

	switch e := err.(type) {
	case *apperrors.AppError:
		response.Code = string(e.Type)
		response.Message = e.Message
		response.Error = e.Detail
		statusCode = e.GetHTTPStatus()
	default:
		log.Errorw("Unexpected error", "error", err)
		response.Code = "INTERNAL_ERROR"
		response.Message = "An unexpected error occurred"
		response.Error = "Internal server error"
		statusCode = http.StatusInternalServerError
	}

	c.JSON(statusCode, response)
}

// ListUserTripsHandler godoc
// @Summary List user trips
// @Description Retrieves all trips that the current user is a member of
// @Tags trips
// @Accept json
// @Produce json
// @Success 200 {array} docs.TripResponse "List of user's trips"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized - User not logged in"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Router /trips [get]
// @Security BearerAuth
func (h *TripHandler) ListUserTripsHandler(c *gin.Context) {
	userID := c.GetString(string(middleware.UserIDKey))

	trips, err := h.tripModel.ListUserTrips(c.Request.Context(), userID)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	c.JSON(http.StatusOK, trips)
}

// DeleteTripHandler godoc
// @Summary Delete a trip
// @Description Deletes a specific trip by its ID.
// @Tags trips
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Success 204 "Successfully deleted"
// @Failure 400 {object} docs.ErrorResponse "Bad request - Invalid trip ID format"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} docs.ErrorResponse "Forbidden - User not authorized to delete this trip"
// @Failure 404 {object} docs.ErrorResponse "Not found - Trip not found"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Router /trips/{id} [delete]
// @Security BearerAuth
func (h *TripHandler) DeleteTripHandler(c *gin.Context) {
	tripID := c.Param("id")

	err := h.tripModel.DeleteTrip(c.Request.Context(), tripID)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// SearchTripsHandler godoc
// @Summary Search for trips
// @Description Searches for trips based on specified criteria in the request body. All criteria are optional.
// @Tags trips
// @Accept json
// @Produce json
// @Param request body docs.TripSearchRequest true "Search criteria"
// @Success 200 {array} docs.TripResponse "A list of trips matching the criteria"
// @Failure 400 {object} docs.ErrorResponse "Bad request - Invalid search criteria"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized - User not logged in"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Router /trips/search [post]
// @Security BearerAuth
func (h *TripHandler) SearchTripsHandler(c *gin.Context) {
	log := logger.GetLogger()

	var criteria types.TripSearchCriteria
	if err := c.ShouldBindJSON(&criteria); err != nil {
		if err := c.Error(apperrors.ValidationFailed("Invalid search criteria", err.Error())); err != nil {
			log.Errorw("Failed to add validation error", "error", err)
		}
		return
	}

	trips, err := h.tripModel.SearchTrips(c.Request.Context(), criteria)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	c.JSON(http.StatusOK, trips)
}

// GetTripWithMembersHandler godoc
// @Summary Get trip with members
// @Description Retrieves a trip's details along with its list of members.
// @Tags trips
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Success 200 {object} docs.TripWithMembersResponse "Trip details including members"
// @Failure 400 {object} docs.ErrorResponse "Bad request - Invalid trip ID"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} docs.ErrorResponse "Forbidden - User not authorized to view this trip"
// @Failure 404 {object} docs.ErrorResponse "Not found - Trip not found"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Router /trips/{id}/details [get]
// @Security BearerAuth
func (h *TripHandler) GetTripWithMembersHandler(c *gin.Context) {
	tripID := c.Param("id")
	userID := c.GetString(string(middleware.UserIDKey))

	tripWithMembers, err := h.tripModel.GetTripWithMembers(c.Request.Context(), tripID, userID)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	c.JSON(http.StatusOK, tripWithMembers)
}

// TriggerWeatherUpdateHandler godoc
// @Summary Trigger weather update for a trip
// @Description Manually triggers an immediate weather forecast update for the specified trip if it has a valid destination.
// @Tags trips,weather
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Success 200 {object} docs.SuccessResponse "Successfully triggered weather update"
// @Failure 400 {object} docs.ErrorResponse "Bad request - Invalid trip ID"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} docs.ErrorResponse "Forbidden - User not authorized or trip has no destination"
// @Failure 404 {object} docs.ErrorResponse "Not found - Trip not found"
// @Failure 500 {object} docs.ErrorResponse "Internal server error or weather service error"
// @Router /trips/{id}/weather/trigger [post]
// @Security BearerAuth
func (h *TripHandler) TriggerWeatherUpdateHandler(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")
	userID := c.GetString(string(middleware.UserIDKey))

	// Fetch the trip to ensure it exists and to get destination details
	// Use GetTripByID which includes membership check implicitly via the model layer
	trip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID, userID)
	if err != nil {
		h.handleModelError(c, err) // Handles NotFound, Forbidden, etc.
		return
	}

	if trip.DestinationLatitude == 0 && trip.DestinationLongitude == 0 {
		log.Warnw("Cannot trigger weather update, trip has no valid destination coordinates", "tripID", tripID, "userID", userID)
		if err := c.Error(apperrors.Forbidden("no_destination", "Trip has no destination set for weather updates.")); err != nil {
			log.Errorw("Failed to set error in context for TriggerWeatherUpdateHandler no destination", "error", err)
		}
		return
	}

	if h.weatherService == nil {
		log.Errorw("Weather service not available in TripHandler", "tripID", tripID, "userID", userID)
		if err := c.Error(apperrors.InternalServerError("Weather service is not configured.")); err != nil {
			log.Errorw("Failed to set error in context for TriggerWeatherUpdateHandler weather service unavailable", "error", err)
		}
		return
	}

	log.Infow("Attempting to trigger weather update for trip", "tripID", tripID, "userID", userID, "lat", trip.DestinationLatitude, "lon", trip.DestinationLongitude)

	// Call the weather service to trigger an immediate update
	if err := h.weatherService.TriggerImmediateUpdate(c.Request.Context(), tripID, trip.DestinationLatitude, trip.DestinationLongitude); err != nil {
		log.Errorw("Failed to trigger weather update", "tripID", tripID, "userID", userID, "error", err)
		// Propagate the error to the client, ensuring it's an AppError
		appErr, ok := err.(*apperrors.AppError)
		if !ok {
			// If it's not an AppError, wrap it as an internal server error
			appErr = apperrors.InternalServerError("Failed to trigger weather update due to an unexpected error")
		}
		if bindErr := c.Error(appErr); bindErr != nil { // Use c.Error to let middleware handle it
			log.Errorw("Failed to bind appErr to context in TriggerWeatherUpdateHandler", "error", bindErr, "originalAppError", appErr)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Weather update triggered successfully for trip " + tripID})
}

// UploadTripImage godoc
// @Summary Upload a trip image
// @Description Uploads an image for a specific trip.
// @Tags trips
// @Tags images
// @Accept multipart/form-data
// @Produce json
// @Param id path string true "Trip ID"
// @Param image formData file true "Image file to upload"
// @Success 201 {object} docs.ImageUploadResponse "Image uploaded successfully"
// @Failure 400 {object} docs.ErrorResponse "Bad request - No file, invalid file type/size, or invalid trip ID"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} docs.ErrorResponse "Forbidden - User not authorized to upload images for this trip"
// @Failure 404 {object} docs.ErrorResponse "Not found - Trip not found"
// @Failure 500 {object} docs.ErrorResponse "Internal server error - Upload failed"
// @Router /trips/{id}/images [post]
// @Security BearerAuth
func (h *TripHandler) UploadTripImage(c *gin.Context) {
	// Implementation needed
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented"})
}

// ListTripImages godoc
// @Summary List trip images
// @Description Retrieves a list of images associated with a specific trip.
// @Tags trips
// @Tags images
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Success 200 {array} docs.ImageResponse "List of trip images"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} docs.ErrorResponse "Forbidden - User not authorized to view images for this trip"
// @Failure 404 {object} docs.ErrorResponse "Not found - Trip not found"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Router /trips/{id}/images [get]
// @Security BearerAuth
func (h *TripHandler) ListTripImages(c *gin.Context) {
	// Implementation needed
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented"})
}

// DeleteTripImage godoc
// @Summary Delete a trip image
// @Description Deletes a specific image associated with a trip.
// @Tags trips
// @Tags images
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Param imageId path string true "Image ID to delete"
// @Success 204 "Image deleted successfully"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} docs.ErrorResponse "Forbidden - User not authorized to delete this image"
// @Failure 404 {object} docs.ErrorResponse "Not found - Trip or image not found"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Router /trips/{id}/images/{imageId} [delete]
// @Security BearerAuth
func (h *TripHandler) DeleteTripImage(c *gin.Context) {
	// Implementation needed
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented"})
}
