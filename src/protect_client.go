package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

// ProtectClient wraps the UniFi Protect WebSocket functionality
type ProtectClient struct {
	config     *ProtectConfig
	mqttClient *MQTTClient
	dialer     websocket.Dialer
	httpClient *http.Client
	interrupt  chan os.Signal
	shutdown   chan struct{}
}

// NewProtectClient creates a new Protect client with the given configuration
func NewProtectClient(config *ProtectConfig, mqttClient *MQTTClient, interrupt chan os.Signal) *ProtectClient {
	return &ProtectClient{
		config:     config,
		mqttClient: mqttClient,
		interrupt:  interrupt,
		shutdown:   make(chan struct{}),
		dialer: websocket.Dialer{
			TLSClientConfig: config.TLSConfig,
		},
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: config.TLSConfig,
			},
		},
	}
}

// Cleanup shuts down all WebSocket connections and cleans up resources
func (p *ProtectClient) Cleanup() {
	slog.Info("Shutting down Protect client...")
	close(p.shutdown)
}

// PublishMetaInfo fetches the meta/info endpoint and publishes the data to MQTT
func (p *ProtectClient) PublishMetaInfo() error {
	return p.fetchAndPublish("GET", "meta/info", nil, true)
}

// PublishCameras fetches the cameras endpoint and publishes each camera to individual topics
func (p *ProtectClient) PublishCameras() error {
	data, err := p.makeRequest("GET", "cameras", nil)
	if err != nil {
		return err
	}

	// Parse the JSON array of cameras
	var cameras []map[string]any
	if err := json.Unmarshal(data, &cameras); err != nil {
		return fmt.Errorf("failed to parse cameras JSON: %v", err)
	}

	// Publish each camera to its individual topic
	for _, camera := range cameras {
		id, ok := camera["id"].(string)
		if !ok {
			slog.Warn("Camera missing ID field, skipping")
			continue
		}

		cameraData, err := json.Marshal(camera)
		if err != nil {
			return fmt.Errorf("failed to marshal camera data: %v", err)
		}

		topic := fmt.Sprintf("cameras/%s", id)
		if err := p.publishResponse(topic, cameraData, true); err != nil {
			slog.Warn("Failed to publish camera", "id", id, "error", err)
		}
	}

	return nil
}

// PublishCameraSnapshot fetches the snapshot for the specified camera and publishes the data to MQTT
func (p *ProtectClient) PublishCameraSnapshot(id string) error {
	return p.fetchAndPublish("GET", fmt.Sprintf("cameras/%s/snapshot", id), nil, false)
}

// PublishCameraRTSPStream fetches the RTSP stream URL for the specified camera and publishes the data to MQTT
func (p *ProtectClient) PublishCameraRTSPStream(id string) error {
	return p.fetchAndPublish("GET", fmt.Sprintf("cameras/%s/rtsps-stream", id), nil, true)
}

// PublishNVRs fetches the NVR endpoint and publishes the data to MQTT
func (p *ProtectClient) PublishNVRs() error {
	return p.fetchAndPublish("GET", "nvrs", nil, true)
}

// SubscribeEvents establishes WebSocket connection to UniFi Protect events endpoint
func (p *ProtectClient) SubscribeEvents() {
	p.subscribeToEndpoint("subscribe/events")
}

// SubscribeDevices establishes WebSocket connection to UniFi Protect devices endpoint
func (p *ProtectClient) SubscribeDevices() {
	p.subscribeToEndpoint("subscribe/devices")
}

// subscribeToEndpoint handles subscription to a specific endpoint
func (p *ProtectClient) subscribeToEndpoint(endpoint string) {
	scheme := "ws"
	if p.config.TLSEnabled {
		scheme = "wss"
	}

	url := fmt.Sprintf("%s://%s:%s/%s/%s", scheme, p.config.Host, p.config.Port, p.config.APIPath, endpoint)
	headers := p.getHeaders()

	go p.connectToEndpoint(url, endpoint, headers)
}

// connectToEndpoint handles connection to a single WebSocket endpoint with reconnection logic
func (p *ProtectClient) connectToEndpoint(url string, endpoint string, headers map[string][]string) {
	for {
		select {
		case <-p.shutdown:
			return
		case <-p.interrupt:
			return
		default:
		}

		slog.Info("Connecting to WebSocket endpoint", "endpoint", endpoint)

		c, _, err := p.dialer.Dial(url, headers)
		if err != nil {
			slog.Warn("WebSocket connection error, retrying", "error", err, "delay", websocketRetryDelay)
			time.Sleep(websocketRetryDelay)
			continue
		}

		slog.Info("Connected to WebSocket endpoint", "endpoint", endpoint)

		pingTicker := time.NewTicker(websocketPingInterval)

		done := make(chan struct{})

		go func() {
			defer pingTicker.Stop()
			for {
				select {
				case <-done:
					return
				case <-pingTicker.C:
					if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
						slog.Warn("Ping error", "error", err)
						return
					}
				}
			}
		}()

		func() {
			defer func() {
				close(done)
				c.Close()
				pingTicker.Stop()
			}()

			for {
				select {
				case <-p.shutdown:
					return
				case <-p.interrupt:
					return
				default:
				}

				_, msg, err := c.ReadMessage()
				if err != nil {
					slog.Warn("WebSocket read error", "error", err)
					return
				}

				slog.Debug("Received message", "endpoint", endpoint, "bytes", len(msg))

				p.publishResponse(endpoint, msg, false)
			}
		}()

		slog.Info("Connection lost, attempting to reconnect", "endpoint", endpoint, "delay", websocketReconnectDelay)
		time.Sleep(websocketReconnectDelay)
	}
}

// fetchAndPublish makes a request to the specified endpoint and publishes the response to MQTT
func (p *ProtectClient) fetchAndPublish(method, endpoint string, body []byte, retain bool) error {
	data, err := p.makeRequest(method, endpoint, body)
	if err != nil {
		return err
	}

	return p.publishResponse(endpoint, data, retain)
}

// makeRequest makes a generic HTTP API request to the specified endpoint with optional body data
func (p *ProtectClient) makeRequest(method, endpoint string, body []byte) ([]byte, error) {
	scheme := "http"
	if p.config.TLSEnabled {
		scheme = "https"
	}

	url := fmt.Sprintf("%s://%s:%s/%s/%s", scheme, p.config.Host, p.config.Port, p.config.APIPath, endpoint)

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	headers := p.getHeaders()
	for key, values := range headers {
		for _, value := range values {
			req.Header.Set(key, value)
		}
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make %s request to %s: %v", method, endpoint, err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(responseBody))
	}

	slog.Debug("API response", "method", method, "endpoint", endpoint, "bytes", len(responseBody))

	return responseBody, nil
}

// publishResponse publishes API response data to MQTT using the endpoint as topic
func (p *ProtectClient) publishResponse(endpoint string, data []byte, retain bool) error {
	if err := p.mqttClient.Publish(endpoint, data, retain); err != nil {
		return fmt.Errorf("failed to publish %s data to MQTT: %v", endpoint, err)
	}
	return nil
}

// getHeaders returns the HTTP headers needed for WebSocket connections
func (p *ProtectClient) getHeaders() map[string][]string {
	return map[string][]string{
		"X-API-KEY": {p.config.APIKey},
		"Accept":    {"application/json"},
	}
}
