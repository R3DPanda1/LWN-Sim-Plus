package forwarder

import (
	"sync"
	"time"

	m "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/forwarder/models"
	pkt "github.com/R3DPanda1/LWN-Sim-Plus/simulator/resources/communication/packets"
	loc "github.com/R3DPanda1/LWN-Sim-Plus/simulator/resources/location"
	"github.com/brocaar/lorawan"
)

// Forwarder allows communication between devices and gateways.
// Routing maps are split across shards keyed by device EUI so that
// concurrent operations on different devices don't contend on the same lock.
type Forwarder struct {
	shards    []*RoutingShard
	numShards int
	gwMu      sync.RWMutex
	gateways  map[lorawan.EUI64]m.InfoGateway
}

// GPSOffset compensates for the drift between UTC and GPS time
const GPSOffset = 18000

func createPacket(info pkt.RXPK) pkt.RXPK {
	now := time.Now()
	offset, _ := time.Parse(time.RFC3339, "1980-01-06T00:00:00Z")
	tmms := now.UnixMilli() - offset.UnixMilli() + GPSOffset
	rxpk := pkt.RXPK{
		Time:      now.Format(time.RFC3339),
		Tmms:      &tmms,
		Tmst:      uint32(now.Unix()),
		Channel:   info.Channel,
		RFCH:      0,
		Frequency: info.Frequency,
		Stat:      1,
		Modu:      info.Modu,
		DatR:      info.DatR,
		Brd:       0,
		CodR:      info.CodR,
		RSSI:      -60, // TODO: Make it variable during the simulation
		LSNR:      7,
		Size:      info.Size,
		Data:      info.Data,
	}
	return rxpk
}

func inRange(d m.InfoDevice, g m.InfoGateway) bool {
	distance := loc.GetDistance(d.Location.Latitude, d.Location.Longitude,
		g.Location.Latitude, g.Location.Longitude)
	return distance <= (d.Range / 1000.0)
}

func (f *Forwarder) getShard(eui lorawan.EUI64) *RoutingShard {
	return f.shards[shardIndex(eui, f.numShards)]
}
