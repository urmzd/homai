---
name: zigbee-skill
description: "AI-native smart home skill — gives agents direct Zigbee device control tailored to user preferences. Use whenever the user wants to manage lights, pair devices, adjust settings, or interact with their home naturally — even casual requests like 'turn off the bedroom light' or 'make it cozy in here'."
---

# Zigbee Skill — AI-Native Smart Home Control

Give AI agents direct control over Zigbee smart home devices. Tailors to user preferences for personalized home automation. All output is JSON — pipe to `jq` for filtering.

## Commands

```bash
zigbee-skill health                                # Check API server health
zigbee-skill devices list                          # List all paired devices
zigbee-skill devices get <id>                      # Get device details
zigbee-skill devices rename <id> --name <name>     # Rename a device
zigbee-skill devices remove <id>                   # Remove a device
zigbee-skill devices state <id>                    # Get device state
zigbee-skill devices set <id> --state ON           # Set device state
zigbee-skill discovery start [--duration 120]      # Start pairing mode
zigbee-skill discovery stop                        # Stop pairing mode
```

`<id>` is a device's IEEE address (e.g. `0x00158D0001A2B3C4`) or friendly name (e.g. `bedroom-lamp`).

Use `--address <url>` to target a different API server (default: `http://localhost:8080`).

## Examples

```bash
# List device names
zigbee-skill devices list | jq '.devices[].friendly_name'

# Turn on with brightness
zigbee-skill devices set bedroom-lamp --state ON --brightness 150

# Turn off
zigbee-skill devices set bedroom-lamp --state OFF

# Get current state
zigbee-skill devices state bedroom-lamp | jq '.state'
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

1. Run `zigbee-skill devices list` to discover available devices and their friendly names
2. Use the friendly name as `<id>` in subsequent commands
3. To check what properties a device supports, look at `state_schema` in the device response
4. Set state with `zigbee-skill devices set <id> --state ON --brightness 150`
