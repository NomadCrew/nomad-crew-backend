package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	userservice "github.com/NomadCrew/nomad-crew-backend/models/user/service"
	"github.com/NomadCrew/nomad-crew-backend/pkg/pexels"
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
	userService    userservice.UserServiceInterface
	pexelsClient   *pexels.Client
}

// NewTripHandler creates a new TripHandler with the given dependencies.
func NewTripHandler(
	tripModel interfaces.TripModelInterface,
	eventService types.EventPublisher,
	supabaseClient *supabase.Client,
	serverConfig *config.ServerConfig,
	weatherService types.WeatherServiceInterface,
	userService userservice.UserServiceInterface,
	pexelsClient *pexels.Client,
) *TripHandler {
	return &TripHandler{
		tripModel:      tripModel,
		eventService:   eventService,
		supabaseClient: supabaseClient,
		serverConfig:   serverConfig,
		weatherService: weatherService,
		userService:    userService,
		pexelsClient:   pexelsClient,
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

// TripWithMembersAndInvitationsResponse is the response for trip creation
// matching the FE's expected shape
// (Consider moving to a shared types or docs file if reused)
type TripWithMembersAndInvitationsResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Destination struct {
		Address     string `json:"address"`
		Coordinates struct {
			Lat float64 `json:"lat"`
			Lng float64 `json:"lng"`
		} `json:"coordinates"`
		PlaceID string `json:"placeId"`
		Name    string `json:"name,omitempty"`
	} `json:"destination"`
	StartDate          time.Time               `json:"startDate"`
	EndDate            time.Time               `json:"endDate"`
	Status             string                  `json:"status"`
	CreatedBy          string                  `json:"createdBy"`
	CreatedAt          time.Time               `json:"createdAt"`
	UpdatedAt          time.Time               `json:"updatedAt"`
	BackgroundImageURL string                  `json:"backgroundImageUrl"`
	Members            []*types.TripMembership `json:"members"`
	Invitations        []*types.TripInvitation `json:"invitations"`
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
	log := logger.GetLogger()
	log.Infow("Received CreateTrip request")

	var req CreateTripRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw("Invalid request for CreateTripHandler", "error", err)
		if bindErr := c.Error(apperrors.ValidationFailed("invalid_request_payload", err.Error())); bindErr != nil {
			log.Errorw("Failed to set error in context for CreateTripHandler", "error", bindErr)
		}
		return
	}

	// Extract the authenticated user ID from Gin context
	userID := c.GetString(string(middleware.UserIDKey))
	log.Infow("[DEBUG] User ID from context", "userID", userID)
	if userID == "" {
		log.Errorw("No user ID found in context for CreateTripHandler")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No authenticated user"})
		return
	}

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
		Status:               req.Status, // Will default to PLANNING if empty via logic below
		BackgroundImageURL:   req.BackgroundImageURL,
		CreatedBy:            &userID,
	}
	log.Infow("[DEBUG] Trip to be created", "tripToCreate", tripToCreate)

	if tripToCreate.Status == "" {
		tripToCreate.Status = types.TripStatusPlanning
	}

	// Fetch background image from Pexels if not provided by frontend
	if tripToCreate.BackgroundImageURL == "" && h.pexelsClient != nil {
		if imageURL := h.fetchBackgroundImage(c.Request.Context(), &tripToCreate); imageURL != "" {
			tripToCreate.BackgroundImageURL = imageURL
			log.Infow("Successfully fetched background image from Pexels", "imageURL", imageURL, "tripName", tripToCreate.Name)
		}
	}

	createdTrip, err := h.tripModel.CreateTrip(c.Request.Context(), &tripToCreate)
	if err != nil {
		log.Errorw("Failed to create trip", "error", err, "tripToCreate", tripToCreate)
		h.handleModelError(c, err)
		return
	}
	log.Infow("Successfully created trip", "trip", createdTrip)

	// Fetch members (should include the creator as owner)
	membersRaw, err := h.tripModel.GetTripMembers(c.Request.Context(), createdTrip.ID)
	if err != nil {
		log.Errorw("Failed to fetch trip members after creation", "tripID", createdTrip.ID, "error", err)
		membersRaw = []types.TripMembership{} // fallback to empty
	}
	members := make([]*types.TripMembership, len(membersRaw))
	for i := range membersRaw {
		members[i] = &membersRaw[i]
	}

	// Fetch invitations (if any)
	invitations := []*types.TripInvitation{}
	if h.tripModel != nil {
		if invGetter, ok := h.tripModel.(interface {
			GetInvitationsByTripID(ctx context.Context, tripID string) ([]*types.TripInvitation, error)
		}); ok {
			invitations, _ = invGetter.GetInvitationsByTripID(c.Request.Context(), createdTrip.ID)
		}
	}

	// Compose destination object
	dest := struct {
		Address     string `json:"address"`
		Coordinates struct {
			Lat float64 `json:"lat"`
			Lng float64 `json:"lng"`
		} `json:"coordinates"`
		PlaceID string `json:"placeId"`
		Name    string `json:"name,omitempty"`
	}{}
	dest.Address = ""
	if createdTrip.DestinationAddress != nil {
		dest.Address = *createdTrip.DestinationAddress
	}
	dest.Coordinates.Lat = createdTrip.DestinationLatitude
	dest.Coordinates.Lng = createdTrip.DestinationLongitude
	dest.PlaceID = ""
	if createdTrip.DestinationPlaceID != nil {
		dest.PlaceID = *createdTrip.DestinationPlaceID
	}
	dest.Name = ""
	if createdTrip.DestinationName != nil {
		dest.Name = *createdTrip.DestinationName
	}

	resp := TripWithMembersAndInvitationsResponse{
		ID:                 createdTrip.ID,
		Name:               createdTrip.Name,
		Description:        createdTrip.Description,
		Destination:        dest,
		StartDate:          createdTrip.StartDate,
		EndDate:            createdTrip.EndDate,
		Status:             string(createdTrip.Status),
		CreatedBy:          derefString(createdTrip.CreatedBy),
		CreatedAt:          createdTrip.CreatedAt,
		UpdatedAt:          createdTrip.UpdatedAt,
		BackgroundImageURL: createdTrip.BackgroundImageURL,
		Members:            members,
		Invitations:        invitations,
	}

	c.JSON(http.StatusCreated, resp)
}

