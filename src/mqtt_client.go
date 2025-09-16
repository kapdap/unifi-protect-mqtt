package main

import (
	"fmt"
	"log/slog"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MQTTClient wraps the MQTT client functionality
type MQTTClient struct {
	config *MQTTConfig
	client mqtt.Client
}

// NewMQTTClient creates a new MQTT client with the given configuration
func NewMQTTClient(config *MQTTConfig) *MQTTClient {
	return &MQTTClient{
		config: config,
	}
}

// Connect establishes connection to the MQTT broker
func (m *MQTTClient) Connect(onConnect func(), onDisconnect func()) error {
	brokerURL := fmt.Sprintf("tcp://%s:%s", m.config.Host, m.config.Port)
	if m.config.TLS {
		brokerURL = fmt.Sprintf("ssl://%s:%s", m.config.Host, m.config.Port)
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(brokerURL)
	opts.SetClientID(m.config.ClientID)

	if m.config.Username != "" {
		opts.SetUsername(m.config.Username)
	}
	if m.config.Password != "" {
		opts.SetPassword(m.config.Password)
	}

	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(mqttConnectRetryInterval)
	opts.SetMaxReconnectInterval(mqttMaxReconnectInterval)

	opts.SetOnConnectHandler(func(client mqtt.Client) {
		slog.Info("Connected to MQTT broker", "url", brokerURL)
		if onConnect != nil {
			onConnect()
		}
	})

	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		slog.Warn("MQTT connection lost", "error", err)
		if onDisconnect != nil {
			onDisconnect()
		}
	})

	m.client = mqtt.NewClient(opts)

	slog.Info("Connecting to MQTT broker", "url", brokerURL)
	if token := m.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %v", token.Error())
	}

	return nil
}

// Subscribe subscribes to an MQTT topic with a message handler
func (m *MQTTClient) Subscribe(topic string, handler mqtt.MessageHandler) error {
	fullTopic := fmt.Sprintf("%s/%s", m.config.Topic, topic)
	token := m.client.Subscribe(fullTopic, m.config.QoS, handler)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to subscribe to MQTT topic %s: %v", fullTopic, token.Error())
	}
	slog.Info("Subscribed to MQTT topic", "topic", fullTopic)
	return nil
}

// Publish sends a message to the specified MQTT topic
func (m *MQTTClient) Publish(topic string, message []byte, retain bool) error {
	fullTopic := fmt.Sprintf("%s/%s", m.config.Topic, topic)
	token := m.client.Publish(fullTopic, m.config.QoS, retain, message)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish to MQTT topic %s: %v", fullTopic, token.Error())
	}
	slog.Info("Published message to MQTT topic", "topic", fullTopic)
	return nil
}

// Disconnect closes the MQTT connection
func (m *MQTTClient) Disconnect() {
	if m.client != nil {
		slog.Info("Disconnecting MQTT client...")
		m.client.Disconnect(mqttDisconnectTimeout)
	}
}

// IsConnected returns true if the client is connected to the broker
func (m *MQTTClient) IsConnected() bool {
	return m.client != nil && m.client.IsConnected()
}
