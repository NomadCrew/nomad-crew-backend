package handlers

import (
	"net/http"

	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
)

// Deprecated: LocationHandler in internal/handlers is not used and should not be used.
// This handler bypasses the service layer and has NO authorization checks.
// Use handlers.LocationHandlerSupabase instead, which is properly routed and secured.
// WARNING: GetTripMemberLocations has NO trip membership verification - security gap.
// TODO(phase-12): Remove this handler after confirming it's not imported anywhere.
type LocationHandler struct {
	store store.LocationStore
}

func NewLocationHandler(store store.LocationStore) *LocationHandler {
	return &LocationHandler{store: store}
}

// Deprecated: UpdateLocation is not routed and bypasses service layer.
// Use handlers.LocationHandlerSupabase.UpdateLocation instead.
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

// Deprecated: GetTripMemberLocations is not routed, bypasses service layer, and has NO authorization.
// WARNING: This method allows ANY authenticated user to query ANY trip's locations - SECURITY GAP.
// Use handlers.LocationHandlerSupabase.GetTripMemberLocations instead.
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