// derefString returns the value of a *string or "" if nil
func derefString(s *string) string {
	if s != nil {
		return *s
	}
	return ""
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
	// Use internal user ID from enhanced middleware
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
	// Use internal user ID from enhanced middleware
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

	// Use internal user ID from enhanced middleware
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
	log := logger.GetLogger()
	// Use Supabase user ID for querying trips (consistent with trip creation)
	supabaseUserID := c.GetString(string(middleware.UserIDKey))

	if supabaseUserID == "" {
		log.Errorw("No Supabase user ID found in context for ListUserTripsHandler")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No authenticated user"})
		return
	}

	log.Infow("Listing trips for user", "supabaseUserID", supabaseUserID)

	trips, err := h.tripModel.ListUserTrips(c.Request.Context(), supabaseUserID) // Use supabaseUserID for consistency with created_by field
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
	// Use internal user ID from enhanced middleware
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
	// Use internal user ID from enhanced middleware
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

// fetchBackgroundImage attempts to fetch a background image from Pexels based on trip destination
// Returns empty string if no image is found or if an error occurs (non-blocking)
func (h *TripHandler) fetchBackgroundImage(ctx context.Context, trip *types.Trip) string {
	log := logger.GetLogger()

	// Build search query from destination information
	searchQuery := pexels.BuildSearchQuery(trip)
	if searchQuery == "" {
		log.Debugw("No suitable search query could be built for Pexels", "tripName", trip.Name)
		return ""
	}

	log.Infow("Attempting to fetch background image from Pexels", "searchQuery", searchQuery, "tripName", trip.Name)

	// Fetch image URL from Pexels (with timeout context)
	imageURL, err := h.pexelsClient.SearchDestinationImage(ctx, searchQuery)
	if err != nil {
		log.Warnw("Failed to fetch background image from Pexels", "error", err, "searchQuery", searchQuery, "tripName", trip.Name)
		return ""
	}

	if imageURL == "" {
		log.Infow("No background image found on Pexels", "searchQuery", searchQuery, "tripName", trip.Name)
		return ""
	}

	return imageURL
}
