package forwarder

import (
	"fmt"

	"github.com/R3DPanda1/LWN-Sim-Plus/shared"
	dl "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/frames/downlink"
	m "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/forwarder/models"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/resources/communication/buffer"
	pkt "github.com/R3DPanda1/LWN-Sim-Plus/simulator/resources/communication/packets"
	"github.com/brocaar/lorawan"
)

func Setup() *Forwarder {
	shared.DebugPrint("Init new Forwarder instance")
	shards := make([]*RoutingShard, DefaultNumShards)
	for i := range shards {
		shards[i] = newShard()
	}
	return &Forwarder{
		shards:     shards,
		numShards:  DefaultNumShards,
		gateways:   make(map[lorawan.EUI64]m.InfoGateway),
		uplinkTmst: make(map[lorawan.EUI64]uint32),
	}
}

func (f *Forwarder) AddDevice(d m.InfoDevice) {
	s := f.getShard(d.DevEUI)
	s.mu.Lock()
	defer s.mu.Unlock()

	shared.DebugPrint(fmt.Sprintf("Add device %v to Forwarder", d.DevEUI))
	s.devices[d.DevEUI] = d
	inner := make(map[lorawan.EUI64]*buffer.BufferUplink)
	s.devToGw[d.DevEUI] = inner

	f.gwMu.RLock()
	defer f.gwMu.RUnlock()

	for _, g := range f.gateways {
		if inRange(d, g) {
			shared.DebugPrint(fmt.Sprintf("Adding communication link with %s", g.MACAddress))
			s.devToGw[d.DevEUI][g.MACAddress] = g.Buffer
		}
	}
}

func (f *Forwarder) AddGateway(g m.InfoGateway) {
	f.gwMu.Lock()
	shared.DebugPrint(fmt.Sprintf("Add/Update gateway %v to Forwarder", g.MACAddress))
	f.gateways[g.MACAddress] = g
	f.gwMu.Unlock()

	// Update device-to-gateway links across all shards
	for _, s := range f.shards {
		s.mu.Lock()
		for _, d := range s.devices {
			if inRange(d, g) {
				shared.DebugPrint(fmt.Sprintf("Adding communication link with %s", d.DevEUI))
				s.devToGw[d.DevEUI][g.MACAddress] = g.Buffer
			}
		}
		s.mu.Unlock()
	}
}

func (f *Forwarder) DeleteDevice(DevEUI lorawan.EUI64) {
	s := f.getShard(DevEUI)
	s.mu.Lock()
	defer s.mu.Unlock()

	shared.DebugPrint(fmt.Sprintf("Delete device %v from Forwarder", DevEUI))
	clear(s.devToGw[DevEUI])
	delete(s.devToGw, DevEUI)
	delete(s.devices, DevEUI)

	f.uplinkTmstMu.Lock()
	delete(f.uplinkTmst, DevEUI)
	f.uplinkTmstMu.Unlock()
}

func (f *Forwarder) DeleteGateway(g m.InfoGateway) {
	f.gwMu.Lock()
	shared.DebugPrint(fmt.Sprintf("Delete gateway %v from Forwarder", g.MACAddress))
	delete(f.gateways, g.MACAddress)
	f.gwMu.Unlock()

	// Remove gateway links from all shards
	for _, s := range f.shards {
		s.mu.Lock()
		for _, d := range s.devices {
			shared.DebugPrint(fmt.Sprintf("Removing communication link with %s", d.DevEUI))
			delete(s.devToGw[d.DevEUI], g.MACAddress)
		}
		s.mu.Unlock()
	}
}

func (f *Forwarder) UpdateDevice(d m.InfoDevice) {
	f.AddDevice(d)
}

