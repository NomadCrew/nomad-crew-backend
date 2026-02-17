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
	pexelsClient   pexels.ClientInterface
}

// NewTripHandler creates a new TripHandler with the given dependencies.
func NewTripHandler(
	tripModel interfaces.TripModelInterface,
	eventService types.EventPublisher,
	supabaseClient *supabase.Client,
	serverConfig *config.ServerConfig,
	weatherService types.WeatherServiceInterface,
	userService userservice.UserServiceInterface,
	pexelsClient pexels.ClientInterface,
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

// DestinationResponse is the destination object in trip responses
type DestinationResponse struct {
	Address     string `json:"address"`
	Coordinates struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	} `json:"coordinates"`
	PlaceID string `json:"placeId"`
	Name    string `json:"name,omitempty"`
}

// TripWithMembersAndInvitationsResponse is the response for trip creation
// matching the FE's expected shape
type TripWithMembersAndInvitationsResponse struct {
	ID                 string                  `json:"id"`
	Name               string                  `json:"name"`
	Description        string                  `json:"description"`
	Destination        DestinationResponse     `json:"destination"`
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
// @Param request body types.CreateTripRequest true "Trip creation details"
// @Success 201 {object} types.TripResponse "Successfully created trip details"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid input data"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips [post]
// @Security BearerAuth
func (h *TripHandler) CreateTripHandler(c *gin.Context) {
	log := logger.GetLogger()

	var req CreateTripRequest
	if !bindJSONOrError(c, &req) {
		return
	}

	userID := getUserIDFromContext(c)
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
	log.Infow("Successfully created trip", "tripID", createdTrip.ID)

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

	resp := buildTripWithMembersResponse(createdTrip, members, invitations)
	c.JSON(http.StatusCreated, resp)
}

// derefString returns the value of a *string or "" if nil
func derefString(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

// buildDestinationResponse builds the destination response object from a Trip
func buildDestinationResponse(trip *types.Trip) DestinationResponse {
	dest := DestinationResponse{}
	dest.Address = derefString(trip.DestinationAddress)
	dest.Coordinates.Lat = trip.DestinationLatitude
	dest.Coordinates.Lng = trip.DestinationLongitude
	dest.PlaceID = derefString(trip.DestinationPlaceID)
	dest.Name = derefString(trip.DestinationName)
	return dest
}

// buildTripWithMembersResponse builds the full trip response with members and invitations
func buildTripWithMembersResponse(
	trip *types.Trip,
	members []*types.TripMembership,
	invitations []*types.TripInvitation,
) TripWithMembersAndInvitationsResponse {
	return TripWithMembersAndInvitationsResponse{
		ID:                 trip.ID,
		Name:               trip.Name,
		Description:        trip.Description,
		Destination:        buildDestinationResponse(trip),
		StartDate:          trip.StartDate,
		EndDate:            trip.EndDate,
		Status:             string(trip.Status),
		CreatedBy:          derefString(trip.CreatedBy),
		CreatedAt:          trip.CreatedAt,
		UpdatedAt:          trip.UpdatedAt,
		BackgroundImageURL: trip.BackgroundImageURL,
		Members:            members,
		Invitations:        invitations,
	}
}

// getUserIDFromContext extracts the authenticated user ID from the Gin context.
// Returns empty string if not found (caller should handle unauthorized response).
func getUserIDFromContext(c *gin.Context) string {
	return c.GetString(string(middleware.UserIDKey))
}

// bindJSONOrError binds JSON request body and sets validation error if binding fails.
// Returns true if binding succeeded, false if error was set (caller should return).
func bindJSONOrError(c *gin.Context, obj interface{}) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		_ = c.Error(apperrors.ValidationFailed("invalid_request_payload", err.Error()))
		return false
	}
	return true
}

