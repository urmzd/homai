# AGENTS.md

## Project Overview

Zigbee Skill is an AI-native smart home tool written in Go. It gives AI agents direct control over Zigbee devices via EZSP serial protocol, enabling homes that adapt to user preferences through natural interaction. No cloud, no MQTT broker.

**Key tech:** Go 1.24, EZSP v13 (EFR32MG21), `just` task runner

**Hardware:** Sonoff Zigbee 3.0 USB Dongle Plus V2 at `/dev/cu.SLAB_USBtoUART`

## Specification References

All protocol work MUST be verified against these specs before implementation:

### BDB 3.1 — `specs/csa-iot/22-65816-030-PRO-BDB-v3.1-Specification.pdf`
Base Device Behavior. Governs coordinator initialization, commissioning, and security.

| Section | Topic | Key details |
|---------|-------|-------------|
| 5.6 | Trust Center configuration | TC policy table (Table 9), backwards compatibility mode |
| 5.6.1 | TC policies | `requireInstallCodesOrPresetPassphrase` default=0x02, `allowRejoinsWithWellKnownKey` default=FALSE |
| 5.6.2 | Backwards compat mode | Set `requireInstallCodesOrPresetPassphrase`=0x01 + `allowRejoinsWithWellKnownKey`=TRUE for legacy devices |
| 5.6.3 | Join policy | IEEELIST_JOIN (0x01) requires `mibJoiningIeeeList` maintenance |
| 5.6.4 | Joining using DLK | DLK only with install code or unique passphrase; reject if no `apsDeviceKeyPairSet` entry |
| 5.6.5 | Symmetric key joiner timeout | TC MUST timeout join within `bdbcfTrustCenterNodeJoinTimeout` seconds |
| 6.4 | Minimum requirements | R23+ stack, profile 0x0104 on all endpoints, simple descriptors |
| 6.5 | Default reporting config | Max reporting interval 0x003d–0xfffe for mandatory reportable attributes |
| 6.10 | APS ACK and security | If command received with APS security, response MUST also use APS security |
| 7.1 | Initialization procedure | Restore persistent data → rejoin → Device_annce → TCLK update |
| 7.2 | On-network TCLK update | Nodes MUST obtain unique verified TC link key after join |
| 7.3 | Ensuring TC connectivity | Keep Alive cluster (server) + Poll Control cluster (client) required on TC |
| 8.1 | Network formation | Energy scan before forming; select quietest channel |
| 9.2 | Symmetric key exchange | Key negotiation flow for devices joining via well-known key |

### Zigbee R23.2 — `specs/csa-iot/docs-06-3474-23-csg-zigbee-specificationR23.2_clean.pdf`
Core Zigbee specification. Covers APS, NWK, security, ZDO, and ZCL layers.

Relevant chapters (large document — use targeted page reads):
- **Chapter 2** — Application layer (APS frame format, endpoints, profiles, descriptors)
- **Chapter 3** — Network layer (routing, joining, address assignment)
- **Chapter 4** — Security (network key, link keys, frame counters, trust center)
- **Annex G** — Inter-PAN

### EZSP (not in repo — Silicon Labs docs)
EmberZNet Serial Protocol. The host↔NCP wire protocol.

| Concept | Details |
|---------|---------|
| Frame format | Legacy (v7-) vs Extended (v8+); extended after successful version negotiation |
| `addEndpoint` (0x0002) | MUST register HA endpoints before NetworkInit or device responses are silently dropped |
| `sendUnicast` (0x0034) | APS frame options: 0x0020=ENCRYPTION, 0x0040=RETRY, 0x0100=ROUTE_DISCOVERY |
| `sendBroadcast` (0x0036) | Used for ZDO broadcasts (Device_annce, permit join) |
| Trust Center Join Handler | `EmberDeviceUpdate`: 0=SECURED_REJOIN, 1=UNSECURED_JOIN, 2=DEVICE_LEFT, 3=UNSECURED_REJOIN |
| Status codes | 0x00=SUCCESS, 0x66=NO_BUFFERS, 0x84=MAX_MESSAGE_LIMIT_REACHED |

## Architecture

```
cmd/cli/main.go          CLI binary (daemon mode or direct)
pkg/app/                  App initialization, wires controller+events+validator
pkg/config/               YAML config (zigbee-skill.yaml)
pkg/daemon/               Background daemon (unix socket server/client)
pkg/device/               Controller interface, Device model, NullController
pkg/device/schema/        JSON schema validator for device state
pkg/zigbee/controller.go  Zigbee coordinator: init, join, discovery, state read/write
pkg/zigbee/ezsp.go        EZSP layer: ASH framing, command/response, callbacks
pkg/zigbee/ash.go         ASH serial transport (RST, DATA, ACK, NAK, ERROR)
pkg/zigbee/zcl.go         ZCL frame encoding/decoding (global + cluster-specific)
pkg/zigbee/serial.go      Serial port abstraction
```

### Message flow: host → device
1. CLI calls `Controller.SetDeviceState()` (or `GetDeviceState()`)
2. Controller builds ZCL frame via `zcl.go` (`BuildOnOffCommand` / `BuildReadAttributesCommand`)
3. Controller calls `ezsp.SendUnicast()` which wraps in APS frame + EZSP command
4. EZSP layer sends via ASH DATA frame over serial
5. NCP transmits over-the-air; device responds
6. ASH read loop → EZSP `readLoop` → callback dispatch → `handleIncomingMessage` → `updateDeviceStateFromZCL`

### Known issues (active)
- Device ZCL Read Attributes gets no response — likely `addEndpoint` not called or NCP buffer full (status 0x84)
- TC join status=1 (UNSECURED_JOIN) means key exchange incomplete; device keeps re-joining
- `SetDeviceState` returns optimistic state, not confirmed device state

## Setup

```bash
# Build with version
go build -ldflags "-X main.version=$(git rev-parse --short HEAD)" -o zigbee-skill ./cmd/cli/

# Or via just
just build
```

## CLI

The CLI outputs JSON to stdout and logs/errors to stderr. It auto-routes through the daemon when running.

```bash
zigbee-skill health
zigbee-skill daemon start|stop|status
zigbee-skill devices list|get|rename|remove|clear|state|set
zigbee-skill discovery start [--duration 120] [--wait-for N]
zigbee-skill discovery stop
```

`<id>` is a device's IEEE address or friendly name.

### Examples

```bash
zigbee-skill daemon start --port /dev/cu.SLAB_USBtoUART
zigbee-skill devices list | jq '.devices[].friendly_name'
zigbee-skill devices set smart-plug --state ON
zigbee-skill devices state smart-plug
zigbee-skill devices clear
zigbee-skill daemon stop
```

### Response shapes

**Device state:** `{"device": "name", "state": {"state": "ON"}, "timestamp": "..."}`

**Errors:** exit code 1, message to stderr

## Development

```bash
just check        # lint + test (default)
just test         # go test ./...
just lint         # gofmt, golangci-lint, go vet
just clean        # remove bin/
```

## Code Style

- Standard Go conventions: `gofmt`, `go vet`, `golangci-lint`
- Package layout: `cmd/` for binaries, `pkg/` for libraries
- Version-based EZSP interfaces (user preference — see memory)

## CI

- CI runs on push/PR via `.github/workflows/ci.yml`
- Release builds cross-platform binaries via `.github/workflows/release.yml` on push to `main`
- Platforms: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`
