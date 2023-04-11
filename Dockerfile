# Use the official Mosquitto image as the base image
FROM eclipse-mosquitto:latest

# Set the working directory
WORKDIR /mosquitto

# Copy configuration file
COPY configs/mosquitto/config/mosquitto.conf /mosquitto/config/mosquitto.conf

COPY ./entrypoint.sh ./entrypoint.sh

# Set the entrypoint
ENTRYPOINT ["./entrypoint.sh"]
