#!/bin/ash

touch /mosquitto/config/mosquitto_passwd && \
    mosquitto_passwd -b /mosquitto/config/mosquitto_passwd ${MQTT_USER} ${MQTT_PASSWORD}

# Pass the arguments to the mosquitto command
exec "$@"
