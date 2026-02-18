package device

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"time"

	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/classes"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/features"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/events"
	dl "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/frames/downlink"
	mac "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/macCommands"
	rp "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/regional_parameters"
	"github.com/brocaar/lorawan"
)

const (
	//MaxMargin is max value for margin (DevStatusReq)
	MaxMargin = int8(64)
)

//***************** MANAGE EXECUTE MAC COMMAND ******************
//*********************Uplink***********************************
//uplink
func (d *Device) newMACComands(CmdS []lorawan.Payload) {

	nCommand := len(CmdS) + len(d.Info.Status.DataUplink.FOpts)
	if nCommand > 15 {

		msg := fmt.Sprintf("Insert %d MACCommands(max 15)", nCommand)
		slog.Warn("too many MAC commands", "component", "device", "dev_eui", d.Info.DevEUI, "count", nCommand)
		d.emitEvent(events.EventMacCommand, map[string]string{"status": msg})

		return
	}

	d.Info.Status.DataUplink.FOpts = append(d.Info.Status.DataUplink.FOpts, CmdS...)

}

//*********************downlink***********************************

func (d *Device) ExecuteMACCommand(downlink dl.InformationDownlink) {

	if !d.CanExecute() {
		return
	}

	var LinkADRReqCommands [][]byte
	msg := ""

	if len(downlink.FOptsReceived) == 0 {
		msg = "None MAC Command"
	} else {
		msg = "Execute MAC Commands"
	}

	slog.Debug(msg, "component", "device", "dev_eui", d.Info.DevEUI, "count", len(downlink.FOptsReceived))
	d.emitEvent(events.EventMacCommand, map[string]string{"status": msg})

	for _, cmd := range downlink.FOptsReceived {

		cid, payloadBytes, err := mac.ParseMACCommand(cmd, false)
		if err != nil {
			slog.Error("failed to parse MAC command", "component", "device", "dev_eui", d.Info.DevEUI, "error", err)
			d.emitErrorEvent(err)
			return
		}

		switch cid {
		case lorawan.LinkCheckAns:
			d.executeLinkCheckAns(payloadBytes)
		case lorawan.LinkADRReq:
			LinkADRReqCommands = append(LinkADRReqCommands, payloadBytes)
		case lorawan.DutyCycleReq:
			d.executeDutyCycleReq(payloadBytes)
		case lorawan.RXParamSetupReq:
			d.executeRXParamSetupReq(payloadBytes)
		case lorawan.DevStatusReq:
			d.executeDevStatusReq()
		case lorawan.NewChannelReq:
			d.executeNewChannelReq(payloadBytes)
		case lorawan.RXTimingSetupReq:
			d.executeRXTimingSetupReq(payloadBytes)
		case lorawan.DLChannelReq:
			d.executeDLChannelReq(payloadBytes)
		case lorawan.TXParamSetupReq:
			d.executeTXParamSetupReq(payloadBytes)
		case lorawan.DeviceTimeAns:
			d.executeDeviceTimeAns(payloadBytes)
		case lorawan.PingSlotChannelReq:
			d.executePingSlotChannelReq(payloadBytes)
		case lorawan.PingSlotInfoAns:
			d.executePingSlotInfoAns(payloadBytes)
		case lorawan.BeaconFreqReq:
			d.executeBeaconFreqReq(payloadBytes)
		}

	}

	if len(LinkADRReqCommands) != 0 {
		d.executeLinkADRReq(LinkADRReqCommands)
	}

}

func (d *Device) executeLinkCheckAns(payload []byte) {

	c := lorawan.LinkCheckAnsPayload{}
	err := c.UnmarshalBinary(payload)
	if err != nil {
		slog.Error("LinkCheckAns unmarshal failed", "component", "device", "dev_eui", d.Info.DevEUI, "error", err)
		d.emitErrorEvent(err)
		return
	}

	slog.Debug("LinkCheckAns received", "component", "device", "dev_eui", d.Info.DevEUI, "margin", c.Margin, "gw_cnt", c.GwCnt)
	d.emitEvent(events.EventMacCommand, map[string]string{"command": "LinkCheckAns", "margin": fmt.Sprintf("%v", c.Margin), "gw_cnt": fmt.Sprintf("%v", c.GwCnt)})

}

