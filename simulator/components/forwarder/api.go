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
		devAddrMap: make(map[lorawan.DevAddr]lorawan.EUI64),
		tmstMap:    make(map[uint32]lorawan.EUI64),
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

	f.devAddrMapMu.Lock()
	f.devAddrMap[d.DevAddr] = d.DevEUI
	f.devAddrMapMu.Unlock()

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

	if d, ok := s.devices[DevEUI]; ok {
		f.devAddrMapMu.Lock()
		delete(f.devAddrMap, d.DevAddr)
		f.devAddrMapMu.Unlock()
	}

	clear(s.devToGw[DevEUI])
	delete(s.devToGw, DevEUI)
	delete(s.devices, DevEUI)
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

func (f *Forwarder) UpdateDevAddr(devEUI lorawan.EUI64, devAddr lorawan.DevAddr) {
	f.devAddrMapMu.Lock()
	f.devAddrMap[devAddr] = devEUI
	f.devAddrMapMu.Unlock()
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
	rxpk := createPacket(data)

	f.tmstMapMu.Lock()
	f.tmstMap[rxpk.Tmst] = DevEUI
	f.tmstMapMu.Unlock()

	s := f.getShard(DevEUI)
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, up := range s.devToGw[DevEUI] {
		up.Push(rxpk)
	}
}

func (f *Forwarder) Downlink(data *lorawan.PHYPayload, freq uint32,
	macAddress lorawan.EUI64, tmst *uint32, rawData []byte) bool {

	// DevAddr-based matching for data frames
	if macPL, ok := data.MACPayload.(*lorawan.MACPayload); ok {
		devAddr := macPL.FHDR.DevAddr

		f.devAddrMapMu.RLock()
		devEUI, found := f.devAddrMap[devAddr]
		f.devAddrMapMu.RUnlock()

		if found {
			s := f.getShard(devEUI)
			s.mu.RLock()
			gwMap, ok := s.gwtoDev[freq][macAddress]
			if ok {
				if recvDl, ok := gwMap[devEUI]; ok {
					buf := make([]byte, len(rawData))
					copy(buf, rawData)
					clone := &lorawan.PHYPayload{}
					if err := clone.UnmarshalBinary(buf); err == nil {
						recvDl.Push(clone)
					}
				}
			}
			s.mu.RUnlock()
			// Device found via DevAddr — don't broadcast even if RX window closed
			return true
		}
	}

	// Try tmst-based routing for JoinAccepts.
	// The PULL_RESP tmst = uplink tmst + RX delay, so subtract common delays.
	if tmst != nil {
		rxDelays := []uint32{5000000, 6000000, 1000000, 2000000}
		for _, delay := range rxDelays {
			uplinkTmst := *tmst - delay

			f.tmstMapMu.RLock()
			targetEUI, found := f.tmstMap[uplinkTmst]
			f.tmstMapMu.RUnlock()

			if found {
				f.tmstMapMu.Lock()
				delete(f.tmstMap, uplinkTmst)
				f.tmstMapMu.Unlock()

				s := f.getShard(targetEUI)
				s.mu.RLock()
				if gwMap, ok := s.gwtoDev[freq][macAddress]; ok {
					if d, ok := gwMap[targetEUI]; ok {
						buf := make([]byte, len(rawData))
						copy(buf, rawData)
						clone := &lorawan.PHYPayload{}
						if err := clone.UnmarshalBinary(buf); err == nil {
							d.Push(clone)
							s.mu.RUnlock()
							return true
						}
					}
				}
				s.mu.RUnlock()
				break
			}
		}
	}

	// Fallback broadcast for unmatched frames
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

	f.devAddrMapMu.Lock()
	clear(f.devAddrMap)
	f.devAddrMapMu.Unlock()

	f.tmstMapMu.Lock()
	clear(f.tmstMap)
	f.tmstMapMu.Unlock()
}
