package device

import (
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/classes"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/features/adr"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/events"
	dl "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/frames/downlink"
	rp "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/regional_parameters"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/metrics"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/util"
	"github.com/brocaar/lorawan"
)

func (d *Device) Execute() {

	var downlink *dl.InformationDownlink
	var err error

	err = nil
	downlink = nil

	d.SwitchChannel()

	uplinks := d.CreateUplink()
	for i := 0; i < len(uplinks); i++ {

		data := d.SetInfo(uplinks[i], false)
		d.Class.SendData(data)

		slog.Debug("uplink sent", "component", "device", "dev_eui", d.Info.DevEUI, "class", d.Class.ToString())
		d.emitEvent(events.EventUp, map[string]string{"status": "uplink sent"})
		metrics.UplinksTotal.Inc()
	}

	slog.Debug("opening receive windows", "component", "device", "dev_eui", d.Info.DevEUI)
	d.emitEvent(events.EventStatus, map[string]string{"status": "opening receive windows"})
	phy := d.Class.ReceiveWindows(0, 0)

	if phy != nil {

		slog.Debug("downlink received", "component", "device", "dev_eui", d.Info.DevEUI)
		d.emitEvent(events.EventDownlink, map[string]string{"status": "downlink received"})
		metrics.DownlinksTotal.Inc()

		downlink, err = d.ProcessDownlink(*phy)
		if err != nil {
			slog.Error("downlink processing failed", "component", "device", "dev_eui", d.Info.DevEUI, "error", err)
			d.emitErrorEvent(err)
			return
		}

		if downlink != nil { //downlink ricevuto

			d.ExecuteMACCommand(*downlink)

			if d.Info.Status.Mode != util.Retransmission {
				d.FPendingProcedure(downlink)
			}

		}

	} else {

		slog.Debug("no downlinks received", "component", "device", "dev_eui", d.Info.DevEUI)
		d.emitEvent(events.EventStatus, map[string]string{"status": "no downlinks received"})

		timerAckTimeout := time.NewTimer(d.Info.Configuration.AckTimeout)
		<-timerAckTimeout.C

		slog.Debug("ack timeout", "component", "device", "dev_eui", d.Info.DevEUI)
		d.emitEvent(events.EventStatus, map[string]string{"status": "ack timeout"})
	}

	d.ADRProcedure()

	//retransmission
	switch d.Info.Status.LastMType {

	case lorawan.ConfirmedDataUp:

		if d.Class.GetClass() == classes.ClassC {
			if d.Info.Status.InfoClassC.GetACK() {
				return
			}
		}

		err := d.Class.RetransmissionCData(downlink)
		if err != nil {

			slog.Error("confirmed retransmission failed", "component", "device", "dev_eui", d.Info.DevEUI, "error", err)
			d.emitErrorEvent(err)

			d.UnJoined()

		}

		if d.Info.Status.Mode == util.Retransmission {

			d.Info.Status.DataRate = rp.DecrementDataRate(d.Info.Configuration.Region, d.Info.Status.DataRate)

		}

	case lorawan.UnconfirmedDataUp:

		err := d.Class.RetransmissionUnCData(downlink)
		if err != nil {
			slog.Error("unconfirmed retransmission failed", "component", "device", "dev_eui", d.Info.DevEUI, "error", err)
			d.emitErrorEvent(err)
		}
	}

}

