package zigbee

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/urmzd/homai/pkg/device"
)

// KnownDevice tracks a Zigbee device discovered on the network.
type KnownDevice struct {
	IEEEAddress [8]byte
	NodeID      uint16
	DeviceType  string
	Endpoint    uint8
	State       device.DeviceState
}

// Controller implements device.Controller and device.EventSubscriber
// for direct EZSP communication with a Sonoff Zigbee dongle.
type Controller struct {
	serial *SerialPort
	ash    *ASHLayer
	ezsp   *EZSPLayer

	devices   map[string]*KnownDevice // IEEE hex string -> device
	devicesMu sync.RWMutex

	subscribers   []chan device.DiscoveryEvent
	subscribersMu sync.Mutex

	connected bool
	connMu    sync.RWMutex

	stopChan chan struct{}
}

// NewController creates and initializes a Zigbee EZSP controller.
func NewController(portPath string) (*Controller, error) {
	log.Info().Str("port", portPath).Msg("Initializing Zigbee controller")
	s, err := OpenSerial(portPath)
	if err != nil {
		return nil, fmt.Errorf("open serial: %w", err)
	}

	ash := NewASHLayer(s)
	ezsp := NewEZSPLayer(ash)

	c := &Controller{
		serial:   s,
		ash:      ash,
		ezsp:     ezsp,
		devices:  make(map[string]*KnownDevice),
		stopChan: make(chan struct{}),
	}

	// Set up callback handler
	ezsp.SetCallbackHandler(c.handleCallback)

	// Connect ASH layer
	log.Info().Msg("Connecting ASH layer")
	if err := ash.Connect(); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("ASH connect: %w", err)
	}

	// Start EZSP processing
	log.Info().Msg("Starting EZSP processing")
	ezsp.Start()

	// Initialize the EZSP stack
	log.Info().Msg("Initializing EZSP stack")
	if err := c.initStack(); err != nil {
		c.Close()
		return nil, fmt.Errorf("init stack: %w", err)
	}

	c.connMu.Lock()
	c.connected = true
	c.connMu.Unlock()

	log.Info().Msg("Zigbee EZSP controller initialized")

	return c, nil
}

// initStack performs EZSP version negotiation, stack configuration, and network setup.
func (c *Controller) initStack() error {
	// Negotiate EZSP version
	log.Info().Msg("Negotiating EZSP version")
	proto, _, stackVer, err := c.ezsp.NegotiateVersion()
	if err != nil {
		return err
	}
	log.Info().Uint8("protocol", proto).Uint16("stack", stackVer).Msg("EZSP version OK")

	// Configure stack
	log.Info().Msg("Configuring EZSP stack")
	if err := c.ezsp.ConfigureStack(); err != nil {
		return err
	}

	// Try to resume existing network
	log.Info().Msg("Initializing Zigbee network")
	status, err := c.ezsp.NetworkInit()
	if err != nil {
		return err
	}

	if status == emberSuccess || status == emberNetworkUp {
		log.Info().Msg("Resumed existing Zigbee network")
		return nil
	}

	log.Info().Uint8("status", status).Msg("No existing network, forming new one")

	// Form a new network
	channel := uint8(15)
	panID := uint16(rand.Intn(0xFFFE) + 1)
	var extPanID [8]byte
	for i := range extPanID {
		extPanID[i] = byte(rand.Intn(256))
	}

	if err := c.ezsp.FormNetwork(channel, panID, extPanID); err != nil {
		return fmt.Errorf("form network: %w", err)
	}

	// Wait briefly for network to come up
	time.Sleep(500 * time.Millisecond)

	return nil
}

// handleCallback processes async EZSP callbacks from the NCP.
func (c *Controller) handleCallback(frameID uint16, data []byte) {
	switch frameID {
	case ezspTrustCenterJoinHandler:
		c.handleTrustCenterJoin(data)
	case ezspIncomingMessageHandler:
		c.handleIncomingMessage(data)
	case ezspStackStatusHandler:
		c.handleStackStatus(data)
	default:
		log.Debug().Uint16("frameID", frameID).Msg("Unhandled EZSP callback")
	}
}

