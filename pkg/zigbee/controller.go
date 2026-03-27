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
	"github.com/urmzd/zigbee-skill/pkg/device"
)

// KnownDevice tracks a Zigbee device discovered on the network.
type KnownDevice struct {
	IEEEAddress  [8]byte
	NodeID       uint16
	FriendlyName string
	DeviceType   string
	Endpoint     uint8
	Clusters     []uint16 // input clusters from Simple Descriptor
	State        device.DeviceState
	stateUpdate  chan struct{} // signalled when State is updated
}

// LoadEntry is used to pre-populate the device map from persistent config on startup.
type LoadEntry struct {
	IEEEAddress  [8]byte
	FriendlyName string
	DeviceType   string
	Endpoint     uint8
	Clusters     []uint16
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

	onDeviceChange func() // called after device join/leave/rename
	stopChan       chan struct{}
}

// SetOnDeviceChange registers a callback invoked after the device list changes.
func (c *Controller) SetOnDeviceChange(fn func()) { c.onDeviceChange = fn }

// ExportDevices returns a snapshot of all known devices for persistence.
func (c *Controller) ExportDevices() []ExportedDevice {
	c.devicesMu.RLock()
	defer c.devicesMu.RUnlock()
	out := make([]ExportedDevice, 0, len(c.devices))
	for ieee, kd := range c.devices {
		out = append(out, ExportedDevice{
			IEEEAddress:  ieee,
			FriendlyName: kd.FriendlyName,
			DeviceType:   kd.DeviceType,
			Endpoint:     kd.Endpoint,
			Clusters:     kd.Clusters,
		})
	}
	return out
}

// ExportedDevice is a snapshot of device data for persistence.
type ExportedDevice struct {
	IEEEAddress  string
	FriendlyName string
	DeviceType   string
	Endpoint     uint8
	Clusters     []uint16
}

// notifyDeviceChange calls the registered callback if set.
func (c *Controller) notifyDeviceChange() {
	if c.onDeviceChange != nil {
		c.onDeviceChange()
	}
}

// LoadDevices pre-populates the in-memory device map from persistent storage.
// Devices loaded this way have NodeID=0 until they rejoin the network.
func (c *Controller) LoadDevices(entries []LoadEntry) {
	c.devicesMu.Lock()
	defer c.devicesMu.Unlock()
	for _, e := range entries {
		ieee := formatIEEE(e.IEEEAddress)
		ep := e.Endpoint
		if ep == 0 {
			ep = 1
		}
		c.devices[ieee] = &KnownDevice{
			IEEEAddress:  e.IEEEAddress,
			FriendlyName: e.FriendlyName,
			DeviceType:   e.DeviceType,
			Endpoint:     ep,
			Clusters:     e.Clusters,
			State:        make(device.DeviceState),
		}
	}
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

	// Issue 2: Validate stack version is R23+ (EmberZNet 7.x+, BDB 6.4)
	if stackVer < 0x0700 {
		log.Warn().Uint16("stackVersion", stackVer).
			Msg("Stack version < 7.0 (R23); some BDB 3.1 features may not be available")
	}

	// Configure stack
	log.Info().Msg("Configuring EZSP stack")
	if err := c.ezsp.ConfigureStack(); err != nil {
		return err
	}

	// Issue 7: Set Trust Center policies (BDB 5.6.1, 5.6.2 backwards compat mode)
	// Allow joins with well-known TC link key so devices can receive the network key.
	if err := c.ezsp.SetPolicy(ezspPolicyTrustCenterPolicy, ezspDecisionAllowJoins); err != nil {
		log.Warn().Err(err).Msg("Failed to set TC policy (non-fatal)")
	}
	if err := c.ezsp.SetPolicy(ezspPolicyTCKeyRequestPolicy, 0x01); err != nil {
		log.Warn().Err(err).Msg("Failed to set TC key request policy (non-fatal)")
	}

	// Register HA endpoint on the coordinator so the NCP can route
	// incoming ZCL messages (responses, reports) to the host.
	haClusters := []uint16{zclClusterOnOff, zclClusterLevelControl}
	if err := c.ezsp.AddEndpoint(1, zclProfileHA, 0x0005, haClusters, haClusters); err != nil {
		return fmt.Errorf("register HA endpoint: %w", err)
	}

	// Try to resume existing network
	log.Info().Msg("Initializing Zigbee network")
	status, err := c.ezsp.NetworkInit()
	if err != nil {
		return err
	}

	if status == emberSuccess || status == emberNetworkUp {
		log.Info().Msg("Resumed existing Zigbee network")
		if err := c.broadcastDeviceAnnce(); err != nil {
			log.Warn().Err(err).Msg("Failed to broadcast Device_annce (non-fatal)")
		}
		return nil
	}

	log.Info().Uint8("status", status).Msg("No existing network, forming new one")

	// Set initial security state with well-known TC link key before forming
	if err := c.ezsp.SetInitialSecurityState(); err != nil {
		return fmt.Errorf("set initial security state: %w", err)
	}

	// Issue 1: Energy scan across BDB primary channel set before forming (BDB 8.1)
	channel, err := c.ezsp.EnergyScan(bdbcTLPrimaryChannelSet, 4)
	if err != nil {
		log.Warn().Err(err).Msg("Primary channel scan failed, trying secondary")
		channel, err = c.ezsp.EnergyScan(bdbcTLSecondaryChannelSet, 4)
		if err != nil {
			log.Warn().Err(err).Msg("Secondary scan failed, defaulting to channel 15")
			channel = 15
		}
	}

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

	// Broadcast Device_annce for newly formed network
	if err := c.broadcastDeviceAnnce(); err != nil {
		log.Warn().Err(err).Msg("Failed to broadcast Device_annce after formation (non-fatal)")
	}

	return nil
}

