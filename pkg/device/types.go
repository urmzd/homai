package device

import (
	"encoding/json"
	"time"
)

// Device represents a protocol-agnostic smart home device
type Device struct {
	ID           string          `json:"id"`           // Unique identifier (e.g., IEEE address for Zigbee)
	Name         string          `json:"name"`         // User-friendly name
	Type         string          `json:"type"`         // Device type (light, switch, sensor, etc.)
	Protocol     string          `json:"protocol"`     // Protocol (zigbee, zwave, matter, wifi)
	Manufacturer string          `json:"manufacturer"` // Device manufacturer/vendor
	Model        string          `json:"model"`        // Device model
	StateSchema  json.RawMessage `json:"state_schema"` // JSON Schema for settable state
	Exposes      json.RawMessage `json:"exposes"`      // Raw protocol capability data
}

// DeviceState represents the current state of a device as a dynamic map.
type DeviceState map[string]any

// DiscoveryEvent represents a device discovery event
type DiscoveryEvent struct {
	Type      string    `json:"type"`             // Event type (device_joined, device_left, etc.)
	Device    *Device   `json:"device,omitempty"` // Device information if available
	Timestamp time.Time `json:"timestamp"`        // When the event occurred
}

// Protocol constants
const (
	ProtocolZigbee = "zigbee"
	ProtocolZWave  = "zwave"
	ProtocolMatter = "matter"
	ProtocolWiFi   = "wifi"
)

// Device type constants
const (
	DeviceTypeLight       = "light"
	DeviceTypeSwitch      = "switch"
	DeviceTypeSensor      = "sensor"
	DeviceTypeThermostat  = "thermostat"
	DeviceTypeLock        = "lock"
	DeviceTypeCoordinator = "coordinator"
)