func (f *Forwarder) Register(freq uint32, devEUI lorawan.EUI64, rDownlink *dl.ReceivedDownlink) {
	s := f.getShard(devEUI)
	s.mu.Lock()
	defer s.mu.Unlock()

	inner, ok := s.gwtoDev[freq]
	if !ok {
		inner = make(map[lorawan.EUI64]map[lorawan.EUI64]*dl.ReceivedDownlink)
		s.gwtoDev[freq] = inner
	}

	for key := range s.devToGw[devEUI] {
		inner, ok := s.gwtoDev[freq][key]
		if !ok {
			inner = make(map[lorawan.EUI64]*dl.ReceivedDownlink)
			s.gwtoDev[freq][key] = inner
		}
		rDownlink.Open()
		s.gwtoDev[freq][key][devEUI] = rDownlink
		_ = inner
	}
}

func (f *Forwarder) UnRegister(freq uint32, devEUI lorawan.EUI64) {
	s := f.getShard(devEUI)
	s.mu.Lock()
	defer s.mu.Unlock()

	for key := range s.devToGw[devEUI] {
		_, ok := s.gwtoDev[freq][key][devEUI]
		if ok {
			s.gwtoDev[freq][key][devEUI].Close()
			delete(s.gwtoDev[freq][key], devEUI)
		}
	}
}

func (f *Forwarder) Uplink(data pkt.RXPK, DevEUI lorawan.EUI64) {
	s := f.getShard(DevEUI)
	s.mu.RLock()
	defer s.mu.RUnlock()

	rxpk := createPacket(data)

	f.uplinkTmstMu.Lock()
	f.uplinkTmst[DevEUI] = rxpk.Tmst
	f.uplinkTmstMu.Unlock()

	for _, up := range s.devToGw[DevEUI] {
		up.Push(rxpk)
	}
}

func (f *Forwarder) Downlink(data *lorawan.PHYPayload, freq uint32,
	macAddress lorawan.EUI64, tmst *uint32, rawData []byte) bool {

	// If we have a tmst, try to match to the specific device that sent the uplink
	if tmst != nil {
		f.uplinkTmstMu.RLock()
		defer f.uplinkTmstMu.RUnlock()

		for _, s := range f.shards {
			s.mu.RLock()
			gwMap, ok := s.gwtoDev[freq][macAddress]
			if ok {
				for devEUI, recvDl := range gwMap {
					deviceTmst, has := f.uplinkTmst[devEUI]
					if !has {
						continue
					}
					diff := *tmst - deviceTmst
					if diff >= 1 && diff <= 6 {
						buf := make([]byte, len(rawData))
						copy(buf, rawData)
						clone := &lorawan.PHYPayload{}
						if err := clone.UnmarshalBinary(buf); err == nil {
							if recvDl.Push(clone) {
								s.mu.RUnlock()
								return true
							}
						}
					}
				}
			}
			s.mu.RUnlock()
		}
	}

	// Fallback: no tmst (imme=true) or no match — broadcast with per-device cloning
	anyDelivered := false
	for _, s := range f.shards {
		s.mu.RLock()
		gwMap, ok := s.gwtoDev[freq][macAddress]
		if ok {
			for _, d := range gwMap {
				buf := make([]byte, len(rawData))
				copy(buf, rawData)
				clone := &lorawan.PHYPayload{}
				if err := clone.UnmarshalBinary(buf); err == nil {
					if d.Push(clone) {
						anyDelivered = true
					}
				}
			}
		}
		s.mu.RUnlock()
	}

	return anyDelivered
}

func (f *Forwarder) Reset() {
	shared.DebugPrint("Reset Forwarder")
	for _, s := range f.shards {
		s.mu.Lock()
		clear(s.devToGw)
		clear(s.gwtoDev)
		clear(s.devices)
		s.mu.Unlock()
	}
	f.gwMu.Lock()
	clear(f.gateways)
	f.gwMu.Unlock()

	f.uplinkTmstMu.Lock()
	clear(f.uplinkTmst)
	f.uplinkTmstMu.Unlock()
}
