# Hom-ai

Pronounced "homie" - a local-first, privacy-focused smart home control system. Manage your Zigbee devices through a REST API without cloud dependencies or privacy concerns.

## Features

- Direct Zigbee device control via EZSP serial protocol (no Zigbee2MQTT/MQTT required)
- REST API for device management
- Real-time device events via Server-Sent Events (SSE)
- Multi-profile support for multiple installations
- MCP server for AI assistant integration
- Swagger API documentation

## Prerequisites

- Go 1.24+
- Zigbee 3.0 USB adapter (e.g., SONOFF Zigbee 3.0 USB Dongle Plus)
- [just](https://github.com/casey/just) command runner

## Quick Start

1. Clone the repository:
   ```bash
   git clone https://github.com/urmzd/homai.git
   cd homai
   ```

2. Build the binaries:
   ```bash
   just build
   ```

3. Run the API server:
   ```bash
   ./bin/api --port /dev/cu.SLAB_USBtoUART
   ```

4. (Optional) Run the MCP server:
   ```bash
   ./bin/mcp
   ```

5. Access the API at http://localhost:8080

## Binaries

| Binary | Description |
|--------|-------------|
| `bin/api` | REST API server for device management and Zigbee control |
| `bin/mcp` | MCP server for AI assistant integration (stdio transport) |

## API Endpoints

### Health
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/api/v1/health` | Health check (versioned) |

### Devices
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/devices` | List all paired devices |
| GET | `/api/v1/devices/:id` | Get device details |
| PATCH | `/api/v1/devices/:id` | Rename a device |
| DELETE | `/api/v1/devices/:id` | Remove a device |

### Control
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/devices/:id/state` | Get device state |
| POST | `/api/v1/devices/:id/state` | Set device state |

### Discovery
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/discovery/start` | Start device pairing mode |
| POST | `/api/v1/discovery/stop` | Stop device pairing mode |
| GET | `/api/v1/discovery/events` | SSE stream of discovery events |

### Documentation
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/swagger/*` | Swagger UI |

## Configuration

### Database

Configuration is stored in SQLite at `~/.config/homai/homai.db`

## Development Commands

| Command | Description |
|---------|-------------|
| `just build` | Build all binaries to `bin/` |
| `just clean` | Remove `dist/` directory |
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
│   SQLite    │     │  MCP Server │
│  (Config)   │     │  (stdio)    │
└─────────────┘     └─────────────┘
```

## Tested Hardware

- SONOFF Zigbee 3.0 USB Dongle Plus
- Sylvania A19 70052
