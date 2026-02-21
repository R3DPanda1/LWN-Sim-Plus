package gateway

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/events"
	pkt "github.com/R3DPanda1/LWN-Sim-Plus/simulator/resources/communication/packets"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/resources/communication/udp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	pushDataCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gateway_data_sent_total",
		Help: "The total number of gateway PUSH DATA",
	})
	pullDataCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gateway_pull_data_total",
		Help: "The total number of gateway PULL DATA",
	})
)

func (g *Gateway) SenderVirtual() {


	go g.KeepAlive()

	for {

		rxpk, ok := g.BufferUplink.Pop() //wait uplink
		if !ok || !g.CanExecute() {
			return
		}

		g.Stat.RXNb++
		g.Stat.RXOK++

		packet, err := g.createPacket(rxpk)
		if err != nil {
			slog.Error("failed to create packet", "component", "gateway", "gateway_mac", g.Info.MACAddress, "error", err)
			g.emitErrorEvent(err)
		}

		_, err = udp.SendDataUDP(g.Info.Connection, packet)
		if err != nil {

			msg := fmt.Sprintf("Unable to send data to %v, it may be off", *g.Info.BridgeAddress)
			slog.Error("unable to send data", "component", "gateway", "gateway_mac", g.Info.MACAddress, "bridge", *g.Info.BridgeAddress)
			g.emitErrorEvent(errors.New(msg))

		} else {
			slog.Debug("push data sent", "component", "gateway", "gateway_mac", g.Info.MACAddress)
			g.emitEvent(events.GwEventPushData, nil)
			pushDataCounter.Inc()
		}

	}

}

func (g *Gateway) SenderReal() {


	for {

		rxpk, ok := g.BufferUplink.Pop() //wait uplink
		if !ok || !g.CanExecute() {
			return
		}

		g.Stat.RXNb++
		g.Stat.RXOK++

		packet, err := g.createPacket(rxpk)
		if err != nil {
			slog.Error("failed to create packet", "component", "gateway", "gateway_mac", g.Info.MACAddress, "error", err)
			g.emitErrorEvent(err)
		}

		_, err = udp.SendDataUDP(g.Info.Connection, packet)
		if err != nil {

			msg := fmt.Sprintf("Unable to send data to %v, it may be off", *g.Info.BridgeAddress)
			slog.Error("unable to send data", "component", "gateway", "gateway_mac", g.Info.MACAddress, "bridge", *g.Info.BridgeAddress)
			g.emitErrorEvent(errors.New(msg))

		} else {
			slog.Debug("push data forwarded", "component", "gateway", "gateway_mac", g.Info.MACAddress, "addr", g.Info.AddrIP, "port", g.Info.Port)
			g.emitEvent(events.GwEventPushData, map[string]string{"addr": g.Info.AddrIP, "port": g.Info.Port})
			pushDataCounter.Inc()
		}

	}
}

func (g *Gateway) sendPullData() error {

	if !g.CanExecute() {
		return nil
	}

	pulldata, _ := pkt.CreatePacket(pkt.TypePullData, g.Info.MACAddress, pkt.Stat{}, nil, 0)

	_, err := udp.SendDataUDP(g.Info.Connection, pulldata)

	return err
}

func (g *Gateway) createPacket(info pkt.RXPK) ([]byte, error) {

	stat := pkt.Stat{
		Time: pkt.GetTime(),
		Lati: g.Info.Location.Latitude,
		Long: g.Info.Location.Longitude,
		Alti: g.Info.Location.Altitude,
		RXNb: g.Stat.RXNb,
		RXOK: g.Stat.RXOK,
		RXFW: g.Stat.RXFW,
		ACKR: g.Stat.ACKR,
		DWNb: g.Stat.DWNb,
		TXNb: g.Stat.TXNb,
	}

	rxpks := []pkt.RXPK{
		info,
	}

	return pkt.CreatePacket(pkt.TypePushData, g.Info.MACAddress, stat, rxpks, 0)
}

func (g *Gateway) KeepAlive() {

	tickerKeepAlive := time.NewTicker(g.Info.KeepAlive)

	for {
		if !g.CanExecute() {

			return

		} else {

			err := g.sendPullData()
			if err != nil {
				slog.Error("failed to send pull data", "component", "gateway", "gateway_mac", g.Info.MACAddress, "error", err)
				g.emitErrorEvent(err)
			} else {
				slog.Debug("pull data sent", "component", "gateway", "gateway_mac", g.Info.MACAddress)
				pullDataCounter.Inc()
			}

		}

		<-tickerKeepAlive.C
	}

}
