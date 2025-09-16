# UniFi Protect MQTT Bridge

A bridge between UniFi Protect and MQTT. Uses the official UniFi Protect API to subscribe to WebSocket endpoints and relay event data (camera events, alarms, etc.) to an MQTT broker.

## Usage

### Basic Usage

1. Create a `.env` file with your configuration or set environment variables directly:

```
MQTT_HOST=192.168.1.100
PROTECT_HOST=192.168.1.10
PROTECT_API_KEY=your-api-key-here
```

2. Build the application: `go build -o ./bin/unifi-protect-mqtt`
3. Run the application: `./bin/unifi-protect-mqtt`

### Command-line Flags

You can also use command-line flags to override environment variables:

```bash
unifi-protect-mqtt -MQTT_HOST=192.168.1.100 -PROTECT_HOST=192.168.1.10 -PROTECT_API_KEY=your-api-key-here
```

## Configuration

Use the following variables to configure the application (see `.env.example` for reference):

### UniFi Protect Settings

-   `PROTECT_HOST`: The hostname or IP address of your UniFi Protect system (default: `unifi`)
-   `PROTECT_PORT`: The port number (default: `443`)
-   `PROTECT_API_KEY`: Your UniFi Protect API key (required) (generated from the UniFi Protect web interface under `Settings > Control Plane > Integrations`)
-   `PROTECT_API_KEY_FILE`: Path to file containing the API key (optional)
-   `PROTECT_API_PATH`: API path (default: `proxy/protect/integration/v1`)
-   `PROTECT_TLS`: Set to "false" to disable TLS for UniFi Protect connection (default: `true`)
-   `PROTECT_TLS_VERIFY`: Set to "true" to enable TLS certificate verification (default: `false`)

### MQTT Broker Settings

-   `MQTT_HOST`: MQTT broker hostname (default: `mqtt`)
-   `MQTT_PORT`: MQTT broker port (default: `1883`)
-   `MQTT_USERNAME`: MQTT username (optional)
-   `MQTT_PASSWORD`: MQTT password (optional)
-   `MQTT_PASSWORD_FILE`: Path to file containing the MQTT password (optional)
-   `MQTT_CLIENT_ID`: MQTT client identifier (default: `unifi-protect-mqtt`)
-   `MQTT_TOPIC`: Prefix for MQTT topics (default: `unifi/protect`)
-   `MQTT_QOS`: MQTT Quality of Service level (default: `0`)
-   `MQTT_TLS`: Set to "true" to enable TLS for MQTT connection (default: `false`)

## MQTT Topics

The application publishes messages to the following topics:

-   `{MQTT_TOPIC}/meta/info` - UniFi Protect system information
-   `{MQTT_TOPIC}/cameras/{camera_id}` - Camera metadata
-   `{MQTT_TOPIC}/cameras/{camera_id}/snapshot` - Camera snapshot image data
-   `{MQTT_TOPIC}/cameras/{camera_id}/rtsps-stream` - Camera RTSPS stream URLs
-   `{MQTT_TOPIC}/subscribe/devices` - Device-related messages (disconnects, status changes, etc.)
-   `{MQTT_TOPIC}/subscribe/events` - Event-related messages (motion, alarms, etc.)

Publish to the following command topics to retrieve data from the UniFi Protect API:

-   `{MQTT_TOPIC}/command/meta/info` - Publishes UniFi Protect system information to `{MQTT_TOPIC}/meta/info`
-   `{MQTT_TOPIC}/command/cameras` - Publishes metadata for all cameras to `{MQTT_TOPIC}/cameras/{camera_id}`
-   `{MQTT_TOPIC}/command/cameras/{camera_id}/snapshot` - Publishes a snapshot for the specified camera to `{MQTT_TOPIC}/cameras/{camera_id}/snapshot`
-   `{MQTT_TOPIC}/command/cameras/{camera_id}/rtsps-stream` - Publishes an RTSPS stream for the specified camera to `{MQTT_TOPIC}/cameras/{camera_id}/rtsps-stream`

## Requirements

-   UniFi Protect version 6+ with API access enabled
-   MQTT broker (e.g., Mosquitto)

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.