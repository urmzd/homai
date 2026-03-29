# Zigbee Protocol Overview

This document explains how Zigbee works from the ground up — the protocol layers, mesh networking, device roles, the cluster model, and how state is persisted. It is written in the context of this project (zigbee-skill) which implements a Zigbee PRO coordinator using EZSP over a Silicon Labs NCP.

## 1. Protocol Stack

Zigbee is a layered protocol built on top of IEEE 802.15.4 radios operating at 2.4 GHz:

```
┌─────────────────────────────────────────────┐
│              Application Layer              │
│       (your code, device logic, CLI)        │
├─────────────────────────────────────────────┤
│   Zigbee Cluster Library (ZCL)              │
│   Standardized clusters: On/Off, Level,     │
│   Color, Temperature, etc.                  │
├─────────────────────────────────────────────┤
│   Application Support Sub-layer (APS)       │
│   Endpoint addressing, binding, groups,     │
│   APS-level encryption & acknowledgement    │
├─────────────────────────────────────────────┤
│   Zigbee Network Layer (NWK)                │
│   Mesh routing, network formation,          │
│   NWK-level encryption                      │
├─────────────────────────────────────────────┤
│   IEEE 802.15.4 MAC / PHY                   │
│   2.4 GHz radio, CSMA-CA, 250 kbps,        │
│   16 channels (11–26)                       │
└─────────────────────────────────────────────┘
```

Each layer has a well-defined responsibility:

- **PHY/MAC (802.15.4)** — handles the radio: modulation, channel access (CSMA-CA), and single-hop frame transmission. Maximum frame size is 127 bytes, which constrains everything above it.
- **NWK** — forms and maintains the mesh network. Assigns 16-bit short addresses, manages routing tables, and handles multi-hop message relay. Applies network-layer encryption (NWK key).
- **APS** — provides endpoint-level addressing (a single device can expose multiple endpoints, like a power strip with 4 sockets). Handles group addressing, binding tables, and optional APS-layer encryption (link keys). APS retries with acknowledgement (APS ACK) ensure reliable delivery.
- **ZCL** — defines a standardized data model. Devices expose *clusters* with *attributes* and *commands* so that a light from vendor A can be controlled by a switch from vendor B without custom integration.

## 2. Mesh Network Topology

### Device Roles

Every device on a Zigbee network has one of three roles:

| Role | Routes messages? | Sleeps? | Example |
|------|-----------------|---------|---------|
| **Coordinator** | Yes | No | USB dongle running this project |
| **Router** | Yes | No | Smart plug, mains-powered light |
| **End Device** | No | Yes | Battery sensor, remote control |

- **Coordinator** — there is exactly one per network. It forms the network (picks the PAN ID, channel, and network key), acts as the Trust Center for security, and participates in routing. In this project, the Sonoff USB dongle is the coordinator.
- **Routers** — any mains-powered device typically acts as a router. Routers extend the mesh by relaying messages on behalf of other devices. The more routers you have, the larger and more resilient your network.
- **End Devices** — battery-powered devices that sleep most of the time to conserve power. They associate with a parent (router or coordinator) that buffers messages for them. The end device wakes periodically, polls its parent for pending messages, then goes back to sleep.

### How the Mesh Works

```
                 ┌──────────┐
          ┌──────│Coordinator│──────┐
          │      └──────────┘      │
          │                        │
     ┌────┴────┐             ┌─────┴───┐
     │ Router A│─────────────│ Router B│
     └────┬────┘             └────┬────┘
          │                       │
    ┌─────┴─────┐          ┌──────┴─────┐
    │End Device │          │  Router C  │
    │ (sensor)  │          └──────┬─────┘
    └───────────┘                 │
                           ┌─────┴──────┐
                           │ End Device │
                           │  (bulb)    │
                           └────────────┘
```

Messages hop through routers to reach their destination. If Router A goes offline, the network layer discovers an alternate route through Router B and Router C. This self-healing behavior is automatic — the NWK layer maintains routing tables and uses AODV (Ad-hoc On-demand Distance Vector) route discovery when needed.

Key mesh concepts:

- **PAN ID** — 16-bit identifier for the network. All devices on the same network share the same PAN ID.
- **Extended PAN ID** — 64-bit unique identifier. Prevents collisions when multiple networks use the same 16-bit PAN ID.
- **Channel** — one of 16 channels (11–26) in the 2.4 GHz band. The coordinator selects a channel at formation time, typically by running an energy scan to find the quietest channel.
- **Short address** — 16-bit network address assigned by the NWK layer when a device joins. Used for all in-network routing (more compact than the 64-bit IEEE/MAC address).
- **IEEE address (EUI-64)** — 64-bit globally unique hardware address burned into each device at the factory. Used for device identification and security key exchange.

## 3. Network Formation and Joining

### Formation (coordinator side)