func (d *Device) executeLinkADRReq(commands [][]byte) {

	var TXPower uint8
	var NbRep uint8

	result := true
	DataRate := -1
	channels := d.Info.Configuration.Channels

	for _, cmd := range commands {

		var response []lorawan.Payload

		c := lorawan.LinkADRReqPayload{}
		err := c.UnmarshalBinary(cmd)
		if err != nil {

			slog.Error("LinkADRReq unmarshal failed", "component", "device", "dev_eui", d.Info.DevEUI, "error", err)
			d.emitErrorEvent(err)
			return

		}

		acks, errs := d.Info.Configuration.Region.LinkAdrReq(c.Redundancy.ChMaskCntl,
			c.ChMask, c.DataRate, &channels)

		if len(errs) != 0 {

			for _, err := range errs {
		slog.Warn("LinkADRReq error", "component", "device", "dev_eui", d.Info.DevEUI, "error", err.Error())
				d.emitEvent(events.EventMacCommand, map[string]string{"command": "LinkADRReq", "status": err.Error()})
			}

		} else {
		slog.Debug("LinkADRReq valid", "component", "device", "dev_eui", d.Info.DevEUI)
			d.emitEvent(events.EventMacCommand, map[string]string{"command": "LinkADRReq", "status": "valid"})

			DataRate = int(c.DataRate)
			TXPower = c.TXPower
			NbRep = c.Redundancy.NbRep

		}

		response = []lorawan.Payload{
			&lorawan.MACCommand{
				CID: lorawan.LinkADRAns,
				Payload: &lorawan.LinkADRAnsPayload{
					ChannelMaskACK: acks[0],
					DataRateACK:    acks[1],
					PowerACK:       acks[2],
				},
			},
		}

		d.newMACComands(response)

		result = result && acks[0] && acks[1] && acks[2]

	}

	if result {

		d.Info.Status.DataRate = uint8(DataRate)
		slog.Debug("LinkADRReq set datarate", "component", "device", "dev_eui", d.Info.DevEUI, "data_rate", d.Info.Status.DataRate)
		d.emitEvent(events.EventMacCommand, map[string]string{"command": "LinkADRReq", "data_rate": fmt.Sprintf("%v", d.Info.Status.DataRate)})

		d.Info.Status.TXPower = TXPower
		slog.Debug("LinkADRReq set tx power", "component", "device", "dev_eui", d.Info.DevEUI, "tx_power", TXPower)
		d.emitEvent(events.EventMacCommand, map[string]string{"command": "LinkADRReq", "tx_power": fmt.Sprintf("%v", TXPower)})

		d.Info.Configuration.NbRepUnconfirmedDataUp = NbRep
		slog.Debug("LinkADRReq set nb rep", "component", "device", "dev_eui", d.Info.DevEUI, "nb_rep", NbRep)
		d.emitEvent(events.EventMacCommand, map[string]string{"command": "LinkADRReq", "nb_rep": fmt.Sprintf("%v", NbRep)})

		d.Info.Configuration.Channels = channels
		slog.Debug("LinkADRReq channels updated", "component", "device", "dev_eui", d.Info.DevEUI)
		d.emitEvent(events.EventMacCommand, map[string]string{"command": "LinkADRReq", "status": "channels updated"})

		slog.Debug("LinkADRReq executed", "component", "device", "dev_eui", d.Info.DevEUI)
		d.emitEvent(events.EventMacCommand, map[string]string{"command": "LinkADRReq", "status": "executed"})

	} else {

		slog.Warn("LinkADRReq refused", "component", "device", "dev_eui", d.Info.DevEUI)
		d.emitEvent(events.EventMacCommand, map[string]string{"command": "LinkADRReq", "status": "refused"})

	}

}

func (d *Device) executeDutyCycleReq(payload []byte) {

	c := lorawan.DutyCycleReqPayload{}

	err := c.UnmarshalBinary(payload)
	if err != nil {

		msg := fmt.Sprintf("UnmarshalBinary %v", err)
		errs := errors.New(msg)
		slog.Error("DutyCycleReq unmarshal failed", "component", "device", "dev_eui", d.Info.DevEUI, "error", err)
		d.emitErrorEvent(errs)

		return
	}

	//invia i dati all'interfaccia
	aggregatedDC := 1 / math.Pow(2, float64(c.MaxDCycle))

	slog.Debug("DutyCycleReq executed", "component", "device", "dev_eui", d.Info.DevEUI, "duty_cycle", aggregatedDC)
	d.emitEvent(events.EventMacCommand, map[string]string{"command": "DutyCycleReq", "duty_cycle": fmt.Sprintf("%v", aggregatedDC)})

	//ack
	response := []lorawan.Payload{
		&lorawan.MACCommand{
			CID:     lorawan.DutyCycleAns,
			Payload: &lorawan.DevStatusAnsPayload{},
		},
	}

	d.newMACComands(response)

}

