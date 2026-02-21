package gateway

import (
	f "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/forwarder"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/gateway/models"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/events"
	res "github.com/R3DPanda1/LWN-Sim-Plus/simulator/resources"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/resources/communication/buffer"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/util"
)

type Gateway struct {
	Id   int                `json:"id"`
	Info models.InfoGateway `json:"info"`

	State int `json:"-"`

	Resources *res.Resources `json:"-"` //is a pointer
	Forwarder *f.Forwarder   `json:"-"` //is a pointer

	Stat models.Stat `json:"-"`

	BufferUplink *buffer.BufferUplink `json:"-"`
	EventBroker  *events.EventBroker  `json:"-"`
}

func (g *Gateway) CanExecute() bool {

	if g.State == util.Stopped {
		return false
	}

	return true

}

func (g *Gateway) emitEvent(eventType string, extra map[string]string) {
	if g.EventBroker == nil {
		return
	}
	g.EventBroker.PublishGatewayEvent(g.Info.MACAddress.String(), events.GatewayEvent{
		GatewayMAC: g.Info.MACAddress.String(),
		GwName:     g.Info.Name,
		Type:       eventType,
		Extra:      extra,
	})
}

func (g *Gateway) emitErrorEvent(err error) {
	if g.EventBroker == nil {
		return
	}
	g.EventBroker.PublishGatewayEvent(g.Info.MACAddress.String(), events.GatewayEvent{
		GatewayMAC: g.Info.MACAddress.String(),
		GwName:     g.Info.Name,
		Type:       events.GwEventError,
		Extra:      map[string]string{"error": err.Error()},
	})
}
