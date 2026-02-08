package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/urmzd/homai/pkg/api/types"
	"github.com/urmzd/homai/pkg/device"
)

// HealthHandler handles health check endpoints
type HealthHandler struct {
	controller device.Controller
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(controller device.Controller) *HealthHandler {
	return &HealthHandler{controller: controller}
}

// Health handles GET /health
// @Summary      Health check
// @Description  Returns the health status of the API and controller
// @Tags         health
// @Produce      json
// @Success      200  {object}  types.HealthResponse  "Service is healthy"
// @Failure      503  {object}  types.HealthResponse  "Service is degraded"
// @Router       /health [get]
func (h *HealthHandler) Health(c *gin.Context) {
	controllerStatus := "disconnected"
	if h.controller.IsConnected() {
		controllerStatus = "connected"
	}

	status := "healthy"
	httpStatus := http.StatusOK

	if controllerStatus != "connected" {
		status = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, types.HealthResponse{
		Status:     status,
		Controller: controllerStatus,
		Timestamp:  time.Now(),
	})
}
