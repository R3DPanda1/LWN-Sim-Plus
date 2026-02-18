package device

import (
	"log/slog"
	"math/rand"
	"time"

	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/events"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/util"

	act "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/activation"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/classes"
	dl "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/frames/downlink"
	"github.com/brocaar/lorawan"
)

const (
	JOINACCEPTDELAY1 = time.Duration(5 * time.Second)
	JOINACCEPTDELAY2 = time.Duration(6 * time.Second)
)

func (d *Device) OtaaActivation() {

	for !d.Info.Status.Joined {

		d.Info.Status.Mode = util.Activation

		if !d.CanExecute() { //stop simulator
			return
		}

		d.SwitchClass(classes.ClassA)

		d.SendJoinRequest()

		slog.Debug("opening receive windows", "component", "device", "dev_eui", d.Info.DevEUI, "phase", "otaa")
		d.emitEvent(events.EventStatus, map[string]string{"status": "opening receive windows"})

		phy := d.Class.ReceiveWindows(JOINACCEPTDELAY1, JOINACCEPTDELAY2)
		if phy != nil {

			slog.Debug("downlink received during join", "component", "device", "dev_eui", d.Info.DevEUI)
			d.emitEvent(events.EventDownlink, map[string]string{"status": "downlink received"})

			_, err := d.ProcessDownlink(*phy)
			if err != nil {
				slog.Error("join accept processing failed", "component", "device", "dev_eui", d.Info.DevEUI, "error", err)
				d.emitErrorEvent(err)

				timerAckTimeout := time.NewTimer(d.Info.Configuration.AckTimeout)
				<-timerAckTimeout.C

				slog.Debug("ack timeout during join", "component", "device", "dev_eui", d.Info.DevEUI)
				d.emitEvent(events.EventStatus, map[string]string{"status": "ack timeout"})
			}
		} else {
			slog.Debug("no downlink received during join", "component", "device", "dev_eui", d.Info.DevEUI)
			d.emitEvent(events.EventStatus, map[string]string{"status": "no downlink received"})
		}

		if d.Info.Status.Joined {

			slog.Info("device joined", "component", "device", "dev_eui", d.Info.DevEUI)
			d.emitEvent(events.EventJoin, map[string]string{"status": "joined"})
			d.Info.Status.Mode = util.Normal

			return
		}

		slog.Debug("device unjoined", "component", "device", "dev_eui", d.Info.DevEUI)
		d.emitEvent(events.EventStatus, map[string]string{"status": "unjoined"})

	}

	return
}

func (d *Device) CreateJoinRequest() []byte {

	rand.Seed(time.Now().UTC().UnixNano())
	random := uint16(rand.Int())

	DevNonce := lorawan.DevNonce(random)
	d.Info.DevNonce = DevNonce

	phy := lorawan.PHYPayload{
		MHDR: lorawan.MHDR{
			MType: lorawan.JoinRequest,
			Major: lorawan.LoRaWANR1,
		},
		MACPayload: &lorawan.JoinRequestPayload{
			JoinEUI:  d.Info.JoinEUI, // appEUI
			DevEUI:   d.Info.DevEUI,
			DevNonce: d.Info.DevNonce,
		},
	}

	if err := phy.SetUplinkJoinMIC(d.Info.AppKey); err != nil {

		slog.Error("failed to set join MIC", "component", "device", "dev_eui", d.Info.DevEUI, "error", err)
		d.emitErrorEvent(err)

		return []byte{}
	}

	bytes, err := phy.MarshalBinary()
	if err != nil {

		slog.Error("failed to marshal join request", "component", "device", "dev_eui", d.Info.DevEUI, "error", err)
		d.emitErrorEvent(err)

		return []byte{}
	}

	return bytes

}

func (d *Device) ProcessJoinAccept(JoinAccPayload *lorawan.JoinAcceptPayload) (*dl.InformationDownlink, error) {

	var downlink dl.InformationDownlink
	var err error

	//setkeys
	d.Info.NwkSKey, err = act.GetKey(JoinAccPayload.HomeNetID, JoinAccPayload.JoinNonce, d.Info.DevNonce, d.Info.AppKey, act.PadNwkSKey)
	if err != nil {
		return nil, err
	}

	d.Info.AppSKey, err = act.GetKey(JoinAccPayload.HomeNetID, JoinAccPayload.JoinNonce, d.Info.DevNonce, d.Info.AppKey, act.PadAppSKey)
	if err != nil {
		return nil, err
	}

	d.Info.Status.Joined = true

	//cflist
	if JoinAccPayload.CFList != nil {

		slog.Debug("applying CFList", "component", "device", "dev_eui", d.Info.DevEUI)
		d.emitEvent(events.EventStatus, map[string]string{"status": "applying CFList"})

		cflist, err := JoinAccPayload.CFList.Payload.MarshalBinary()
		if err != nil {
			return nil, err
		}

		if JoinAccPayload.CFList.CFListType == lorawan.CFListChannel { //list of channel

			var CFList lorawan.CFListChannelPayload

			err = CFList.UnmarshalBinary(false, cflist)
			if err != nil {
				return nil, err
			}

			for i, c := range CFList.Channels {
				index := i + d.Info.Configuration.Region.GetNbReservedChannels()
				d.setChannel(uint8(index), c, 0, 5)
			}

		} else { //list of ChMask

			var CFList lorawan.CFListChannelMaskPayload
			err = CFList.UnmarshalBinary(false, cflist)
			if err != nil {
				return nil, err
			}

			for i, mask := range CFList.ChannelMasks {

				for j, enable := range mask {

					index := j + i*16
					d.Info.Configuration.Channels[index].EnableUplink = enable

				}
			}

		}
	}

	d.Info.JoinNonce = JoinAccPayload.JoinNonce
	d.Info.DevAddr = JoinAccPayload.DevAddr
	d.Info.NetID = JoinAccPayload.HomeNetID

	Delay := 1000
	if JoinAccPayload.RXDelay != 0 {
		Delay = Delay * int(JoinAccPayload.RXDelay)
	}

	d.Info.RX[0].Delay = time.Duration(Delay) * time.Millisecond
	d.Info.RX[1].Delay = time.Duration(Delay) * time.Millisecond

	d.Info.Configuration.RX1DROffset = JoinAccPayload.DLSettings.RX1DROffset
	d.Info.RX[1].DataRate = JoinAccPayload.DLSettings.RX2DataRate
	downlink.MType = lorawan.JoinAccept

	return &downlink, nil
}
