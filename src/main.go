package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	websocketReconnectDelay  = 2 * time.Second
	websocketRetryDelay      = 5 * time.Second
	websocketPingInterval    = 30 * time.Second
	shutdownGracePeriod      = 1 * time.Second
	mqttConnectRetryInterval = 5 * time.Second
	mqttMaxReconnectInterval = 60 * time.Second
	mqttDisconnectTimeout    = 250
)

func main() {
	// Initialize configuration and logging
	InitializeConfig()
	ConfigureLogging()

	slog.Info("Starting Unifi Protect MQTT")

	// Load MQTT and Protect configuration
	mqttConfig := LoadMQTTConfig()
	protectConfig := LoadProtectConfig()

	// Create MQTT client
	mqttClient := NewMQTTClient(mqttConfig)
	var protectClient *ProtectClient

	// Set up interrupt channel
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	// Define MQTT connection handlers
	onConnect := func() {
		// Create and setup ProtectClient when MQTT connects
		protectClient = NewProtectClient(protectConfig, mqttClient, interrupt)

		// Subscribe to MQTT commands
		protectClient.SubscribeCommands()

		// Subscribe to Protect events
		protectClient.SubscribeDevices()
		protectClient.SubscribeEvents()

		// Fetch Protect metadata
		protectClient.PublishMetaInfo()
		protectClient.PublishNVRs()
		protectClient.PublishCameras()
	}

	onDisconnect := func() {
		// Cleanup ProtectClient when MQTT disconnects
		if protectClient != nil {
			protectClient.Cleanup()
		}
	}

	// Connect to MQTT
	if err := mqttClient.Connect(onConnect, onDisconnect); err != nil {
		slog.Error("Failed to connect MQTT client", "error", err)
		os.Exit(1)
	}
	defer mqttClient.Disconnect()

	<-interrupt
	slog.Info("Shutdown signal received...")

	time.Sleep(shutdownGracePeriod)
	slog.Info("Application shutdown complete")
}
