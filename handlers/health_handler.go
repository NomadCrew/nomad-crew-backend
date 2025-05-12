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

// LivenessCheck godoc
// @Summary Kubernetes liveness probe
// @Description Simple endpoint that returns 200 OK if the service is running
// @Tags health
// @Produce json
// @Success 200 "Service is alive"
// @Router /health/liveness [get]
func (h *HealthHandler) LivenessCheck(c *gin.Context) {
	c.Status(http.StatusOK)
}

// ReadinessCheck godoc
// @Summary Kubernetes readiness probe
// @Description Checks if the service is ready to accept requests (DB and Redis connections working)
// @Tags health
// @Produce json
// @Success 200 {object} types.HealthCheck "Service is ready"
// @Failure 503 {object} types.HealthCheck "Service is not ready"
// @Router /health/readiness [get]
func (h *HealthHandler) ReadinessCheck(c *gin.Context) {
	health := h.healthService.CheckHealth(c.Request.Context())

	if health.Status == types.HealthStatusDown {
		c.JSON(http.StatusServiceUnavailable, health)
		return
	}

	c.JSON(http.StatusOK, health)
}

// DetailedHealth godoc
// @Summary Detailed health check
// @Description Provides detailed health information about all service dependencies
// @Tags health
// @Produce json
// @Success 200 {object} types.HealthCheck "Detailed health information"
// @Router /health [get]
func (h *HealthHandler) DetailedHealth(c *gin.Context) {
	health := h.healthService.CheckHealth(c.Request.Context())
	c.JSON(http.StatusOK, health)
}