// broadcastDeviceAnnce sends a ZDO Device_annce broadcast (BDB 7.1 step 4).
func (c *Controller) broadcastDeviceAnnce() error {
	eui64, err := c.ezsp.GetEUI64()
	if err != nil {
		return fmt.Errorf("get EUI64: %w", err)
	}

	nodeID, err := c.ezsp.GetNodeID()
	if err != nil {
		return fmt.Errorf("get NodeID: %w", err)
	}

	// ZDO Device_annce payload: NWK addr (2) + IEEE addr (8) + capability (1)
	payload := make([]byte, 11)
	binary.LittleEndian.PutUint16(payload[0:2], nodeID)
	copy(payload[2:10], eui64[:])
	// Capability: allocate address | RX on when idle | main powered = 0x8C
	payload[10] = 0x8C

	log.Info().Str("eui64", formatIEEE(eui64)).Uint16("nodeID", nodeID).Msg("Broadcasting Device_annce")
	return c.ezsp.SendBroadcast(0xFFFD, zdoProfileID, zdoClusterDeviceAnnce, 0, 0, payload, 0)
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
		c.notifyDeviceChange()
		return
	}

	c.devicesMu.Lock()
	existing, found := c.devices[ieeeStr]
	if found {
		// Device rejoining — update NodeID but preserve friendly name and type.
		existing.NodeID = nodeID
		c.devicesMu.Unlock()
		log.Info().Str("ieee", ieeeStr).Uint16("nodeID", nodeID).Msg("Known device rejoined, updated NodeID")
	} else {
		// New device
		kd := &KnownDevice{
			IEEEAddress:  ieee,
			NodeID:       nodeID,
			FriendlyName: ieeeStr, // default name = IEEE address
			DeviceType:   device.DeviceTypeLight,
			Endpoint:     1,
			State:        make(device.DeviceState),
		}
		c.devices[ieeeStr] = kd
		c.devicesMu.Unlock()
		c.notifyDeviceChange()
	}

	c.devicesMu.RLock()
	kd := c.devices[ieeeStr]
	c.devicesMu.RUnlock()

	dev := c.knownToDevice(ieeeStr, kd)
	c.publishEvent(device.DiscoveryEvent{
		Type:      "device_joined",
		Device:    &dev,
		Timestamp: time.Now(),
	})

	// Discover device clusters and configure reporting after a brief stabilization delay.
	go func() {
		time.Sleep(2 * time.Second)
		c.discoverDeviceClusters(kd)
		c.configureDeviceReporting(kd)
	}()
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

	profileID := binary.LittleEndian.Uint16(data[1:3])

	log.Debug().
		Uint16("cluster", clusterID).
		Uint16("sender", sender).
		Int("msgLen", int(msgLen)).
		Msg("Incoming message")

	// Handle ZDO responses (profile 0x0000)
	if profileID == zdoProfileID {
		c.handleZDOResponse(clusterID, sender, message)
		return
	}

	// Issue 6: Respond to Keep Alive Read Attributes requests (BDB 7.3.1)
	if clusterID == zclClusterKeepAlive && len(message) >= 3 {
		frameControl := message[0]
		seqNum := message[1]
		cmdID := message[2]
		if frameControl&0x01 == 0 && cmdID == zclGlobalReadAttributes {
			c.handleKeepAliveRequest(sender, seqNum)
			return
		}
	}

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
		updated := false
		switch clusterID {
		case zclClusterOnOff:
			if val, ok := attrs[zclAttrOnOff]; ok && len(val) > 0 {
				kd.State["state"] = boolToOnOff(val[0] != 0)
				updated = true
			}
		case zclClusterLevelControl:
			if val, ok := attrs[zclAttrCurrentLevel]; ok && len(val) > 0 {
				kd.State["brightness"] = int(val[0])
				updated = true
			}
		}
		if updated && kd.stateUpdate != nil {
			select {
			case kd.stateUpdate <- struct{}{}:
			default:
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
	name := kd.FriendlyName
	if name == "" {
		name = ieeeStr
	}
	stateSchema, _ := json.Marshal(buildStateSchema(kd.Clusters))
	return device.Device{
		ID:           ieeeStr,
		Name:         name,
		Type:         kd.DeviceType,
		Protocol:     device.ProtocolZigbee,
		Manufacturer: "Unknown",
		Model:        "Unknown",
		StateSchema:  stateSchema,
	}
}

// buildStateSchema generates a JSON schema based on the device's actual clusters.
func buildStateSchema(clusters []uint16) map[string]any {
	props := map[string]any{}
	has := func(id uint16) bool {
		for _, c := range clusters {
			if c == id {
				return true
			}
		}
		return false
	}

	if has(zclClusterOnOff) {
		props["state"] = map[string]any{
			"type": "string",
			"enum": []string{"ON", "OFF", "TOGGLE"},
		}
	}
	if has(zclClusterLevelControl) {
		props["brightness"] = map[string]any{
			"type": "integer", "minimum": 0, "maximum": 254,
		}
	}
	if has(zclClusterColorControl) {
		props["color_temp"] = map[string]any{
			"type": "integer", "minimum": 153, "maximum": 500,
			"description": "color temperature in mireds",
		}
	}
	if has(zclClusterTemperature) {
		props["temperature"] = map[string]any{
			"type": "number", "readOnly": true,
			"description": "temperature in °C",
		}
	}
	if has(zclClusterRelativeHumidity) {
		props["humidity"] = map[string]any{
			"type": "number", "readOnly": true,
			"description": "relative humidity %",
		}
	}
	if has(zclClusterOccupancy) {
		props["occupancy"] = map[string]any{
			"type": "boolean", "readOnly": true,
		}
	}
	if has(zclClusterDoorLock) {
		props["lock_state"] = map[string]any{
			"type": "string",
			"enum": []string{"LOCK", "UNLOCK"},
		}
	}
	if has(zclClusterThermostat) {
		props["heating_setpoint"] = map[string]any{
			"type": "number",
			"description": "heating setpoint in °C",
		}
	}

	// Fallback: if no clusters known, assume on/off
	if len(props) == 0 {
		props["state"] = map[string]any{
			"type": "string",
			"enum": []string{"ON", "OFF", "TOGGLE"},
		}
	}

	return map[string]any{"type": "object", "properties": props}
}

// deviceTypeFromClusters infers the device type from its cluster list.
func deviceTypeFromClusters(clusters []uint16) string {
	has := func(id uint16) bool {
		for _, c := range clusters {
			if c == id {
				return true
			}
		}
		return false
	}

	switch {
	case has(zclClusterDoorLock):
		return device.DeviceTypeLock
	case has(zclClusterThermostat):
		return device.DeviceTypeThermostat
	case has(zclClusterTemperature) || has(zclClusterRelativeHumidity) ||
		has(zclClusterOccupancy) || has(zclClusterIlluminance) || has(zclClusterPressure):
		return device.DeviceTypeSensor
	case has(zclClusterLevelControl) || has(zclClusterColorControl):
		return device.DeviceTypeLight
	case has(zclClusterOnOff):
		return device.DeviceTypeSwitch
	default:
		return device.DeviceTypeSwitch
	}
}

// waitForDevice ensures a device has a valid NodeID (i.e., has rejoined the network).
// If NodeID is 0 (loaded from config but not yet rejoined), it enables permit-join
// and waits up to 30 seconds for the device to rejoin.
func (c *Controller) waitForDevice(kd *KnownDevice, id string) error {
	if kd.NodeID != 0 {
		return nil
	}

	log.Info().Str("device", id).Msg("Device has no NodeID, waiting for rejoin (up to 30s)...")
	// Enable permit join briefly to allow the device to reconnect
	_ = c.ezsp.PermitJoining(30)

	for range 60 {
		time.Sleep(500 * time.Millisecond)
		c.devicesMu.RLock()
		nodeID := kd.NodeID
		c.devicesMu.RUnlock()
		if nodeID != 0 {
			log.Info().Str("device", id).Uint16("nodeID", nodeID).Msg("Device rejoined")
			return nil
		}
	}
	return fmt.Errorf("%w: device %s has not rejoined the network (try power-cycling it)", device.ErrTimeout, id)
}

// resolveDevice finds a KnownDevice by IEEE address or friendly name.
// Must be called with devicesMu held (at least RLock).
func (c *Controller) resolveDevice(id string) (*KnownDevice, bool) {
	if kd, ok := c.devices[id]; ok {
		return kd, true
	}
	for _, kd := range c.devices {
		if strings.EqualFold(kd.FriendlyName, id) {
			return kd, true
		}
	}
	return nil, false
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
		// Search by friendly name
		for ieee, d := range c.devices {
			if strings.EqualFold(d.FriendlyName, id) {
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
	c.devicesMu.Lock()
	kd, ok := c.devices[id]
	if !ok {
		// Search by friendly name
		for ieee, d := range c.devices {
			if strings.EqualFold(d.FriendlyName, id) {
				kd = d
				ok = true
				_ = ieee
				break
			}
		}
	}
	if !ok {
		c.devicesMu.Unlock()
		return device.ErrNotFound
	}
	kd.FriendlyName = newName
	c.devicesMu.Unlock()
	c.notifyDeviceChange()
	return nil
}

func (c *Controller) RemoveDevice(_ context.Context, id string, force bool) error {
	c.devicesMu.Lock()
	kd, ok := c.resolveDevice(id)
	if !ok {
		c.devicesMu.Unlock()
		return device.ErrNotFound
	}
	// Find the actual map key (IEEE address) for deletion
	for ieee, d := range c.devices {
		if d == kd {
			id = ieee
			break
		}
	}

	// Issue 10: Send ZDO Mgmt_Leave_req before local removal (BDB 13.4)
	ieee := kd.IEEEAddress
	nodeID := kd.NodeID
	delete(c.devices, id)
	c.devicesMu.Unlock()

	// ZDO Mgmt_Leave_req payload: IEEE address (8) + options (1)
	payload := make([]byte, 9)
	copy(payload[0:8], ieee[:])
	payload[8] = 0x00 // options: no rejoin, no remove children
	if err := c.ezsp.SendUnicast(nodeID, zdoProfileID, zdoClusterMgmtLeaveReq, 0, 0, payload); err != nil {
		log.Warn().Err(err).Str("device", id).Msg("Failed to send ZDO Leave request (device removed locally)")
	}

	c.notifyDeviceChange()
	return nil
}

func (c *Controller) ClearDevices(_ context.Context) error {
	c.devicesMu.Lock()
	devices := make(map[string]*KnownDevice, len(c.devices))
	for k, v := range c.devices {
		devices[k] = v
	}
	c.devices = make(map[string]*KnownDevice)
	c.devicesMu.Unlock()

	// Send ZDO Leave to each device
	for ieee, kd := range devices {
		payload := make([]byte, 9)
		copy(payload[0:8], kd.IEEEAddress[:])
		payload[8] = 0x00
		if err := c.ezsp.SendUnicast(kd.NodeID, zdoProfileID, zdoClusterMgmtLeaveReq, 0, 0, payload); err != nil {
			log.Warn().Err(err).Str("device", ieee).Msg("Failed to send ZDO Leave request")
		}
	}

	c.notifyDeviceChange()
	return nil
}

func (c *Controller) GetDeviceState(_ context.Context, id string) (device.DeviceState, error) {
	c.devicesMu.RLock()
	kd, ok := c.resolveDevice(id)
	c.devicesMu.RUnlock()

	if !ok {
		return nil, device.ErrNotFound
	}

	if err := c.waitForDevice(kd, id); err != nil {
		return nil, err
	}

	// Set up channel to wait for the state response.
	ch := make(chan struct{}, 1)
	c.devicesMu.Lock()
	kd.stateUpdate = ch
	c.devicesMu.Unlock()
	defer func() {
		c.devicesMu.Lock()
		kd.stateUpdate = nil
		c.devicesMu.Unlock()
	}()

	// Send Read Attributes to refresh state (retry on transient NCP buffer-full errors)
	readOnOff := BuildReadAttributesCommand(zclAttrOnOff)
	for attempt := range 3 {
		err := c.ezsp.SendUnicast(kd.NodeID, zclProfileHA, zclClusterOnOff, 1, kd.Endpoint, readOnOff)
		if err == nil {
			break
		}
		log.Warn().Err(err).Int("attempt", attempt+1).Str("device", id).Msg("Failed to send ReadAttributes, retrying")
		time.Sleep(500 * time.Millisecond)
	}

	// Wait for the response or timeout
	select {
	case <-ch:
		log.Debug().Str("device", id).Msg("Received state update from device")
	case <-time.After(5 * time.Second):
		log.Warn().Str("device", id).Msg("Timed out waiting for state response")
	}

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
	kd, ok := c.resolveDevice(id)
	c.devicesMu.RUnlock()

	if !ok {
		return nil, device.ErrNotFound
	}

	if err := c.waitForDevice(kd, id); err != nil {
		return nil, err
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
	if !enable {
		return c.ezsp.PermitJoining(0)
	}

	// Load the well-known TC link key ("ZigBeeAlliance09") as a transient key
	// so the NCP can encrypt the APS Transport Key for joining devices.
	var wildcardEui [8]byte
	wellKnownKey := [16]byte{
		0x5A, 0x69, 0x67, 0x42, 0x65, 0x65, 0x41, 0x6C,
		0x6C, 0x69, 0x61, 0x6E, 0x63, 0x65, 0x30, 0x39,
	}
	if err := c.ezsp.ImportTransientKey(wildcardEui, wellKnownKey); err != nil {
		log.Warn().Err(err).Msg("Failed to import transient link key (join may fail)")
	}

	// Issue 3: BDB 9.7 requires permit join >= bdbcMinCommissioningTime (180s)
	if duration < bdbcMinCommissioningTime {
		duration = bdbcMinCommissioningTime
	}

	// EZSP permitJoining accepts uint8 max 254. For durations > 254s,
	// issue the first chunk and schedule re-issue in the background.
	chunk := min(duration, 254)
	if err := c.ezsp.PermitJoining(uint8(chunk)); err != nil {
		return err
	}

	remaining := duration - chunk
	if remaining > 0 {
		go c.reissuePermitJoin(remaining)
	}
	return nil
}

// reissuePermitJoin extends permit joining in 254s chunks until the total duration is met.
func (c *Controller) reissuePermitJoin(remaining int) {
	for remaining > 0 {
		chunk := min(remaining, 254)
		// Wait until just before the current permit window expires, then re-issue
		select {
		case <-time.After(time.Duration(chunk-4) * time.Second):
		case <-c.stopChan:
			return
		}
		if err := c.ezsp.PermitJoining(uint8(chunk)); err != nil {
			log.Warn().Err(err).Msg("Failed to re-issue permitJoining")
			return
		}
		remaining -= chunk
	}
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

// handleKeepAliveRequest responds to a Keep Alive cluster Read Attributes request (BDB 7.3.1).
func (c *Controller) handleKeepAliveRequest(sender uint16, seqNum uint8) {
	// TC Keep-Alive Base (attr 0x0000): 10 minutes, uint16
	// TC Keep-Alive Jitter (attr 0x0001): 300 seconds, uint16
	attrs := []ZCLAttrValue{
		{0x0000, 0x21, []byte{0x0A, 0x00}}, // 10 minutes
		{0x0001, 0x21, []byte{0x2C, 0x01}}, // 300 seconds
	}

	respPayload := BuildReadAttributesResponsePayload(attrs)
	frame := EncodeZCLGlobalResponse(seqNum, zclGlobalReadAttributesResponse, respPayload)

	if err := c.ezsp.SendUnicast(sender, zclProfileHA, zclClusterKeepAlive, 1, 1, frame); err != nil {
		log.Warn().Err(err).Uint16("sender", sender).Msg("Failed to send Keep Alive response")
	} else {
		log.Debug().Uint16("sender", sender).Msg("Sent Keep Alive response")
	}
}

// discoverDeviceClusters probes a device for known ZCL clusters by sending
// Read Attributes requests and checking which ones get responses.
func (c *Controller) discoverDeviceClusters(kd *KnownDevice) {
	ieeeStr := formatIEEE(kd.IEEEAddress)

	// Probe these clusters — send a Read Attributes for attribute 0x0000 on each.
	// If the device supports the cluster, it responds; otherwise silence/error.
	probeClusters := []uint16{
		zclClusterOnOff,
		zclClusterLevelControl,
		zclClusterColorControl,
		zclClusterTemperature,
		zclClusterRelativeHumidity,
		zclClusterOccupancy,
		zclClusterDoorLock,
		zclClusterThermostat,
	}

	for _, cluster := range probeClusters {
		cmd := BuildReadAttributesCommand(0x0000)
		_ = c.ezsp.SendUnicast(kd.NodeID, zclProfileHA, cluster, 1, kd.Endpoint, cmd)
	}

	// Wait for responses — the incoming message handler updates kd.State for
	// recognized clusters. We check which state keys appeared.
	time.Sleep(3 * time.Second)

	var discovered []uint16
	c.devicesMu.Lock()
	if _, ok := kd.State["state"]; ok {
		discovered = append(discovered, zclClusterOnOff)
	}
	if _, ok := kd.State["brightness"]; ok {
		discovered = append(discovered, zclClusterLevelControl)
	}
	// Even if no state keys were set, check if OnOff responded by looking
	// at whether the device ACK'd — we assume On/Off at minimum for any
	// device that successfully joined and responds to messages.
	if len(discovered) == 0 {
		discovered = append(discovered, zclClusterOnOff)
	}
	kd.Clusters = discovered
	kd.DeviceType = deviceTypeFromClusters(discovered)
	c.devicesMu.Unlock()

	c.notifyDeviceChange()
	log.Info().Str("device", ieeeStr).
		Int("clusters", len(discovered)).
		Str("type", kd.DeviceType).
		Msg("Device clusters discovered")
}

// handleZDOResponse processes ZDO response messages (Active Endpoints, Simple Descriptor).
func (c *Controller) handleZDOResponse(clusterID uint16, sender uint16, message []byte) bool {
	switch clusterID {
	case zdoClusterActiveEndpointsResp:
		return c.handleActiveEndpointsResponse(sender, message)
	case zdoClusterSimpleDescriptorResp:
		return c.handleSimpleDescriptorResponse(sender, message)
	}
	return false
}

func (c *Controller) handleActiveEndpointsResponse(sender uint16, data []byte) bool {
	// ZDO Active_EP_rsp: seq(1) + status(1) + nwkAddr(2) + count(1) + endpoints(N)
	if len(data) < 5 {
		return false
	}
	status := data[1]
	if status != 0x00 {
		log.Warn().Uint8("status", status).Uint16("sender", sender).Msg("Active Endpoints response error")
		return true
	}
	count := int(data[4])
	if len(data) < 5+count {
		return false
	}
	endpoints := data[5 : 5+count]

	log.Debug().Uint16("sender", sender).Int("count", count).Msg("Active Endpoints response")

	// For each endpoint, send Simple Descriptor Request
	for _, ep := range endpoints {
		if ep == 0 || ep == 242 { // skip ZDO endpoint and Green Power
			continue
		}
		payload := make([]byte, 3)
		binary.LittleEndian.PutUint16(payload, sender)
		payload[2] = ep
		if err := c.ezsp.SendUnicast(sender, zdoProfileID, zdoClusterSimpleDescriptorReq, 0, 0, payload); err != nil {
			log.Warn().Err(err).Uint8("endpoint", ep).Msg("Failed to send Simple Descriptor Request")
		}
	}
	return true
}

func (c *Controller) handleSimpleDescriptorResponse(sender uint16, data []byte) bool {
	// ZDO Simple_Desc_rsp: seq(1) + status(1) + nwkAddr(2) + length(1) + descriptor(N)
	// descriptor: endpoint(1) + profileID(2) + deviceID(2) + version(1) + inputCount(1) + inputs(N*2) + outputCount(1) + outputs(N*2)
	if len(data) < 6 {
		return false
	}
	status := data[1]
	if status != 0x00 {
		return true
	}
	descLen := int(data[4])
	if descLen == 0 || len(data) < 5+descLen {
		return false
	}
	desc := data[5 : 5+descLen]
	if len(desc) < 6 {
		return false
	}

	endpoint := desc[0]
	// profileID := binary.LittleEndian.Uint16(desc[1:3])
	// deviceID := binary.LittleEndian.Uint16(desc[3:5])
	// version := desc[5]

	pos := 6
	if pos >= len(desc) {
		return false
	}
	inputCount := int(desc[pos])
	pos++

	var inputClusters []uint16
	for i := range inputCount {
		_ = i
		if pos+2 > len(desc) {
			break
		}
		inputClusters = append(inputClusters, binary.LittleEndian.Uint16(desc[pos:pos+2]))
		pos += 2
	}

	log.Info().
		Uint16("sender", sender).
		Uint8("endpoint", endpoint).
		Int("inputClusters", len(inputClusters)).
		Msg("Simple Descriptor response")

	// Update the device with discovered clusters
	c.devicesMu.Lock()
	for _, kd := range c.devices {
		if kd.NodeID == sender {
			kd.Endpoint = endpoint
			// Merge clusters (device may have multiple endpoints)
			seen := map[uint16]bool{}
			for _, cl := range kd.Clusters {
				seen[cl] = true
			}
			for _, cl := range inputClusters {
				if !seen[cl] {
					kd.Clusters = append(kd.Clusters, cl)
				}
			}
			kd.DeviceType = deviceTypeFromClusters(kd.Clusters)
			break
		}
	}
	c.devicesMu.Unlock()
	c.notifyDeviceChange()
	return true
}

// configureDeviceReporting sends default reporting configuration to a newly joined device (BDB 6.5).
func (c *Controller) configureDeviceReporting(kd *KnownDevice) {
	// On/Off cluster, attribute 0x0000 (Boolean): min=0, max=3600s, no reportable change for discrete
	onOffReport := BuildConfigureReportingCommand(zclAttrOnOff, 0x10, 0, 3600, nil)
	if err := c.ezsp.SendUnicast(kd.NodeID, zclProfileHA, zclClusterOnOff, 1, kd.Endpoint, onOffReport); err != nil {
		log.Warn().Err(err).Uint16("nodeID", kd.NodeID).Msg("Failed to configure On/Off reporting")
	}

	// Level Control cluster, attribute 0x0000 (uint8): min=1, max=3600s, reportable change=1
	levelReport := BuildConfigureReportingCommand(zclAttrCurrentLevel, 0x20, 1, 3600, []byte{0x01})
	if err := c.ezsp.SendUnicast(kd.NodeID, zclProfileHA, zclClusterLevelControl, 1, kd.Endpoint, levelReport); err != nil {
		log.Warn().Err(err).Uint16("nodeID", kd.NodeID).Msg("Failed to configure Level reporting")
	}
}
