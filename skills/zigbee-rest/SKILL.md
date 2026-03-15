---
name: zigbee-rest
description: "Control smart home Zigbee devices via the zigbee-rest CLI. Use this skill whenever the user wants to turn lights on/off, adjust brightness, list or rename devices, pair new devices, or check device state. Triggers on any smart home, lighting, Zigbee, or device control request — even casual ones like 'turn off the bedroom light' or 'what lights are on right now'."
---

# Zigbee REST — Smart Home Device Control

Control Zigbee smart home devices using the `zigbee-rest` CLI. All output is JSON — pipe to `jq` for filtering.

## Commands

```bash
zigbee-rest health                                # Check API server health
zigbee-rest devices list                          # List all paired devices
zigbee-rest devices get <id>                      # Get device details
zigbee-rest devices rename <id> --name <name>     # Rename a device
zigbee-rest devices remove <id>                   # Remove a device
zigbee-rest devices state <id>                    # Get device state
zigbee-rest devices set <id> --state ON           # Set device state
zigbee-rest discovery start [--duration 120]      # Start pairing mode
zigbee-rest discovery stop                        # Stop pairing mode
```

`<id>` is a device's IEEE address (e.g. `0x00158D0001A2B3C4`) or friendly name (e.g. `bedroom-lamp`).

Use `--address <url>` to target a different API server (default: `http://localhost:8080`).

## Examples

```bash
# List device names
zigbee-rest devices list | jq '.devices[].friendly_name'

# Turn on with brightness
zigbee-rest devices set bedroom-lamp --state ON --brightness 150

# Turn off
zigbee-rest devices set bedroom-lamp --state OFF

# Get current state
zigbee-rest devices state bedroom-lamp | jq '.state'
```

## Response Shapes

**List devices:** `{"devices": [{"ieee_address": "...", "friendly_name": "...", "type": "light", "state": {...}}], "count": N}`

**Device state:** `{"device": "name", "state": {"state": "ON", "brightness": 200}, "timestamp": "..."}`

**Errors:** `{"error": "code", "message": "..."}` — 400 (bad input), 404 (not found), 504 (timeout)

## State Properties

State objects vary per device. For lights:

| Property | Type | Values |
|----------|------|--------|
| `state` | string | `"ON"` or `"OFF"` |
| `brightness` | number | Device-specific range |

## Workflow

1. Run `zigbee-rest devices list` to discover available devices and their friendly names
2. Use the friendly name as `<id>` in subsequent commands
3. To check what properties a device supports, look at `state_schema` in the device response
4. Set state with `zigbee-rest devices set <id> --state ON --brightness 150`
