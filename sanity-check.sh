#!/usr/bin/env bash
set -euo pipefail

# Sanity check: run Zigbee2MQTT with the provided device mapping.
# Usage: ./sanity-check.sh

DEFAULT_DEVICE="/dev/cu.SLAB_USBtoUART"
FALLBACK_DEVICE="/dev/tty.SLAB_USBtoUART"
DEVICE_PATH="${ZIGBEE_DEVICE_PATH:-$DEFAULT_DEVICE}"
if [ ! -e "$DEVICE_PATH" ]; then
  if [ -e "$FALLBACK_DEVICE" ] && [ -z "${ZIGBEE_DEVICE_PATH:-}" ]; then
    DEVICE_PATH="$FALLBACK_DEVICE"
  else
    echo "Device not found: $DEVICE_PATH" >&2
    echo "Set ZIGBEE_DEVICE_PATH to the correct device path." >&2
    exit 1
  fi
fi

if docker ps -a --format '{{.Names}}' | rg -qx 'zigbee2mqtt'; then
  echo "Removing existing zigbee2mqtt container..."
  docker rm -f zigbee2mqtt >/dev/null
fi

docker run \
  --name zigbee2mqtt \
  --restart=unless-stopped \
  --device="$DEVICE_PATH:/dev/ttyACM0" \
  -p 8080:8080 \
  -v "$(pwd)/data:/app/data" \
  -v /run/udev:/run/udev:ro \
  -e TZ=Europe/Amsterdam \
  ghcr.io/koenkk/zigbee2mqtt
