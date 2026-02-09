# Hom-ai

Pronounced "homie" - a local-first, privacy-focused smart home control system. Manage your Zigbee devices through a REST API without cloud dependencies or privacy concerns.

## Features

- Local Zigbee device control via Zigbee2MQTT
- REST API for device management
- Real-time device events via Server-Sent Events (SSE)
- Multi-profile support for multiple installations
- Encrypted credential storage
- Swagger API documentation

## Prerequisites

- Go 1.24+
- Docker & Docker Compose
- Zigbee 3.0 USB adapter (e.g., SONOFF Zigbee 3.0 USB Dongle Plus)
- [just](https://github.com/casey/just) command runner

## Quick Start

1. Clone the repository:
   ```bash
   git clone https://github.com/urmzd/homai.git
   cd homai
   ```

2. Set environment variables:
   ```bash
   export MQTT_USER=your_username
   export MQTT_PASSWORD=your_password
   ```

3. Start the services:
   ```bash
   just up
   ```

4. In a new terminal, run the API:
   ```bash
   just api
   ```

5. Access the API at http://localhost:8080

## API Endpoints

### Health
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/api/v1/health` | Health check (versioned) |

### Bridge
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/bridge/status` | Get Zigbee2MQTT bridge status |

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

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `MQTT_USER` | MQTT broker username | `root` |
| `MQTT_PASSWORD` | MQTT broker password | `pass` |
| `ZIGBEE_DEVICE_PATH` | Path to Zigbee USB adapter | `/dev/ttyUSB0` |
| `TZ` | Timezone | `America/Toronto` |
| `CREDENTIALS_PASSPHRASE` | Encryption key for stored credentials | - |

### Database

Configuration is stored in SQLite at `~/.config/homai/homai.db`

## Development Commands

| Command | Description |
|---------|-------------|
| `just api` | Run the API server |
| `just up` | Start Docker services (foreground) |
| `just up-detached` | Start Docker services (background) |
| `just down` | Stop and remove Docker services |
| `just swagger` | Generate Swagger documentation |
| `just devices` | List all paired Zigbee devices |
| `just permit-join` | Enable device pairing (default: 120s) |
| `just permit-join-off` | Disable device pairing |
| `just discover` | Discover nearby Zigbee devices |
| `just db` | Open database in sqlite3 |
| `just db-reset` | Delete the database |
| `just build` | Build all binaries |
| `just clean` | Remove built binaries |

## Architecture

```
┌─────────────┐     ┌──────┐     ┌─────────────┐     ┌────────────────┐
│  API Server │────▶│ MQTT │────▶│ Zigbee2MQTT │────▶│ Zigbee Devices │
│   (Go/Gin)  │◀────│      │◀────│             │◀────│                │
└─────────────┘     └──────┘     └─────────────┘     └────────────────┘
       │
       ▼
┌─────────────┐
│   SQLite    │
│  (Config)   │
└─────────────┘
```

## Tested Hardware

- SONOFF Zigbee 3.0 USB Dongle Plus
- Sylvania A19 70052
