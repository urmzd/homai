package device

import "context"

// NullController is a no-op controller used when Zigbee2MQTT is unavailable.
// It allows the API to run in limited mode without a Zigbee adapter.
type NullController struct{}

// NewNullController creates a new NullController.
func NewNullController() *NullController {
	return &NullController{}
}

func (c *NullController) ListDevices(ctx context.Context) ([]Device, error) {
	return []Device{}, nil
}

func (c *NullController) GetDevice(ctx context.Context, id string) (*Device, error) {
	return nil, ErrNotFound
}

func (c *NullController) RenameDevice(ctx context.Context, id, newName string) error {
	return ErrNotConnected
}

func (c *NullController) RemoveDevice(ctx context.Context, id string, force bool) error {
	return ErrNotConnected
}

func (c *NullController) GetDeviceState(ctx context.Context, id string) (DeviceState, error) {
	return nil, ErrNotConnected
}

func (c *NullController) SetDeviceState(ctx context.Context, id string, state map[string]any) (DeviceState, error) {
	return nil, ErrNotConnected
}

func (c *NullController) PermitJoin(ctx context.Context, enable bool, duration int) error {
	return ErrNotConnected
}

func (c *NullController) IsConnected() bool {
	return false
}

func (c *NullController) Close() {}

// NullEventSubscriber is a no-op event subscriber used when Zigbee2MQTT is unavailable.
type NullEventSubscriber struct{}

// NewNullEventSubscriber creates a new NullEventSubscriber.
func NewNullEventSubscriber() *NullEventSubscriber {
	return &NullEventSubscriber{}
}

func (s *NullEventSubscriber) Subscribe() chan DiscoveryEvent {
	ch := make(chan DiscoveryEvent)
	// Channel is never sent to; callers should check IsConnected() on the controller
	return ch
}

func (s *NullEventSubscriber) Unsubscribe(ch chan DiscoveryEvent) {
	close(ch)
}
