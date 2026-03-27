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

1. Start the API server:
   ```bash
   ./bin/api --port /dev/cu.SLAB_USBtoUART
   ```

2. Use the CLI to control devices:
   ```bash
   zigbee-skill devices list
   zigbee-skill devices set bedroom-lamp --state ON --brightness 150
   zigbee-skill devices state bedroom-lamp | jq '.state'
   ```

## CLI

All output is JSON to stdout. Errors go to stderr.

```
zigbee-skill health                                Check API server health
zigbee-skill devices list                          List all paired devices
zigbee-skill devices get <id>                      Get device details
zigbee-skill devices rename <id> --name <name>     Rename a device
zigbee-skill devices remove <id>                   Remove a device
zigbee-skill devices state <id>                    Get device state
zigbee-skill devices set <id> --state ON           Set device state
zigbee-skill discovery start [--duration 120]      Start pairing mode
zigbee-skill discovery stop                        Stop pairing mode
```

Use `--address <url>` to target a different API server (default: `http://localhost:8080`).

## API

See [AGENTS.md](AGENTS.md) for the full endpoint reference.

| Action | Method | Path |
|--------|--------|------|
| List devices | GET | `/api/v1/devices` |
| Get device | GET | `/api/v1/devices/{id}` |
| Rename device | PATCH | `/api/v1/devices/{id}` |
| Remove device | DELETE | `/api/v1/devices/{id}` |
| Get state | GET | `/api/v1/devices/{id}/state` |
| Set state | POST | `/api/v1/devices/{id}/state` |
| Start pairing | POST | `/api/v1/discovery/start` |
| Stop pairing | POST | `/api/v1/discovery/stop` |
| Health check | GET | `/api/v1/health` |

## Agent Integration

AI agents can control devices via the CLI (preferred) or REST API:

- **[AGENTS.md](AGENTS.md)** — Full CLI and API reference. Supported by 20+ AI coding tools.
- **[skills/zigbee-skill](skills/zigbee-skill/SKILL.md)** — Claude Code skill for device control.

## Configuration

Configuration is stored in SQLite at `~/.config/zigbee-skill/zigbee-skill.db`.

## Development

| Command | Description |
|---------|-------------|
| `just build` | Build all binaries to `bin/` |
| `just clean` | Remove build artifacts |
| `just test` | Run all tests |
| `just lint` | Run gofmt, golangci-lint, go vet |
| `just check` | Run lint + test (default) |
| `just swagger` | Generate Swagger documentation |
| `just run` | Run API server with live reload (air) |
| `just open-db` | Open database in sqlite3 |
| `just reset-db` | Delete the database |

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌────────────────┐
│  API Server │────▶│  EZSP Layer  │────▶│ Zigbee Devices │
│   (Go/Gin)  │◀────│  (Serial)    │◀────│                │
└─────────────┘     └──────────────┘     └────────────────┘
       │
       ▼
┌─────────────┐     ┌─────────────┐
│   SQLite    │     │     CLI     │
│  (Config)   │     │  (HTTP→API) │
└─────────────┘     └─────────────┘
```

## Agent Skill

This repo's conventions are available as portable agent skills in [`skills/`](skills/).

## Specifications

This project implements the Zigbee PRO stack. The relevant specs are available (free, registration required) from the [CSA specifications page](https://csa-iot.org/developer-resource/specifications-download-request/):

- **Zigbee Specification R23.2** — Core protocol: network layer, APS, ZDO, security
- **PRO Base Device Behavior v3.1** — Zigbee 3.0 commissioning, trust center, network steering
- **Zigbee Cluster Library R8** — ZCL cluster/command definitions (On/Off, Level Control, etc.)

## Tested Hardware

- SONOFF Zigbee 3.0 USB Dongle Plus
- Sylvania A19 70052
