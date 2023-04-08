# Sunlight Lamp

## Prerequisities

```bash
# install docker compose
# install aws-cli
# install cdk
```

Tested Using:
- Zigbee 3.0 USB Dongle Plus
- Syvania A19 70052

## Usage

```bash
# Run Dependencies
docker compose up

# CLI
./sunrise-lamp increase
```

## Notes

- In the morning, it should go from current brightness to max (as sun rises): it increments in sizes of 255 / civil twilight duration.
- While the sun sets, turn the lights on.
- We can send text messages to turn it on or off.

