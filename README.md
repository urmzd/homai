<p align="center">
  <h1 align="center">zigbee-skill</h1>
  <p align="center">
    An AI-native smart home skill that lets AI agents control your Zigbee devices directly — no cloud, no hub, just natural interaction tailored to your preferences.
    <br /><br />
    <a href="https://github.com/urmzd/zigbee-skill/releases">Download</a>
    &middot;
    <a href="https://github.com/urmzd/zigbee-skill/issues">Report Bug</a>
    &middot;
    <a href="https://github.com/urmzd/zigbee-skill/blob/main/AGENTS.md">API Docs</a>
  </p>
</p>

<p align="center">
  <a href="https://github.com/urmzd/zigbee-skill/actions/workflows/ci.yml"><img src="https://github.com/urmzd/zigbee-skill/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
</p>

## Showcase

<p align="center">
  <strong>CLI Help</strong><br>
  <img src="showcase/cli-help.png" alt="zigbee-skill CLI help" width="600">
</p>

<p align="center">
  <strong>JSON Output</strong><br>
  <img src="showcase/cli-json-output.png" alt="zigbee-skill JSON output" width="600">
</p>

## Features

- Direct Zigbee device control via EZSP serial protocol (no Zigbee2MQTT or MQTT broker required)
- REST API for device management with Swagger documentation
- CLI with JSON output for scripting and AI agent integration
- Real-time device discovery events via Server-Sent Events (SSE)
- Multi-profile support for multiple installations
- Cross-platform binaries (Linux, macOS — amd64/arm64)

## Prerequisites

- Zigbee 3.0 USB adapter (e.g., SONOFF Zigbee 3.0 USB Dongle Plus)