//require ack
func (d *Device) executeRXParamSetupReq(payload []byte) {

	c := lorawan.RXParamSetupReqPayload{}
	err := c.UnmarshalBinary(payload)
	if err != nil {
		slog.Error("RXParamSetupReq unmarshal failed", "component", "device", "dev_eui", d.Info.DevEUI, "error", err)
		d.emitErrorEvent(err)
		return
	}

	//RX[0]
	RX1DROffsetACK := false

	if err = d.Info.Configuration.Region.RX1DROffsetSupported(c.DLSettings.RX1DROffset); err != nil {
		slog.Warn("RXParamSetupReq RX1DROffset unsupported", "component", "device", "dev_eui", d.Info.DevEUI, "error", err)
		d.emitEvent(events.EventMacCommand, map[string]string{"command": "RXParamSetupReq", "status": err.Error()})
	} else {
		RX1DROffsetACK = true
	}

	//RX[1]
	ChannelACK := false
	if err = d.isSupportedFrequency(c.Frequency); err != nil {
		slog.Warn("RXParamSetupReq frequency unsupported", "component", "device", "dev_eui", d.Info.DevEUI, "error", err)
		d.emitEvent(events.EventMacCommand, map[string]string{"command": "RXParamSetupReq", "status": err.Error()})
	} else {
		ChannelACK = true
	}

	RX2DataRateACK := false
	if err = d.isSupportedDR(c.DLSettings.RX2DataRate); err != nil {
		slog.Warn("RXParamSetupReq RX2 data rate unsupported", "component", "device", "dev_eui", d.Info.DevEUI, "error", err)
		d.emitEvent(events.EventMacCommand, map[string]string{"command": "RXParamSetupReq", "status": err.Error()})
	} else {
		RX2DataRateACK = true
	}

	if RX1DROffsetACK && ChannelACK && RX2DataRateACK {

		d.Info.Configuration.RX1DROffset = c.DLSettings.RX1DROffset //RX1DROffset ACK
		d.Info.RX[1].SetListeningFrequency(c.Frequency)             //Channel Frequency RX2
		d.Info.RX[1].DataRate = c.DLSettings.RX2DataRate            //RX2DataRate

		slog.Debug("RXParamSetupReq executed", "component", "device", "dev_eui", d.Info.DevEUI)
		d.emitEvent(events.EventMacCommand, map[string]string{"command": "RXParamSetupReq", "status": "executed"})

	} else {
		slog.Warn("RXParamSetupReq refused", "component", "device", "dev_eui", d.Info.DevEUI)
		d.emitEvent(events.EventMacCommand, map[string]string{"command": "RXParamSetupReq", "status": "refused"})
	}

	//ack
	response := []lorawan.Payload{
		&lorawan.MACCommand{
			CID: lorawan.RXParamSetupAns,
			Payload: &lorawan.RXParamSetupAnsPayload{
				ChannelACK:     ChannelACK,
				RX2DataRateACK: RX2DataRateACK,
				RX1DROffsetACK: RX1DROffsetACK,
			},
		},
	}

	d.Info.Status.DataUplink.AckMacCommand.SetRXParamSetupAns(response)

}

func (d *Device) executeDevStatusReq() {

	rand.Seed(time.Now().UTC().UnixNano())
	margin := int8(rand.Int()) % MaxMargin //range

	if margin < 0 {
		margin = -margin
	}

	if margin <= 32 {
		margin = margin - 32
	} else {
		margin %= 32
	}

	response := []lorawan.Payload{
		&lorawan.MACCommand{
			CID: lorawan.DevStatusAns,
			Payload: &lorawan.DevStatusAnsPayload{
				Battery: d.Info.Status.Battery,
				Margin:  margin,
			},
		},
	}

	slog.Debug("DevStatusReq executed", "component", "device", "dev_eui", d.Info.DevEUI, "battery", d.Info.Status.Battery, "margin", margin)
	d.emitEvent(events.EventMacCommand, map[string]string{"command": "DevStatusReq", "status": "executed"})

	d.newMACComands(response)
}

