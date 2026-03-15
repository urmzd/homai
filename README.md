# Zigbee REST

A local-first, privacy-focused smart home control system. Manage your Zigbee devices through a REST API and CLI without cloud dependencies.

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

Download the latest binaries from the [Releases](https://github.com/urmzd/zigbee-rest/releases) page:

| Platform | API Server | CLI |
|----------|-----------|-----|
| Linux amd64 | `zigbee-rest-api-linux-amd64` | `zigbee-rest-cli-linux-amd64` |
| Linux arm64 | `zigbee-rest-api-linux-arm64` | `zigbee-rest-cli-linux-arm64` |
| macOS amd64 | `zigbee-rest-api-darwin-amd64` | `zigbee-rest-cli-darwin-amd64` |
| macOS arm64 | `zigbee-rest-api-darwin-arm64` | `zigbee-rest-cli-darwin-arm64` |

### From Source

```bash
git clone https://github.com/urmzd/zigbee-rest.git
cd zigbee-rest
just build
```

## Quick Start

1. Start the API server:
   ```bash
   ./bin/api --port /dev/cu.SLAB_USBtoUART
   ```

2. Use the CLI to control devices:
   ```bash
   zigbee-rest devices list
   zigbee-rest devices set bedroom-lamp --state ON --brightness 150
   zigbee-rest devices state bedroom-lamp | jq '.state'
   ```

## CLI

All output is JSON to stdout. Errors go to stderr.

```
zigbee-rest health                                Check API server health
zigbee-rest devices list                          List all paired devices
zigbee-rest devices get <id>                      Get device details
zigbee-rest devices rename <id> --name <name>     Rename a device
zigbee-rest devices remove <id>                   Remove a device
zigbee-rest devices state <id>                    Get device state
zigbee-rest devices set <id> --state ON           Set device state
zigbee-rest discovery start [--duration 120]      Start pairing mode
zigbee-rest discovery stop                        Stop pairing mode
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
- **[skills/zigbee-rest](skills/zigbee-rest/SKILL.md)** — Claude Code skill for device control.

## Configuration

Configuration is stored in SQLite at `~/.config/zigbee-rest/zigbee-rest.db`.

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

## Tested Hardware

- SONOFF Zigbee 3.0 USB Dongle Plus
- Sylvania A19 70052
