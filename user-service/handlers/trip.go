// handlers/trip.go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/user-service/db"
	"github.com/NomadCrew/nomad-crew-backend/user-service/models"
	"github.com/gin-gonic/gin"
)

type TripHandler struct {
	db *db.TripDB
}

func NewTripHandler(db *db.TripDB) *TripHandler {
    return &TripHandler{db: db}
}

func (h *TripHandler) CreateTripHandler(c *gin.Context) {
    var trip models.Trip
    if err := json.NewDecoder(c.Request.Body).Decode(&trip); err!= nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Validate trip data
    if trip.Name == "" || trip.Destination == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid trip data"})
        return
    }

    // Create trip in database
    ctx := c.Request.Context()
    tripID, err := h.db.CreateTrip(ctx, trip)
    if err!= nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusCreated, gin.H{"trip_id": tripID})
}

func (h *TripHandler) GetTripHandler(c *gin.Context) {
    tripIDStr := c.Param("id")
    tripID, err := strconv.ParseInt(tripIDStr, 10, 64)
    if err!= nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Get trip from database
    ctx := c.Request.Context()
    trip, err := h.db.GetTrip(ctx, tripID)
    if err!= nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Trip not found"})
        return
    }

    c.JSON(http.StatusOK, trip)
}

func (h *TripHandler) UpdateTripHandler(c *gin.Context) {
    tripIDStr := c.Param("id")
    tripID, err := strconv.ParseInt(tripIDStr, 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var tripUpdate models.TripUpdate
    if err := json.NewDecoder(c.Request.Body).Decode(&tripUpdate); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    ctx := c.Request.Context()
    err = h.db.UpdateTrip(ctx, tripID, tripUpdate)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Trip updated successfully"})
}

func (h *TripHandler) DeleteTripHandler(c *gin.Context) {
    tripIDStr := c.Param("id")
    tripID, err := strconv.ParseInt(tripIDStr, 10, 64)
    if err!= nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Delete trip from database
    ctx := c.Request.Context()
    err = h.db.DeleteTrip(ctx, tripID)
    if err!= nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Trip deleted successfully"})
}

type TripUpdate struct {
    Name        string    `json:"name"`
    Description string    `json:"description"`
    Destination string    `json:"destination"`
    StartDate   time.Time `json:"start_date"`
    EndDate     time.Time `json:"end_date"`
}