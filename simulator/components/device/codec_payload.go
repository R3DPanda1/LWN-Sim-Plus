package device

import (
	"github.com/R3DPanda1/LWN-Sim-Plus/codec"
	"github.com/brocaar/lorawan"
)

// CodecManager is a global codec manager instance
// It will be initialized by the simulator
var CodecManager *codec.Manager

// GenerateCodecPayload generates a payload using the configured codec
func (d *Device) GenerateCodecPayload() lorawan.Payload {
	// Safety check
	if CodecManager == nil {
		d.Print("Codec manager not initialized, using static payload", nil, 1)
		return d.Info.Status.Payload
	}

	if d.Info.Configuration.CodecID == "" {
		d.Print("No codec ID configured, using static payload", nil, 1)
		return d.Info.Status.Payload
	}

	// Get DevEUI as string
	devEUI := d.Info.DevEUI.String()

	// Default fPort (could be configurable later)
	fPort := uint8(1)

	// Use PayloadConfig if available, otherwise use empty object
	payloadConfig := d.Info.Configuration.PayloadConfig
	if payloadConfig == nil {
		payloadConfig = make(map[string]interface{})
	}

	// Generate payload using codec
	payload, err := CodecManager.GeneratePayloadFromConfig(
		d.Info.Configuration.CodecID,
		devEUI,
		fPort,
		payloadConfig,
	)

	if err != nil {
		d.Print("Codec execution failed, using static payload", err, 1)
		return d.Info.Status.Payload
	}

	// Update state with the generated payload (for message history)
	if state := CodecManager.GetState(devEUI); state != nil {
		dataPayload, ok := payload.(*lorawan.DataPayload)
		if ok && dataPayload != nil {
			state.AddMessage(codec.MessageRecord{
				FCnt:      d.Info.Status.DataUplink.FCnt,
				Timestamp: state.UpdatedAt,
				Payload:   dataPayload.Bytes,
				FPort:     fPort,
			})
		}
	}

	return payload
}
