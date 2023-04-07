# Use the official Mosquitto image as the base image
FROM eclipse-mosquitto:latest

# Set the working directory
WORKDIR /mosquitto

# Add build arguments for the username and password
ARG MQTT_USERNAME
ARG MQTT_PASSWORD

# set the password
RUN touch /mosquitto/config/mosquitto_passwd && \
    mosquitto_passwd -b /mosquitto/config/mosquitto_passwd ${MQTT_USERNAME} ${MQTT_PASSWORD}

# Copy configuration file
COPY configs/mosquitto/config/mosquitto.conf /mosquitto/config/mosquitto.conf

# Set the entrypoint
ENTRYPOINT ["mosquitto", "-c", "/mosquitto/config/mosquitto.conf"]
