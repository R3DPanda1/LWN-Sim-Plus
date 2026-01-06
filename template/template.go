package template

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrTemplateNotFound = errors.New("template not found")
	ErrInvalidTemplate  = errors.New("invalid template")
)

// DeviceTemplate represents a template for bulk device creation
// Templates use ABP activation; DevEUI, DevAddr, NwkSKey, AppSKey, and Name are auto-generated
type DeviceTemplate struct {
	ID   string `json:"id"`
	Name string `json:"name"` // Template name (e.g., "AM319 Temperature Sensor")

	// Region configuration
	Region int `json:"region"` // Region code (1=EU868, 2=US915, etc.)

	// Class support
	SupportedClassB bool `json:"supportedClassB"`
	SupportedClassC bool `json:"supportedClassC"`

	// Features
	SupportedADR bool    `json:"supportedADR"`
	Range        float64 `json:"range"` // Antenna range in meters

	// Data rate
	DataRate    uint8 `json:"dataRate"`    // Initial uplink data rate
	RX1DROffset uint8 `json:"rx1DROffset"` // RX1 data rate offset

	// Timing (all in milliseconds for RX windows, seconds for intervals)
	SendInterval int `json:"sendInterval"` // Uplink interval in seconds
	AckTimeout   int `json:"ackTimeout"`   // ACK timeout in seconds

	// RX1 Window settings (milliseconds)
	RX1Delay    int `json:"rx1Delay"`
	RX1Duration int `json:"rx1Duration"`

	// RX2 Window settings (milliseconds)
	RX2Delay     int     `json:"rx2Delay"`
	RX2Duration  int     `json:"rx2Duration"`
	RX2Frequency float64 `json:"rx2Frequency"` // RX2 frequency in Hz
	RX2DataRate  int     `json:"rx2DataRate"`

	// Frame settings
	FPort            uint8 `json:"fport"`
	NbRetransmission int   `json:"nbRetransmission"`
	MType            int   `json:"mtype"` // 0=UnconfirmedDataUp, 1=ConfirmedDataUp

	// Payload settings
	SupportedFragment bool `json:"supportedFragment"` // true=fragment, false=truncate

	// Codec configuration
	UseCodec bool   `json:"useCodec"`
	CodecID  string `json:"codecId"`

	// ChirpStack Integration configuration
	IntegrationEnabled bool   `json:"integrationEnabled"`
	IntegrationID      string `json:"integrationId"`
	DeviceProfileID    string `json:"deviceProfileId"`
}

// NewDeviceTemplate creates a new template with auto-generated ID
func NewDeviceTemplate(name string) *DeviceTemplate {
	t := &DeviceTemplate{
		Name: name,
		// Defaults
		Region:            1, // EU868
		SupportedADR:      true,
		Range:             10000, // 10km
		DataRate:          0,
		RX1DROffset:       0,
		SendInterval:      60,   // 1 minute
		AckTimeout:        2,    // 2 seconds
		RX1Delay:          1000, // 1 second
		RX1Duration:       3000, // 3 seconds (increased for reliable downlink reception)
		RX2Delay:          2000, // 2 seconds
		RX2Duration:       3000, // 3 seconds (increased for reliable downlink reception)
		RX2Frequency:      869525000, // Default EU868 RX2
		RX2DataRate:       0,
		FPort:             1,
		NbRetransmission:  1,
		MType:             0, // UnconfirmedDataUp
		SupportedFragment: false,
	}
	t.ID = t.generateID()
	return t
}

// generateID creates a unique ID based on name
func (t *DeviceTemplate) generateID() string {
	hash := sha256.Sum256([]byte(t.Name + fmt.Sprintf("%d", t.Region)))
	return hex.EncodeToString(hash[:])[:16]
}

// RegenerateID regenerates the ID (useful after name change)
func (t *DeviceTemplate) RegenerateID() {
	t.ID = t.generateID()
}

// Validate checks if the template has all required fields
func (t *DeviceTemplate) Validate() error {
	if strings.TrimSpace(t.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidTemplate)
	}
	if t.Region < 1 || t.Region > 10 {
		return fmt.Errorf("%w: invalid region code", ErrInvalidTemplate)
	}
	if t.SendInterval < 1 {
		return fmt.Errorf("%w: send interval must be at least 1 second", ErrInvalidTemplate)
	}
	if t.Range <= 0 {
		return fmt.Errorf("%w: range must be positive", ErrInvalidTemplate)
	}
	return nil
}

// Clone returns a deep copy of the template
func (t *DeviceTemplate) Clone() *DeviceTemplate {
	return &DeviceTemplate{
		ID:                 t.ID,
		Name:               t.Name,
		Region:             t.Region,
		SupportedClassB:    t.SupportedClassB,
		SupportedClassC:    t.SupportedClassC,
		SupportedADR:       t.SupportedADR,
		Range:              t.Range,
		DataRate:           t.DataRate,
		RX1DROffset:        t.RX1DROffset,
		SendInterval:       t.SendInterval,
		AckTimeout:         t.AckTimeout,
		RX1Delay:           t.RX1Delay,
		RX1Duration:        t.RX1Duration,
		RX2Delay:           t.RX2Delay,
		RX2Duration:        t.RX2Duration,
		RX2Frequency:       t.RX2Frequency,
		RX2DataRate:        t.RX2DataRate,
		FPort:              t.FPort,
		NbRetransmission:   t.NbRetransmission,
		MType:              t.MType,
		SupportedFragment:  t.SupportedFragment,
		UseCodec:           t.UseCodec,
		CodecID:            t.CodecID,
		IntegrationEnabled: t.IntegrationEnabled,
		IntegrationID:      t.IntegrationID,
		DeviceProfileID:    t.DeviceProfileID,
	}
}
