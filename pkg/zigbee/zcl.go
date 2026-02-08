package zigbee

import "encoding/binary"

// ZCL cluster IDs
const (
	zclClusterOnOff        uint16 = 0x0006
	zclClusterLevelControl uint16 = 0x0008
)

// ZCL command IDs for On/Off cluster
const (
	zclCmdOff    uint8 = 0x00
	zclCmdOn     uint8 = 0x01
	zclCmdToggle uint8 = 0x02
)

// ZCL command IDs for Level Control cluster
const (
	zclCmdMoveToLevel          uint8 = 0x00
	zclCmdMoveToLevelWithOnOff uint8 = 0x04
)

// ZCL frame types
const (
	zclFrameTypeGlobal          uint8 = 0x00
	zclFrameTypeClusterSpecific uint8 = 0x01
)

// ZCL global commands
const (
	zclGlobalReadAttributes         uint8 = 0x00
	zclGlobalReadAttributesResponse uint8 = 0x01
)

// ZCL direction
const (
	zclDirectionClientToServer uint8 = 0x00
	zclDirectionServerToClient uint8 = 0x08
)

// HA profile
const (
	zclProfileHA uint16 = 0x0104
)

// ZCL attribute IDs
const (
	zclAttrOnOff        uint16 = 0x0000 // On/Off cluster: on/off state
	zclAttrCurrentLevel uint16 = 0x0000 // Level Control: current level
)

// ZCLHeader represents a ZCL frame header.
type ZCLHeader struct {
	FrameControl uint8
	SeqNumber    uint8
	CommandID    uint8
}

var zclSeqCounter uint8

func nextZCLSeq() uint8 {
	zclSeqCounter++
	return zclSeqCounter
}

// EncodeZCLClusterCommand builds a ZCL cluster-specific command frame.
func EncodeZCLClusterCommand(commandID uint8, payload []byte) []byte {
	header := ZCLHeader{
		FrameControl: zclFrameTypeClusterSpecific | zclDirectionClientToServer,
		SeqNumber:    nextZCLSeq(),
		CommandID:    commandID,
	}

	frame := make([]byte, 0, 3+len(payload))
	frame = append(frame, header.FrameControl)
	frame = append(frame, header.SeqNumber)
	frame = append(frame, header.CommandID)
	frame = append(frame, payload...)
	return frame
}

// EncodeZCLGlobalCommand builds a ZCL global command frame (e.g., Read Attributes).
func EncodeZCLGlobalCommand(commandID uint8, payload []byte) []byte {
	header := ZCLHeader{
		FrameControl: zclFrameTypeGlobal | zclDirectionClientToServer,
		SeqNumber:    nextZCLSeq(),
		CommandID:    commandID,
	}

	frame := make([]byte, 0, 3+len(payload))
	frame = append(frame, header.FrameControl)
	frame = append(frame, header.SeqNumber)
	frame = append(frame, header.CommandID)
	frame = append(frame, payload...)
	return frame
}

// BuildOnOffCommand builds a ZCL On/Off cluster command.
func BuildOnOffCommand(cmd uint8) []byte {
	return EncodeZCLClusterCommand(cmd, nil)
}

// BuildMoveToLevelCommand builds a ZCL Level Control move-to-level command.
func BuildMoveToLevelCommand(level uint8, transitionTime uint16) []byte {
	payload := make([]byte, 3)
	payload[0] = level
	binary.LittleEndian.PutUint16(payload[1:3], transitionTime)
	return EncodeZCLClusterCommand(zclCmdMoveToLevelWithOnOff, payload)
}

// BuildReadAttributesCommand builds a ZCL Read Attributes command.
func BuildReadAttributesCommand(attrIDs ...uint16) []byte {
	payload := make([]byte, len(attrIDs)*2)
	for i, id := range attrIDs {
		binary.LittleEndian.PutUint16(payload[i*2:], id)
	}
	return EncodeZCLGlobalCommand(zclGlobalReadAttributes, payload)
}

// ParseReadAttributesResponse extracts attribute values from a Read Attributes Response.
// Returns a map of attrID -> value bytes.
func ParseReadAttributesResponse(data []byte) map[uint16][]byte {
	result := make(map[uint16][]byte)
	offset := 0

	for offset+4 <= len(data) {
		attrID := binary.LittleEndian.Uint16(data[offset:])
		offset += 2
		status := data[offset]
		offset++

		if status != 0x00 {
			// Attribute read failed, skip
			continue
		}

		if offset >= len(data) {
			break
		}

		dataType := data[offset]
		offset++

		valueLen := zclDataTypeLength(dataType, data[offset:])
		if valueLen <= 0 || offset+valueLen > len(data) {
			break
		}

		value := make([]byte, valueLen)
		copy(value, data[offset:offset+valueLen])
		result[attrID] = value
		offset += valueLen
	}

	return result
}

// zclDataTypeLength returns the byte length of a ZCL data type value.
func zclDataTypeLength(dataType uint8, data []byte) int {
	switch dataType {
	case 0x10: // Boolean
		return 1
	case 0x20: // uint8
		return 1
	case 0x21: // uint16
		return 2
	case 0x22: // uint24
		return 3
	case 0x23: // uint32
		return 4
	case 0x28: // int8
		return 1
	case 0x29: // int16
		return 2
	case 0x30: // enum8
		return 1
	case 0x31: // enum16
		return 2
	case 0x42: // octet string
		if len(data) < 1 {
			return -1
		}
		return 1 + int(data[0])
	default:
		return -1
	}
}
