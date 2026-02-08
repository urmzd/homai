package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/urmzd/homai/pkg/api/types"
	"github.com/urmzd/homai/pkg/device"
	"github.com/urmzd/homai/pkg/device/schema"
)

// ControlHandler handles device state control endpoints
type ControlHandler struct {
	controller device.Controller
	validator  *schema.Validator
}

// NewControlHandler creates a new control handler
func NewControlHandler(controller device.Controller, validator *schema.Validator) *ControlHandler {
	return &ControlHandler{controller: controller, validator: validator}
}

// GetState handles GET /devices/:id/state
// @Summary      Get device state
// @Description  Returns the current state of a device
// @Tags         devices
// @Produce      json
// @Param        id   path      string  true  "Device IEEE address or friendly name"
// @Success      200  {object}  types.StateResponse
// @Failure      404  {object}  types.ErrorResponse  "Device not found"
// @Failure      504  {object}  types.ErrorResponse  "Request timed out"
// @Failure      500  {object}  types.ErrorResponse  "Device error"
// @Router       /devices/{id}/state [get]
func (h *ControlHandler) GetState(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	// Verify device exists and get its name
	d, err := h.controller.GetDevice(ctx, id)
	if err != nil {
		if errors.Is(err, device.ErrNotFound) {
			c.JSON(http.StatusNotFound, types.ErrorResponse{
				Error:   "not_found",
				Message: "Device not found",
			})
			return
		}
		if errors.Is(err, device.ErrTimeout) {
			c.JSON(http.StatusGatewayTimeout, types.ErrorResponse{
				Error:   "timeout",
				Message: "Request timed out waiting for controller response",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Error:   "controller_error",
			Message: err.Error(),
		})
		return
	}

	// Get state
	state, err := h.controller.GetDeviceState(ctx, d.Name)
	if err != nil {
		if errors.Is(err, device.ErrTimeout) {
			c.JSON(http.StatusGatewayTimeout, types.ErrorResponse{
				Error:   "timeout",
				Message: "Request timed out waiting for device response",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Error:   "device_error",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.StateResponse{
		Device:    d.Name,
		State:     state,
		Timestamp: time.Now(),
	})
}

// SetState handles POST /devices/:id/state
// @Summary      Set device state
// @Description  Sets the state of a device using a free-form JSON object validated against the device's schema
// @Tags         devices
// @Accept       json
// @Produce      json
// @Param        id       path      string  true  "Device IEEE address or friendly name"
// @Param        request  body      object  true  "State to set"
// @Success      200      {object}  types.StateResponse
// @Failure      400      {object}  types.ErrorResponse  "Invalid request"
// @Failure      404      {object}  types.ErrorResponse  "Device not found"
// @Failure      504      {object}  types.ErrorResponse  "Request timed out"
// @Failure      500      {object}  types.ErrorResponse  "Device error"
// @Router       /devices/{id}/state [post]
func (h *ControlHandler) SetState(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	var req map[string]any
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid request body",
		})
		return
	}

	// Verify device exists and get its info
	d, err := h.controller.GetDevice(ctx, id)
	if err != nil {
		if errors.Is(err, device.ErrNotFound) {
			c.JSON(http.StatusNotFound, types.ErrorResponse{
				Error:   "not_found",
				Message: "Device not found",
			})
			return
		}
		if errors.Is(err, device.ErrTimeout) {
			c.JSON(http.StatusGatewayTimeout, types.ErrorResponse{
				Error:   "timeout",
				Message: "Request timed out waiting for controller response",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Error:   "controller_error",
			Message: err.Error(),
		})
		return
	}

	// Validate against device schema
	if err := h.validator.Validate(d.StateSchema, req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	// Set state
	state, err := h.controller.SetDeviceState(ctx, d.Name, req)
	if err != nil {
		if errors.Is(err, device.ErrTimeout) {
			c.JSON(http.StatusGatewayTimeout, types.ErrorResponse{
				Error:   "timeout",
				Message: "Request timed out waiting for device response",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Error:   "device_error",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.StateResponse{
		Device:    d.Name,
		State:     state,
		Timestamp: time.Now(),
	})
}