func (d *Device) executeNewChannelReq(payload []byte) {

	switch d.Info.Configuration.Region.GetCode() {
	case rp.Code_Us915, rp.Code_Au915:

		slog.Debug("NewChannelReq not implemented in region", "component", "device", "dev_eui", d.Info.DevEUI)
		d.emitEvent(events.EventMacCommand, map[string]string{"command": "NewChannelReq", "status": "not implemented in region"})

		return

	}

	c := lorawan.NewChannelReqPayload{}
	err := c.UnmarshalBinary(payload)

	if err != nil {

		slog.Error("NewChannelReq unmarshal failed", "component", "device", "dev_eui", d.Info.DevEUI, "error", err)
		d.emitErrorEvent(err)
		return

	}

	DataRateOK, FreqOK := d.setChannel(c.ChIndex, c.Freq, c.MinDR, c.MaxDR)
	if DataRateOK && FreqOK {

		slog.Debug("NewChannelReq executed", "component", "device", "dev_eui", d.Info.DevEUI, "ch_index", c.ChIndex)
		d.emitEvent(events.EventMacCommand, map[string]string{"command": "NewChannelReq", "status": "executed"})

	} else {

		slog.Warn("NewChannelReq refused", "component", "device", "dev_eui", d.Info.DevEUI)
		d.emitEvent(events.EventMacCommand, map[string]string{"command": "NewChannelReq", "status": "refused"})

	}

	//response
	response := []lorawan.Payload{
		&lorawan.MACCommand{
			CID: lorawan.NewChannelAns,
			Payload: &lorawan.NewChannelAnsPayload{
				DataRateRangeOK:    DataRateOK,
				ChannelFrequencyOK: FreqOK,
			},
		},
	}

	d.newMACComands(response)

}

//require ack
func (d *Device) executeRXTimingSetupReq(payload []byte) {

	c := lorawan.RXTimingSetupReqPayload{}

	err := c.UnmarshalBinary(payload)
	if err != nil {

		slog.Error("RXTimingSetupReq unmarshal failed", "component", "device", "dev_eui", d.Info.DevEUI, "error", err)
		d.emitErrorEvent(err)
		return

	}

	delay := c.Delay
	if delay == 0 {
		delay = features.ReceiveDelay
	}

	d.Info.RX[0].Delay = time.Duration(delay) * time.Second

	slog.Debug("RXTimingSetupReq executed", "component", "device", "dev_eui", d.Info.DevEUI, "delay", delay)
	d.emitEvent(events.EventMacCommand, map[string]string{"command": "RXTimingSetupReq", "status": "executed"})
	//ack
	response := []lorawan.Payload{
		&lorawan.MACCommand{
			CID: lorawan.RXTimingSetupAns,
		},
	}

	d.Info.Status.DataUplink.AckMacCommand.SetRXTimingSetupAns(response)
}

//require ack
func (d *Device) executeDLChannelReq(payload []byte) {

	switch d.Info.Configuration.Region.GetCode() {
	case rp.Code_Us915, rp.Code_Au915:

		slog.Debug("DLChannelReq not implemented in region", "component", "device", "dev_eui", d.Info.DevEUI)
		d.emitEvent(events.EventMacCommand, map[string]string{"command": "DLChannelReq", "status": "not implemented in region"})

		return
	}

	c := lorawan.DLChannelReqPayload{}

	err := c.UnmarshalBinary(payload)
	if err != nil {

		msg := fmt.Sprintf("UnmarshalBinary %v", err)
		errs := errors.New(msg)
		slog.Error("DLChannelReq unmarshal failed", "component", "device", "dev_eui", d.Info.DevEUI, "error", err)
		d.emitErrorEvent(errs)

		return
	}

	FreqUpExists, FreqOk := false, false

	err = d.isSupportedFrequency(c.Freq)
	if err == nil {
		FreqUpExists = d.setFrequencyDownlink(c.ChIndex, c.Freq)
		FreqOk = true
	}

	//ack
	if FreqUpExists && FreqOk {

		slog.Debug("DLChannelReq executed", "component", "device", "dev_eui", d.Info.DevEUI)
		d.emitEvent(events.EventMacCommand, map[string]string{"command": "DLChannelReq", "status": "executed"})

	} else {

		slog.Warn("DLChannelReq refused", "component", "device", "dev_eui", d.Info.DevEUI)
		d.emitEvent(events.EventMacCommand, map[string]string{"command": "DLChannelReq", "status": "refused"})

	}

	response := []lorawan.Payload{
		&lorawan.MACCommand{
			CID: lorawan.DLChannelAns,
			Payload: &lorawan.DLChannelAnsPayload{
				ChannelFrequencyOK:    FreqOk,
				UplinkFrequencyExists: FreqUpExists,
			},
		},
	}

	d.Info.Status.DataUplink.AckMacCommand.SetDLChannelAns(response)

}

