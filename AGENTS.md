# AGENTS.md

## Project Overview

Zigbee REST is a local-first smart home controller written in Go. It controls Zigbee devices directly via EZSP serial protocol and exposes a REST API + CLI. No cloud, no MQTT broker.

**Key tech:** Go 1.24, Gin, SQLite, EZSP/Zigbee, `just` task runner

## Setup

```bash
# Build all binaries (outputs to bin/)
just build

# Or build manually
go build -o bin/api ./cmd/api
go build -o bin/cli ./cmd/cli
```

## Running

```bash
# Start API server (default: 0.0.0.0:8080)
./bin/api --port /dev/cu.SLAB_USBtoUART

# Custom database path (default: ~/.config/zigbee-rest/zigbee-rest.db)
./bin/api --db /path/to/zigbee-rest.db --port /dev/ttyUSB0
```

The database is auto-created and migrated on first run.

## CLI

The CLI outputs JSON to stdout and errors to stderr. It talks to the running API server.

```bash
zigbee-rest health
zigbee-rest devices list
zigbee-rest devices get <id>
zigbee-rest devices rename <id> --name <name>
zigbee-rest devices remove <id>
zigbee-rest devices state <id>
zigbee-rest devices set <id> --state ON --brightness 150
zigbee-rest discovery start [--duration 120]
zigbee-rest discovery stop
```

`<id>` is a device's IEEE address or friendly name.

Use `--address <url>` to target a different API server (default: `http://localhost:8080`).

### Examples

```bash
# List device names
zigbee-rest devices list | jq '.devices[].friendly_name'

# Turn on a light
zigbee-rest devices set bedroom-lamp --state ON

# Set brightness
zigbee-rest devices set bedroom-lamp --state ON --brightness 150

# Turn off
zigbee-rest devices set bedroom-lamp --state OFF

# Get current state
zigbee-rest devices state bedroom-lamp | jq '.state'

# Pair a new device
zigbee-rest discovery start --duration 120
```

### Response shapes

**List devices:** `{"devices": [{...}], "count": N}` — each device has `ieee_address`, `friendly_name`, `type`, `state`

**Device state:** `{"device": "name", "state": {"state": "ON", "brightness": 200}, "timestamp": "..."}`

**Errors:** `{"error": "error_code", "message": "..."}` — 400 (validation), 404 (not found), 504 (timeout)

### State properties

State objects are device-specific and validated against each device's JSON schema. Common light properties:

| Property | Type | Description |
|----------|------|-------------|
| `state` | `"ON"` / `"OFF"` | Power state |
| `brightness` | number | Brightness level (device-specific range) |

## Development

```bash
just check        # lint + test (default)
just test         # go test ./...
just lint         # gofmt, golangci-lint, go vet
just swagger      # regenerate swagger docs
just run          # live reload with air
just open-db      # open sqlite3 shell
just reset-db     # delete database file
just clean        # remove bin/
```

## Code Style

- Standard Go conventions: `gofmt`, `go vet`, `golangci-lint`
- Package layout: `cmd/` for binaries, `pkg/` for libraries
- `pkg/zigbee/` — EZSP serial protocol and Zigbee stack
- `pkg/api/` — Gin HTTP handlers and router
- `pkg/device/` — device abstraction layer
- `pkg/db/` — SQLite persistence and config

## CI

- CI runs on push/PR via `.github/workflows/ci.yml`
- Release builds cross-platform binaries via `.github/workflows/release.yml` on push to `main`
- Platforms: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`
