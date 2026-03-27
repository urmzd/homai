package zigbee

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// EZSP frame IDs
const (
	ezspVersion                 uint16 = 0x0000
	ezspAddEndpoint             uint16 = 0x0002
	ezspSetConfigurationValue   uint16 = 0x0053
	ezspGetNetworkParameters    uint16 = 0x0028
	ezspNetworkInit             uint16 = 0x0017
	ezspStartScan               uint16 = 0x001A
	ezspLeaveNetwork            uint16 = 0x0020
	ezspFormNetwork             uint16 = 0x001E
	ezspPermitJoining           uint16 = 0x0022
	ezspSendUnicast             uint16 = 0x0034
	ezspSendBroadcast           uint16 = 0x0036
	ezspGetEUI64                uint16 = 0x0026
	ezspSetPolicy               uint16 = 0x0055
	ezspSetInitialSecurityState uint16 = 0x0068
	ezspImportTransientKey      uint16 = 0x0111
	ezspGetNodeID               uint16 = 0x0027

	// Callbacks
	ezspTrustCenterJoinHandler  uint16 = 0x0024
	ezspIncomingMessageHandler  uint16 = 0x0045
	ezspMessageSentHandler      uint16 = 0x003F
	ezspStackStatusHandler      uint16 = 0x0019
	ezspScanCompleteHandler     uint16 = 0x001C
	ezspEnergyScanResultHandler uint16 = 0x0048

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
	// EMBER_APS_OPTION_ENCRYPTION enables APS-layer encryption using the
	// Trust Center link key, required for security-compliant devices.
	emberApsOptionEncryption = 0x0020
	// EMBER_APS_OPTION_RETRY enables APS-layer retries with acknowledgement,
	// satisfying BDB 6.10 requirement for APS Acknowledgement usage.
	emberApsOptionRetry                = 0x0040
	emberApsOptionEnableRouteDiscovery = 0x0100

	// EZSP policy IDs
	ezspPolicyTrustCenterPolicy   uint8 = 0x00
	ezspPolicyTCKeyRequestPolicy  uint8 = 0x05
	ezspPolicyAppKeyRequestPolicy uint8 = 0x06 //nolint:unused

	// EZSP TC policy decisions (bitmask — from Gecko SDK ezsp-enum.h EzspDecisionBitmask)
	ezspDecisionAllowJoins             uint8 = 0x01 // EZSP_DECISION_ALLOW_JOINS
	ezspDecisionAllowUnsecuredRejoins  uint8 = 0x02 // EZSP_DECISION_ALLOW_UNSECURED_REJOINS //nolint:unused
	ezspDecisionSendKeyInClear         uint8 = 0x04 // EZSP_DECISION_SEND_KEY_IN_CLEAR //nolint:unused
	ezspDecisionJoinsUseInstallCodeKey uint8 = 0x10 // EZSP_DECISION_JOINS_USE_INSTALL_CODE_KEY //nolint:unused
	ezspDecisionDeferJoins             uint8 = 0x20 // EZSP_DECISION_DEFER_JOINS //nolint:unused

	// EZSP TC key request policy decisions (from Gecko SDK ezsp-enum.h)
	ezspAllowTCKeyRequestsAndSendCurrentKey uint8 = 0x51 //nolint:unused

	// BDB channel sets (2.4GHz, bitmask where bit N = channel N)
	bdbcTLPrimaryChannelSet   uint32 = 0x02108800 // channels 11, 15, 20, 25
	bdbcTLSecondaryChannelSet uint32 = 0x05EF7000 // remaining 2.4GHz channels

	// Scan types
	ezspEnergyScan uint8 = 0x00

	// ZDO constants
	zdoProfileID                   uint16 = 0x0000
	zdoClusterActiveEndpointsReq   uint16 = 0x0005
	zdoClusterActiveEndpointsResp  uint16 = 0x8005
	zdoClusterSimpleDescriptorReq  uint16 = 0x0004
	zdoClusterSimpleDescriptorResp uint16 = 0x8004
	zdoClusterDeviceAnnce          uint16 = 0x0013
	zdoClusterMgmtLeaveReq         uint16 = 0x0034

	// BDB constants
	bdbcMinCommissioningTime = 180 // seconds
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

// Reinitialize performs ASH reset + EZSP version negotiation + stack configuration.
// Use after LeaveNetwork to get the NCP into a clean state for FormNetwork.
func (e *EZSPLayer) Reinitialize() error {
	if err := e.ash.Reset(); err != nil {
		return fmt.Errorf("ASH reset: %w", err)
	}
	if _, _, _, err := e.NegotiateVersion(); err != nil {
		return fmt.Errorf("version negotiation: %w", err)
	}
	if err := e.ConfigureStack(); err != nil {
		return fmt.Errorf("configure stack: %w", err)
	}
	return nil
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
		frame = append(frame, 0x00, 0x01)                      // FC_lo=0x00 (command), FC_hi=0x01 (extended frame format v1)
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
	if len(data) < 3 {
		log.Debug().Int("len", len(data)).Msg("EZSP frame too short")
		return
	}

	// Try to parse and deliver the frame. In extended mode, try extended first
	// then legacy fallback — the version response is always legacy even when
	// the NCP is in extended mode.
	type parsed struct {
		frameID uint16
		params  []byte
	}

	var candidates []parsed

	if e.extendedFormat && len(data) >= 5 {
		candidates = append(candidates, parsed{
			frameID: binary.LittleEndian.Uint16(data[3:5]),
			params:  data[5:],
		})
	}
	// Always include legacy parse as fallback (or primary in legacy mode).
	candidates = append(candidates, parsed{
		frameID: uint16(data[2]),
		params:  data[3:],
	})

	for _, c := range candidates {
		e.responseMu.Lock()
		ch, ok := e.responseChan[c.frameID]
		e.responseMu.Unlock()

		if ok {
			log.Debug().
				Uint16("frameID", c.frameID).
				Int("params_len", len(c.params)).
				Str("raw_hex", hex.EncodeToString(data)).
				Msg("EZSP RX response")
			select {
			case ch <- c.params:
			default:
			}
			return
		}
	}

	// No pending response matched — use the preferred parse for callback dispatch.
	best := candidates[0]
	log.Debug().
		Uint16("frameID", best.frameID).
		Int("params_len", len(best.params)).
		Str("raw_hex", hex.EncodeToString(data)).
		Msg("EZSP RX callback")

	e.callbackMu.RLock()
	handler := e.callbackHandler
	e.callbackMu.RUnlock()

	if handler != nil {
		handler(best.frameID, best.params)
	}
}

// isCallbackFrameID returns true if the given frame ID is a known EZSP async callback.
// Used for extended format where FC bits don't reliably indicate callbacks.
func isCallbackFrameID(id uint16) bool {
	switch id {
	case ezspTrustCenterJoinHandler,
		ezspIncomingMessageHandler,
		ezspMessageSentHandler,
		ezspStackStatusHandler,
		ezspScanCompleteHandler,
		ezspEnergyScanResultHandler:
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

		// The NCP requires a full ASH RST/RSTACK cycle before it will accept
		// another version command after a mismatch (see UG101 §3.1).
		if err := e.ash.Reset(); err != nil {
			return 0, 0, 0, fmt.Errorf("ASH reset before version retry: %w", err)
		}

		// The version command always uses legacy frame format — the NCP only
		// switches to extended format after a successful version exchange.
		// Keep extendedFormat=false here; switch after we get the 4-byte response.

		// Reset EZSP sequence after ASH reset.
		e.seqMu.Lock()
		e.seq = 0
		e.seqMu.Unlock()

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

	// EZSP v8+ requires extended frame format for all non-version commands.
	// The NCP must see an extended-format version command after ASH reset to
	// confirm the format switch. The version RESPONSE is always in legacy format
	// (processFrame's legacy fallback handles this).
	if protocolVersion >= 8 {
		if err := e.ash.Reset(); err != nil {
			return 0, 0, 0, fmt.Errorf("ASH reset for format switch: %w", err)
		}
		e.extendedFormat = true
		e.seqMu.Lock()
		e.seq = 0
		e.seqMu.Unlock()

		log.Debug().Msg("Sending extended-format version command to confirm format switch")
		resp, err = e.SendCommand(ezspVersion, []byte{protocolVersion})
		if err != nil {
			return 0, 0, 0, fmt.Errorf("extended format confirmation: %w", err)
		}
		log.Debug().
			Int("len", len(resp)).
			Str("raw", hex.EncodeToString(resp)).
			Msg("EZSP version response (extended confirm)")
		if len(resp) < 4 {
			return 0, 0, 0, fmt.Errorf("extended confirm response too short: %d", len(resp))
		}
		log.Info().Msg("Extended EZSP frame format confirmed")
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

// SetInitialSecurityState configures the NCP's security parameters before
// forming or joining a network. Must be called before FormNetwork.
func (e *EZSPLayer) SetInitialSecurityState() error {
	// EmberInitialSecurityState:
	//   bitmask (2) + preconfiguredKey (16) + networkKey (16) +
	//   keySequence (1) + preconfiguredTrustCenterEui64 (8)

	// Bitmask for EZSP v8+ (from Gecko SDK ember-types.h):
	// HAVE_PRECONFIGURED_KEY (0x0100) | HAVE_NETWORK_KEY (0x0200) |
	// TRUST_CENTER_GLOBAL_LINK_KEY (0x0004)
	bitmask := uint16(0x0100 | 0x0200 | 0x0004)

	params := make([]byte, 0, 43)
	params = append(params, byte(bitmask), byte(bitmask>>8))

	// Preconfigured key = well-known TC link key "ZigBeeAlliance09"
	wellKnownKey := []byte{0x5A, 0x69, 0x67, 0x42, 0x65, 0x65, 0x41, 0x6C,
		0x6C, 0x69, 0x61, 0x6E, 0x63, 0x65, 0x30, 0x39}
	params = append(params, wellKnownKey...)

	// Network key = random (let NCP generate, but we must provide 16 bytes)
	var nwkKey [16]byte
	for i := range nwkKey {
		nwkKey[i] = byte(rand.Intn(256))
	}
	params = append(params, nwkKey[:]...)

	params = append(params, 0x00) // keySequence

	// Trust Center EUI64 = all zeros (use local)
	params = append(params, 0, 0, 0, 0, 0, 0, 0, 0)

	resp, err := e.SendCommand(ezspSetInitialSecurityState, params)
	if err != nil {
		return err
	}
	if len(resp) < 1 || resp[0] != emberSuccess {
		status := byte(0xFF)
		if len(resp) >= 1 {
			status = resp[0]
		}
		return fmt.Errorf("setInitialSecurityState failed: status 0x%02X", status)
	}
	return nil
}

// ImportTransientKey loads a transient link key into the NCP for a joining device.
// In EZSP v12+, this replaces the deprecated addTransientLinkKey.
// The NCP uses this key to encrypt the APS Transport Key sent to joining devices.
// Use eui64 all-zeros as a wildcard to apply to any joining device.
func (e *EZSPLayer) ImportTransientKey(eui64 [8]byte, key [16]byte) error {
	// params: eui64(8) + plaintext_key(16) + flags(1)
	params := make([]byte, 0, 25)
	params = append(params, eui64[:]...)
	params = append(params, key[:]...)
	params = append(params, 0x00) // SecurityManagerContextFlags: NONE

	resp, err := e.SendCommand(ezspImportTransientKey, params)
	if err != nil {
		return err
	}
	if len(resp) < 1 || resp[0] != emberSuccess {
		status := byte(0xFF)
		if len(resp) >= 1 {
			status = resp[0]
		}
		return fmt.Errorf("importTransientKey failed: status 0x%02X", status)
	}
	return nil
}

// LeaveNetwork causes the NCP to leave the current network, clearing its
// persisted network state. The next NetworkInit will return "not joined",
// forcing a fresh FormNetwork with current TC policies.
func (e *EZSPLayer) LeaveNetwork() error {
	resp, err := e.SendCommand(ezspLeaveNetwork, nil)
	if err != nil {
		return err
	}
	if len(resp) < 1 || resp[0] != emberSuccess {
		status := byte(0xFF)
		if len(resp) >= 1 {
			status = resp[0]
		}
		return fmt.Errorf("leaveNetwork failed: status 0x%02X", status)
	}
	return nil
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
	// APS frame counter is managed by the NCP stack (BDB 6.2 / R23.2 §2.2.7).
	// No host-side tracking is needed.
	apsFrame = append(apsFrame, 0x00) // sequence (filled by stack)

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

// AddEndpoint registers an application endpoint on the NCP so that it can
// send and receive ZCL messages on that endpoint. Without this, the NCP
// silently drops incoming messages addressed to unregistered endpoints.
func (e *EZSPLayer) AddEndpoint(endpoint uint8, profileID uint16, deviceID uint16, inputClusters, outputClusters []uint16) error {
	// params: endpoint(1) + profileID(2) + deviceID(2) + appFlags(1) +
	//         inputClusterCount(1) + outputClusterCount(1) +
	//         inputClusters(N*2) + outputClusters(N*2)
	inCount := len(inputClusters)
	outCount := len(outputClusters)
	params := make([]byte, 0, 8+inCount*2+outCount*2)
	params = append(params, endpoint)
	params = append(params, byte(profileID), byte(profileID>>8))
	params = append(params, byte(deviceID), byte(deviceID>>8))
	params = append(params, 0x00)          // appFlags
	params = append(params, byte(inCount)) // inputClusterCount
	params = append(params, byte(outCount))
	for _, c := range inputClusters {
		params = append(params, byte(c), byte(c>>8))
	}
	for _, c := range outputClusters {
		params = append(params, byte(c), byte(c>>8))
	}

	resp, err := e.SendCommand(ezspAddEndpoint, params)
	if err != nil {
		return err
	}
	if len(resp) < 1 || resp[0] != emberSuccess {
		status := byte(0xFF)
		if len(resp) >= 1 {
			status = resp[0]
		}
		return fmt.Errorf("addEndpoint failed: status 0x%02X", status)
	}
	return nil
}

// SetPolicy sets an EZSP Trust Center or stack policy (BDB 5.6.1).
func (e *EZSPLayer) SetPolicy(policyID uint8, decisionID uint8) error {
	params := []byte{policyID, decisionID}
	resp, err := e.SendCommand(ezspSetPolicy, params)
	if err != nil {
		return err
	}
	if len(resp) < 1 || resp[0] != emberSuccess {
		status := byte(0xFF)
		if len(resp) >= 1 {
			status = resp[0]
		}
		return fmt.Errorf("setPolicy 0x%02X failed: status 0x%02X", policyID, status)
	}
	return nil
}

// GetNodeID retrieves the coordinator's short network address.
func (e *EZSPLayer) GetNodeID() (uint16, error) {
	resp, err := e.SendCommand(ezspGetNodeID, nil)
	if err != nil {
		return 0, err
	}
	if len(resp) < 2 {
		return 0, fmt.Errorf("getNodeID response too short: %d bytes", len(resp))
	}
	return binary.LittleEndian.Uint16(resp[0:2]), nil
}

// EnergyScan performs an IEEE 802.15.4 energy scan across the given channel mask
// and returns the channel with the lowest detected energy (BDB 8.1).
func (e *EZSPLayer) EnergyScan(channelMask uint32, duration uint8) (uint8, error) {
	type scanResult struct {
		channel uint8
		rssi    int8
	}

	results := make(chan scanResult, 27)
	done := make(chan uint8, 1)

	e.callbackMu.RLock()
	origHandler := e.callbackHandler
	e.callbackMu.RUnlock()

	e.callbackMu.Lock()
	e.callbackHandler = func(frameID uint16, data []byte) {
		switch frameID {
		case ezspEnergyScanResultHandler:
			if len(data) >= 2 {
				results <- scanResult{channel: data[0], rssi: int8(data[1])}
			}
		case ezspScanCompleteHandler:
			if len(data) >= 1 {
				done <- data[0]
			}
		default:
			if origHandler != nil {
				origHandler(frameID, data)
			}
		}
	}
	e.callbackMu.Unlock()

	defer func() {
		e.callbackMu.Lock()
		e.callbackHandler = origHandler
		e.callbackMu.Unlock()
	}()

	params := make([]byte, 7)
	params[0] = ezspEnergyScan
	binary.LittleEndian.PutUint32(params[1:5], channelMask)
	params[5] = duration
	params[6] = 0

	resp, err := e.SendCommand(ezspStartScan, params)
	if err != nil {
		return 0, fmt.Errorf("startScan: %w", err)
	}
	if len(resp) < 1 || resp[0] != emberSuccess {
		status := byte(0xFF)
		if len(resp) >= 1 {
			status = resp[0]
		}
		return 0, fmt.Errorf("startScan failed: status 0x%02X", status)
	}

	var bestChannel uint8
	bestRSSI := int8(127)

	timeout := time.After(30 * time.Second)
	for {
		select {
		case r := <-results:
			log.Debug().Uint8("channel", r.channel).Int8("rssi", r.rssi).Msg("Energy scan result")
			if r.rssi < bestRSSI {
				bestRSSI = r.rssi
				bestChannel = r.channel
			}
		case status := <-done:
			if status != emberSuccess {
				log.Warn().Uint8("status", status).Msg("Scan completed with error")
			}
			if bestChannel == 0 {
				return 0, fmt.Errorf("no channels found in energy scan")
			}
			log.Info().Uint8("channel", bestChannel).Int8("rssi", bestRSSI).Msg("Best channel from energy scan")
			return bestChannel, nil
		case <-timeout:
			return 0, fmt.Errorf("energy scan timed out")
		case <-e.stopChan:
			return 0, fmt.Errorf("stopped")
		}
	}
}

// SendBroadcast sends a broadcast message (used for ZDO Device_annce etc).
func (e *EZSPLayer) SendBroadcast(destination uint16, profileID, clusterID uint16, srcEndpoint, dstEndpoint uint8, payload []byte, radius uint8) error {
	apsFrame := make([]byte, 0, 12)
	apsFrame = append(apsFrame, byte(profileID), byte(profileID>>8))
	apsFrame = append(apsFrame, byte(clusterID), byte(clusterID>>8))
	apsFrame = append(apsFrame, srcEndpoint)
	apsFrame = append(apsFrame, dstEndpoint)
	options := uint16(0)
	apsFrame = append(apsFrame, byte(options), byte(options>>8))
	apsFrame = append(apsFrame, 0x00, 0x00) // groupId
	apsFrame = append(apsFrame, 0x00)       // sequence (filled by stack)

	params := make([]byte, 0, 2+len(apsFrame)+3+len(payload))
	params = append(params, byte(destination), byte(destination>>8))
	params = append(params, apsFrame...)
	params = append(params, radius)
	params = append(params, 0x01)               // messageTag
	params = append(params, byte(len(payload))) // messageLength
	params = append(params, payload...)

	resp, err := e.SendCommand(ezspSendBroadcast, params)
	if err != nil {
		return err
	}
	if len(resp) < 1 || resp[0] != emberSuccess {
		status := byte(0xFF)
		if len(resp) >= 1 {
			status = resp[0]
		}
		return fmt.Errorf("sendBroadcast failed: status 0x%02X", status)
	}
	return nil
}
