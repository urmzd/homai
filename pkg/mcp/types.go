package mcp

import (
	"encoding/json"

	"github.com/urmzd/homai/pkg/device"
)

// --- Health Tool ---

// GetHealthInput is the input for the get_health tool
type GetHealthInput struct{}

// GetHealthOutput is the output for the get_health tool
type GetHealthOutput struct {
	Status     string `json:"status" jsonschema:"description=Overall health status (healthy or unhealthy)"`
	Controller string `json:"controller" jsonschema:"description=Device controller connection status"`
	Timestamp  string `json:"timestamp" jsonschema:"description=ISO8601 timestamp"`
}

// --- List Devices Tool ---

// ListDevicesInput is the input for the list_devices tool
type ListDevicesInput struct{}

// ListDevicesOutput is the output for the list_devices tool
type ListDevicesOutput struct {
	Devices []DeviceInfo `json:"devices" jsonschema:"description=List of paired devices"`
	Count   int          `json:"count" jsonschema:"description=Total number of devices"`
}

// DeviceInfo represents a device in tool outputs
type DeviceInfo struct {
	ID           string          `json:"id" jsonschema:"description=Unique device identifier (IEEE address)"`
	Name         string          `json:"name" jsonschema:"description=User-friendly device name"`
	Type         string          `json:"type" jsonschema:"description=Device type (light/switch/sensor/coordinator)"`
	Protocol     string          `json:"protocol" jsonschema:"description=Communication protocol"`
	Manufacturer string          `json:"manufacturer,omitempty" jsonschema:"description=Device manufacturer"`
	Model        string          `json:"model,omitempty" jsonschema:"description=Device model"`
	StateSchema  json.RawMessage `json:"state_schema,omitempty" jsonschema:"description=JSON Schema for settable state"`
	State        map[string]any  `json:"state,omitempty" jsonschema:"description=Current device state"`
}

// --- Get Device Tool ---

// GetDeviceInput is the input for the get_device tool
type GetDeviceInput struct {
	ID string `json:"id" jsonschema:"required,description=Device ID (IEEE address) or friendly name"`
}

// GetDeviceOutput is the output for the get_device tool
type GetDeviceOutput struct {
	Device DeviceInfo `json:"device" jsonschema:"description=Device information"`
}

// --- Rename Device Tool ---

// RenameDeviceInput is the input for the rename_device tool
type RenameDeviceInput struct {
	ID      string `json:"id" jsonschema:"required,description=Device ID (IEEE address) or current friendly name"`
	NewName string `json:"new_name" jsonschema:"required,description=New friendly name for the device"`
}

// RenameDeviceOutput is the output for the rename_device tool
type RenameDeviceOutput struct {
	Success bool   `json:"success" jsonschema:"description=Whether the rename succeeded"`
	Message string `json:"message" jsonschema:"description=Status message"`
}

// --- Remove Device Tool ---

// RemoveDeviceInput is the input for the remove_device tool
type RemoveDeviceInput struct {
	ID    string `json:"id" jsonschema:"required,description=Device ID (IEEE address) or friendly name"`
	Force bool   `json:"force,omitempty" jsonschema:"description=Force removal even if device is unavailable"`
}

// RemoveDeviceOutput is the output for the remove_device tool
type RemoveDeviceOutput struct {
	Success bool   `json:"success" jsonschema:"description=Whether the removal succeeded"`
	Message string `json:"message" jsonschema:"description=Status message"`
}

// --- Get Device State Tool ---

// GetDeviceStateInput is the input for the get_device_state tool
type GetDeviceStateInput struct {
	ID string `json:"id" jsonschema:"required,description=Device ID (IEEE address) or friendly name"`
}

// GetDeviceStateOutput is the output for the get_device_state tool
type GetDeviceStateOutput struct {
	DeviceID string         `json:"device_id" jsonschema:"description=Device identifier"`
	State    map[string]any `json:"state" jsonschema:"description=Current device state"`
}

// --- Set Device State Tool ---

// SetDeviceStateInput is the input for the set_device_state tool
type SetDeviceStateInput struct {
	ID    string         `json:"id" jsonschema:"required,description=Device ID (IEEE address) or friendly name"`
	State map[string]any `json:"state" jsonschema:"required,description=State properties to set (validated against device schema)"`
}

// SetDeviceStateOutput is the output for the set_device_state tool
type SetDeviceStateOutput struct {
	DeviceID string         `json:"device_id" jsonschema:"description=Device identifier"`
	State    map[string]any `json:"state" jsonschema:"description=New device state after the change"`
}

// --- Start Discovery Tool ---

// StartDiscoveryInput is the input for the start_discovery tool
type StartDiscoveryInput struct {
	DurationSeconds int `json:"duration_seconds,omitempty" jsonschema:"description=How long to enable pairing mode (default 120 seconds)"`
}

// StartDiscoveryOutput is the output for the start_discovery tool
type StartDiscoveryOutput struct {
	Success         bool   `json:"success" jsonschema:"description=Whether pairing mode was enabled"`
	Message         string `json:"message" jsonschema:"description=Status message"`
	DurationSeconds int    `json:"duration_seconds" jsonschema:"description=Duration pairing mode will be active"`
}

// --- Stop Discovery Tool ---

// StopDiscoveryInput is the input for the stop_discovery tool
type StopDiscoveryInput struct{}

// StopDiscoveryOutput is the output for the stop_discovery tool
type StopDiscoveryOutput struct {
	Success bool   `json:"success" jsonschema:"description=Whether pairing mode was disabled"`
	Message string `json:"message" jsonschema:"description=Status message"`
}

// --- Turn On Tool ---

// TurnOnInput is the input for the turn_on tool
type TurnOnInput struct {
	ID         string `json:"id" jsonschema:"required,description=Device ID (IEEE address) or friendly name"`
	Brightness *int   `json:"brightness,omitempty" jsonschema:"description=Brightness level 0-100 (optional)"`
}

// TurnOnOutput is the output for the turn_on tool
type TurnOnOutput struct {
	DeviceID string         `json:"device_id" jsonschema:"description=Device identifier"`
	State    map[string]any `json:"state" jsonschema:"description=New device state"`
}

// --- Turn Off Tool ---

// TurnOffInput is the input for the turn_off tool
type TurnOffInput struct {
	ID string `json:"id" jsonschema:"required,description=Device ID (IEEE address) or friendly name"`
}

// TurnOffOutput is the output for the turn_off tool
type TurnOffOutput struct {
	DeviceID string         `json:"device_id" jsonschema:"description=Device identifier"`
	State    map[string]any `json:"state" jsonschema:"description=New device state"`
}

// --- Helper conversions ---

// DeviceToInfo converts a device.Device to DeviceInfo
func DeviceToInfo(d *device.Device) DeviceInfo {
	return DeviceInfo{
		ID:           d.ID,
		Name:         d.Name,
		Type:         d.Type,
		Protocol:     d.Protocol,
		Manufacturer: d.Manufacturer,
		Model:        d.Model,
		StateSchema:  d.StateSchema,
	}
}