For building from source:
- Go 1.24+
- [just](https://github.com/casey/just) command runner

## Install

### From GitHub Releases

Download the latest binaries from the [Releases](https://github.com/urmzd/zigbee-skill/releases) page:

| Platform | API Server | CLI |
|----------|-----------|-----|
| Linux amd64 | `zigbee-skill-api-linux-amd64` | `zigbee-skill-cli-linux-amd64` |
| Linux arm64 | `zigbee-skill-api-linux-arm64` | `zigbee-skill-cli-linux-arm64` |
| macOS amd64 | `zigbee-skill-api-darwin-amd64` | `zigbee-skill-cli-darwin-amd64` |
| macOS arm64 | `zigbee-skill-api-darwin-arm64` | `zigbee-skill-cli-darwin-arm64` |

### From Source

```bash
git clone https://github.com/urmzd/zigbee-skill.git
cd zigbee-skill
just build
```

## Quick Start

1. Find your Zigbee adapter's serial port:
   ```bash
   # macOS — look for "Sonoff Zigbee" in the output
   ioreg -p IOUSB -l | grep -B 2 -A 8 '"USB Product Name"'

   # Common paths
   ls /dev/cu.usbserial-*   # macOS (Sonoff V2)
   ls /dev/ttyUSB*           # Linux
   ```

2. Start the daemon (keeps the Zigbee connection alive):
   ```bash
   zigbee-skill daemon start --port /dev/cu.usbserial-XXXX
   ```

3. Pair a device (put it in pairing mode first):
   ```bash
   zigbee-skill discovery start --wait-for 1
   ```

4. Control it:
   ```bash
   zigbee-skill devices list
   zigbee-skill devices set bedroom-lamp --state ON --brightness 150
   zigbee-skill devices state bedroom-lamp | jq '.state'
   ```

5. Stop the daemon when done:
   ```bash
   zigbee-skill daemon stop
   ```

## CLI

All output is JSON to stdout. Errors and progress go to stderr.

### Daemon

The daemon keeps the Zigbee serial connection alive in the background. When running, all CLI commands route through it automatically via a Unix socket.

```
zigbee-skill daemon start [--port <path>]   Start background daemon
zigbee-skill daemon stop                    Stop the daemon
zigbee-skill daemon status                  Check if daemon is running
```

Without the daemon, each command opens and closes the serial connection, which means devices must rejoin every time. **The daemon is recommended for normal use.**

### Devices

```
zigbee-skill devices list                          List all paired devices
zigbee-skill devices get <id>                      Get device details
zigbee-skill devices rename <id> --name <name>     Rename a device
zigbee-skill devices remove <id> [--force]         Remove a device
zigbee-skill devices clear                         Remove all devices
zigbee-skill devices state <id>                    Get device state
zigbee-skill devices set <id> --state ON           Set device state
```

### Discovery

```
zigbee-skill discovery start [--duration 120] [--wait-for 1]  Start pairing mode
zigbee-skill discovery stop                                   Stop pairing mode
```

`--wait-for N` blocks until N devices join, then stops discovery automatically.

### Network

```
zigbee-skill network reset    Clear Zigbee network (forms fresh on next start)
```

Use this when devices join but can't communicate, or when switching adapters. All devices must be factory-reset and re-paired after a network reset.

### Other

```
zigbee-skill health           Check controller health
```

### Global Flags

```
--config <path>   Config file path (default: ./zigbee-skill.yaml)
--port <path>     Zigbee serial port (overrides config file)
--socket <path>   Daemon Unix socket (default: /tmp/zigbee-skill.sock)
--pid <path>      Daemon PID file (default: /tmp/zigbee-skill.pid)
--log <path>      Daemon log file (default: /tmp/zigbee-skill.log)
```

## Troubleshooting

### Device joins but keeps blinking / doesn't respond

The device didn't complete the security key exchange. Reset the network and re-pair:

```bash
zigbee-skill daemon stop
zigbee-skill network reset
zigbee-skill daemon start --port /dev/cu.usbserial-XXXX
# Factory-reset the device (hold button ~10s), then:
zigbee-skill discovery start --wait-for 1
```

### Wrong serial port

If you have multiple USB-serial adapters, make sure you're using the EZSP-compatible one (Sonoff V2 / EFR32). A CP210x device (`/dev/cu.SLAB_USBtoUART`) may be a TI-based dongle that uses a different protocol. See [FAQ](docs/faq.md) for details.

### Device doesn't reconnect after restart

Devices loaded from config have no active network address until they rejoin. Use the daemon to keep the connection alive, or power-cycle the device to trigger a rejoin.

See [docs/faq.md](docs/faq.md) for more troubleshooting.

## Agent Integration

AI agents can control devices via the CLI (preferred) or REST API:

- **[AGENTS.md](AGENTS.md)** — Full CLI and API reference. Supported by 20+ AI coding tools.
- **[skills/zigbee-skill](skills/zigbee-skill/SKILL.md)** — Claude Code skill for device control.

## Configuration

Configuration is stored in `zigbee-skill.yaml` (current directory by default, override with `--config`). Paired devices and the serial port are persisted automatically.

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌──────────────┐     ┌────────────────┐
│     CLI     │────▶│    Daemon    │────▶│  EZSP Layer  │────▶│ Zigbee Devices │
│  (commands) │◀────│ (Unix socket)│◀────│  (Serial)    │◀────│                │
└─────────────┘     └──────────────┘     └──────────────┘     └────────────────┘
                           │
                           ▼
                    ┌─────────────┐
                    │    YAML     │
                    │  (Config)   │
                    └─────────────┘
```

The daemon holds the serial connection and Zigbee network state in memory. CLI commands communicate with the daemon over a Unix socket. Without the daemon, the CLI connects directly to the serial port (but devices must rejoin each time).

## Agent Skill

This repo's conventions are available as portable agent skills in [`skills/`](skills/).

## Specifications

This project implements the Zigbee PRO stack. The relevant specs are available (free, registration required) from the [CSA specifications page](https://csa-iot.org/developer-resource/specifications-download-request/):

- **Zigbee Specification R23.2** — Core protocol: network layer, APS, ZDO, security
- **PRO Base Device Behavior v3.1** — Zigbee 3.0 commissioning, trust center, network steering
- **Zigbee Cluster Library R8** — ZCL cluster/command definitions (On/Off, Level Control, etc.)

## Supported Devices

<!-- embed-src src="docs/supported-devices.md" -->
| Manufacturer | Model | Model ID | Type | Clusters | Notes |
|---|---|---|---|---|---|
| Third Reality | Smart Plug Gen2 | 3RSP019BZ | switch | On/Off (0x0006) | Ships in BLE mode — hold button 5s to switch to Zigbee. Factory reset: hold 10s. |
| SONOFF | Zigbee 3.0 USB Dongle Plus | — | coordinator | — | EFR32MG21 based. EZSP protocol. Used as the Zigbee coordinator. |
| Sylvania | A19 70052 | — | light | On/Off, Level Control | — |
<!-- /embed-src -->
