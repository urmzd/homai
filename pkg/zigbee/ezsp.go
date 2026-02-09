package zigbee

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// EZSP frame IDs
const (
	ezspVersion               uint16 = 0x0000
	ezspSetConfigurationValue uint16 = 0x0053
	ezspGetNetworkParameters  uint16 = 0x0028
	ezspNetworkInit           uint16 = 0x0017
	ezspFormNetwork           uint16 = 0x001E
	ezspPermitJoining         uint16 = 0x0022
	ezspSendUnicast           uint16 = 0x0034
	ezspGetEUI64              uint16 = 0x0026

	// Callbacks
	ezspTrustCenterJoinHandler uint16 = 0x0024
	ezspIncomingMessageHandler uint16 = 0x0045
	ezspMessageSentHandler     uint16 = 0x003F
	ezspStackStatusHandler     uint16 = 0x0019

	// EZSP config IDs
	ezspConfigStackProfile                uint8 = 0x0C
	ezspConfigSecurityLevel               uint8 = 0x0D
	ezspConfigMaxEndDeviceChildren        uint8 = 0x03
	ezspConfigIndirectTransmissionTimeout uint8 = 0x12
	ezspConfigMaxHops                     uint8 = 0x10
	ezspConfigTrustCenterAddressCacheSize uint8 = 0x19
	ezspConfigSourceRouteTableSize        uint8 = 0x1A
	ezspConfigAddressTableSize            uint8 = 0x05

	// EZSP protocol version
	ezspProtocolVersion = 13

	// EmberStatus values
	emberSuccess     = 0x00
	emberNotJoined   = 0x93
	emberNetworkUp   = 0x90
	emberNetworkDown = 0x91
	emberInvalidCall = 0x70

	// Ember network status (EmberNetworkStatus enum — protocol documentation constants)
	emberNoNetwork      = 0x00 //nolint:unused
	emberJoiningNetwork = 0x01 //nolint:unused
	emberJoinedNetwork  = 0x02 //nolint:unused

	// Send options
	emberApsOptionRetry                = 0x0040
	emberApsOptionEnableRouteDiscovery = 0x0100
)

// EZSPLayer handles EZSP command/response framing over ASH.
type EZSPLayer struct {
	ash   *ASHLayer
	seq   uint8
	seqMu sync.Mutex

	// Frame format: false = legacy 3-byte header, true = extended 5-byte header.
	// Starts as legacy; set to extended after version negotiation confirms v8+.
	extendedFormat bool

	// Response handling
	responseChan map[uint16]chan []byte
	responseMu   sync.Mutex

	// Callback handling
	callbackHandler func(frameID uint16, data []byte)
	callbackMu      sync.RWMutex

	stopChan chan struct{}
}

// NewEZSPLayer creates a new EZSP layer.
func NewEZSPLayer(ash *ASHLayer) *EZSPLayer {
	return &EZSPLayer{
		ash:          ash,
		responseChan: make(map[uint16]chan []byte),
		stopChan:     make(chan struct{}),
	}
}

// Start begins processing EZSP frames from ASH.
func (e *EZSPLayer) Start() {
	go e.readLoop()
}

// SetCallbackHandler sets the handler for async EZSP callbacks.
func (e *EZSPLayer) SetCallbackHandler(handler func(frameID uint16, data []byte)) {
	e.callbackMu.Lock()
	defer e.callbackMu.Unlock()
	e.callbackHandler = handler
}

// Close stops the EZSP layer.
func (e *EZSPLayer) Close() {
	close(e.stopChan)
}

