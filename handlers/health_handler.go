package handlers

import (
	"net/http"

	"github.com/NomadCrew/nomad-crew-backend/services"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	healthService *services.HealthService
}

func NewHealthHandler(healthService *services.HealthService) *HealthHandler {
	return &HealthHandler{
		healthService: healthService,
	}
}

// LivenessCheck handles kubernetes liveness probe
func (h *HealthHandler) LivenessCheck(c *gin.Context) {
	c.Status(http.StatusOK)
}

// ReadinessCheck handles kubernetes readiness probe
func (h *HealthHandler) ReadinessCheck(c *gin.Context) {
	health := h.healthService.CheckHealth(c.Request.Context())

	if health.Status == types.HealthStatusDown {
		c.JSON(http.StatusServiceUnavailable, health)
		return
	}

	c.JSON(http.StatusOK, health)
}

// DetailedHealth provides detailed health information
func (h *HealthHandler) DetailedHealth(c *gin.Context) {
	health := h.healthService.CheckHealth(c.Request.Context())
	c.JSON(http.StatusOK, health)
}