1. **Energy scan** — the coordinator scans candidate channels and measures background noise. The BDB spec defines primary channels (11, 15, 20, 25) and secondary channels (the rest).
2. **Pick PAN ID and channel** — selects the quietest channel and a random PAN ID.
3. **Generate network key** — a 128-bit AES key that encrypts all NWK-layer traffic. Distributed to devices during joining via the Trust Center.
4. **Start beaconing** — the coordinator begins accepting join requests.

### Joining (device side)

1. **Beacon request** — the joining device scans channels for coordinators/routers advertising the network.
2. **Association request** — sends a join request to the chosen parent.
3. **Trust Center authorization** — the coordinator's Trust Center decides whether to admit the device. In this project, the policy is set via `ezspSetPolicy(TRUST_CENTER_POLICY)`.
4. **Key delivery** — the Trust Center sends the network key to the new device, encrypted with a well-known link key (for initial join) or a previously established key (for rejoins).
5. **Device announce** — the new device broadcasts its presence. The coordinator receives a `trustCenterJoinHandler` callback with the device's IEEE address.

After joining, the coordinator queries the device's **Simple Descriptor** to learn which endpoints and clusters it supports. This is how we discover that a device is a light (On/Off + Level Control clusters) vs. a sensor (Temperature + Humidity clusters).

## 4. Endpoints and Clusters

### Endpoints

A single physical device can expose multiple **endpoints** (1–240), each representing a logical sub-device. Think of a 4-gang power strip: it has one Zigbee radio but 4 endpoints, each with its own On/Off cluster.

- Endpoint **0** is reserved for the Zigbee Device Object (ZDO) — used for network management (device discovery, binding, etc.)
- Endpoints **1–240** are application endpoints
- Endpoint **242** is used for the Green Power proxy

### Clusters

A **cluster** is a standardized set of attributes and commands that describe a capability. Clusters are defined in the Zigbee Cluster Library (ZCL). Each endpoint declares which clusters it supports as either:

- **Server** (input) clusters — the device implements this capability (e.g., a light is an On/Off *server*)
- **Client** (output) clusters — the device consumes this capability (e.g., a switch is an On/Off *client*)

Common clusters implemented in this project:

| Cluster | ID | Purpose | Key Attributes |
|---------|------|---------|---------------|
| On/Off | 0x0006 | Binary on/off control | `OnOff` (bool) |
| Level Control | 0x0008 | Brightness dimming | `CurrentLevel` (uint8, 0–254) |
| Color Control | 0x0300 | Color temperature / RGB | `ColorTemperatureMireds`, `CurrentHue` |
| Temperature | 0x0402 | Temperature sensing | `MeasuredValue` (int16, 0.01 C) |
| Humidity | 0x0405 | Relative humidity | `MeasuredValue` (uint16, 0.01%) |
| Occupancy | 0x0406 | Motion detection | `Occupancy` (bitmap8) |
| Electrical Measurement | 0x0B04 | Power monitoring | `ActivePower`, `RMSVoltage` |
| Thermostat | 0x0201 | HVAC control | `LocalTemperature`, `OccupiedHeatingSetpoint` |
| Door Lock | 0x0101 | Lock/unlock | `LockState` (enum8) |
| Window Covering | 0x0102 | Blinds/shades | `CurrentPositionLiftPercentage` |

### ZCL Frame Structure

Every ZCL message has a common header:

```
┌───────────────┬───────────┬───────────┬─────────┐
│ Frame Control │ Seq Number│ Command ID│ Payload │
│   (1 byte)    │ (1 byte)  │ (1 byte)  │  (var)  │
└───────────────┴───────────┴───────────┴─────────┘
```

- **Frame Control** — encodes the frame type (global vs. cluster-specific), direction (client-to-server or server-to-client), and whether the manufacturer code is present.
- **Sequence Number** — monotonically increasing counter for matching responses to requests.
- **Command ID** — for global commands: Read Attributes (0x00), Write Attributes (0x02), Configure Reporting (0x06), etc. For cluster-specific commands: Off (0x00), On (0x01), Toggle (0x02) in the On/Off cluster.

### Attributes and Commands

**Attributes** are named, typed values that represent device state. You read/write them with global ZCL commands:

```
Read Attributes Request:
  Cluster: 0x0006 (On/Off)
  Attribute IDs: [0x0000]
  →  Response: OnOff = true (boolean)
```

**Commands** are actions you invoke on a cluster:

```
Cluster-Specific Command:
  Cluster: 0x0006 (On/Off)
  Command: 0x01 (On)
  →  The light turns on
```

The distinction matters: reading the `OnOff` attribute tells you the *current* state; sending the `On` command *changes* it.

### Attribute Reporting

Instead of polling, devices can be configured to **report** attribute changes automatically. You send a Configure Reporting command specifying:

- **Minimum interval** — don't report more often than this (prevents flooding)
- **Maximum interval** — report at least this often even if unchanged (heartbeat)
- **Reportable change** — minimum value delta to trigger a report (e.g., 1 degree)

