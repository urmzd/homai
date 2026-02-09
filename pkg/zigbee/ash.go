package zigbee

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ASH protocol constants
const (
	ashFlagByte   = 0x7E
	ashEscapeByte = 0x7D
	ashXON        = 0x11
	ashXOFF       = 0x13
	ashFlipBit    = 0x20
	ashCancelByte = 0x1A
	ashSubstitute = 0x18

	// Frame types (encoded in control byte)
	ashFrameData   = 0x00 // bit 7 = 0
	ashFrameACK    = 0x80 // 0b10000xxx
	ashFrameNAK    = 0xA0 // 0b10100xxx
	ashFrameRST    = 0xC0
	ashFrameRSTACK = 0xC1
	ashFrameERROR  = 0xC2

	ashMaxRetries   = 3
	ashRetryTimeout = 1 * time.Second
	ashMaxFrameLen  = 256
)

// ASH connection states
type ashState int

const (
	ashStateDisconnected ashState = iota
	ashStateResetPending
	ashStateConnected
)

// ASHLayer handles ASH framing over a serial connection.
type ASHLayer struct {
	serial  *SerialPort
	state   ashState
	stateMu sync.RWMutex

	// Sequence numbers
	sendSeq uint8 // frmNum: next frame to send
	ackNum  uint8 // ackNum: next frame we expect to receive
	recvSeq uint8 // next frame we expect from NCP

	seqMu sync.Mutex

	// Pending data frames waiting for ACK
	pending   map[uint8][]byte
	pendingMu sync.Mutex

	// Channel for received EZSP data frames
	recvChan chan []byte
	// Channel to signal connection established
	connChan chan struct{}

	stopChan chan struct{}
	stopped  bool
	stopMu   sync.Mutex
}

// NewASHLayer creates a new ASH framing layer.
func NewASHLayer(s *SerialPort) *ASHLayer {
	return &ASHLayer{
		serial:   s,
		state:    ashStateDisconnected,
		pending:  make(map[uint8][]byte),
		recvChan: make(chan []byte, 16),
		connChan: make(chan struct{}, 1),
		stopChan: make(chan struct{}),
	}
}

// Connect sends RST and waits for RSTACK to establish the ASH connection.
func (a *ASHLayer) Connect() error {
	a.stateMu.Lock()
	a.state = ashStateResetPending
	a.stateMu.Unlock()

	// Send RST frame
	if err := a.sendRST(); err != nil {
		return fmt.Errorf("send RST: %w", err)
	}

	// Start reader goroutine
	go a.readLoop()

	// Wait for RSTACK
	select {
	case <-a.connChan:
		log.Info().Msg("ASH connection established")
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for RSTACK")
	case <-a.stopChan:
		return fmt.Errorf("stopped")
	}
}

// SendData sends an EZSP payload wrapped in an ASH DATA frame.
func (a *ASHLayer) SendData(payload []byte) error {
	a.stateMu.RLock()
	if a.state != ashStateConnected {
		a.stateMu.RUnlock()
		return fmt.Errorf("ASH not connected")
	}
	a.stateMu.RUnlock()

	a.seqMu.Lock()
	seq := a.sendSeq
	a.sendSeq = (a.sendSeq + 1) & 0x07
	ack := a.recvSeq
	a.seqMu.Unlock()

	// Build DATA control byte: bit7=0, frmNum[2:0] in bits 6-4, reTx=0, ackNum[2:0] in bits 2-0
	control := (seq << 4) | (ack & 0x07)

	frame := a.buildDataFrame(control, payload)

	// Store for potential retransmission
	a.pendingMu.Lock()
	a.pending[seq] = frame
	a.pendingMu.Unlock()

	log.Debug().
		Uint8("seq", seq).
		Uint8("ack", ack).
		Int("payload_len", len(payload)).
		Msg("ASH TX DATA")

	_, err := a.serial.Write(frame)
	if err != nil {
		return fmt.Errorf("write DATA frame: %w", err)
	}

	return nil
}

// RecvData returns the channel for receiving EZSP payloads.
func (a *ASHLayer) RecvData() <-chan []byte {
	return a.recvChan
}

// IsConnected returns true if the ASH layer is connected.
func (a *ASHLayer) IsConnected() bool {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.state == ashStateConnected
}

// Close stops the ASH layer.
func (a *ASHLayer) Close() {
	a.stopMu.Lock()
	defer a.stopMu.Unlock()
	if !a.stopped {
		a.stopped = true
		close(a.stopChan)
	}
}

