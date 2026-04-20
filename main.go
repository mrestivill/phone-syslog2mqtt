package main

import (
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"gopkg.in/mcuadros/go-syslog.v2"
	"gopkg.in/mcuadros/go-syslog.v2/format"
)

var (
	// Configuration from environment variables
	mqttBroker   = getEnv("MQTT_BROKER", "tcp://localhost:1883")
	mqttClientID = getEnv("MQTT_CLIENT_ID", "grandstream-mqtt")
	mqttUser     = getEnv("MQTT_USER", "")
	mqttPass     = getEnv("MQTT_PASS", "")
	syslogPort   = getEnv("SYSLOG_PORT", "514")
	deviceIP     = getEnv("DEVICE_IP", "192.168.1.1")

	// MQTT topics
	topicStatus     = getEnv("MQTT_TOPIC_STATUS", "phone/status")
	topicLastCaller = getEnv("MQTT_TOPIC_CALLER", "phone/last_caller_id")

	// Regex to extract caller ID from "startRing with CID" message
	// Example: "...Attempting to deliver CID 3464534534, +3464534534 on port 0"
	callerIDRegex = regexp.MustCompile(`\+(\d+)`)

	// Additional syslog patterns (adjust based on your device's actual messages)
	callConnectedMsg = "Call connected"
	callEndedMsg     = "Call ended"
	outboundCallMsg  = "Outbound call"
)

type CallState string

const (
	StateIdle     CallState = "Idle"
	StateIncoming CallState = "Incoming"
	StateOutgoing CallState = "Outgoing"
	StateInCall   CallState = "InCall"
)

var currentState = StateIdle
var lastCallerID = ""

func main() {
	// Connect to MQTT
	mqttClient := connectMQTT()
	defer mqttClient.Disconnect(250)

	// Publish initial state (retained)
	publishState(mqttClient, StateIdle, "")

	// Setup syslog server
	channel := make(syslog.LogPartsChannel)
	handler := syslog.NewChannelHandler(channel)

	server := syslog.NewServer()
	server.SetFormat(syslog.Automatic)
	server.SetHandler(handler)
	if err := server.ListenUDP("0.0.0.0:" + syslogPort); err != nil {
		log.Fatal("Failed to start UDP syslog server:", err)
	}
	if err := server.Boot(); err != nil {
		log.Fatal("Failed to boot syslog server:", err)
	}

	log.Printf("Syslog server listening on UDP port %s", syslogPort)
	log.Printf("Filtering messages from IP: %s", deviceIP)

	// Process incoming syslog messages
	go func(channel syslog.LogPartsChannel) {
		for logParts := range channel {
			processLogParts(logParts, mqttClient)
		}
	}(channel)

	// Wait forever
	select {}
}

func processLogParts(logParts format.LogParts, client mqtt.Client) {
	// Filter by source IP
	sourceIP, ok := logParts["client"].(string)
	if !ok || sourceIP != deviceIP {
		return
	}

	content, ok := logParts["content"].(string)
	if !ok {
		return
	}

	// Check for different call events
	switch {
	case strings.Contains(content, "startRing with CID"):
		// Incoming call detected
		callerID := extractCallerID(content)
		lastCallerID = callerID
		currentState = StateIncoming
		log.Printf("Incoming call from %s", callerID)
		publishState(client, StateIncoming, callerID)

	case strings.Contains(content, callConnectedMsg):
		// Call answered
		if currentState == StateIncoming {
			currentState = StateInCall
		} else if currentState == StateOutgoing {
			currentState = StateInCall
		}
		log.Printf("Call connected")
		publishState(client, currentState, lastCallerID)

	case strings.Contains(content, callEndedMsg):
		// Call ended
		currentState = StateIdle
		log.Printf("Call ended")
		publishState(client, StateIdle, "")

	case strings.Contains(content, outboundCallMsg):
		// Outbound call - extract dialed number if needed
		// For now, just update state
		currentState = StateOutgoing
		log.Printf("Outbound call initiated")
		publishState(client, StateOutgoing, "")
	}
}

func extractCallerID(msg string) string {
	match := callerIDRegex.FindStringSubmatch(msg)
	if len(match) > 1 {
		return "+" + match[1]
	}
	return "Unknown"
}

func publishState(client mqtt.Client, state CallState, callerID string) {
	// Publish status with retain flag
	statusToken := client.Publish(topicStatus, 0, true, string(state))
	statusToken.Wait()
	if statusToken.Error() != nil {
		log.Printf("MQTT publish error (status): %v", statusToken.Error())
	}

	// Publish caller ID only if it's not empty
	if callerID != "" {
		callerToken := client.Publish(topicLastCaller, 0, true, callerID)
		callerToken.Wait()
		if callerToken.Error() != nil {
			log.Printf("MQTT publish error (caller): %v", callerToken.Error())
		}
	}
}

func connectMQTT() mqtt.Client {
	opts := mqtt.NewClientOptions().
		AddBroker(mqttBroker).
		SetClientID(mqttClientID).
		SetCleanSession(true).
		SetAutoReconnect(true).
		SetMaxReconnectInterval(5 * time.Second)

	if mqttUser != "" {
		opts.SetUsername(mqttUser)
	}
	if mqttPass != "" {
		opts.SetPassword(mqttPass)
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal("MQTT connection failed:", token.Error())
	}
	log.Println("Connected to MQTT broker")
	return client
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
