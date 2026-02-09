package types

import (
	"encoding/json"
	"time"
)

// --- Request DTOs ---

// StartDiscoveryRequest is the request body for POST /discovery/start
type StartDiscoveryRequest struct {
	DurationSeconds int `json:"duration_seconds"`
}

// RenameDeviceRequest is the request body for PATCH /devices/:id
type RenameDeviceRequest struct {
	FriendlyName string `json:"friendly_name" binding:"required"`
}

// --- Response DTOs ---

// ErrorResponse represents an API error
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// HealthResponse is returned from GET /health
type HealthResponse struct {
	Status     string    `json:"status"`
	Controller string    `json:"controller"`
	Timestamp  time.Time `json:"timestamp"`
}

// ListDevicesResponse is returned from GET /devices
type ListDevicesResponse struct {
	Devices []DeviceWithState `json:"devices"`
	Count   int               `json:"count"`
}

// DeviceWithState combines device info with current state
type DeviceWithState struct {
	IEEEAddress  string          `json:"ieee_address"`
	FriendlyName string          `json:"friendly_name"`
	Model        string          `json:"model,omitempty"`
	Vendor       string          `json:"vendor,omitempty"`
	Type         string          `json:"type"`
	StateSchema  json.RawMessage `json:"state_schema,omitempty"`
	State        map[string]any  `json:"state,omitempty"`
}

// DeviceResponse is returned from GET /devices/:id
type DeviceResponse struct {
	Device DeviceWithState `json:"device"`
}

// StateResponse is returned from GET/POST /devices/:id/state
type StateResponse struct {
	Device    string         `json:"device"`
	State     map[string]any `json:"state"`
	Timestamp time.Time      `json:"timestamp"`
}

// StartDiscoveryResponse is returned from POST /discovery/start
type StartDiscoveryResponse struct {
	Status          string    `json:"status"`
	ExpiresAt       time.Time `json:"expires_at"`
	DurationSeconds int       `json:"duration_seconds"`
}

// StopDiscoveryResponse is returned from POST /discovery/stop
type StopDiscoveryResponse struct {
	Status string `json:"status"`
}
