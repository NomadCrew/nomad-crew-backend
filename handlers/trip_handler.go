package handlers

import (
    "net/http"
    "time"

    "github.com/NomadCrew/nomad-crew-backend/errors"
    "github.com/NomadCrew/nomad-crew-backend/logger"
    "github.com/NomadCrew/nomad-crew-backend/models"
    "github.com/NomadCrew/nomad-crew-backend/types"
    "github.com/gin-gonic/gin"
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
    StartDate   time.Time `json:"startDate" binding:"required"`
    EndDate     time.Time `json:"endDate" binding:"required"`
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

    // Get user ID from context (set by auth middleware)
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
        CreatedBy:   userID.(string), // Already string UUID from Supabase
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
    tripID := c.Param("id")
    userID := c.GetString("user_id")

    trip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID)
    if err != nil {
        log.Errorw("Failed to get trip", "tripId", tripID, "error", err)
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    // Verify user has access to this trip
    if trip.CreatedBy != userID {
        if err := c.Error(errors.AuthenticationFailed("Not authorized to view this trip")); err != nil {
            log.Errorw("Failed to add authentication error", "error", err)
        }
        return
    }

    c.JSON(http.StatusOK, trip)
}

func (h *TripHandler) UpdateTripHandler(c *gin.Context) {
    log := logger.GetLogger()
    tripID := c.Param("id")
    userID := c.GetString("user_id")

    trip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID)
    if err != nil {
        log.Errorw("Failed to get trip", "tripId", tripID, "error", err)
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    // Verify ownership
    if trip.CreatedBy != userID {
        if err := c.Error(errors.AuthenticationFailed("Not authorized to update this trip")); err != nil {
            log.Errorw("Failed to add authentication error", "error", err)
        }
        return
    }

    var update types.TripUpdate
    if err := c.ShouldBindJSON(&update); err != nil {
        if err := c.Error(errors.ValidationFailed("Invalid request body", err.Error())); err != nil {
            log.Errorw("Failed to add validation error", "error", err)
        }
        return
    }

    if err := h.tripModel.UpdateTrip(c.Request.Context(), tripID, &update); err != nil {
        log.Errorw("Failed to update trip", "tripId", tripID, "error", err)
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Trip updated successfully"})
}

func (h *TripHandler) ListUserTripsHandler(c *gin.Context) {
    log := logger.GetLogger()
    userID := c.GetString("user_id")

    trips, err := h.tripModel.ListUserTrips(c.Request.Context(), userID)
    if err != nil {
        log.Errorw("Failed to list trips", "userId", userID, "error", err)
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    c.JSON(http.StatusOK, trips)
}

func (h *TripHandler) DeleteTripHandler(c *gin.Context) {
    log := logger.GetLogger()
    tripID := c.Param("id")
    userID := c.GetString("user_id")

    trip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID)
    if err != nil {
        log.Errorw("Failed to get trip", "tripId", tripID, "error", err)
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    // Verify ownership
    if trip.CreatedBy != userID {
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