// SendCommand sends an EZSP command and waits for the response.
func (e *EZSPLayer) SendCommand(frameID uint16, params []byte) ([]byte, error) {
	e.seqMu.Lock()
	seq := e.seq
	e.seq++
	e.seqMu.Unlock()

	// Register response channel
	ch := make(chan []byte, 1)
	e.responseMu.Lock()
	e.responseChan[frameID] = ch
	e.responseMu.Unlock()

	defer func() {
		e.responseMu.Lock()
		delete(e.responseChan, frameID)
		e.responseMu.Unlock()
	}()

	// Build EZSP frame based on negotiated format
	var frame []byte
	if e.extendedFormat {
		// Extended 5-byte header: seq(1) + frameControl(2) + frameID(2) + params
		frame = make([]byte, 0, 5+len(params))
		frame = append(frame, seq)
		frame = append(frame, 0x01, 0x00)                      // FC_lo=0x01 (frame format v1), FC_hi=0x00
		frame = append(frame, byte(frameID), byte(frameID>>8)) // frameID as LE uint16
		frame = append(frame, params...)
	} else {
		// Legacy 3-byte header: seq(1) + frameControl(1) + frameID(1) + params
		frame = make([]byte, 0, 3+len(params))
		frame = append(frame, seq)
		frame = append(frame, 0x00)          // frameControl (command)
		frame = append(frame, byte(frameID)) // frameID as single byte
		frame = append(frame, params...)
	}

	log.Debug().
		Uint8("seq", seq).
		Uint16("frameID", frameID).
		Int("params_len", len(params)).
		Msg("EZSP TX command")

	if err := e.ash.SendData(frame); err != nil {
		return nil, fmt.Errorf("send EZSP command 0x%04X: %w", frameID, err)
	}

	// Wait for response
	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout waiting for EZSP response 0x%04X", frameID)
	case <-e.stopChan:
		return nil, fmt.Errorf("stopped")
	}
}

// readLoop processes incoming EZSP frames from ASH.
func (e *EZSPLayer) readLoop() {
	for {
		select {
		case <-e.stopChan:
			return
		case data := <-e.ash.RecvData():
			e.processFrame(data)
		}
	}
}

// processFrame decodes and dispatches an EZSP frame.
func (e *EZSPLayer) processFrame(data []byte) {
	var frameID uint16
	var params []byte
	var isCallback bool

	if e.extendedFormat {
		// Extended 5-byte header: seq(1) + frameControl(2) + frameID(2) + params
		if len(data) < 5 {
			log.Debug().Int("len", len(data)).Msg("EZSP frame too short (extended)")
			return
		}
		frameID = binary.LittleEndian.Uint16(data[3:5])
		params = data[5:]
		isCallback = isCallbackFrameID(frameID)
	} else {
		// Legacy 3-byte header: seq(1) + frameControl(1) + frameID(1) + params
		if len(data) < 3 {
			log.Debug().Int("len", len(data)).Msg("EZSP frame too short (legacy)")
			return
		}
		frameControl := data[1]
		frameID = uint16(data[2])
		params = data[3:]
		isCallback = frameControl&0x04 != 0
	}

	log.Debug().
		Uint16("frameID", frameID).
		Bool("callback", isCallback).
		Int("params_len", len(params)).
		Str("raw_hex", hex.EncodeToString(data)).
		Msg("EZSP RX frame")

	if isCallback {
		e.callbackMu.RLock()
		handler := e.callbackHandler
		e.callbackMu.RUnlock()

		if handler != nil {
			handler(frameID, params)
		}
		return
	}

	// Response — deliver to waiting goroutine
	e.responseMu.Lock()
	ch, ok := e.responseChan[frameID]
	e.responseMu.Unlock()

	if ok {
		select {
		case ch <- params:
		default:
		}
	}
}

// isCallbackFrameID returns true if the given frame ID is a known EZSP async callback.
// Used for extended format where FC bits don't reliably indicate callbacks.
func isCallbackFrameID(id uint16) bool {
	switch id {
	case ezspTrustCenterJoinHandler,
		ezspIncomingMessageHandler,
		ezspMessageSentHandler,
		ezspStackStatusHandler:
		return true
	default:
		return false
	}
}

