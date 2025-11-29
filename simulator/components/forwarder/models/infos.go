package models

import (
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/resources/communication/buffer"
	loc "github.com/R3DPanda1/LWN-Sim-Plus/simulator/resources/location"
	"github.com/brocaar/lorawan"
)

// InfoDevice is struct that contains information about a device
type InfoDevice struct {
	DevEUI   lorawan.EUI64 // Device EUI
	Location loc.Location  // Device location
	Range    float64       // Device range
}

// InfoGateway is struct that contains information about a gateway
type InfoGateway struct {
	MACAddress lorawan.EUI64        // Gateway MAC address
	Buffer     *buffer.BufferUplink // Gateway buffer
	Location   loc.Location         // Gateway location
}
