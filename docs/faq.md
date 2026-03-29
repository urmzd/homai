# FAQ & Troubleshooting

## Hardware

### What Zigbee adapter is supported?

Any Silicon Labs EFR32-based adapter running EmberZNet firmware with EZSP protocol support. Tested with:

- **Sonoff Zigbee 3.0 USB Dongle Plus V2** (EFR32MG21, EZSP v13, EmberZNet 7.4)

The adapter communicates via serial (UART) using the ASH framing protocol over EZSP.

### How do I find my adapter's serial port?

```bash
# macOS
ls /dev/cu.* | grep -iE 'usb|slab|serial'

# Linux
ls /dev/ttyUSB* /dev/ttyACM*
```

Common port names:
- `/dev/cu.usbserial-XXXX` (macOS, Sonoff V2 / EFR32-based dongles)
- `/dev/ttyUSB0` (Linux)

**Important:** If you have multiple USB-serial devices, verify you're using the correct one. The Sonoff Zigbee 3.0 USB Dongle Plus V2 typically appears as `/dev/cu.usbserial-XXXX`. You can confirm with:

```bash
# macOS — look for "Sonoff Zigbee" in the output
ioreg -p IOUSB -l | grep -B 2 -A 8 '"USB Product Name"'
```

A CP210x bridge (`/dev/cu.SLAB_USBtoUART`) may be a different device (e.g., Sonoff V1 with a TI CC2652 chip) that uses a completely different protocol (Z-Stack/ZNP, not EZSP). Connecting to the wrong adapter will appear to work at the serial level but devices will never respond to commands.

## EZSP Protocol

### What is EZSP?

EmberZNet Serial Protocol (EZSP) is Silicon Labs' host-to-NCP (Network Co-Processor) protocol. The host (this application) sends commands to the Zigbee radio (NCP) over a serial connection. EZSP sits on top of the ASH (Asynchronous Serial Host) framing layer.

**Protocol stack:**
```
Application
    ↕
EZSP (commands/responses/callbacks)
    ↕
ASH  (framing, CRC, flow control, data randomization)
    ↕
UART (serial port)
```

### What EZSP versions are supported?

Currently EZSP v8–v13 (extended frame format). The version is negotiated automatically at startup. The NCP reports its supported version and the host adapts.

### What is the EZSP frame format?

**Legacy (v4–v7):** `seq(1) + FC(1) + frameID(1) + params`
**Extended (v8+):** `seq(1) + FC(2) + frameID(2) + params`

The version command always uses legacy format regardless of the negotiated version. All other commands use extended format for v8+.

### Why does version negotiation require an ASH reset?

After the initial version probe, the NCP may not switch frame formats until it sees a fresh ASH connection. The sequence is:

1. Send `version(desired)` in **legacy** format → NCP responds with its supported version
2. **ASH reset** (RST/RSTACK handshake)
3. Send `version(negotiated)` in **extended** format → NCP confirms and switches
4. All subsequent commands use extended format

### What is ASH data randomization?

The ASH protocol XORs DATA frame payloads with a pseudo-random sequence (LFSR, seed `0x42`) before CRC computation. This prevents long runs of flag/escape bytes that would bloat the frame due to byte stuffing. Both TX and RX must apply the same randomization. See UG101 §4.3.

### What does FC_hi=0x01 mean in extended frame format?

In EZSP v8+ extended frames, `FC_hi` (frame control high byte) must be `0x01` to indicate extended frame format version 1. Setting it to `0x00` causes the NCP to interpret the frame as legacy format, leading to garbled commands.

## Network & Discovery

### How does device discovery work?

1. Run `zigbee-skill discovery start` to enable permit-join mode on the coordinator
2. Put your Zigbee device in pairing mode (usually hold a button for 5+ seconds)
3. The NCP sends a `trustCenterJoinHandler` callback when a device joins
4. The controller registers the device and saves it to `zigbee-skill.yaml`

### My device joined but shows as "Unknown" model/vendor

Device identification relies on ZCL attribute reads (Basic cluster) which happen after the initial join. If the device hasn't responded to attribute queries yet, it will show as Unknown. The device type may also be incorrectly classified (e.g., a smart plug showing as "light") until proper endpoint/cluster enumeration is implemented.

### What happens to devices on restart?

The coordinator resumes the existing Zigbee network from NCP flash. Previously paired devices are loaded from `zigbee-skill.yaml` and typically rejoin the network within **~30 seconds**. During this window, the device appears in `devices list` but may not respond to commands until it reconnects.

If a device doesn't reconnect after 30 seconds:
1. Power-cycle the device (unplug and replug)
2. If that doesn't work, factory reset it (hold the button for 10+ seconds until the LED blinks rapidly)
3. Re-pair via `zigbee-skill discovery start`

### My device joined but keeps blinking / doesn't respond to commands

The device is stuck in commissioning mode — it joined at the MAC layer but never completed the Trust Center key exchange (it didn't receive the network key). Symptoms:

- `Message delivered successfully` in logs but device doesn't act
- Device LED keeps blinking (pairing mode)
- Zero `Incoming message` callbacks from the device
- Device may rejoin repeatedly with different NodeIDs

**Fix:** Reset the network and re-pair:

```bash
zigbee-skill daemon stop
zigbee-skill network reset
zigbee-skill daemon start --port /dev/cu.usbserial-XXXX
zigbee-skill discovery start --wait-for 1
```

Then factory-reset your device (hold button ~10s) and let it re-pair. The device LED should stop blinking once the key exchange completes.

**Root cause:** The NCP's Trust Center security state must be configured before the network is initialized. If the coordinator resumed an existing network without setting security state, joining devices can't receive the network key.

### How do I reset the Zigbee network?

```bash
zigbee-skill network reset
```

This clears the NCP's persisted network state. On the next startup, a fresh network is formed with a new PAN ID, channel, and network key. All previously paired devices will need to be factory-reset and re-paired.

Use this when:
- Devices join but can't communicate (security state issue)
- You want to start fresh on a different channel
- You switched adapters and need a clean network

### The coordinator says "No existing network, forming new one"

This is normal on first run or after `network reset`. The coordinator forms a new Zigbee network with a random PAN ID and extended PAN ID on the best available channel (determined by energy scan). On subsequent runs, it resumes the existing network from NCP flash.

## Common Errors

### `timeout waiting for EZSP response`

The NCP didn't respond within 5 seconds. Causes:
- Frame format mismatch (legacy vs extended)
- Missing ASH data randomization
- Wrong FC bytes in extended format
- Serial port busy or disconnected

### `setConfigurationValue failed: status 0x35`

`EZSP_ERROR_INVALID_ID` — the config ID isn't supported by this NCP firmware version. Non-fatal; the controller continues without that config value.

### `ASH out-of-sequence DATA`

The NCP sent a DATA frame with an unexpected sequence number. This typically happens when reconnecting to an NCP that still has pending frames from a previous session. The ASH layer sends a NAK and the NCP retransmits correctly.

## Specifications

- **Zigbee R23.2**: `specs/csa-iot/docs-06-3474-23-csg-zigbee-specificationR23.2_clean.pdf`
- **BDB 3.1**: `specs/csa-iot/22-65816-030-PRO-BDB-v3.1-Specification.pdf`
- **EZSP Reference**: Silicon Labs UG100
- **ASH Protocol**: Silicon Labs UG101
- **CSA-IoT Spec (web)**: https://csa-iot.org/wp-content/uploads/2023/04/05-3474-23-csg-zigbee-specification-compressed.pdf
