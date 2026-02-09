package device

import "context"

// Controller defines the interface for controlling smart home devices.
// This abstraction allows the API to work with different protocols
// (Zigbee, Z-Wave, Matter, WiFi) through a unified interface.
type Controller interface {
	// ListDevices returns all paired devices
	ListDevices(ctx context.Context) ([]Device, error)

	// GetDevice returns a single device by ID
	GetDevice(ctx context.Context, id string) (*Device, error)

	// RenameDevice changes a device's friendly name
	RenameDevice(ctx context.Context, id, newName string) error

	// RemoveDevice removes a device from the network
	RemoveDevice(ctx context.Context, id string, force bool) error

	// GetDeviceState retrieves the current state of a device
	GetDeviceState(ctx context.Context, id string) (DeviceState, error)

	// SetDeviceState sets the state of a device
	SetDeviceState(ctx context.Context, id string, state map[string]any) (DeviceState, error)

	// PermitJoin enables or disables device pairing mode
	PermitJoin(ctx context.Context, enable bool, duration int) error

	// IsConnected returns true if the controller is connected
	IsConnected() bool

	// Close disconnects the controller
	Close()
}

// EventSubscriber defines the interface for subscribing to device events
type EventSubscriber interface {
	// Subscribe returns a channel that receives discovery events
	Subscribe() chan DiscoveryEvent

	// Unsubscribe removes a subscription
	Unsubscribe(ch chan DiscoveryEvent)
}
