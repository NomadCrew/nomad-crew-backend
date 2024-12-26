package handlers

import (
    "net/http"
    "strconv"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/NomadCrew/nomad-crew-backend/errors"
    "github.com/NomadCrew/nomad-crew-backend/models"
    "github.com/NomadCrew/nomad-crew-backend/types"
    "github.com/NomadCrew/nomad-crew-backend/logger"
)

type TripHandler struct {
    tripModel *models.TripModel
}

func NewTripHandler(model *models.TripModel) *TripHandler {
    return &TripHandler{tripModel: model}
}

// CreateTripRequest represents the request body for creating a trip
type CreateTripRequest struct {
    Name        string    `json:"name" binding:"required"`
    Description string    `json:"description"`
    Destination string    `json:"destination" binding:"required"`
    StartDate   time.Time `json:"start_date" binding:"required"`
    EndDate     time.Time `json:"end_date" binding:"required"`
}

// UpdateTripRequest represents the request body for updating a trip
type UpdateTripRequest struct {
    Name        string    `json:"name"`
    Description string    `json:"description"`
    Destination string    `json:"destination"`
    StartDate   time.Time `json:"start_date"`
    EndDate     time.Time `json:"end_date"`
}

// SearchTripsRequest represents the request body for searching trips
type SearchTripsRequest struct {
    Destination   string    `json:"destination"`
    StartDateFrom time.Time `json:"start_date_from"`
    StartDateTo   time.Time `json:"start_date_to"`
}

func (h *TripHandler) CreateTripHandler(c *gin.Context) {
    log := logger.GetLogger()
    
    var req CreateTripRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        log.Errorw("Invalid trip creation request", "error", err)
        _ = c.Error(errors.ValidationFailed("Invalid request body", err.Error()))
        return
    }

    // Get user ID from context (assuming it was set by auth middleware)
    userID, exists := c.Get("user_id")
    if !exists {
        _ = c.Error(errors.AuthenticationFailed("User not authenticated"))
        return
    }

    trip := &types.Trip{
        Name:        req.Name,
        Description: req.Description,
        Destination: req.Destination,
        StartDate:   req.StartDate,
        EndDate:     req.EndDate,
        CreatedBy:   userID.(int64),
    }

    if err := h.tripModel.CreateTrip(c.Request.Context(), trip); err != nil {
        log.Errorw("Failed to create trip", "error", err)
        _ = c.Error(err)
        return
    }

    c.JSON(http.StatusCreated, trip)
}

func (h *TripHandler) GetTripHandler(c *gin.Context) {
    log := logger.GetLogger()
    
    tripID, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        _ = c.Error(errors.ValidationFailed("Invalid trip ID", err.Error()))
        return
    }

    trip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID)
    if err != nil {
        log.Errorw("Failed to get trip", "tripId", tripID, "error", err)
        _ = c.Error(err)
        return
    }

    c.JSON(http.StatusOK, trip)
}

func (h *TripHandler) UpdateTripHandler(c *gin.Context) {
    log := logger.GetLogger()
    
    tripID, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        log.Errorw("Invalid trip ID format", "id", c.Param("id"))
        _ = c.Error(errors.ValidationFailed("Invalid trip ID", err.Error()))
        return
    }

    var req UpdateTripRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        log.Errorw("Invalid trip update request", "error", err)
        _ = c.Error(errors.ValidationFailed("Invalid request body", err.Error()))
        return
    }

    // Verify trip ownership
    trip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID)
    if err != nil {
        log.Errorw("Failed to get trip for update", "tripId", tripID, "error", err)
        _ = c.Error(err)
        return
    }

    userID, _ := c.Get("user_id")
    if trip.CreatedBy != userID.(int64) {
        _ = c.Error(errors.AuthenticationFailed("Not authorized to update this trip"))
        return
    }

    update := &types.TripUpdate{
        Name:        req.Name,
        Description: req.Description,
        Destination: req.Destination,
        StartDate:   req.StartDate,
        EndDate:     req.EndDate,
    }

    if err := h.tripModel.UpdateTrip(c.Request.Context(), tripID, update); err != nil {
        log.Errorw("Failed to update trip", "tripId", tripID, "error", err)
        _ = c.Error(err)
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Trip updated successfully"})
}

func (h *TripHandler) DeleteTripHandler(c *gin.Context) {
    log := logger.GetLogger()
    
    tripID, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        log.Errorw("Invalid trip ID format", "id", c.Param("id"))
        _ = c.Error(errors.ValidationFailed("Invalid trip ID", err.Error()))
        return
    }

    // Verify trip ownership
    trip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID)
    if err != nil {
        log.Errorw("Failed to get trip for deletion", "tripId", tripID, "error", err)
        _ = c.Error(err)
        return
    }

    userID, _ := c.Get("user_id")
    if trip.CreatedBy != userID.(int64) {
        _ = c.Error(errors.AuthenticationFailed("Not authorized to delete this trip"))
        return
    }

    if err := h.tripModel.DeleteTrip(c.Request.Context(), tripID); err != nil {
        log.Errorw("Failed to delete trip", "tripId", tripID, "error", err)
        _ = c.Error(err)
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Trip deleted successfully"})
}

// ListUserTripsHandler handles retrieving all trips for the current user
func (h *TripHandler) ListUserTripsHandler(c *gin.Context) {
    log := logger.GetLogger()
    
    userID, exists := c.Get("user_id")
    if !exists {
        log.Error("User ID not found in context")
        c.Error(errors.AuthenticationFailed("User not authenticated"))
        return
    }

    userIDInt, ok := userID.(int64)
    if !ok {
        log.Errorw("Invalid user ID type in context", "userID", userID)
        c.Error(errors.AuthenticationFailed("Invalid user ID"))
        return
    }

    trips, err := h.tripModel.ListUserTrips(c.Request.Context(), userIDInt)
    if err != nil {
        log.Errorw("Failed to list user trips", "userId", userIDInt, "error", err)
        c.Error(err)
        return
    }

    c.JSON(http.StatusOK, trips)
}

// SearchTripsHandler handles searching for trips based on criteria
func (h *TripHandler) SearchTripsHandler(c *gin.Context) {
    log := logger.GetLogger()
    
    var req SearchTripsRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        log.Errorw("Invalid trip search request", "error", err)
        _ = c.Error(errors.ValidationFailed("Invalid request body", err.Error()))
        return
    }

    criteria := types.TripSearchCriteria{
        Destination:   req.Destination,
        StartDateFrom: req.StartDateFrom,
        StartDateTo:   req.StartDateTo,
    }

    trips, err := h.tripModel.SearchTrips(c.Request.Context(), criteria)
    if err != nil {
        log.Errorw("Failed to search trips", "error", err)
        _ = c.Error(err)
        return
    }

    c.JSON(http.StatusOK, trips)
}