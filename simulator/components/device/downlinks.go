package device

import (
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/util"

	act "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/activation"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/classes"
	dl "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/frames/downlink"
	"github.com/brocaar/lorawan"
)

func (d *Device) ProcessDownlink(phy lorawan.PHYPayload) (*dl.InformationDownlink, error) {

	var payload *dl.InformationDownlink
	var err error

	mtype := phy.MHDR.MType
	err = nil

	switch mtype {

	case lorawan.JoinAccept:
		Ja, err := act.DecryptJoinAccept(phy, d.Info.DevNonce, d.Info.JoinEUI, d.Info.AppKey)
		if err != nil {
			return nil, err
		}

		return d.ProcessJoinAccept(Ja)

	case lorawan.UnconfirmedDataDown:

		payload, err = dl.GetDownlink(phy, d.Info.Configuration.DisableFCntDown, d.Info.Status.FCntDown,
			d.Info.NwkSKey, d.Info.AppSKey)
		if err != nil {
			return nil, err
		}

		// Decode downlink using codec if configured
		d.decodeDownlinkWithCodec(payload, &phy)

	case lorawan.ConfirmedDataDown: //ack

		payload, err = dl.GetDownlink(phy, d.Info.Configuration.DisableFCntDown, d.Info.Status.FCntDown,
			d.Info.NwkSKey, d.Info.AppSKey)
		if err != nil {
			return nil, err
		}

		d.SendAck()

		// Decode downlink using codec if configured
		d.decodeDownlinkWithCodec(payload, &phy)

	}

	d.Info.Status.FCntDown = (d.Info.Status.FCntDown + 1) % util.MAXFCNTGAP

	switch d.Class.GetClass() {

	case classes.ClassA:
		d.Info.Status.DataUplink.AckMacCommand.CleanFOptsRXParamSetupAns()
		d.Info.Status.DataUplink.AckMacCommand.CleanFOptsRXTimingSetupAns()
		break

	case classes.ClassC:
		d.Info.Status.InfoClassC.SetACK(false) //Reset

	}

	msg := d.Info.Status.DataUplink.ADR.Reset()
	if msg != "" {
		d.Print(msg, nil, util.PrintBoth)
	}

	d.Info.Status.DataUplink.AckMacCommand.CleanFOptsDLChannelAns()

	return payload, err
}

// decodeDownlinkWithCodec executes the OnDownlink codec function for its side effects
func (d *Device) decodeDownlinkWithCodec(payload *dl.InformationDownlink, phy *lorawan.PHYPayload) {
	// Check if codec is configured and payload has data
	if Codecs == nil || d.Info.Configuration.CodecID == 0 || payload == nil {
		return
	}

	// Check if there's actual payload data
	if payload.DataPayload == nil || len(payload.DataPayload) == 0 {
		return
	}

	devEUI := d.Info.DevEUI.String()

	// Extract FPort from PHYPayload (default to 1 if not set)
	fPort := uint8(1)
	if macPL, ok := phy.MACPayload.(*lorawan.MACPayload); ok {
		if macPL.FPort != nil {
			fPort = *macPL.FPort
		}
	}

	// Execute OnDownlink for side effects (log, setState, setSendInterval)
	err := Codecs.DecodePayload(
		d.Info.Configuration.CodecID,
		devEUI,
		payload.DataPayload,
		fPort,
		d,
	)

	if err != nil {
		d.Print("Codec OnDownlink failed: "+err.Error(), err, util.PrintBoth)
	}
}