// NegotiateVersion sends the EZSP version command and validates the response.
// If the NCP does not support the requested version, it responds with a single
// byte indicating the version it supports. We then retry with that version.
func (e *EZSPLayer) NegotiateVersion() (uint8, uint8, uint16, error) {
	desiredVersion := uint8(ezspProtocolVersion)

	// Version command is always the first EZSP command after ASH connect — start at seq 0.
	e.seqMu.Lock()
	e.seq = 0
	e.seqMu.Unlock()

	resp, err := e.SendCommand(ezspVersion, []byte{desiredVersion})
	if err != nil {
		return 0, 0, 0, fmt.Errorf("version negotiation: %w", err)
	}
	log.Debug().
		Int("len", len(resp)).
		Str("raw", hex.EncodeToString(resp)).
		Msg("EZSP version response (initial)")

	// A 1-byte response means version mismatch — the NCP tells us what it supports.
	if len(resp) == 1 {
		ncpVersion := resp[0]
		log.Info().
			Uint8("requested", desiredVersion).
			Uint8("ncpSupports", ncpVersion).
			Msg("EZSP version mismatch, retrying with NCP version")

		// EZSP v8+ requires extended frame format. Switch before the retry
		// so the NCP sees the correct frame format version in FC_lo.
		// No ASH reset — bellows doesn't do one and the NCP doesn't need it.
		if ncpVersion >= 8 {
			e.extendedFormat = true
			log.Debug().Msg("Switching to extended EZSP frame format for version retry")
		}

		resp, err = e.SendCommand(ezspVersion, []byte{ncpVersion})
		if err != nil {
			return 0, 0, 0, fmt.Errorf("version negotiation retry: %w", err)
		}
		log.Debug().
			Int("len", len(resp)).
			Str("raw", hex.EncodeToString(resp)).
			Msg("EZSP version response (retry)")
	}

	if len(resp) < 4 {
		return 0, 0, 0, fmt.Errorf("version response too short: %d bytes (raw: 0x%s)", len(resp), hex.EncodeToString(resp))
	}

	protocolVersion := resp[0]
	stackType := resp[1]
	stackVersion := binary.LittleEndian.Uint16(resp[2:4])

	log.Info().
		Uint8("protocol", protocolVersion).
		Uint8("stackType", stackType).
		Uint16("stackVersion", stackVersion).
		Msg("EZSP version negotiated")

	// Switch to extended frame format now that version negotiation is complete
	if protocolVersion >= 8 {
		e.extendedFormat = true
		log.Debug().Msg("Switched to extended EZSP frame format")
	}

	return protocolVersion, stackType, stackVersion, nil
}

// SetConfigValue sets an EZSP stack configuration value.
func (e *EZSPLayer) SetConfigValue(configID uint8, value uint16) error {
	params := []byte{configID, byte(value), byte(value >> 8)}
	resp, err := e.SendCommand(ezspSetConfigurationValue, params)
	if err != nil {
		return err
	}
	if len(resp) < 1 || resp[0] != emberSuccess {
		status := byte(0xFF)
		if len(resp) >= 1 {
			status = resp[0]
		}
		return fmt.Errorf("setConfigurationValue 0x%02X failed: status 0x%02X", configID, status)
	}
	return nil
}

// ConfigureStack sets up the NCP stack configuration for a coordinator.
func (e *EZSPLayer) ConfigureStack() error {
	configs := []struct {
		id    uint8
		value uint16
	}{
		{ezspConfigStackProfile, 2},          // ZigBee Pro
		{ezspConfigSecurityLevel, 5},         // Standard security
		{ezspConfigMaxEndDeviceChildren, 32}, // Max child devices
		{ezspConfigAddressTableSize, 16},     // Address table
		{ezspConfigSourceRouteTableSize, 16}, // Source route table
		{ezspConfigMaxHops, 30},              // Max hops
	}

	for _, cfg := range configs {
		if err := e.SetConfigValue(cfg.id, cfg.value); err != nil {
			log.Warn().Err(err).Uint8("configID", cfg.id).Msg("Config value set failed (non-fatal)")
		}
	}

	return nil
}

// GetNetworkParameters retrieves the current network state and parameters.
func (e *EZSPLayer) GetNetworkParameters() (uint8, *NetworkParams, error) {
	resp, err := e.SendCommand(ezspGetNetworkParameters, nil)
	if err != nil {
		return 0, nil, err
	}

	if len(resp) < 2 {
		return 0, nil, fmt.Errorf("network params response too short")
	}

	status := resp[0]
	nodeType := resp[1]

	var params NetworkParams
	if len(resp) >= 18 {
		copy(params.ExtendedPanID[:], resp[2:10])
		params.PanID = binary.LittleEndian.Uint16(resp[10:12])
		params.RadioTxPower = int8(resp[12])
		params.RadioChannel = resp[13]
	}

	return status, &NetworkParams{
		NodeType:      nodeType,
		ExtendedPanID: params.ExtendedPanID,
		PanID:         params.PanID,
		RadioTxPower:  params.RadioTxPower,
		RadioChannel:  params.RadioChannel,
	}, nil
}

// NetworkParams holds Zigbee network parameters.
type NetworkParams struct {
	NodeType      uint8
	ExtendedPanID [8]byte
	PanID         uint16
	RadioTxPower  int8
	RadioChannel  uint8
}

// NetworkInit tries to resume an existing network.
func (e *EZSPLayer) NetworkInit() (uint8, error) {
	// networkInitStruct: bitmask (2 bytes) = 0x0000
	params := []byte{0x00, 0x00}
	resp, err := e.SendCommand(ezspNetworkInit, params)
	if err != nil {
		return 0, err
	}
	if len(resp) < 1 {
		return 0, fmt.Errorf("networkInit response empty")
	}
	return resp[0], nil
}

