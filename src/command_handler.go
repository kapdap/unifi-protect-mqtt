package main

import (
	"fmt"
	"log/slog"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// CommandRoute represents a command route with its pattern and handler
type CommandRoute struct {
	pattern string
	handler func(command string, params map[string]string, payload []byte) error
}

// CommandHandler handles MQTT command routing and execution
type CommandHandler struct {
	protectClient *ProtectClient
	mqttClient    *MQTTClient
	routes        []CommandRoute
}

// Route pattern constants
const (
	RouteMetaInfo         = "meta/info"
	RouteCameras          = "cameras"
	RouteCameraSnapshot   = "cameras/{id}/snapshot"
	RouteCameraRTSPStream = "cameras/{id}/rtsps-stream"
	RouteNVRs             = "nvrs"
)

// NewCommandHandler creates a new command handler
func NewCommandHandler(protectClient *ProtectClient, mqttClient *MQTTClient) *CommandHandler {
	h := &CommandHandler{
		protectClient: protectClient,
		mqttClient:    mqttClient,
	}
	h.routes = h.setupRoutes()
	return h
}

// SubscribeCommands subscribes to the command topic
func (h *CommandHandler) SubscribeCommands() {
	h.mqttClient.Subscribe("commands/#", h.HandleCommand)
}

// setupRoutes initializes and returns the command routes
func (h *CommandHandler) setupRoutes() []CommandRoute {
	return []CommandRoute{
		{
			pattern: RouteMetaInfo,
			handler: h.handleMetaInfo,
		},
		{
			pattern: RouteCameras,
			handler: h.handleCameras,
		},
		{
			pattern: RouteCameraSnapshot,
			handler: h.handleCameraSnapshot,
		},
		{
			pattern: RouteCameraRTSPStream,
			handler: h.handleCameraRTSPStream,
		},
		{
			pattern: RouteNVRs,
			handler: h.handleNVRs,
		},
	}
}

// matchRoute checks if a command matches a route pattern and extracts parameters
func (h *CommandHandler) matchRoute(command, pattern string) (map[string]string, bool) {
	commandParts := strings.Split(command, "/")
	patternParts := strings.Split(pattern, "/")

	if len(commandParts) != len(patternParts) {
		return nil, false
	}

	params := make(map[string]string)

	for i, patternPart := range patternParts {
		if strings.HasPrefix(patternPart, "{") && strings.HasSuffix(patternPart, "}") {
			paramName := patternPart[1 : len(patternPart)-1]
			params[paramName] = commandParts[i]
		} else if patternPart != commandParts[i] {
			return nil, false
		}
	}

	return params, true
}

// Individual handler methods
func (h *CommandHandler) handleMetaInfo(command string, params map[string]string, _ []byte) error {
	h.executeCommand(command, h.protectClient.PublishMetaInfo)
	return nil
}

func (h *CommandHandler) handleCameras(command string, params map[string]string, _ []byte) error {
	h.executeCommand(command, h.protectClient.PublishCameras)
	return nil
}

func (h *CommandHandler) handleCameraSnapshot(command string, params map[string]string, _ []byte) error {
	cameraID := params["id"]
	h.executeCommand(command, func() error {
		return h.protectClient.PublishCameraSnapshot(cameraID)
	}, cameraID)
	return nil
}

func (h *CommandHandler) handleCameraRTSPStream(command string, params map[string]string, _ []byte) error {
	cameraID := params["id"]
	h.executeCommand(command, func() error {
		return h.protectClient.PublishCameraRTSPStream(cameraID)
	}, cameraID)
	return nil
}

func (h *CommandHandler) handleNVRs(command string, params map[string]string, _ []byte) error {
	h.executeCommand(command, h.protectClient.PublishNVRs)
	return nil
}

// HandleCommand processes incoming MQTT command messages using a router pattern
func (h *CommandHandler) HandleCommand(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := msg.Payload()

	slog.Info("Received command", "topic", topic)

	// Extract the command from the topic (remove "commands/" prefix)
	prefix := fmt.Sprintf("%s/%s/", h.mqttClient.config.Topic, "commands")
	command := strings.TrimPrefix(topic, prefix)

	if command == "" {
		slog.Warn("Empty command", "topic", topic)
		return
	}

	// Try to match the command against routes
	for _, route := range h.routes {
		if params, matched := h.matchRoute(command, route.pattern); matched {
			route.handler(command, params, payload)
			return
		}
	}

	slog.Warn("Unknown command", "command", command)
}

// executeCommand executes a command function and logs the result
func (h *CommandHandler) executeCommand(commandName string, fn func() error, args ...string) {
	if err := fn(); err != nil {
		if len(args) > 0 {
			slog.Error("Failed to execute command", "command", commandName, "target", args[0], "error", err)
		} else {
			slog.Error("Failed to execute command", "command", commandName, "error", err)
		}
	} else {
		if len(args) > 0 {
			slog.Info("Successfully executed command", "command", commandName, "target", args[0])
		} else {
			slog.Info("Successfully executed command", "command", commandName)
		}
	}
}