// Reset re-initiates the ASH handshake on an already-running connection.
// This is required after an EZSP version mismatch — the NCP won't accept
// another version command until a full RST/RSTACK cycle completes.
func (a *ASHLayer) Reset() error {
	a.stateMu.Lock()
	a.state = ashStateResetPending
	a.stateMu.Unlock()

	log.Info().Msg("ASH reset requested")

	// Drain any stale connChan signal
	select {
	case <-a.connChan:
	default:
	}

	if err := a.sendRST(); err != nil {
		return fmt.Errorf("send RST: %w", err)
	}

	select {
	case <-a.connChan:
		log.Info().Msg("ASH connection re-established after reset")
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for RSTACK after reset")
	case <-a.stopChan:
		return fmt.Errorf("stopped")
	}
}

// sendRST sends an ASH RST frame (0xC0 + CRC + flag).
func (a *ASHLayer) sendRST() error {
	// RST frame: just the cancel byte to flush, then 0xC0 with CRC and flag
	// First send cancel byte to reset NCP receiver state
	if _, err := a.serial.Write([]byte{ashCancelByte}); err != nil {
		return err
	}

	raw := []byte{ashFrameRST}
	crc := crcCCITT(raw)
	raw = append(raw, byte(crc>>8), byte(crc&0xFF))

	frame := ashStuff(raw)
	frame = append(frame, ashFlagByte)

	log.Debug().Msg("ASH TX RST")

	_, err := a.serial.Write(frame)
	return err
}

// sendACK sends an ASH ACK frame.
func (a *ASHLayer) sendACK() error {
	a.seqMu.Lock()
	ack := a.recvSeq
	a.seqMu.Unlock()

	control := byte(ashFrameACK) | (ack & 0x07)
	raw := []byte{control}
	crc := crcCCITT(raw)
	raw = append(raw, byte(crc>>8), byte(crc&0xFF))

	frame := ashStuff(raw)
	frame = append(frame, ashFlagByte)

	log.Debug().Uint8("ack", ack).Msg("ASH TX ACK")

	_, err := a.serial.Write(frame)
	return err
}

// readLoop continuously reads frames from the serial port.
func (a *ASHLayer) readLoop() {
	buf := make([]byte, 0, ashMaxFrameLen)

	for {
		select {
		case <-a.stopChan:
			return
		default:
		}

		b, err := a.serial.ReadByte()
		if err != nil {
			a.stopMu.Lock()
			stopped := a.stopped
			a.stopMu.Unlock()
			if stopped {
				return
			}
			log.Error().Err(err).Msg("ASH read error")
			continue
		}

		if b == ashCancelByte {
			buf = buf[:0]
			continue
		}

		if b == ashSubstitute {
			buf = buf[:0]
			continue
		}

		if b == ashXON || b == ashXOFF {
			continue
		}

		if b == ashFlagByte {
			if len(buf) > 0 {
				a.processFrame(buf)
				buf = buf[:0]
			}
			continue
		}

		buf = append(buf, b)
		if len(buf) > ashMaxFrameLen {
			buf = buf[:0]
		}
	}
}

// processFrame handles a complete ASH frame (after flag byte removal).
func (a *ASHLayer) processFrame(stuffed []byte) {
	raw := ashUnstuff(stuffed)

	if len(raw) < 3 {
		log.Debug().Int("len", len(raw)).Msg("ASH frame too short, discarding")
		return
	}

	// Verify CRC
	payload := raw[:len(raw)-2]
	receivedCRC := uint16(raw[len(raw)-2])<<8 | uint16(raw[len(raw)-1])
	computedCRC := crcCCITT(payload)

	if receivedCRC != computedCRC {
		log.Warn().
			Uint16("received", receivedCRC).
			Uint16("computed", computedCRC).
			Msg("ASH CRC mismatch")
		return
	}

	control := payload[0]

	switch {
	case control == ashFrameRSTACK:
		// RSTACK
		a.handleRSTACK(payload)
	case control == ashFrameERROR:
		// ERROR
		log.Error().Hex("frame", payload).Msg("ASH ERROR frame received")
	case control&0x80 == ashFrameData:
		// DATA frame
		a.handleData(payload)
	case control&0xE0 == ashFrameACK:
		// ACK
		a.handleACK(control)
	case control&0xE0 == ashFrameNAK:
		// NAK
		a.handleNAK(control)
	default:
		log.Debug().Uint8("control", control).Msg("ASH unknown frame type")
	}
}

// handleRSTACK processes RSTACK frame.
func (a *ASHLayer) handleRSTACK(payload []byte) {
	log.Info().Hex("payload", payload).Msg("ASH RSTACK received")

	a.seqMu.Lock()
	a.sendSeq = 0
	a.ackNum = 0
	a.recvSeq = 0
	a.seqMu.Unlock()

	a.pendingMu.Lock()
	a.pending = make(map[uint8][]byte)
	a.pendingMu.Unlock()

	a.stateMu.Lock()
	a.state = ashStateConnected
	a.stateMu.Unlock()

	select {
	case a.connChan <- struct{}{}:
	default:
	}
}