func (d *Device) executeDeviceTimeAns(payload []byte) {
	c := lorawan.DeviceTimeAnsPayload{}

	err := c.UnmarshalBinary(payload)
	if err != nil {

		slog.Error("DeviceTimeAns unmarshal failed", "component", "device", "dev_eui", d.Info.DevEUI, "error", err)
		d.emitErrorEvent(err)
		return

	}

	slog.Debug("DeviceTimeAns received", "component", "device", "dev_eui", d.Info.DevEUI, "time_since_gps_epoch", c.TimeSinceGPSEpoch)
	d.emitEvent(events.EventMacCommand, map[string]string{"command": "DeviceTimeAns", "time": fmt.Sprintf("%v", c.TimeSinceGPSEpoch)})

}

func (d *Device) executeTXParamSetupReq(payload []byte) {

	switch d.Info.Configuration.Region.GetCode() {
	case rp.Code_Au915, rp.Code_As923:
	default:
		slog.Debug("TXParamSetupReq not implemented in region", "component", "device", "dev_eui", d.Info.DevEUI)
		d.emitEvent(events.EventMacCommand, map[string]string{"command": "TXParamSetupReq", "status": "not implemented in region"})
		return
	}

	c := lorawan.TXParamSetupReqPayload{}

	err := c.UnmarshalBinary(payload)
	if err != nil {

		slog.Error("TXParamSetupReq unmarshal failed", "component", "device", "dev_eui", d.Info.DevEUI, "error", err)
		d.emitErrorEvent(err)
		return

	}

	//c.MaxEIRP
	d.Info.Status.DataUplink.DwellTime = c.UplinkDwellTime
	d.Info.Status.DataDownlink.DwellTime = c.DownlinkDwelltime

	response := []lorawan.Payload{
		&lorawan.MACCommand{
			CID: lorawan.TXParamSetupAns,
		},
	}

	slog.Debug("TXParamSetupReq executed", "component", "device", "dev_eui", d.Info.DevEUI)
	d.emitEvent(events.EventMacCommand, map[string]string{"command": "TXParamSetupReq", "status": "executed"})

	d.newMACComands(response)
}

/****************CLASS B MAC COMMAND****************/

func (d *Device) executePingSlotInfoAns(payload []byte) {

	if !d.Info.Configuration.SupportedClassB {
		return
	}

	d.SwitchClass(classes.ClassB)

}

func (d *Device) executeBeaconFreqReq(payload []byte) {

	command := lorawan.BeaconFreqReqPayload{}

	if !d.Info.Configuration.SupportedClassB {
		return
	}

	err := command.UnmarshalBinary(payload)
	if err != nil {
		slog.Error("BeaconFreqReq unmarshal failed", "component", "device", "dev_eui", d.Info.DevEUI, "error", err)
		d.emitErrorEvent(err)

		return
	}

	freqOk := false
	if command.Frequency == 0 {
		d.Info.Status.InfoClassB.FrequencyBeacon = d.Info.Configuration.Region.GetFrequencyBeacon()
	} else {
		err := d.isSupportedFrequency(command.Frequency)
		if err == nil {
			freqOk = true
			d.Info.Status.InfoClassB.FrequencyBeacon = command.Frequency
		}
	}

	response := []lorawan.Payload{
		&lorawan.MACCommand{
			CID: lorawan.BeaconFreqAns,
			Payload: &lorawan.BeaconFreqAnsPayload{
				BeaconFrequencyOK: freqOk,
			},
		},
	}

	d.newMACComands(response)
}

func (d *Device) executePingSlotChannelReq(payload []byte) {

	if !d.Info.Configuration.SupportedClassB {
		return
	}

	c := lorawan.PingSlotChannelReqPayload{}

	err := c.UnmarshalBinary(payload)
	if err != nil {

		slog.Error("PingSlotChannelReq unmarshal failed", "component", "device", "dev_eui", d.Info.DevEUI, "error", err)
		d.emitErrorEvent(err)
		return

	}

	FreqOK, DataRateOK := false, false
	err = d.isSupportedFrequency(c.Frequency)
	if err == nil {
		FreqOK = true
	}

	err = d.isSupportedDR(c.DR)
	if err == nil {
		DataRateOK = true
	}

	if FreqOK && DataRateOK {
		d.Info.Status.InfoClassB.PingSlot.SetListeningFrequency(c.Frequency) //set frequency listen
		d.Info.Status.InfoClassB.PingSlot.DataRate = c.DR                    // set datarate
	}

	//response
	response := []lorawan.Payload{
		&lorawan.MACCommand{
			CID: lorawan.PingSlotChannelAns,
			Payload: &lorawan.PingSlotChannelAnsPayload{
				DataRateOK:         DataRateOK,
				ChannelFrequencyOK: FreqOK,
			},
		},
	}

	d.newMACComands(response)

}

func PrintMACCommand(cmd string, content string) string {
	return fmt.Sprintf("%v | %v |", cmd, content)
}
