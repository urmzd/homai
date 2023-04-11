#!/usr/bin/sh

MQTT_USERNAME="$1"
MQTT_PASSWORD="$2"

touch /mosquitto/config/mosquitto_passwd && \
    mosquitto_passwd -b /mosquitto/config/mosquitto_passwd ${MQTT_USERNAME} ${MQTT_PASSWORD}

mosquitto -c /mosquitto/config/mosquitto.conf