// handleData processes a DATA frame.
func (a *ASHLayer) handleData(payload []byte) {
	control := payload[0]
	frmNum := (control >> 4) & 0x07
	// ackNum from NCP is in bits 2:0
	npcAck := control & 0x07

	log.Debug().
		Uint8("frmNum", frmNum).
		Uint8("npcAck", npcAck).
		Int("payload_len", len(payload)-1).
		Msg("ASH RX DATA")

	// Remove acknowledged frames
	a.pendingMu.Lock()
	for seq := range a.pending {
		if ashSeqLessThan(seq, npcAck) {
			delete(a.pending, seq)
		}
	}
	a.pendingMu.Unlock()

	a.seqMu.Lock()
	expected := a.recvSeq
	if frmNum == expected {
		a.recvSeq = (expected + 1) & 0x07
		a.seqMu.Unlock()

		// Send ACK
		if err := a.sendACK(); err != nil {
			log.Error().Err(err).Msg("Failed to send ACK")
		}

		// Extract EZSP data (skip control byte)
		ezspData := make([]byte, len(payload)-1)
		copy(ezspData, payload[1:])

		select {
		case a.recvChan <- ezspData:
		default:
			log.Warn().Msg("ASH recv channel full, dropping frame")
		}
	} else {
		a.seqMu.Unlock()
		log.Warn().
			Uint8("expected", expected).
			Uint8("got", frmNum).
			Msg("ASH out-of-sequence DATA, sending NAK")
		a.sendNAK()
	}
}

// handleACK processes an ACK frame.
func (a *ASHLayer) handleACK(control byte) {
	ackNum := control & 0x07
	log.Debug().Uint8("ack", ackNum).Msg("ASH RX ACK")

	a.pendingMu.Lock()
	for seq := range a.pending {
		if ashSeqLessThan(seq, ackNum) {
			delete(a.pending, seq)
		}
	}
	a.pendingMu.Unlock()
}

// handleNAK processes a NAK frame and retransmits.
func (a *ASHLayer) handleNAK(control byte) {
	nakNum := control & 0x07
	log.Warn().Uint8("nak", nakNum).Msg("ASH RX NAK, retransmitting")

	a.pendingMu.Lock()
	frame, ok := a.pending[nakNum]
	a.pendingMu.Unlock()

	if ok {
		if _, err := a.serial.Write(frame); err != nil {
			log.Error().Err(err).Msg("ASH retransmit failed")
		}
	}
}

// sendNAK sends an ASH NAK frame.
func (a *ASHLayer) sendNAK() {
	a.seqMu.Lock()
	ack := a.recvSeq
	a.seqMu.Unlock()

	control := byte(ashFrameNAK) | (ack & 0x07)
	raw := []byte{control}
	crc := crcCCITT(raw)
	raw = append(raw, byte(crc>>8), byte(crc&0xFF))

	frame := ashStuff(raw)
	frame = append(frame, ashFlagByte)

	if _, err := a.serial.Write(frame); err != nil {
		log.Error().Err(err).Msg("ASH NAK send failed")
	}
}

// buildDataFrame builds a complete ASH DATA frame with byte stuffing.
func (a *ASHLayer) buildDataFrame(control byte, payload []byte) []byte {
	raw := make([]byte, 0, len(payload)+3)
	raw = append(raw, control)
	raw = append(raw, payload...)

	crc := crcCCITT(raw)
	raw = append(raw, byte(crc>>8), byte(crc&0xFF))

	frame := ashStuff(raw)
	frame = append(frame, ashFlagByte)
	return frame
}

// ashStuff performs ASH byte stuffing.
func ashStuff(data []byte) []byte {
	out := make([]byte, 0, len(data)*2)
	for _, b := range data {
		if b == ashFlagByte || b == ashEscapeByte || b == ashXON || b == ashXOFF || b == ashSubstitute || b == ashCancelByte {
			out = append(out, ashEscapeByte, b^ashFlipBit)
		} else {
			out = append(out, b)
		}
	}
	return out
}

// ashUnstuff reverses ASH byte stuffing.
func ashUnstuff(data []byte) []byte {
	out := make([]byte, 0, len(data))
	escaped := false
	for _, b := range data {
		if escaped {
			out = append(out, b^ashFlipBit)
			escaped = false
		} else if b == ashEscapeByte {
			escaped = true
		} else {
			out = append(out, b)
		}
	}
	return out
}

// crcCCITT computes CRC-CCITT (0xFFFF initial, poly 0x1021).
func crcCCITT(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// ashSeqLessThan compares 3-bit sequence numbers with wraparound.
func ashSeqLessThan(a, b uint8) bool {
	a &= 0x07
	b &= 0x07
	diff := (b - a) & 0x07
	return diff > 0 && diff <= 4
}
