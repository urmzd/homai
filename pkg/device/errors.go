package device

import "errors"

var (
	// ErrNotFound indicates a device was not found
	ErrNotFound = errors.New("device not found")

	// ErrTimeout indicates an operation timed out
	ErrTimeout = errors.New("operation timed out")

	// ErrNotConnected indicates the controller is not connected
	ErrNotConnected = errors.New("controller not connected")

	// ErrUnsupported indicates an operation is not supported by the device
	ErrUnsupported = errors.New("operation not supported")

	// ErrValidation indicates a state payload failed schema validation
	ErrValidation = errors.New("validation error")
)
