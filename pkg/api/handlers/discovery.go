package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/urmzd/homai/pkg/api/types"
	"github.com/urmzd/homai/pkg/device"
)

// DiscoveryHandler handles device discovery endpoints
type DiscoveryHandler struct {
	controller device.Controller
	subscriber device.EventSubscriber
}

// NewDiscoveryHandler creates a new discovery handler
func NewDiscoveryHandler(controller device.Controller, subscriber device.EventSubscriber) *DiscoveryHandler {
	return &DiscoveryHandler{
		controller: controller,
		subscriber: subscriber,
	}
}

// StartDiscovery handles POST /discovery/start
// @Summary      Start device discovery
// @Description  Enables pairing mode to allow new devices to join
// @Tags         discovery
// @Accept       json
// @Produce      json
// @Param        request  body      types.StartDiscoveryRequest  false  "Discovery duration (default 120 seconds, max 600)"
// @Success      200      {object}  types.StartDiscoveryResponse
// @Failure      400      {object}  types.ErrorResponse  "Invalid duration"
// @Failure      504      {object}  types.ErrorResponse  "Request timed out"
// @Failure      500      {object}  types.ErrorResponse  "Controller error"
// @Router       /discovery/start [post]
func (h *DiscoveryHandler) StartDiscovery(c *gin.Context) {
	ctx := c.Request.Context()

	var req types.StartDiscoveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Use default duration if not provided
		req.DurationSeconds = 120
	}

	if req.DurationSeconds <= 0 {
		req.DurationSeconds = 120
	}

	if req.DurationSeconds > 600 {
		c.JSON(http.StatusBadRequest, types.ErrorResponse{
			Error:   "invalid_duration",
			Message: "Duration cannot exceed 600 seconds",
		})
		return
	}

	if err := h.controller.PermitJoin(ctx, true, req.DurationSeconds); err != nil {
		if errors.Is(err, device.ErrNotConnected) {
			c.JSON(http.StatusServiceUnavailable, types.ErrorResponse{
				Error:   "controller_disconnected",
				Message: err.Error(),
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

	expiresAt := time.Now().Add(time.Duration(req.DurationSeconds) * time.Second)

	c.JSON(http.StatusOK, types.StartDiscoveryResponse{
		Status:          "pairing_enabled",
		ExpiresAt:       expiresAt,
		DurationSeconds: req.DurationSeconds,
	})
}

// StopDiscovery handles POST /discovery/stop
// @Summary      Stop device discovery
// @Description  Disables pairing mode
// @Tags         discovery
// @Produce      json
// @Success      200  {object}  types.StopDiscoveryResponse
// @Failure      504  {object}  types.ErrorResponse  "Request timed out"
// @Failure      500  {object}  types.ErrorResponse  "Controller error"
// @Router       /discovery/stop [post]
func (h *DiscoveryHandler) StopDiscovery(c *gin.Context) {
	ctx := c.Request.Context()

	if err := h.controller.PermitJoin(ctx, false, 0); err != nil {
		if errors.Is(err, device.ErrNotConnected) {
			c.JSON(http.StatusServiceUnavailable, types.ErrorResponse{
				Error:   "controller_disconnected",
				Message: err.Error(),
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

	c.JSON(http.StatusOK, types.StopDiscoveryResponse{
		Status: "pairing_disabled",
	})
}

// Events handles GET /discovery/events (SSE stream)
// @Summary      Subscribe to discovery events
// @Description  Server-Sent Events stream for real-time device join/leave notifications
// @Tags         discovery
// @Produce      text/event-stream
// @Success      200  {string}  string  "SSE event stream"
// @Router       /discovery/events [get]
func (h *DiscoveryHandler) Events(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Subscribe to events
	eventChan := h.subscriber.Subscribe()
	defer h.subscriber.Unsubscribe(eventChan)

	// Send initial connection event
	sendSSEEvent(c.Writer, "connected", map[string]any{
		"timestamp": time.Now(),
		"message":   "Connected to discovery event stream",
	})
	c.Writer.Flush()

	// Get client gone channel
	clientGone := c.Request.Context().Done()

	// Heartbeat ticker
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-clientGone:
			return

		case event, ok := <-eventChan:
			if !ok {
				return
			}
			sendSSEEvent(c.Writer, event.Type, map[string]any{
				"type":      event.Type,
				"device":    event.Device,
				"timestamp": event.Timestamp,
			})
			c.Writer.Flush()

		case <-ticker.C:
			sendSSEEvent(c.Writer, "heartbeat", map[string]any{
				"timestamp": time.Now(),
			})
			c.Writer.Flush()
		}
	}
}

// sendSSEEvent writes an SSE event to the response
func sendSSEEvent(w io.Writer, eventType string, data any) {
	jsonData, _ := json.Marshal(data)
	io.WriteString(w, "event: "+eventType+"\n")
	io.WriteString(w, "data: "+string(jsonData)+"\n\n")
}
