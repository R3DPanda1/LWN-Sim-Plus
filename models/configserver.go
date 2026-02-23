package models

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/logging"
)

// EventsConfig holds history retention settings for the event broker.
type EventsConfig struct {
	HistoryPerDevice  int `json:"historyPerDevice"`
	HistoryPerGateway int `json:"historyPerGateway"`
}

// PerformanceConfig holds tuning parameters for the scheduler and forwarder.
type PerformanceConfig struct {
	ForwarderShards     int    `json:"forwarderShards"`
	UplinkBufferSize    int    `json:"uplinkBufferSize"`
	WorkerCount         int    `json:"workerCount"`
	SchedulerResolution string `json:"schedulerResolution"`
	WorkQueueSize       int    `json:"workQueueSize"`
}

// ServerConfig holds the configuration for the server including address, ports, and other settings.
type ServerConfig struct {
	Address       string            `json:"address"`       // Address to bind to (e.g., "localhost")
	Port          int               `json:"port"`           // Port to bind to (default is 8000)
	MetricsPort   int               `json:"metricsPort"`    // Port to bind to for metrics (default is 8081)
	ConfigDirname string            `json:"configDirname"`  // Directory name for configuration files
	AutoStart     bool              `json:"autoStart"`      // Flag to automatically start the simulation when the server starts
	Verbose       bool              `json:"verbose"`        // Flag to enable verbose logging
	Logging       logging.Config    `json:"logging"`
	Performance   PerformanceConfig `json:"performance"`
	Events        EventsConfig      `json:"events"`
}

// GetConfigFile loads the configuration from the specified file path, parses it as JSON,
// and returns a ServerConfig instance. It returns an error if the file cannot be read or parsed.
func GetConfigFile(path string) (*ServerConfig, error) {
	config := &ServerConfig{}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file: %w", err)
	}
	return config, nil
}
