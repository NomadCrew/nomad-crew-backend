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
        if err := c.Error(errors.ValidationFailed("Invalid request body", err.Error())); err != nil {
            log.Errorw("Failed to add validation error", "error", err)
        }
        return
    }

    userID, exists := c.Get("user_id")
    if !exists {
        if err := c.Error(errors.AuthenticationFailed("User not authenticated")); err != nil {
            log.Errorw("Failed to add authentication error", "error", err)
        }
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
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    c.JSON(http.StatusCreated, trip)
}

func (h *TripHandler) GetTripHandler(c *gin.Context) {
    log := logger.GetLogger()

    tripID, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        if err := c.Error(errors.ValidationFailed("Invalid trip ID", "Invalid input provided")); err != nil {
            log.Errorw("Failed to add validation error", "error", err)
        }
        return
    }

    trip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID)
    if err != nil {
        log.Errorw("Failed to get trip", "tripId", tripID, "error", err)
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    c.JSON(http.StatusOK, trip)
}

func (h *TripHandler) UpdateTripHandler(c *gin.Context) {
    log := logger.GetLogger()

    tripID, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        log.Errorw("Invalid trip ID format", "id", c.Param("id"))
        if err := c.Error(errors.ValidationFailed("Invalid trip ID", "Invalid input provided")); err != nil {
            log.Errorw("Failed to add validation error", "error", err)
        }
        return
    }

    var req UpdateTripRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        log.Errorw("Invalid trip update request", "error", err)
        if err := c.Error(errors.ValidationFailed("Invalid request body", err.Error())); err != nil {
            log.Errorw("Failed to add validation error", "error", err)
        }
        return
    }

    trip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID)
    if err != nil {
        log.Errorw("Failed to get trip for update", "tripId", tripID, "error", err)
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    userID, _ := c.Get("user_id")
    if trip.CreatedBy != userID.(int64) {
        if err := c.Error(errors.AuthenticationFailed("Not authorized to update this trip")); err != nil {
            log.Errorw("Failed to add authentication error", "error", err)
        }
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
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Trip updated successfully"})
}

func (h *TripHandler) DeleteTripHandler(c *gin.Context) {
    log := logger.GetLogger()

    tripID, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        log.Errorw("Invalid trip ID format", "id", c.Param("id"))
        if err := c.Error(errors.ValidationFailed("Invalid trip ID", "Invalid input provided")); err != nil {
            log.Errorw("Failed to add validation error", "error", err)
        }
        return
    }

    trip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID)
    if err != nil {
        log.Errorw("Failed to get trip for deletion", "tripId", tripID, "error", err)
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    userID, _ := c.Get("user_id")
    if trip.CreatedBy != userID.(int64) {
        if err := c.Error(errors.AuthenticationFailed("Not authorized to delete this trip")); err != nil {
            log.Errorw("Failed to add authentication error", "error", err)
        }
        return
    }

    if err := h.tripModel.DeleteTrip(c.Request.Context(), tripID); err != nil {
        log.Errorw("Failed to delete trip", "tripId", tripID, "error", err)
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Trip deleted successfully"})
}

func (h *TripHandler) ListUserTripsHandler(c *gin.Context) {
    log := logger.GetLogger()

    userID, exists := c.Get("user_id")
    if !exists {
        if err := c.Error(errors.AuthenticationFailed("User not authenticated")); err != nil {
            log.Errorw("Failed to add authentication error", "error", err)
        }
        log.Error("User ID not found in context")
        return
    }

    userIDInt, ok := userID.(int64)
    if !ok {
        if err := c.Error(errors.AuthenticationFailed("Invalid user ID")); err != nil {
            log.Errorw("Failed to add authentication error", "error", err)
        }
        log.Errorw("Invalid user ID type in context", "userID", userID)
        return
    }

    trips, err := h.tripModel.ListUserTrips(c.Request.Context(), userIDInt)
    if err != nil {
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        log.Errorw("Failed to list user trips", "userId", userIDInt, "error", err)
        return
    }

    c.JSON(http.StatusOK, trips)
}

func (h *TripHandler) SearchTripsHandler(c *gin.Context) {
    log := logger.GetLogger()

    var req SearchTripsRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        if err := c.Error(errors.ValidationFailed("Invalid request body", err.Error())); err != nil {
            log.Errorw("Failed to add validation error", "error", err)
        }
        log.Errorw("Invalid trip search request", "error", err)
        return
    }

    criteria := types.TripSearchCriteria{
        Destination:   req.Destination,
        StartDateFrom: req.StartDateFrom,
        StartDateTo:   req.StartDateTo,
    }

    trips, err := h.tripModel.SearchTrips(c.Request.Context(), criteria)
    if err != nil {
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        log.Errorw("Failed to search trips", "error", err)
        return
    }

    c.JSON(http.StatusOK, trips)
}
