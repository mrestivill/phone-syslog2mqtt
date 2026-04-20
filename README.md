# phone-syslog2mqtt

Syslog server that publishes sip phone events to mqtt


## suported devices

- Grandstream HT801V2 ATA

## Introduction

A lightweight Go program that listens for UDP syslog messages from a Grandstream HT801V2 ATA and publishes call state and caller ID to an MQTT broker.

This is designed to be a replacement for the [docker-pap2t-mqtt](https://github.com/mrestivill/docker-pap2t-mqtt) solution, but for Grandstream devices that support syslog.

## Features

* Listens for UDP syslog messages on a configurable port (default 514).
* Filters messages by source IP to only process events from your Grandstream.
* Extracts caller ID from `"startRing with CID" events using regex.
* Maintains a call state machine: `Idle`, `Incoming`, `Outgoing`, `InCall`.
* Publishes state and caller ID to MQTT as retained messages (persistent).
* Configurable via environment variables.
* Lightweight Docker image (~MB) for easy deployment.

## Requirements

* Go 1.21+ (for building from source)
* MQTT Broker (e.g., Mosquitto, Home Assistant MQTT, etc.)
* Grandstream HT801V2 with syslog enabled and pointing to this bridge's IP.

## Installation

#### Option 1: Run with Docker (recommended)

```bash
docker run -d \
  --name grandstream-mqtt \
  --restart unless-stopped \
  -p 514:514/udp \
  -e MQTTOP_BROKER="tcp://your-mqtt-broker:1883" \
  -e MQTT_USER="optional-username" \
  -e MQTT_PASS="optional-password" \
  -e DEVICE_IP="192.168.10.7" \
  mrestivill/grandstream-mqtt:latest
```

#### Option 2: Build from Source

```bash
git clone https://github.com/mrestivill/grandstream-mqtt.git
cd grandstream-mqtt
go build .o grandstream-mqtt ./...

/ Then run:
export MQTTOP_BROKER="tcp://localhost:1883"
export DEVICE_IP="192.168.10.7"
./grandstream-mqtt
```

## Configuration

The bridge is configured via the following environment variables:

| Variable            | Description                                      | Default                |
|---------------------|----------------------------------------------------|-------------------------|
| MQTT_BROKER         | URI of the MQTT broker                         | tcp://localhost:1883    |
| MQTT_CLIENT_ID      | MQTT client identifier                         | grandstream-mqtt      |
| MQTT_USER           | MQTT username (optional)                      |                       |
| MQTT_PASS           | MQTT password (optional)                      |                       |
| SYSLOG_PORT         | UDP port to listen on                          | 514                   |
| DEVICE_IP           | IP address of your Grandstream from which to accept messages | 192.168.10.7          |
| MQTT_TOPIC_STATUS  | MQTT topic for call state                     | phone/status          |
| MQTT_TOPIC_CALLER | MQQT topic for last caller ID                | phone/last_caller_id  |

## Grandstream HT801V2 Syslog Setup

1. Access the HT801V2 web interface.
2. Navigate to **Advanced Settings > Syslog**.
3. Set the following:
   - **Syslog Server**: IP address of the machine running this bridge.
   - **Syslog Level**: `INFO` (or `DEBUG if you want more verbosity).
   - **Syslog Protocol**: `UDP`.
4. Click **Apply** and then **Reboot** the device.

## MQTT Topics and Payloads

The bridge publishes retained messages to two topics (configurable):

````md
phone/status          -> "Incoming" (or "Idle", "InCall", "Outgoing")
phone/last_caller_id  -> "+3464534534"


Call state changes are published immediately upon receiving the relevant syslog messages. The last caller ID is updated only when an incoming call is detected.

## Testing

You can simulate a syslog message using the `logger` command:

```bash
logger -n your-bridge-ip -P 514 "Apr 20 21:24:34 HT801V2 [ec:74:d7:b3:c6:cc] [1.0.9.3] GS_ATA: Nuvoton::startRing with CID, Attempting to deliver CID 3464534534, +3464534534 on port 0"
```

## Customization

The program uses hard-coded strings to identify call events (incoming ring, call connected, call ended). If your HT801V2's syslog messages differ, you may need to modify the strings in `main.go`. Please file an issue or pull request if you have different firmware versions that use alternative wording.

## License

MIT

## Acknowledgements

Inspired by [docker-pap2t-mqtt](https://github.com/mrestivill/docker-pap2t-mqtt) and the need to modernize for Grandstream devices.