func (d *Device) FPendingProcedure(downlink *dl.InformationDownlink) {

	var err error
	if !d.CanExecute() {
		return
	}

	startProcedure := 0 //per la print finale

	for downlink != nil {

		if downlink.FPending {

			slog.Debug("fpending set", "component", "device", "dev_eui", d.Info.DevEUI)
			d.emitEvent(events.EventStatus, map[string]string{"status": "fpending set"})

			if startProcedure == 0 {
				d.Info.Status.Mode = util.FPending
				slog.Debug("start fpending procedure", "component", "device", "dev_eui", d.Info.DevEUI)
				d.emitEvent(events.EventStatus, map[string]string{"status": "start fpending procedure"})
				startProcedure = 1
			}

			if downlink.MType == lorawan.UnconfirmedDataDown {
				d.SendEmptyFrame()
			}
			//ack sent in resolveDownlinks ergo open Receive Windows

			slog.Debug("opening receive windows", "component", "device", "dev_eui", d.Info.DevEUI)
			d.emitEvent(events.EventStatus, map[string]string{"status": "opening receive windows"})
			phy := d.Class.ReceiveWindows(0, 0)

			if !d.CanExecute() { //stop
				return
			}

			if phy != nil {

				slog.Debug("downlink received", "component", "device", "dev_eui", d.Info.DevEUI)
				d.emitEvent(events.EventDownlink, map[string]string{"status": "downlink received"})
				metrics.DownlinksTotal.Inc()

				downlink, err = d.ProcessDownlink(*phy)
				if err != nil {
					slog.Error("downlink processing failed", "component", "device", "dev_eui", d.Info.DevEUI, "error", err)
					d.emitErrorEvent(err)

				}

				if downlink != nil { //downlink ricevuto

					d.ExecuteMACCommand(*downlink)

				}

			} else {

				downlink = nil

				slog.Debug("no downlinks received", "component", "device", "dev_eui", d.Info.DevEUI)
				d.emitEvent(events.EventStatus, map[string]string{"status": "no downlinks received"})

				timerAckTimeout := time.NewTimer(d.Info.Configuration.AckTimeout)
				<-timerAckTimeout.C

				slog.Debug("ack timeout", "component", "device", "dev_eui", d.Info.DevEUI)
				d.emitEvent(events.EventStatus, map[string]string{"status": "ack timeout"})

			}

			d.ADRProcedure()

		} else {
			slog.Debug("fpending unset", "component", "device", "dev_eui", d.Info.DevEUI)
			d.emitEvent(events.EventStatus, map[string]string{"status": "fpending unset"})
			break
		}

	}

	if startProcedure == 1 {
		slog.Debug("fpending procedure finished", "component", "device", "dev_eui", d.Info.DevEUI)
		d.emitEvent(events.EventStatus, map[string]string{"status": "fpending procedure finished"})
	}

	d.Info.Status.Mode = util.Normal

}

func (d *Device) ADRProcedure() {

	dr, code := d.Info.Status.DataUplink.ADR.ADRProcedure(d.Info.Status.DataRate, d.Info.Configuration.Region, d.Info.Configuration.SupportedADR)

	switch code {

	case adr.CodeNoneError:
		d.Info.Status.DataRate = dr
		break

	case adr.CodeADRFlagReqSet:
		slog.Debug("set ADRACKReq flag", "component", "device", "dev_eui", d.Info.DevEUI)
		d.emitEvent(events.EventStatus, map[string]string{"status": "set ADRACKReq flag"})
		break

	case adr.CodeUnjoined:
		if UnJoined := d.UnJoined(); UnJoined {

			d.OtaaActivation()

			msg := d.Info.Status.DataUplink.ADR.Reset()
			if msg != "" {
				slog.Debug("adr reset", "component", "device", "dev_eui", d.Info.DevEUI, "msg", msg)
				d.emitEvent(events.EventStatus, map[string]string{"status": msg})
			}

		}
	}

}