// handleTrustCenterJoin processes device join/leave events.
func (c *Controller) handleTrustCenterJoin(data []byte) {
	if len(data) < 11 {
		return
	}

	nodeID := binary.LittleEndian.Uint16(data[0:2])
	var ieee [8]byte
	copy(ieee[:], data[2:10])
	status := data[10]

	ieeeStr := formatIEEE(ieee)

	log.Info().
		Str("ieee", ieeeStr).
		Uint16("nodeID", nodeID).
		Uint8("status", status).
		Msg("Trust center join event")

	// Status 1 = STANDARD_SECURITY_SECURED_REJOIN or new join
	// Status 3 = DEVICE_LEFT
	if status == 3 {
		c.devicesMu.Lock()
		delete(c.devices, ieeeStr)
		c.devicesMu.Unlock()

		c.publishEvent(device.DiscoveryEvent{
			Type:      "device_left",
			Timestamp: time.Now(),
			Device:    &device.Device{ID: ieeeStr},
		})
		return
	}

	// New device joined
	kd := &KnownDevice{
		IEEEAddress: ieee,
		NodeID:      nodeID,
		DeviceType:  device.DeviceTypeLight, // default assumption
		Endpoint:    1,                      // most HA devices use endpoint 1
		State:       make(device.DeviceState),
	}

	c.devicesMu.Lock()
	c.devices[ieeeStr] = kd
	c.devicesMu.Unlock()

	dev := c.knownToDevice(ieeeStr, kd)
	c.publishEvent(device.DiscoveryEvent{
		Type:      "device_joined",
		Device:    &dev,
		Timestamp: time.Now(),
	})
}

// handleIncomingMessage processes incoming ZCL messages from devices.
func (c *Controller) handleIncomingMessage(data []byte) {
	// Parse the incoming message callback structure
	// type(1) + apsFrame(12) + lastHopLqi(1) + lastHopRssi(1) + sender(2) + bindingIndex(1) + addressIndex(1) + messageLength(1) + message(N)
	if len(data) < 19 {
		return
	}

	// Extract APS frame fields
	// profileID := binary.LittleEndian.Uint16(data[1:3])
	clusterID := binary.LittleEndian.Uint16(data[3:5])
	// srcEndpoint := data[5]
	// dstEndpoint := data[6]

	sender := binary.LittleEndian.Uint16(data[14:16])
	msgLen := data[18]

	if len(data) < 19+int(msgLen) {
		return
	}

	message := data[19 : 19+int(msgLen)]

	log.Debug().
		Uint16("cluster", clusterID).
		Uint16("sender", sender).
		Int("msgLen", int(msgLen)).
		Msg("Incoming ZCL message")

	// Try to find device by nodeID and update state
	c.devicesMu.Lock()
	for _, kd := range c.devices {
		if kd.NodeID == sender {
			c.updateDeviceStateFromZCL(kd, clusterID, message)
			break
		}
	}
	c.devicesMu.Unlock()
}

// updateDeviceStateFromZCL updates device state based on ZCL message content.
func (c *Controller) updateDeviceStateFromZCL(kd *KnownDevice, clusterID uint16, message []byte) {
	if len(message) < 3 {
		return
	}

	frameControl := message[0]
	// seq := message[1]
	cmdID := message[2]
	payload := message[3:]

	isGlobal := frameControl&0x01 == 0

	if isGlobal && cmdID == zclGlobalReadAttributesResponse {
		attrs := ParseReadAttributesResponse(payload)
		switch clusterID {
		case zclClusterOnOff:
			if val, ok := attrs[zclAttrOnOff]; ok && len(val) > 0 {
				kd.State["state"] = boolToOnOff(val[0] != 0)
			}
		case zclClusterLevelControl:
			if val, ok := attrs[zclAttrCurrentLevel]; ok && len(val) > 0 {
				kd.State["brightness"] = int(val[0])
			}
		}
	}
}

