package device

import (
	"errors"
	"log/slog"
	"sync"

	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/classes"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/events"
	mup "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/frames/uplink/models"
	f "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/forwarder"
	res "github.com/R3DPanda1/LWN-Sim-Plus/simulator/resources"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/util"
	"github.com/brocaar/lorawan"
)

func (d *Device) Setup(Resources *res.Resources, forwarder *f.Forwarder) {

	d.State = util.Stopped

	d.Exit = make(chan struct{}, 1) // Buffered to avoid blocking TurnOFF

	d.Info.JoinEUI = lorawan.EUI64{0, 0, 0, 0, 0, 0, 0, 0}
	d.Info.NetID = lorawan.NetID{0, 0, 0}

	if !d.Info.Configuration.SupportedOtaa { //ABP

		d.Info.Status.Joined = true
		d.Info.Status.Mode = util.Normal

	} else { //otaa

		d.Info.Status.Joined = false
		d.Info.Status.Mode = util.Activation

	}

	d.Info.Configuration.Region.Setup()
	d.Info.Status.DataUplink.ADR.Setup(d.Info.Configuration.SupportedADR)

	d.Info.Status.DataUplink.DwellTime = lorawan.DwellTime400ms
	d.Info.Status.DataRate = d.Info.Configuration.DataRateInitial
	d.Info.Status.IndexchannelActive = 0

	d.Info.Status.Battery = util.ConnectedPowerSource

	d.Info.Status.InfoChannelsUS915.FirstPass = true
	d.Info.Status.InfoChannelsUS915.ListChannelsLastPass = [8]int{-1, -1, -1, -1, -1, -1, -1, -1}

	d.Info.Status.CounterRepUnConfirmedDataUp = 1
	d.Info.Configuration.NbRepUnconfirmedDataUp = 1

	//class C
	if d.Info.Configuration.SupportedClassC {
		d.Info.Status.InfoClassC.Setup()
	}

	d.Resources = Resources
	d.Info.Forwarder = forwarder

	d.Info.ReceivedDownlink.Notify = sync.NewCond(&d.Info.ReceivedDownlink.Mutex)

	d.Info.Configuration.Channels = d.Info.Configuration.Region.GetChannels()

	d.Class = classes.GetClass(classes.ClassA)
	d.Class.Setup(&d.Info)

	slog.Debug("device setup complete", "component", "device", "dev_eui", d.Info.DevEUI, "name", d.Info.Name)

}

func (d *Device) TurnOFF() {

	d.Mutex.Lock()
	d.State = util.Stopped
	d.Mutex.Unlock()

	// Non-blocking send to Exit channel (buffered size 1)
	select {
	case d.Exit <- struct{}{}:
	default:
		// Already signaled
	}

	// Wake up any goroutine blocked in ReceivedDownlink.Pull()
	d.Info.ReceivedDownlink.Signal()

}

func (d *Device) TurnON() {

	d.State = util.Running

	go d.Run()

	slog.Info("device turned on", "component", "device", "dev_eui", d.Info.DevEUI, "name", d.Info.Name)
	d.emitEvent(events.EventStatus, map[string]string{"status": "turned on"})
}

func (d *Device) IsOn() bool {

	if d.State == util.Running {
		return true
	}

	return false
}

func (d *Device) SendMACCommand(cid lorawan.CID, periodicity uint8) error {

	var command []lorawan.Payload

	if cid == lorawan.PingSlotInfoReq {

		if !d.Info.Configuration.SupportedClassB {
			return errors.New("Device don't support Class B")
		}

		command = []lorawan.Payload{
			&lorawan.MACCommand{
				CID: cid,
				Payload: &lorawan.PingSlotInfoReqPayload{
					Periodicity: periodicity,
				},
			},
		}
		d.Info.Status.InfoClassB.Periodicity = periodicity

	} else {

		command = []lorawan.Payload{
			&lorawan.MACCommand{
				CID: cid,
			},
		}

	}

	d.newMACComands(command)

	return nil
}

func (d *Device) NewUplink(mtype lorawan.MType, payload string) {

	FRMPayload := &lorawan.DataPayload{
		Bytes: []byte(payload),
	}

	info := mup.InfoFrame{
		MType:   mtype,
		Payload: FRMPayload,
	}

	d.Info.Status.BufferUplinks = append(d.Info.Status.BufferUplinks, info)

}

func (d *Device) ChangePayload(mtype lorawan.MType, payload lorawan.Payload) {

	d.Info.Status.MType = mtype
	d.Info.Status.Payload = payload

}

func (d *Device) ChangeLocation(lat float64, lng float64, alt int32) {

	d.Info.Location.Latitude = lat
	d.Info.Location.Longitude = lng
	d.Info.Location.Altitude = alt

}
