package zigbee

import (
	"fmt"
	"io"
	"sync"

	"github.com/rs/zerolog/log"
	"go.bug.st/serial"
)

// SerialPort wraps a serial connection to the Zigbee USB dongle.
type SerialPort struct {
	port serial.Port
	mu   sync.Mutex
}

// OpenSerial opens the serial port at 115200 baud, 8N1.
func OpenSerial(portPath string) (*SerialPort, error) {
	mode := &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(portPath, mode)
	if err != nil {
		return nil, fmt.Errorf("open serial port %s: %w", portPath, err)
	}

	// Silicon Labs EZSP dongles require RTS/CTS hardware flow control.
	if err := port.SetRTS(true); err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("set RTS: %w", err)
	}

	log.Info().Str("port", portPath).Msg("Serial port opened")

	return &SerialPort{port: port}, nil
}

// Write sends raw bytes to the serial port.
func (s *SerialPort) Write(data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port.Write(data)
}

// Read reads raw bytes from the serial port.
func (s *SerialPort) Read(buf []byte) (int, error) {
	return s.port.Read(buf)
}

// Close closes the serial port.
func (s *SerialPort) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port.Close()
}

// ReadByte reads a single byte from the serial port.
func (s *SerialPort) ReadByte() (byte, error) {
	buf := make([]byte, 1)
	_, err := io.ReadFull(s.port, buf)
	if err != nil {
		return 0, err
	}
	return buf[0], nil
}
