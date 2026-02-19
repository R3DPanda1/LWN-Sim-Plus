package forwarder

import (
	"hash/fnv"
	"sync"

	dl "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/frames/downlink"
	m "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/forwarder/models"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/resources/communication/buffer"
	"github.com/brocaar/lorawan"
)

const DefaultNumShards = 16

type RoutingShard struct {
	mu      sync.RWMutex
	devToGw map[lorawan.EUI64]map[lorawan.EUI64]*buffer.BufferUplink
	gwtoDev map[uint32]map[lorawan.EUI64]map[lorawan.EUI64]*dl.ReceivedDownlink
	devices map[lorawan.EUI64]m.InfoDevice
}

func newShard() *RoutingShard {
	return &RoutingShard{
		devToGw: make(map[lorawan.EUI64]map[lorawan.EUI64]*buffer.BufferUplink),
		gwtoDev: make(map[uint32]map[lorawan.EUI64]map[lorawan.EUI64]*dl.ReceivedDownlink),
		devices: make(map[lorawan.EUI64]m.InfoDevice),
	}
}

func shardIndex(eui lorawan.EUI64, numShards int) int {
	h := fnv.New32a()
	h.Write(eui[:])
	return int(h.Sum32()) % numShards
}