func (d *Device) SwitchChannel() {

	rand.Seed(time.Now().UTC().UnixNano())

	lenChannels := len(d.Info.Configuration.Channels)
	chanUsed := make(map[int]bool)
	lenTrue := 1

	var random int
	var indexGroup int
	regionCode := d.Info.Configuration.Region.GetCode()

	if regionCode == rp.Code_Us915 {

		indexGroup = int(d.Info.Status.IndexchannelActive / 8)

		switch indexGroup {

		case 0:

			if d.Info.Status.InfoChannelsUS915.FirstPass {
				d.Info.Status.InfoChannelsUS915.FirstPass = false
			} else {
				indexGroup++
			}

			break

		case 1, 2, 3, 4, 5, 6:
			indexGroup++
			break

		case 7:

			random = indexGroup + 64

			msg := fmt.Sprintf("Switch channel from %v to %v", d.Info.Status.IndexchannelActive, random)
			slog.Debug("switch channel", "component", "device", "dev_eui", d.Info.DevEUI, "from", d.Info.Status.IndexchannelActive, "to", random)
			d.emitEvent(events.EventStatus, map[string]string{"status": msg})

			d.Info.Status.IndexchannelActive = uint16(random)
			return

		default:
			indexGroup = 0
			break
		}

		lenChannels = 8

	}

	for lenTrue != lenChannels {

		//random
		if regionCode == rp.Code_Us915 {

			random = (rand.Int() % 8) + indexGroup*8

			for random == d.Info.Status.InfoChannelsUS915.ListChannelsLastPass[indexGroup] {
				random = (rand.Int() % 8) + indexGroup*8
			}

		} else {
			random = rand.Int() % lenChannels
		}

		if !chanUsed[random] { //evita il loop infinito

			if d.Info.Configuration.Channels[random].Active &&
				d.Info.Configuration.Channels[random].EnableUplink { //attivo e enable Uplink

				if d.Info.Configuration.Channels[random].IsSupportedDR(d.Info.Status.DataRate) == nil {

					oldindex := d.Info.Status.IndexchannelActive

					if oldindex != uint16(random) {
						d.Info.Status.IndexchannelActive = uint16(random)

						msg := fmt.Sprintf("Switch channel from %v to %v", oldindex, random)
						slog.Debug("switch channel", "component", "device", "dev_eui", d.Info.DevEUI, "from", oldindex, "to", random)
						d.emitEvent(events.EventStatus, map[string]string{"status": msg})

						d.Info.Status.InfoChannelsUS915.ListChannelsLastPass[indexGroup] = random // lo fa anche se region non è US_915 (no problem)

						return
					}

				}

			}

			chanUsed[random] = true
			lenTrue++

		}

	}

	if lenTrue == lenChannels { //nessun canale abilitato all'uplink supporta il DataRate

		var msg string
		oldindex := d.Info.Status.IndexchannelActive

		slog.Warn("no channels available", "component", "device", "dev_eui", d.Info.DevEUI)
		d.emitEvent(events.EventStatus, map[string]string{"status": "no channels available"})

		if regionCode == rp.Code_Us915 {

			d.Info.Status.InfoChannelsUS915.ListChannelsLastPass[indexGroup] = indexGroup * 8
			d.Info.Status.IndexchannelActive = uint16(indexGroup * 8)
			d.Info.Configuration.Channels[d.Info.Status.IndexchannelActive].EnableUplink = true

			msg = fmt.Sprintf("Channel %v enabled to send uplinks", d.Info.Status.IndexchannelActive)
			slog.Debug("channel enabled for uplinks", "component", "device", "dev_eui", d.Info.DevEUI, "channel", d.Info.Status.IndexchannelActive)
			d.emitEvent(events.EventStatus, map[string]string{"status": msg})

		} else {
			d.Info.Status.IndexchannelActive = uint16(0)
		}

		d.Info.Status.DataRate = d.Info.Configuration.Channels[d.Info.Status.IndexchannelActive].MaxDR
		if oldindex == d.Info.Status.IndexchannelActive {
			msg = fmt.Sprintf("Use channel[%v] with dataRate %v", d.Info.Status.IndexchannelActive, d.Info.Status.DataRate)
		} else {
			msg = fmt.Sprintf("Switch channel from %v to %v with DataRate %v", oldindex, d.Info.Status.IndexchannelActive, d.Info.Status.DataRate)
		}

		slog.Debug("channel update", "component", "device", "dev_eui", d.Info.DevEUI, "channel", d.Info.Status.IndexchannelActive, "data_rate", d.Info.Status.DataRate)
		d.emitEvent(events.EventStatus, map[string]string{"status": msg})

		return
	}

}

func (d *Device) SwitchClass(class int) {

	if class == d.Class.GetClass() {
		return
	}

	switch class {

	case classes.ClassA:
		d.Class = classes.GetClass(classes.ClassA)
		d.Class.Setup(&d.Info)

	case classes.ClassB:

		d.Class = classes.GetClass(classes.ClassB)
		d.Class.Setup(&d.Info)

	case classes.ClassC:

		d.Class = classes.GetClass(classes.ClassC)
		d.Class.Setup(&d.Info)
		go d.DownlinkReceivedRX2ClassC()

	default:
		slog.Warn("class not supported", "component", "device", "dev_eui", d.Info.DevEUI, "class", class)
		d.emitEvent(events.EventError, map[string]string{"error": "class not supported"})

	}

	msg := fmt.Sprintf("Switch in class %v", d.Class.ToString())
	slog.Debug("switched class", "component", "device", "dev_eui", d.Info.DevEUI, "class", d.Class.ToString())
	d.emitEvent(events.EventStatus, map[string]string{"status": msg})

}

//se il dispositivo non supporta OTAA non può essere unjoined
func (d *Device) UnJoined() bool {

	if d.Info.Configuration.SupportedOtaa {
		d.Info.Status.Joined = false
		return true //Otaa
	}
	return false //ABP

}
