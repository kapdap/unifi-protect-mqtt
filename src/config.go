package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// ConfigField represents a configuration field definition
type ConfigField struct {
	Key         string
	Description string
	Default     string
	IsFile      bool
	Flag        *string
}

// MQTTConfig holds MQTT broker configuration
type MQTTConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	ClientID string
	Topic    string
	QoS      byte
	TLS      bool
}

// ProtectConfig holds UniFi Protect configuration
type ProtectConfig struct {
	APIKey     string
	Host       string
	Port       string
	APIPath    string
	TLSEnabled bool
	TLSConfig  *tls.Config
}

// Configuration field definitions
var (
	configFields = []ConfigField{
		// Logging Configuration
		{Key: "LOG_LEVEL", Description: "Log level (DEBUG, INFO, WARN, ERROR)", Default: "INFO", IsFile: false},

		// MQTT Configuration
		{Key: "MQTT_HOST", Description: "MQTT broker host", Default: "mqtt", IsFile: false},
		{Key: "MQTT_PORT", Description: "MQTT broker port", Default: "1883", IsFile: false},
		{Key: "MQTT_USERNAME", Description: "MQTT username", Default: "", IsFile: false},
		{Key: "MQTT_PASSWORD", Description: "MQTT password", Default: "", IsFile: false},
		{Key: "MQTT_PASSWORD_FILE", Description: "Path to file containing MQTT password", Default: "", IsFile: true},
		{Key: "MQTT_CLIENT_ID", Description: "MQTT client ID", Default: "unifi-protect-mqtt", IsFile: false},
		{Key: "MQTT_TOPIC", Description: "MQTT topic prefix", Default: "unifi/protect", IsFile: false},
		{Key: "MQTT_QOS", Description: "MQTT QoS", Default: "0", IsFile: false},
		{Key: "MQTT_RETAIN", Description: "MQTT retain", Default: "false", IsFile: false},
		{Key: "MQTT_TLS", Description: "MQTT TLS", Default: "false", IsFile: false},

		// Protect Configuration
		{Key: "PROTECT_API_KEY", Description: "Protect API key", Default: "", IsFile: false},
		{Key: "PROTECT_API_KEY_FILE", Description: "Path to file containing Protect API key", Default: "", IsFile: true},
		{Key: "PROTECT_HOST", Description: "Protect host name", Default: "unifi", IsFile: false},
		{Key: "PROTECT_PORT", Description: "Protect host port", Default: "443", IsFile: false},
		{Key: "PROTECT_API_PATH", Description: "Protect API path", Default: "proxy/protect/integration/v1", IsFile: false},
		{Key: "PROTECT_TLS", Description: "Connect using TLS protocol", Default: "true", IsFile: false},
		{Key: "PROTECT_TLS_VERIFY", Description: "Verify TLS certificates", Default: "false", IsFile: false},
	}
)

// InitializeConfig initializes the configuration system
func InitializeConfig() {
	_ = godotenv.Load()

	// Register all configuration flags
	for i := range configFields {
		description := configFields[i].Description
		if configFields[i].Default != "" {
			description = fmt.Sprintf("%s (default: %s)", description, configFields[i].Default)
		}
		configFields[i].Flag = flag.String(configFields[i].Key, "", description)
	}

	flag.Parse()
}

// ConfigureLogging sets up slog with the configured log level
func ConfigureLogging() {
	logLevel := getConfigValue("LOG_LEVEL")

	var level slog.Level
	switch strings.ToUpper(logLevel) {
	case "DEBUG":
		level = slog.LevelDebug
	case "INFO":
		level = slog.LevelInfo
	case "WARN", "WARNING":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
		slog.Warn("Unknown log level, defaulting to INFO", "level", logLevel)
	}

	// Create a new logger with the specified level
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

// LoadMQTTConfig loads and returns MQTT configuration
func LoadMQTTConfig() *MQTTConfig {
	brokerHost := getConfigValue("MQTT_HOST")
	brokerPort := getConfigValue("MQTT_PORT")
	username := getConfigValue("MQTT_USERNAME")

	// Handle password with file fallback
	password := getConfigValue("MQTT_PASSWORD")
	if password == "" {
		password = getConfigValue("MQTT_PASSWORD_FILE")
	}

	clientID := getConfigValue("MQTT_CLIENT_ID")
	topic := getConfigValue("MQTT_TOPIC")

	qos := parseConfigValue("MQTT_QOS", strconv.Atoi)
	tlsEnabled := parseConfigValue("MQTT_TLS", strconv.ParseBool)

	return &MQTTConfig{
		Host:     brokerHost,
		Port:     brokerPort,
		Username: username,
		Password: password,
		ClientID: clientID,
		Topic:    topic,
		QoS:      byte(qos),
		TLS:      tlsEnabled,
	}
}

// LoadProtectConfig loads and returns Protect configuration
func LoadProtectConfig() *ProtectConfig {
	// Handle API key with file fallback
	apiKey := getConfigValue("PROTECT_API_KEY")
	if apiKey == "" {
		apiKey = getConfigValue("PROTECT_API_KEY_FILE")
	}
	if apiKey == "" {
		slog.Error("PROTECT_API_KEY or PROTECT_API_KEY_FILE is required")
		os.Exit(1)
	}

	hostName := getConfigValue("PROTECT_HOST")
	hostPort := getConfigValue("PROTECT_PORT")
	apiPath := getConfigValue("PROTECT_API_PATH")

	tlsEnabled := parseConfigValue("PROTECT_TLS", strconv.ParseBool)
	tlsVerify := parseConfigValue("PROTECT_TLS_VERIFY", strconv.ParseBool)

	tlsConfig := &tls.Config{InsecureSkipVerify: !tlsVerify}

	return &ProtectConfig{
		APIKey:     apiKey,
		Host:       hostName,
		Port:       hostPort,
		APIPath:    apiPath,
		TLSEnabled: tlsEnabled,
		TLSConfig:  tlsConfig,
	}
}

// parseConfigValue parses a configuration value with fallback to default on error
func parseConfigValue[T any](configKey string, parser func(string) (T, error)) T {
	value := getConfigValue(configKey)
	result, err := parser(value)
	if err != nil {
		defaultValue := getConfigField(configKey).Default
		slog.Warn("Invalid config value, using default", "key", configKey, "default", defaultValue, "error", err)
		result, _ = parser(defaultValue)
	}
	return result
}

// getConfigField finds a ConfigField by its key
func getConfigField(key string) ConfigField {
	for _, field := range configFields {
		if field.Key == key {
			return field
		}
	}

	return ConfigField{Key: key, Default: "", IsFile: false}
}

// getConfigValue retrieves a configuration value from flag, environment, or default
func getConfigValue(key string) string {
	var value string

	field := getConfigField(key)

	// Get value from flag first, then environment, then default
	if field.Flag != nil && *field.Flag != "" {
		value = *field.Flag
	} else if envValue := os.Getenv(field.Key); envValue != "" {
		value = envValue
	} else {
		value = field.Default
	}

	// If this is a file-based config and we have a file path, read from file
	if field.IsFile && value != "" {
		fileValue, err := readFileValue(value)
		if err != nil {
			slog.Error("Failed to read value from file", "file", value, "error", err)
			os.Exit(1)
		}
		if fileValue != "" {
			return fileValue
		}
	}

	return value
}

// readFileValue reads and returns the content of a file
func readFileValue(filePath string) (string, error) {
	if filePath == "" {
		return "", nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %v", filePath, err)
	}

	return strings.TrimSpace(string(data)), nil
}