The device then sends unsolicited Report Attributes messages whenever the criteria are met. This is how sensors push temperature/humidity readings without being polled.

## 5. Addressing and Message Delivery

When this project sends a command to a device, the full addressing path is:

```
Source:  Coordinator (NodeID 0x0000), Endpoint 1, Profile 0x0104 (HA)
   │
   ▼
Destination:  NodeID 0x1234, Endpoint 1, Cluster 0x0006
   │
   ▼
NWK Layer:  Route via mesh (may hop through routers)
   │
   ▼
APS Layer:  APS ACK requested (EMBER_APS_OPTION_RETRY)
   │
   ▼
Target Device:  Endpoint 1 server processes On/Off command
```

The combination of **NodeID + Endpoint + Cluster** uniquely identifies the destination capability. APS retries (enabled via `EMBER_APS_OPTION_RETRY`) ensure that transient radio failures don't lose the command — the APS layer retransmits until it receives an ACK or times out.

## 6. Security

Zigbee uses two layers of AES-128 encryption:

- **Network key** — encrypts all NWK-layer frames. Every device on the network shares this key. Protects against external eavesdropping.
- **Link key** — encrypts APS-layer frames between two specific devices. Used for Trust Center communication and optionally for application-level security between device pairs.

The **Trust Center** (coordinator) manages key distribution:

1. On initial join, the TC sends the network key encrypted with a well-known "default link key" (`ZigbeeAlliance09`). This is the weakest point — anyone within radio range during a join can capture the key.
2. For subsequent communication, the TC can establish unique link keys per device.
3. Install codes (Zigbee 3.0) provide a way to pre-share a unique key with each device, avoiding the well-known key vulnerability.

## 7. State Persistence

State is persisted at two different levels:

### NCP Flash (Radio Firmware)

The Silicon Labs NCP stores network state in its own flash memory:

- Network parameters (PAN ID, extended PAN ID, channel, network key)
- The coordinator's node ID and IEEE address
- Neighbor table, child table, routing table
- Security keys and frame counters

This means the Zigbee network survives application restarts. When `networkInit` is called on startup, the NCP resumes the existing network from flash. Previously paired devices reconnect automatically (typically within ~30 seconds).

### Application Config (`zigbee-skill.yaml`)

The host application persists device metadata that the NCP doesn't track:

```yaml
devices:
  - ieee_address: "00:11:22:33:44:55:66:77"
    friendly_name: "bedroom-lamp"
    type: "light"
    endpoint: 1
    clusters: [6, 8]       # On/Off, Level Control
    last_seen: 2026-03-25T10:30:00Z
```

This includes:

- **IEEE address** — the device's unique hardware identifier (the NCP knows this, but the mapping to friendly names is application-level)
- **Friendly name** — user-assigned label ("bedroom-lamp")
- **Device type** — classification derived from cluster analysis ("light", "sensor", "plug")
- **Endpoint and clusters** — cached from the Simple Descriptor query at join time
- **Last seen** — timestamp of last communication

On startup, the application loads this file and pre-populates its in-memory device map. Devices have `NodeID=0` until they rejoin and the NCP assigns them a fresh short address.

### What is NOT Persisted on the Host

- **Device state** (on/off, brightness, temperature readings) — this is volatile. The application queries the device for current state on demand via ZCL Read Attributes.
- **Routing tables** — managed entirely by the NCP.
- **Security frame counters** — managed by the NCP. Resetting these (e.g., by reflashing the NCP) breaks security and requires all devices to rejoin.

## 8. Putting It All Together

Here is the full path of turning on a light, from CLI command to radio frame:

```
1. CLI:        zigbee-skill devices set bedroom-lamp --state ON
2. REST API:   POST /api/v1/devices/{id}/state  {"on": true}
3. Controller: Look up device by friendly name → IEEE addr + NodeID + endpoint
4. ZCL:        Build On/Off cluster command (cluster 0x0006, cmd 0x01)
5. EZSP:       sendUnicast(nodeID, endpoint, cluster, zclFrame)
6. ASH:        Frame the EZSP command with CRC, byte stuffing, data randomization
7. UART:       Transmit over serial to the NCP
8. NCP:        Encrypts with NWK key, routes through mesh, waits for APS ACK
9. Device:     Receives frame, decrypts, processes On command, turns on LED
10. APS ACK:   Device sends acknowledgement back through mesh
11. NCP:       Reports messageSentHandler callback with success/failure
```

## References

- **Zigbee Specification R23.2** — `specs/csa-iot/docs-06-3474-23-csg-zigbee-specificationR23.2_clean.pdf`
- **PRO Base Device Behavior v3.1** — `specs/csa-iot/22-65816-030-PRO-BDB-v3.1-Specification.pdf`
- **EZSP Reference (UG100)** — Silicon Labs
- **ASH Protocol (UG101)** — Silicon Labs
- **Zigbee Cluster Library R8** — CSA-IoT
