package events

import "time"

// Device event types
const (
	EventUp         = "up"
	EventJoin       = "join"
	EventDownlink   = "downlink"
	EventAck        = "ack"
	EventMacCommand = "mac_command"
	EventStatus     = "status"
	EventError      = "error"
)

// Gateway event types
const (
	GwEventPushData     = "push_data"
	GwEventPullResp     = "pull_resp"
	GwEventConnected    = "connected"
	GwEventDisconnected = "disconnected"
	GwEventError        = "error"
)

// System event types
const (
	SysEventStarted = "started"
	SysEventStopped = "stopped"
	SysEventSetup   = "setup"
	SysEventError   = "error"
)

type DeviceEvent struct {
	ID        string            `json:"id"`
	Time      time.Time         `json:"time"`
	DevEUI    string            `json:"devEUI"`
	DevName   string            `json:"devName"`
	Type      string            `json:"type"`
	FCnt      *uint32           `json:"fCnt,omitempty"`
	FPort     *uint8            `json:"fPort,omitempty"`
	DR        *int              `json:"dr,omitempty"`
	Frequency *uint32           `json:"frequency,omitempty"`
	Payload   string            `json:"payload,omitempty"`
	Class     string            `json:"class,omitempty"`
	Mode      string            `json:"mode,omitempty"`
	GatewayID string            `json:"gatewayId,omitempty"`
	Extra     map[string]string `json:"extra,omitempty"`
}

type GatewayEvent struct {
	ID         string            `json:"id"`
	Time       time.Time         `json:"time"`
	GatewayMAC string            `json:"gatewayMAC"`
	GwName     string            `json:"gwName"`
	Type       string            `json:"type"`
	Extra      map[string]string `json:"extra,omitempty"`
}

type SystemEvent struct {
	ID      string    `json:"id"`
	Time    time.Time `json:"time"`
	Type    string    `json:"type"`
	Message string    `json:"message"`
	IsError bool      `json:"isError"`
}

func DeviceTopic(devEUI string) string { return "device:" + devEUI }
func GatewayTopic(gwMAC string) string { return "gateway:" + gwMAC }

const SystemTopic = "system"
const ErrorsTopic = "errors"
