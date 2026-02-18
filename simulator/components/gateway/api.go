package gateway

import (
	"log/slog"
	"sync"

	f "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/forwarder"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/events"
	res "github.com/R3DPanda1/LWN-Sim-Plus/simulator/resources"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/resources/communication/buffer"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/resources/communication/udp"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/util"
)

func (g *Gateway) Setup(BridgeAddress *string,
	Resources *res.Resources, Forwarder *f.Forwarder) {

	g.State = util.Stopped

	g.Info.BridgeAddress = BridgeAddress

	g.Resources = Resources
	g.Forwarder = Forwarder

	g.BufferUplink = buffer.BufferUplink{}
	g.BufferUplink.Notify = sync.NewCond(&g.BufferUplink.Mutex)

	slog.Debug("gateway setup complete", "component", "gateway", "gateway_mac", g.Info.MACAddress, "name", g.Info.Name)

}

func (g *Gateway) TurnON() {

	var err error

	g.State = util.Running

	//udp
	if g.Info.TypeGateway { //real
		g.Info.Connection, err = udp.ConnectTo(g.Info.AddrIP + ":" + g.Info.Port)
	} else { //virtual
		g.Info.Connection, err = udp.ConnectTo(*g.Info.BridgeAddress)
	}

	if err != nil {
		slog.Error("gateway udp connection failed", "component", "gateway", "gateway_mac", g.Info.MACAddress, "error", err)
		g.emitErrorEvent(err)
	} else {
		slog.Info("gateway connected", "component", "gateway", "gateway_mac", g.Info.MACAddress, "remote", g.Info.Connection.RemoteAddr().String())
		g.emitEvent(events.GwEventConnected, map[string]string{"remote": g.Info.Connection.RemoteAddr().String()})
	}

	go g.Receiver()

	if g.Info.TypeGateway { //real
		go g.SenderReal()
	} else { //virtual
		go g.SenderVirtual()
	}

	slog.Info("gateway turned on", "component", "gateway", "gateway_mac", g.Info.MACAddress, "name", g.Info.Name)
}

func (g *Gateway) TurnOFF() {

	g.State = util.Stopped

	g.BufferUplink.Signal() //signal to sender
	if g.Info.Connection != nil {
		g.Info.Connection.Close() //signal to receiver
	}

}

func (g *Gateway) IsOn() bool {

	if g.State == util.Running {
		return true
	}

	return false

}