// FormNetwork creates a new Zigbee network.
func (e *EZSPLayer) FormNetwork(channel uint8, panID uint16, extPanID [8]byte) error {
	// EmberNetworkParameters struct for formNetwork
	params := make([]byte, 0, 32)
	params = append(params, extPanID[:]...)              // extendedPanId (8)
	params = append(params, byte(panID), byte(panID>>8)) // panId (2)
	params = append(params, 3)                           // radioTxPower (1)
	params = append(params, channel)                     // radioChannel (1)
	params = append(params, 0x00)                        // joinMethod: USE_MAC_ASSOCIATION (1)
	params = append(params, 0xFF, 0xFF)                  // nwkManagerId (2)
	params = append(params, 0x00)                        // nwkUpdateId (1)
	params = append(params, 0x00, 0x00, 0x00, 0x00)      // channels (4) - not used for form

	resp, err := e.SendCommand(ezspFormNetwork, params)
	if err != nil {
		return err
	}
	if len(resp) < 1 || resp[0] != emberSuccess {
		status := byte(0xFF)
		if len(resp) >= 1 {
			status = resp[0]
		}
		return fmt.Errorf("formNetwork failed: status 0x%02X", status)
	}

	log.Info().
		Uint8("channel", channel).
		Uint16("panID", panID).
		Msg("Network formed")

	return nil
}

// PermitJoining enables or disables device joining.
func (e *EZSPLayer) PermitJoining(duration uint8) error {
	params := []byte{duration}
	resp, err := e.SendCommand(ezspPermitJoining, params)
	if err != nil {
		return err
	}
	if len(resp) < 1 || resp[0] != emberSuccess {
		status := byte(0xFF)
		if len(resp) >= 1 {
			status = resp[0]
		}
		return fmt.Errorf("permitJoining failed: status 0x%02X", status)
	}
	return nil
}

// GetEUI64 retrieves the coordinator's IEEE address.
func (e *EZSPLayer) GetEUI64() ([8]byte, error) {
	resp, err := e.SendCommand(ezspGetEUI64, nil)
	if err != nil {
		return [8]byte{}, err
	}
	if len(resp) < 8 {
		return [8]byte{}, fmt.Errorf("EUI64 response too short: %d bytes", len(resp))
	}
	var eui [8]byte
	copy(eui[:], resp[:8])
	return eui, nil
}

// SendUnicast sends a unicast message to a device.
func (e *EZSPLayer) SendUnicast(nodeID uint16, profileID, clusterID uint16, srcEndpoint, dstEndpoint uint8, payload []byte) error {
	// EmberApsFrame structure
	apsFrame := make([]byte, 0, 12)
	apsFrame = append(apsFrame, byte(profileID), byte(profileID>>8)) // profileId
	apsFrame = append(apsFrame, byte(clusterID), byte(clusterID>>8)) // clusterId
	apsFrame = append(apsFrame, srcEndpoint)                         // sourceEndpoint
	apsFrame = append(apsFrame, dstEndpoint)                         // destinationEndpoint
	options := uint16(emberApsOptionRetry | emberApsOptionEnableRouteDiscovery)
	apsFrame = append(apsFrame, byte(options), byte(options>>8)) // options
	apsFrame = append(apsFrame, 0x00, 0x00)                      // groupId
	apsFrame = append(apsFrame, 0x00)                            // sequence (filled by stack)

	// Build sendUnicast params:
	// type (1) + indexOrDestination (2) + apsFrame (12) + messageTag (1) + messageLength (1) + message
	params := make([]byte, 0, 4+len(apsFrame)+2+len(payload))
	params = append(params, 0x00)                          // EMBER_OUTGOING_DIRECT
	params = append(params, byte(nodeID), byte(nodeID>>8)) // destination
	params = append(params, apsFrame...)
	params = append(params, 0x01)               // messageTag
	params = append(params, byte(len(payload))) // messageLength
	params = append(params, payload...)

	resp, err := e.SendCommand(ezspSendUnicast, params)
	if err != nil {
		return err
	}
	if len(resp) < 1 || resp[0] != emberSuccess {
		status := byte(0xFF)
		if len(resp) >= 1 {
			status = resp[0]
		}
		return fmt.Errorf("sendUnicast failed: status 0x%02X", status)
	}
	return nil
}