// GetTripHandler godoc
// @Summary Get trip details
// @Description Retrieves detailed information about a specific trip
// @Tags trips
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Success 200 {object} types.TripResponse "Trip details"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid trip ID"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User not a member of this trip"
// @Failure 404 {object} types.ErrorResponse "Not found - Trip not found"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips/{id} [get]
// @Security BearerAuth
func (h *TripHandler) GetTripHandler(c *gin.Context) {
	tripID := c.Param("id")
	userID := getUserIDFromContext(c)

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
// @Param request body types.TripUpdateRequest true "Fields to update"
// @Success 200 {object} types.TripResponse "Successfully updated trip details"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid trip ID or update data"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User not authorized to update this trip"
// @Failure 404 {object} types.ErrorResponse "Not found - Trip not found"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips/{id} [put]
// @Security BearerAuth
func (h *TripHandler) UpdateTripHandler(c *gin.Context) {
	tripID := c.Param("id")
	userID := getUserIDFromContext(c)

	var update types.TripUpdate
	if !bindJSONOrError(c, &update) {
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
// @Param request body types.UpdateTripStatusRequest true "New status for the trip"
// @Success 200 {object} types.TripResponse "Successfully updated trip status"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid trip ID or status value"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User not authorized to update this trip's status, or invalid status transition"
// @Failure 404 {object} types.ErrorResponse "Not found - Trip not found"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips/{id}/status [patch]
// @Security BearerAuth
// UpdateTripStatusHandler updates trip status using the facade
func (h *TripHandler) UpdateTripStatusHandler(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")

	var req UpdateTripStatusRequest
	if !bindJSONOrError(c, &req) {
		return
	}

	newStatus := types.TripStatus(req.Status)

	err := h.tripModel.UpdateTripStatus(c.Request.Context(), tripID, newStatus)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	updatedTrip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID, getUserIDFromContext(c))
	if err != nil {
		log.Errorw("Failed to fetch updated trip after status change", "tripID", tripID, "error", err)
		c.JSON(http.StatusOK, gin.H{"message": "Trip status updated successfully"})
		return
	}

	c.JSON(http.StatusOK, updatedTrip)
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
// @Success 200 {array} types.TripResponse "List of user's trips"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips [get]
// @Security BearerAuth
func (h *TripHandler) ListUserTripsHandler(c *gin.Context) {
	log := logger.GetLogger()
	userID := getUserIDFromContext(c)
	if userID == "" {
		log.Errorw("No user ID found in context for ListUserTripsHandler")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No authenticated user"})
		return
	}

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
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid trip ID format"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User not authorized to delete this trip"
// @Failure 404 {object} types.ErrorResponse "Not found - Trip not found"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
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
// @Param request body types.TripSearchRequest true "Search criteria"
// @Success 200 {array} types.TripResponse "A list of trips matching the criteria"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid search criteria"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips/search [post]
// @Security BearerAuth
func (h *TripHandler) SearchTripsHandler(c *gin.Context) {
	var criteria types.TripSearchCriteria
	if !bindJSONOrError(c, &criteria) {
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
// @Success 200 {object} types.TripWithMembersResponse "Trip details including members"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid trip ID"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User not authorized to view this trip"
// @Failure 404 {object} types.ErrorResponse "Not found - Trip not found"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips/{id}/details [get]
// @Security BearerAuth
func (h *TripHandler) GetTripWithMembersHandler(c *gin.Context) {
	tripID := c.Param("id")
	userID := getUserIDFromContext(c)

	tripWithMembers, err := h.tripModel.GetTripWithMembers(c.Request.Context(), tripID, userID)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	c.JSON(http.StatusOK, tripWithMembers)
}

// TriggerWeatherUpdateHandler godoc
// @Summary Trigger weather update for a trip
// @Description Fetches and returns fresh weather data for the specified trip's destination.
// @Tags trips,weather
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Success 200 {object} types.WeatherInfo "Current weather information"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid trip ID"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User not authorized or trip has no destination"
// @Failure 404 {object} types.ErrorResponse "Not found - Trip not found"
// @Failure 500 {object} types.ErrorResponse "Internal server error or weather service error"
// @Router /trips/{id}/weather/trigger [post]
// @Security BearerAuth
func (h *TripHandler) TriggerWeatherUpdateHandler(c *gin.Context) {
	// Delegates to GetWeatherHandler — both now do the same thing (on-demand fetch)
	h.GetWeatherHandler(c)
}

// GetWeatherHandler godoc
// @Summary Get current weather for a trip
// @Description Returns the current weather for the specified trip's destination via the Open-Meteo API.
// @Tags trips,weather
// @Produce json
// @Param id path string true "Trip ID"
// @Success 200 {object} types.WeatherInfo "Current weather information"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid trip ID"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User not authorized or trip has no destination"
// @Failure 404 {object} types.ErrorResponse "Not found - Trip not found"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips/{id}/weather [get]
// @Security BearerAuth
func (h *TripHandler) GetWeatherHandler(c *gin.Context) {
	tripID := c.Param("id")
	userID := getUserIDFromContext(c)

	if h.weatherService == nil {
		_ = c.Error(apperrors.InternalServerError("Weather service is not configured."))
		return
	}

	trip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID, userID)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	if trip.DestinationLatitude == 0 && trip.DestinationLongitude == 0 {
		_ = c.Error(apperrors.Forbidden("no_destination", "Trip has no destination set for weather updates."))
		return
	}

	weather, err := h.weatherService.GetWeather(c.Request.Context(), tripID, trip.DestinationLatitude, trip.DestinationLongitude)
	if err != nil {
		appErr, ok := err.(*apperrors.AppError)
		if !ok {
			appErr = apperrors.InternalServerError("Failed to fetch weather data")
		}
		_ = c.Error(appErr)
		return
	}

	c.JSON(http.StatusOK, weather)
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
// @Success 201 {object} types.ImageUploadResponse "Image uploaded successfully"
// @Failure 400 {object} types.ErrorResponse "Bad request - No file, invalid file type/size, or invalid trip ID"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User not authorized to upload images for this trip"
// @Failure 404 {object} types.ErrorResponse "Not found - Trip not found"
// @Failure 500 {object} types.ErrorResponse "Internal server error - Upload failed"
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
// @Success 200 {array} types.ImageResponse "List of trip images"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User not authorized to view images for this trip"
// @Failure 404 {object} types.ErrorResponse "Not found - Trip not found"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
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
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User not authorized to delete this image"
// @Failure 404 {object} types.ErrorResponse "Not found - Trip or image not found"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
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
