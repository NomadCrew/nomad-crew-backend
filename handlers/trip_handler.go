package handlers

import (
	"net/http"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
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
	Name        string            `json:"name" binding:"required"`
	Description string            `json:"description"`
	Destination types.Destination `json:"destination" binding:"required"`
	StartDate   time.Time         `json:"startDate" binding:"required"`
	EndDate     time.Time         `json:"endDate" binding:"required"`
	Status      types.TripStatus  `json:"status"`
}

// CreateTripHandler godoc
// @Summary Create a new trip
// @Description Creates a new trip with the given details
// @Tags trips
// @Accept json
// @Produce json
// @Param request body docs.TripResponse true "Trip information"
// @Success 201 {object} docs.TripResponse "Created trip information"
// @Failure 400 {object} docs.ErrorResponse "Bad request - Invalid input data"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized - User not logged in"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Router /trips [post]
// @Security BearerAuth
func (h *TripHandler) CreateTripHandler(c *gin.Context) {
	var req types.Trip
	if err := c.ShouldBindJSON(&req); err != nil {
		log := logger.GetLogger()
		log.Errorw("Invalid request", "error", err)
		if err := c.Error(apperrors.ValidationFailed("invalid_request", err.Error())); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
		return
	}

	req.CreatedBy = c.GetString("user_id")

	createdTrip, err := h.tripModel.CreateTrip(c.Request.Context(), &req)
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
	userID := c.GetString("user_id")

	trip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID, userID)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	c.JSON(http.StatusOK, trip)
}

func (h *TripHandler) UpdateTripHandler(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")
	userID := c.GetString("user_id")

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

	updatedTrip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID, c.GetString("user_id"))
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
	userID := c.GetString("user_id")

	trips, err := h.tripModel.ListUserTrips(c.Request.Context(), userID)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	c.JSON(http.StatusOK, trips)
}

func (h *TripHandler) DeleteTripHandler(c *gin.Context) {
	tripID := c.Param("id")

	err := h.tripModel.DeleteTrip(c.Request.Context(), tripID)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

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

func (h *TripHandler) GetTripWithMembersHandler(c *gin.Context) {
	tripID := c.Param("id")
	userID := c.GetString("user_id")

	tripWithMembers, err := h.tripModel.GetTripWithMembers(c.Request.Context(), tripID, userID)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	c.JSON(http.StatusOK, tripWithMembers)
}

func (h *TripHandler) TriggerWeatherUpdateHandler(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")

	trip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID, c.GetString("user_id"))
	if err != nil {
		log.Errorw("Failed to get trip details for weather update trigger", "tripID", tripID, "error", err)
		h.handleModelError(c, err)
		return
	}

	if h.weatherService == nil {
		log.Errorw("Weather service not configured in handler", "tripID", tripID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Weather service unavailable"})
		return
	}

	log.Infow("Triggering immediate weather update", "tripID", tripID, "destination", trip.Destination.Address)
	if err := h.weatherService.TriggerImmediateUpdate(c.Request.Context(), tripID, trip.Destination); err != nil {
		log.Errorw("Failed to trigger immediate weather update", "tripID", tripID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to trigger weather update"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Weather update triggered"})
}

// UploadTripImage handles uploading an image for a specific trip.
func (h *TripHandler) UploadTripImage(c *gin.Context) {
	log := logger.GetLogger()

	log.Info("Image upload is temporarily disabled")
	c.JSON(http.StatusNotImplemented, gin.H{
		"message": "Image upload functionality is temporarily disabled",
	})
}

// ListTripImages retrieves a list of images associated with a trip.
func (h *TripHandler) ListTripImages(c *gin.Context) {
	log := logger.GetLogger()

	log.Info("Image listing is temporarily disabled")
	c.JSON(http.StatusNotImplemented, gin.H{
		"message": "Image listing functionality is temporarily disabled",
		"images":  []interface{}{},
	})
}

// DeleteTripImage handles deleting an image associated with a trip.
func (h *TripHandler) DeleteTripImage(c *gin.Context) {
	log := logger.GetLogger()

	log.Info("Image deletion is temporarily disabled")
	c.JSON(http.StatusNotImplemented, gin.H{
		"message": "Image deletion functionality is temporarily disabled",
	})
}
