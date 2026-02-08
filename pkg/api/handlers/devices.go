package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/urmzd/homai/pkg/api/types"
	"github.com/urmzd/homai/pkg/device"
)

// DevicesHandler handles device CRUD endpoints
type DevicesHandler struct {
	controller device.Controller
}

// NewDevicesHandler creates a new devices handler
func NewDevicesHandler(controller device.Controller) *DevicesHandler {
	return &DevicesHandler{controller: controller}
}

// ListDevices handles GET /devices
// @Summary      List all devices
// @Description  Returns a list of all paired devices (excluding coordinator)
// @Tags         devices
// @Produce      json
// @Success      200  {object}  types.ListDevicesResponse
// @Failure      504  {object}  types.ErrorResponse  "Request timed out"
// @Failure      500  {object}  types.ErrorResponse  "Controller error"
// @Router       /devices [get]
func (h *DevicesHandler) ListDevices(c *gin.Context) {
	ctx := c.Request.Context()

	devices, err := h.controller.ListDevices(ctx)
	if err != nil {
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

	// Convert to response format, excluding coordinator
	var result []types.DeviceWithState
	for _, d := range devices {
		if d.Type == device.DeviceTypeCoordinator {
			continue
		}

		dws := types.DeviceWithState{
			IEEEAddress:  d.ID,
			FriendlyName: d.Name,
			Type:         d.Type,
			Model:        d.Model,
			Vendor:       d.Manufacturer,
			StateSchema:  d.StateSchema,
		}

		// Try to get state (non-blocking, ignore errors)
		if state, err := h.controller.GetDeviceState(ctx, d.Name); err == nil {
			dws.State = state
		}

		result = append(result, dws)
	}

	c.JSON(http.StatusOK, types.ListDevicesResponse{
		Devices: result,
		Count:   len(result),
	})
}

// GetDevice handles GET /devices/:id
// @Summary      Get device details
// @Description  Returns details for a specific device by IEEE address or friendly name
// @Tags         devices
// @Produce      json
// @Param        id   path      string  true  "Device IEEE address or friendly name"
// @Success      200  {object}  types.DeviceResponse
// @Failure      404  {object}  types.ErrorResponse  "Device not found"
// @Failure      504  {object}  types.ErrorResponse  "Request timed out"
// @Failure      500  {object}  types.ErrorResponse  "Controller error"
// @Router       /devices/{id} [get]
func (h *DevicesHandler) GetDevice(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

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

	result := types.DeviceWithState{
		IEEEAddress:  d.ID,
		FriendlyName: d.Name,
		Type:         d.Type,
		Model:        d.Model,
		Vendor:       d.Manufacturer,
		StateSchema:  d.StateSchema,
	}

	// Try to get state
	if state, err := h.controller.GetDeviceState(ctx, d.Name); err == nil {
		result.State = state
	}

	c.JSON(http.StatusOK, types.DeviceResponse{
		Device: result,
	})
}

// RenameDevice handles PATCH /devices/:id
// @Summary      Rename a device
// @Description  Changes the friendly name of a device
// @Tags         devices
// @Accept       json
// @Produce      json
// @Param        id       path      string                       true  "Device IEEE address or friendly name"
// @Param        request  body      types.RenameDeviceRequest    true  "New friendly name"
// @Success      200      {object}  types.DeviceResponse
// @Failure      400      {object}  types.ErrorResponse  "Invalid request"
// @Failure      404      {object}  types.ErrorResponse  "Device not found"
// @Failure      504      {object}  types.ErrorResponse  "Request timed out"
// @Failure      500      {object}  types.ErrorResponse  "Controller error"
// @Router       /devices/{id} [patch]
func (h *DevicesHandler) RenameDevice(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	var req types.RenameDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse{
			Error:   "invalid_request",
			Message: "friendly_name is required",
		})
		return
	}

	// Verify device exists
	d, err := h.controller.GetDevice(ctx, id)
	if err != nil {
		if errors.Is(err, device.ErrNotFound) {
			c.JSON(http.StatusNotFound, types.ErrorResponse{
				Error:   "not_found",
				Message: "Device not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Error:   "controller_error",
			Message: err.Error(),
		})
		return
	}

	// Rename device
	if err := h.controller.RenameDevice(ctx, id, req.FriendlyName); err != nil {
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

	// Return updated device info
	result := types.DeviceWithState{
		IEEEAddress:  d.ID,
		FriendlyName: req.FriendlyName,
		Type:         d.Type,
		Model:        d.Model,
		Vendor:       d.Manufacturer,
		StateSchema:  d.StateSchema,
	}

	c.JSON(http.StatusOK, types.DeviceResponse{
		Device: result,
	})
}

// RemoveDevice handles DELETE /devices/:id
// @Summary      Remove a device
// @Description  Removes a device from the network
// @Tags         devices
// @Produce      json
// @Param        id     path   string  true   "Device IEEE address or friendly name"
// @Param        force  query  bool    false  "Force removal even if device is offline"
// @Success      204    "Device removed successfully"
// @Failure      404    {object}  types.ErrorResponse  "Device not found"
// @Failure      504    {object}  types.ErrorResponse  "Request timed out"
// @Failure      500    {object}  types.ErrorResponse  "Controller error"
// @Router       /devices/{id} [delete]
func (h *DevicesHandler) RemoveDevice(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	// Check for force parameter
	force := c.Query("force") == "true"

	// Verify device exists
	_, err := h.controller.GetDevice(ctx, id)
	if err != nil {
		if errors.Is(err, device.ErrNotFound) {
			c.JSON(http.StatusNotFound, types.ErrorResponse{
				Error:   "not_found",
				Message: "Device not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Error:   "controller_error",
			Message: err.Error(),
		})
		return
	}

	// Remove device
	if err := h.controller.RemoveDevice(ctx, id, force); err != nil {
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

	c.Status(http.StatusNoContent)
}