// handleStackStatus processes stack status changes.
func (c *Controller) handleStackStatus(data []byte) {
	if len(data) < 1 {
		return
	}
	status := data[0]
	switch status {
	case emberNetworkUp:
		log.Info().Msg("Stack status: network up")
	case emberNetworkDown:
		log.Warn().Msg("Stack status: network down")
	default:
		log.Info().Uint8("status", status).Msg("Stack status changed")
	}
}

// publishEvent sends a discovery event to all subscribers.
func (c *Controller) publishEvent(evt device.DiscoveryEvent) {
	c.subscribersMu.Lock()
	defer c.subscribersMu.Unlock()

	for _, ch := range c.subscribers {
		select {
		case ch <- evt:
		default:
		}
	}
}

// knownToDevice converts a KnownDevice to a device.Device.
func (c *Controller) knownToDevice(ieeeStr string, kd *KnownDevice) device.Device {
	stateSchema, _ := json.Marshal(lightStateSchema())
	return device.Device{
		ID:           ieeeStr,
		Name:         ieeeStr,
		Type:         kd.DeviceType,
		Protocol:     device.ProtocolZigbee,
		Manufacturer: "Unknown",
		Model:        "Unknown",
		StateSchema:  stateSchema,
	}
}

// lightStateSchema returns a basic JSON schema for light devices.
func lightStateSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"state": map[string]any{
				"type": "string",
				"enum": []string{"ON", "OFF", "TOGGLE"},
			},
			"brightness": map[string]any{
				"type":    "integer",
				"minimum": 0,
				"maximum": 254,
			},
		},
	}
}

// --- device.Controller interface ---

func (c *Controller) ListDevices(_ context.Context) ([]device.Device, error) {
	c.devicesMu.RLock()
	defer c.devicesMu.RUnlock()

	devices := make([]device.Device, 0, len(c.devices))
	for ieee, kd := range c.devices {
		devices = append(devices, c.knownToDevice(ieee, kd))
	}
	return devices, nil
}

func (c *Controller) GetDevice(_ context.Context, id string) (*device.Device, error) {
	c.devicesMu.RLock()
	defer c.devicesMu.RUnlock()

	kd, ok := c.devices[id]
	if !ok {
		// Also search by name
		for ieee, d := range c.devices {
			if ieee == id {
				kd = d
				ok = true
				id = ieee
				break
			}
		}
		if !ok {
			return nil, device.ErrNotFound
		}
	}

	dev := c.knownToDevice(id, kd)
	return &dev, nil
}

func (c *Controller) RenameDevice(_ context.Context, id, newName string) error {
	// Zigbee doesn't have a native rename; we could store names locally.
	// For now, this is unsupported.
	return device.ErrUnsupported
}

func (c *Controller) RemoveDevice(_ context.Context, id string, force bool) error {
	c.devicesMu.Lock()
	_, ok := c.devices[id]
	if !ok {
		c.devicesMu.Unlock()
		return device.ErrNotFound
	}
	delete(c.devices, id)
	c.devicesMu.Unlock()

	// TODO: send ZDO Leave request to the device
	return nil
}

func (c *Controller) GetDeviceState(_ context.Context, id string) (device.DeviceState, error) {
	c.devicesMu.RLock()
	kd, ok := c.devices[id]
	c.devicesMu.RUnlock()

	if !ok {
		return nil, device.ErrNotFound
	}

	// Send Read Attributes to refresh state
	readOnOff := BuildReadAttributesCommand(zclAttrOnOff)
	if err := c.ezsp.SendUnicast(kd.NodeID, zclProfileHA, zclClusterOnOff, 1, kd.Endpoint, readOnOff); err != nil {
		log.Warn().Err(err).Str("device", id).Msg("Failed to read On/Off state")
	}

	// Brief wait for response
	time.Sleep(200 * time.Millisecond)

	c.devicesMu.RLock()
	state := make(device.DeviceState)
	for k, v := range kd.State {
		state[k] = v
	}
	c.devicesMu.RUnlock()

	return state, nil
}

