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
	pushAckCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gateway_push_ack_total",
		Help: "The total number of gateway PUSH ACK",
	})
	pullAckCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gateway_pull_ack_total",
		Help: "The total number of gateway PULL ACK",
	})
	pullRespCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gateway_pull_resp_total",
		Help: "The total number of gateway PULL RESP",
	})
)

func (g *Gateway) Receiver() {

	ReceiveBuffer := make([]byte, 1024)

	defer g.Resources.ExitGroup.Done()

	for {
		var n int
		var err error

		if !g.CanExecute() {

			slog.Info("gateway receiver turned off", "component", "gateway", "gateway_mac", g.Info.MACAddress)
			g.emitEvent(events.GwEventDisconnected, nil)
			return

		}

		for g.Info.Connection == nil {

			if !g.CanExecute() {

				slog.Info("gateway receiver turned off", "component", "gateway", "gateway_mac", g.Info.MACAddress)
				g.emitEvent(events.GwEventDisconnected, nil)
				return

			}

			g.Info.Connection, err = udp.ConnectTo(*g.Info.BridgeAddress) //stabilish new connection
			if err != nil {

				msg := fmt.Sprintf("Unable Connect to %v", g.Info.BridgeAddress)
				slog.Error("unable to connect", "component", "gateway", "gateway_mac", g.Info.MACAddress, "bridge", fmt.Sprintf("%v", g.Info.BridgeAddress))
				g.emitErrorEvent(errors.New(msg))

				continue

			}

		}

		n, _, err = g.Info.Connection.ReadFromUDP(ReceiveBuffer)

		if !g.CanExecute() {
			slog.Info("gateway receiver turned off", "component", "gateway", "gateway_mac", g.Info.MACAddress)
			g.emitEvent(events.GwEventDisconnected, nil)
			return
		}

		if err != nil {

			msg := fmt.Sprintf("No connection with %v, it may be off", *g.Info.BridgeAddress)
			slog.Error("no connection with bridge", "component", "gateway", "gateway_mac", g.Info.MACAddress, "bridge", *g.Info.BridgeAddress)
			g.emitErrorEvent(errors.New(msg))

			continue

		}

		receivedPack := ReceiveBuffer[:n]

		g.Stat.DWNb++

		err = pkt.ParseReceivePacket(receivedPack)
		if err != nil {
			slog.Warn("unsupported packet received", "component", "gateway", "gateway_mac", g.Info.MACAddress)
			continue
		}

		time.Sleep(time.Second) //sync le print

		slog.Debug("packet received", "component", "gateway", "gateway_mac", g.Info.MACAddress, "type", pkt.PacketToString(receivedPack[3]))

		typepkt := pkt.GetTypePacket(receivedPack)
		switch *typepkt {

		case pkt.TypePushAck:
			g.Stat.ACKR++
			pushAckCounter.Inc()

		case pkt.TypePullAck:
			pullAckCounter.Inc()
			break

		case pkt.TypePullResp:

			phy, freq, err := pkt.GetInfoPullResp(receivedPack)
			if err != nil {
				slog.Error("failed to parse pull resp", "component", "gateway", "gateway_mac", g.Info.MACAddress, "error", err)
				g.emitErrorEvent(err)
				continue
			}

			delivered := g.Forwarder.Downlink(phy, *freq, g.Info.MACAddress)

			g.Stat.RXFW++

			pullRespCounter.Inc()

			slog.Debug("pull resp received", "component", "gateway", "gateway_mac", g.Info.MACAddress, "delivered", delivered)
			g.emitEvent(events.GwEventPullResp, map[string]string{"delivered": fmt.Sprintf("%v", delivered)})

			// Only send TX ACK if at least one device received the downlink
			if !delivered {
				slog.Debug("no device listening, tx ack not sent", "component", "gateway", "gateway_mac", g.Info.MACAddress)
				continue
			}

			//TX ACK
			packet, err := pkt.CreatePacket(pkt.TypeTxAck, g.Info.MACAddress, pkt.Stat{}, nil, pkt.GetTokenFromPullResp(receivedPack))
			if err != nil {
				slog.Error("failed to create tx ack", "component", "gateway", "gateway_mac", g.Info.MACAddress, "error", err)
				g.emitErrorEvent(err)
			}

			_, err = udp.SendDataUDP(g.Info.Connection, packet)

			if !g.CanExecute() {
				slog.Info("gateway receiver turned off", "component", "gateway", "gateway_mac", g.Info.MACAddress)
				g.emitEvent(events.GwEventDisconnected, nil)
				return
			}

			if err != nil {
				msg := fmt.Sprintf("No connection with %v, it may be off", *g.Info.BridgeAddress)
				slog.Error("no connection with bridge", "component", "gateway", "gateway_mac", g.Info.MACAddress, "bridge", *g.Info.BridgeAddress)
				g.emitErrorEvent(errors.New(msg))
			} else {

				g.Stat.TXNb++
				slog.Debug("tx ack sent", "component", "gateway", "gateway_mac", g.Info.MACAddress)

			}

		default:
			slog.Warn("unsupported packet received", "component", "gateway", "gateway_mac", g.Info.MACAddress)

		}

	}

}
