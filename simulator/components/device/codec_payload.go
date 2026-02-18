package device

import (
	"log/slog"
	"time"

	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/codec"
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
		slog.Debug("codec registry not initialized, using static payload", "component", "device", "dev_eui", d.Info.DevEUI)
		return d.Info.Status.Payload
	}

	if d.Info.Configuration.CodecID == 0 {
		slog.Debug("no codec ID configured, using static payload", "component", "device", "dev_eui", d.Info.DevEUI)
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
		slog.Error("codec execution failed", "component", "device", "dev_eui", d.Info.DevEUI, "codec_id", d.Info.Configuration.CodecID, "error", err)
		d.emitErrorEvent(err)
		return d.Info.Status.Payload
	}

	// Update device's fPort
	d.Info.Status.DataUplink.FPort = &fPort

	// Create and return payload
	return &lorawan.DataPayload{Bytes: bytes}
}