func (c *Controller) SetDeviceState(_ context.Context, id string, state map[string]any) (device.DeviceState, error) {
	c.devicesMu.RLock()
	kd, ok := c.devices[id]
	c.devicesMu.RUnlock()

	if !ok {
		return nil, device.ErrNotFound
	}

	// Handle "state" field (On/Off)
	if stateVal, ok := state["state"]; ok {
		if strVal, ok := stateVal.(string); ok {
			var cmd uint8
			switch strings.ToUpper(strVal) {
			case "ON":
				cmd = zclCmdOn
			case "OFF":
				cmd = zclCmdOff
			case "TOGGLE":
				cmd = zclCmdToggle
			default:
				return nil, fmt.Errorf("%w: invalid state value %q", device.ErrValidation, strVal)
			}

			payload := BuildOnOffCommand(cmd)
			if err := c.ezsp.SendUnicast(kd.NodeID, zclProfileHA, zclClusterOnOff, 1, kd.Endpoint, payload); err != nil {
				return nil, fmt.Errorf("send on/off command: %w", err)
			}

			c.devicesMu.Lock()
			kd.State["state"] = strings.ToUpper(strVal)
			c.devicesMu.Unlock()
		}
	}

	// Handle "brightness" field (Level Control)
	if brightnessVal, ok := state["brightness"]; ok {
		var level uint8
		switch v := brightnessVal.(type) {
		case float64:
			level = uint8(v)
		case int:
			level = uint8(v)
		case json.Number:
			n, _ := v.Int64()
			level = uint8(n)
		default:
			return nil, fmt.Errorf("%w: invalid brightness type", device.ErrValidation)
		}

		payload := BuildMoveToLevelCommand(level, 10) // 1 second transition
		if err := c.ezsp.SendUnicast(kd.NodeID, zclProfileHA, zclClusterLevelControl, 1, kd.Endpoint, payload); err != nil {
			return nil, fmt.Errorf("send level command: %w", err)
		}

		c.devicesMu.Lock()
		kd.State["brightness"] = int(level)
		c.devicesMu.Unlock()
	}

	// Return updated state
	c.devicesMu.RLock()
	result := make(device.DeviceState)
	for k, v := range kd.State {
		result[k] = v
	}
	c.devicesMu.RUnlock()

	return result, nil
}

func (c *Controller) PermitJoin(_ context.Context, enable bool, duration int) error {
	var dur uint8
	if enable {
		if duration <= 0 || duration > 254 {
			dur = 254
		} else {
			dur = uint8(duration)
		}
	}

	return c.ezsp.PermitJoining(dur)
}

func (c *Controller) IsConnected() bool {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.connected && c.ash.IsConnected()
}

func (c *Controller) Close() {
	c.connMu.Lock()
	c.connected = false
	c.connMu.Unlock()

	c.ezsp.Close()
	c.ash.Close()
	if err := c.serial.Close(); err != nil {
		log.Warn().Err(err).Msg("Failed to close serial port")
	}

	log.Info().Msg("Zigbee controller closed")
}

// --- device.EventSubscriber interface ---

func (c *Controller) Subscribe() chan device.DiscoveryEvent {
	ch := make(chan device.DiscoveryEvent, 16)
	c.subscribersMu.Lock()
	c.subscribers = append(c.subscribers, ch)
	c.subscribersMu.Unlock()
	return ch
}

func (c *Controller) Unsubscribe(ch chan device.DiscoveryEvent) {
	c.subscribersMu.Lock()
	defer c.subscribersMu.Unlock()

	for i, sub := range c.subscribers {
		if sub == ch {
			c.subscribers = append(c.subscribers[:i], c.subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

// --- Helpers ---

// formatIEEE formats an 8-byte IEEE address as a colon-separated hex string.
func formatIEEE(addr [8]byte) string {
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x",
		addr[7], addr[6], addr[5], addr[4], addr[3], addr[2], addr[1], addr[0])
}

func boolToOnOff(b bool) string {
	if b {
		return "ON"
	}
	return "OFF"
}
