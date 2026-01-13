package device

import (
	"time"

	"github.com/R3DPanda1/LWN-Sim-Plus/codec"
	"github.com/brocaar/lorawan"
)

// Codecs is a global codec registry instance
// It will be initialized by the simulator
var Codecs *codec.Registry

// GetSendInterval returns the device's send interval (implements codec.DeviceInterface)
func (d *Device) GetSendInterval() time.Duration {
	return d.Info.Configuration.SendInterval
}

// SetSendInterval sets the device's send interval (implements codec.DeviceInterface)
func (d *Device) SetSendInterval(interval time.Duration) {
	d.Info.Configuration.SendInterval = interval

	// Signal the device loop to reset its ticker (non-blocking)
	if d.IntervalChanged != nil {
		select {
		case d.IntervalChanged <- struct{}{}:
		default:
			// Channel already has a pending signal, skip
		}
	}
}

// GenerateCodecPayload generates a payload using the configured codec
func (d *Device) GenerateCodecPayload() lorawan.Payload {
	// Safety check
	if Codecs == nil {
		d.Print("Codec registry not initialized, using static payload", nil, 1)
		return d.Info.Status.Payload
	}

	if d.Info.Configuration.CodecID == 0 {
		d.Print("No codec ID configured, using static payload", nil, 1)
		return d.Info.Status.Payload
	}

	// Get DevEUI as string
	devEUI := d.Info.DevEUI.String()

	// Encode using codec (returns bytes and fPort)
	bytes, fPort, err := Codecs.EncodePayload(
		d.Info.Configuration.CodecID,
		devEUI,
		d, // Pass device for getSendInterval/setSendInterval
	)

	if err != nil {
		d.Print("Codec execution failed: "+err.Error()+", using static payload", err, 1)
		return d.Info.Status.Payload
	}

	// Update device's fPort
	d.Info.Status.DataUplink.FPort = &fPort

	// Create and return payload
	return &lorawan.DataPayload{Bytes: bytes}
}
