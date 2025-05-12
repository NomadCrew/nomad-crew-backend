package handlers

import (
	"net/http"

	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
)

type LocationHandler struct {
	store store.LocationStore
}

func NewLocationHandler(store store.LocationStore) *LocationHandler {
	return &LocationHandler{store: store}
}

// @Summary Update user location
// @Description Updates the current user's location with the provided coordinates
// @Tags location
// @Accept json
// @Produce json
// @Param location body types.LocationUpdate true "Location update information"
// @Success 200 {object} types.Location "Location updated successfully"
// @Failure 400 {object} gin.H "Bad request"
// @Failure 401 {object} gin.H "Unauthorized"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /location [put]
// UpdateLocation handles updating a user's location
func (h *LocationHandler) UpdateLocation(c *gin.Context) {
	userID, exists := c.Get(string(middleware.UserIDKey))
	if !exists || userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userIDStr := userID.(string)

	var update types.LocationUpdate
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	location, err := h.store.UpdateLocation(c.Request.Context(), userIDStr, &update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, location)
}

// @Summary Get trip member locations
// @Description Retrieves the locations of all members in a specific trip
// @Tags location
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Success 200 {array} types.Location "Trip member locations"
// @Failure 400 {object} gin.H "Bad request"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /trips/{id}/locations [get]
// GetTripMemberLocations retrieves all member locations for a trip
func (h *LocationHandler) GetTripMemberLocations(c *gin.Context) {
	tripID := c.Param("id")
	if tripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trip_id is required"})
		return
	}

	locations, err := h.store.ListTripMemberLocations(c.Request.Context(), tripID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, locations)
}
