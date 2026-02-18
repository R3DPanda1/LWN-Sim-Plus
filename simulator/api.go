package simulator

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"math"
	mrand "math/rand"
	"strings"
	"time"

	"github.com/brocaar/lorawan"

	"github.com/R3DPanda1/LWN-Sim-Plus/codes"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/codec"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/integration"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/integration/chirpstack"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/template"
	"github.com/R3DPanda1/LWN-Sim-Plus/models"
	"github.com/R3DPanda1/LWN-Sim-Plus/shared"

	dev "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device"
	devChannels "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/features/channels"
	devFeatures "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/features"
	devModels "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/models"
	rp "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/regional_parameters"
	f "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/forwarder"
	mfw "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/forwarder/models"
	gw "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/gateway"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/events"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/resources/location"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/util"
	"github.com/R3DPanda1/LWN-Sim-Plus/socket"
)

func GetInstance() *Simulator {
	var s Simulator
	shared.DebugPrint("Init new Simulator instance")
	// Initial state of the simulator is stopped
	s.State = util.Stopped
	// Load saved data
	s.loadData()
	// Initialized the active devices and gateways maps
	s.ActiveDevices = make(map[int]int)
	s.ActiveGateways = make(map[int]int)
	// Init Forwarder
	s.Forwarder = *f.Setup()
	// Initialize codec manager (Phase 1-3 enhancement)
	if dev.Codecs == nil {
		dev.Codecs = codec.NewRegistry(codec.DefaultExecutorConfig())

		// Load codec library from disk
		pathDir, err := util.GetPath()
		codecLibLoaded := false
		if err == nil {
			codecLibPath := pathDir + "/codecs.json"
			if err := dev.Codecs.Load(codecLibPath); err != nil {
				shared.DebugPrint(fmt.Sprintf("Warning: %v", err))
			} else {
				shared.DebugPrint("Codec library loaded from disk")
				codecLibLoaded = true
			}
		}

		// If no codecs loaded from disk, load defaults
		if !codecLibLoaded || dev.Codecs.GetCodecCount() == 0 {
			dev.Codecs.LoadDefaults()
			shared.DebugPrint("Default codecs loaded")
		}

		shared.DebugPrint("Codec manager initialized")
	}

	// Initialize integrations (direct map pattern like Devices/Gateways)
	s.setupIntegrations()

	// Initialize templates (direct map pattern like Devices/Gateways)
	s.setupTemplates()

	return &s
}

// Run starts the simulation environment
func (s *Simulator) Run() {
	shared.DebugPrint("Executing Run")
	s.State = util.Running
	s.setup()
	slog.Info("simulator started", "component", "simulator")
	s.EventBroker.PublishSystemEvent(events.SystemEvent{
		Type:    events.SysEventStarted,
		Message: "Simulator started",
	})
	shared.DebugPrint("Turning ON active components")
	for _, id := range s.ActiveGateways {
		s.turnONGateway(id)
	}
	for _, id := range s.ActiveDevices {
		s.turnONDevice(id)
	}
}

// Stop terminates the simulation environment
func (s *Simulator) Stop() {
	shared.DebugPrint("Executing Stop")
	s.State = util.Stopped
	s.Resources.ExitGroup.Add(len(s.ActiveGateways) + len(s.ActiveDevices) - s.ComponentsInactiveTmp)
	shared.DebugPrint("Turning OFF active components")
	for _, id := range s.ActiveGateways {
		s.Gateways[id].TurnOFF()
	}
	for _, id := range s.ActiveDevices {
		s.Devices[id].TurnOFF()
	}
	s.Resources.ExitGroup.Wait()

	// Save all state (includes integrations and templates now)
	s.saveStatus()

	// Save codec library (codec uses its own registry)
	if dev.Codecs != nil {
		pathDir, err := util.GetPath()
		if err == nil {
			codecLibPath := pathDir + "/codecs.json"
			if err := dev.Codecs.Save(codecLibPath); err != nil {
				shared.DebugPrint(fmt.Sprintf("Warning: failed to save codec library: %v", err))
			} else {
				shared.DebugPrint("Codec library saved to disk")
			}
		}
	}

	s.Forwarder.Reset()
	slog.Info("simulator stopped", "component", "simulator")
	s.EventBroker.PublishSystemEvent(events.SystemEvent{
		Type:    events.SysEventStopped,
		Message: "Simulator stopped",
	})
	s.reset()
}

// SaveBridgeAddress stores the bridge address in the simulator struct and saves it to the simulator.json file
func (s *Simulator) SaveBridgeAddress(remoteAddr models.AddressIP) error {
	// Store the bridge address in the simulator struct
	s.BridgeAddress = fmt.Sprintf("%v:%v", remoteAddr.Address, remoteAddr.Port)
	pathDir, err := util.GetPath()
	if err != nil {
		log.Fatal(err)
	}
	path := pathDir + "/simulator.json"
	s.saveComponent(path, &s)
	slog.Info("bridge address saved", "component", "simulator", "bridge", s.BridgeAddress)
	return nil
}

// GetBridgeAddress returns the bridge address stored in the simulator struct
func (s *Simulator) GetBridgeAddress() models.AddressIP {
	// Create an empty AddressIP struct with default values
	var rServer models.AddressIP
	if s.BridgeAddress == "" {
		return rServer
	}
	// Split the bridge address into address and port
	parts := strings.Split(s.BridgeAddress, ":")
	rServer.Address = parts[0]
	rServer.Port = parts[1]
	return rServer
}

// GetGateways returns an array of all gateways in the simulator
func (s *Simulator) GetGateways() []gw.Gateway {
	var gateways []gw.Gateway
	for _, g := range s.Gateways {
		gateways = append(gateways, *g)
	}
	return gateways
}

// GetDevices returns an array of all devices in the simulator
func (s *Simulator) GetDevices() []dev.Device {
	var devices []dev.Device
	for _, d := range s.Devices {
		devices = append(devices, *d)
	}
	return devices
}

// SetGateway adds or updates a gateway
func (s *Simulator) SetGateway(gateway *gw.Gateway, update bool) (int, int, error) {
	shared.DebugPrint(fmt.Sprintf("Adding/Updating Gateway [%s]", gateway.Info.MACAddress.String()))
	emptyAddr := lorawan.EUI64{0, 0, 0, 0, 0, 0, 0, 0}
	// Check if the MAC address is valid
	if gateway.Info.MACAddress == emptyAddr {
		slog.Warn("invalid gateway MAC address", "component", "simulator")
		return codes.CodeErrorAddress, -1, errors.New("Error: MAC Address invalid")
	}
	// If the gateway is new, assign a new ID
	if !update {
		gateway.Id = s.NextIDGw
	} else { // If the gateway is being updated, it must be turned off
		if s.Gateways[gateway.Id].IsOn() {
			return codes.CodeErrorDeviceActive, -1, errors.New("Gateway is running, unable update")
		}
	}
	// Check if the name is already used
	code, err := s.searchName(gateway.Info.Name, gateway.Id, true)
	if err != nil {
		return code, -1, err
	}
	// Check if the name is already used
	code, err = s.searchAddress(gateway.Info.MACAddress, gateway.Id, true)
	if err != nil {
		return code, -1, err
	}
	if !gateway.Info.TypeGateway {

		if s.BridgeAddress == "" {
			return codes.CodeNoBridge, -1, errors.New("No gateway bridge configured")
		}

	}

	s.Gateways[gateway.Id] = gateway

	pathDir, err := util.GetPath()
	if err != nil {
		log.Fatal(err)
	}

	path := pathDir + "/gateways.json"
	s.saveComponent(path, &s.Gateways)
	path = pathDir + "/simulator.json"
	s.saveComponent(path, &s)

	slog.Info("gateway saved", "component", "simulator", "gateway_id", gateway.Id, "name", gateway.Info.Name)

	if gateway.Info.Active {

		s.ActiveGateways[gateway.Id] = gateway.Id

		if s.State == util.Running {
			s.Gateways[gateway.Id].Setup(&s.BridgeAddress, &s.Resources, &s.Forwarder)
			s.turnONGateway(gateway.Id)
		}

	} else {
		_, ok := s.ActiveGateways[gateway.Id]
		if ok {
			delete(s.ActiveGateways, gateway.Id)
		}
	}
	s.NextIDGw++
	return codes.CodeOK, gateway.Id, nil
}

func (s *Simulator) DeleteGateway(Id int) bool {

	if s.Gateways[Id].IsOn() {
		return false
	}

	delete(s.Gateways, Id)
	delete(s.ActiveGateways, Id)

	pathDir, err := util.GetPath()
	if err != nil {
		log.Fatal(err)
	}

	path := pathDir + "/gateways.json"
	s.saveComponent(path, &s.Gateways)

	slog.Info("gateway deleted", "component", "simulator", "gateway_id", Id)

	return true
}

func (s *Simulator) SetDevice(device *dev.Device, update bool) (int, int, error) {

	emptyAddr := lorawan.EUI64{0, 0, 0, 0, 0, 0, 0, 0}

	if device.Info.DevEUI == emptyAddr {

		slog.Warn("invalid device DevEUI", "component", "simulator")
		return codes.CodeErrorAddress, -1, errors.New("Error: DevEUI invalid")

	}

	if !update { //new

		device.Id = s.NextIDDev
		s.NextIDDev++

	} else {

		if s.Devices[device.Id].IsOn() {
			return codes.CodeErrorDeviceActive, -1, errors.New("Device is running, unable update")
		}

	}

	code, err := s.searchName(device.Info.Name, device.Id, false)
	if err != nil {

		return code, -1, err

	}

	code, err = s.searchAddress(device.Info.DevEUI, device.Id, false)
	if err != nil {

		return code, -1, err

	}

	s.Devices[device.Id] = device

	pathDir, err := util.GetPath()
	if err != nil {
		log.Fatal(err)
	}

	path := pathDir + "/devices.json"
	s.saveComponent(path, &s.Devices)
	path = pathDir + "/simulator.json"
	s.saveComponent(path, &s)

	slog.Info("device saved", "component", "simulator", "device_id", device.Id, "name", device.Info.Name)

	// Provision device to ChirpStack if integration is enabled (only for new devices)
	if !update && device.Info.Configuration.IntegrationEnabled {
		devEUI := hex.EncodeToString(device.Info.DevEUI[:])

		var err error
		if device.Info.Configuration.SupportedOtaa {
			// OTAA: provision with AppKey
			appKey := hex.EncodeToString(device.Info.AppKey[:])
			err = s.ProvisionDevice(
				device.Info.Configuration.IntegrationID,
				devEUI,
				device.Info.Name,
				device.Info.Configuration.DeviceProfileID,
				appKey,
			)
		} else {
			// ABP: provision with session keys and activate immediately
			devAddr := hex.EncodeToString(device.Info.DevAddr[:])
			nwkSKey := hex.EncodeToString(device.Info.NwkSKey[:])
			appSKey := hex.EncodeToString(device.Info.AppSKey[:])
			err = s.ProvisionDeviceABP(
				device.Info.Configuration.IntegrationID,
				devEUI,
				device.Info.Name,
				device.Info.Configuration.DeviceProfileID,
				devAddr,
				nwkSKey,
				appSKey,
			)
		}

		if err != nil {
		}
	}

	if device.Info.Status.Active {

		s.ActiveDevices[device.Id] = device.Id

		if s.State == util.Running {
			s.turnONDevice(device.Id)
		}

	} else {
		_, ok := s.ActiveDevices[device.Id]
		if ok {
			delete(s.ActiveDevices, device.Id)
		}
	}

	return codes.CodeOK, device.Id, nil
}

func (s *Simulator) DeleteDevice(Id int) bool {

	if s.Devices[Id].IsOn() {
		return false
	}

	// Delete device from ChirpStack if integration was enabled for this specific device
	device := s.Devices[Id]
	if device.Info.Configuration.IntegrationEnabled {
		devEUI := hex.EncodeToString(device.Info.DevEUI[:])
		if err := s.DeleteDeviceFromChirpStack(device.Info.Configuration.IntegrationID, devEUI); err != nil {
		} else {
		}
	}

	delete(s.Devices, Id)
	delete(s.ActiveDevices, Id)

	pathDir, err := util.GetPath()
	if err != nil {
		log.Fatal(err)
	}

	path := pathDir + "/devices.json"
	s.saveComponent(path, &s.Devices)

	slog.Info("device deleted", "component", "simulator", "device_id", Id)

	return true
}

func (s *Simulator) ToggleStateDevice(Id int) {

	if s.Devices[Id].State == util.Stopped {
		s.turnONDevice(Id)
	} else if s.Devices[Id].State == util.Running {
		s.turnOFFDevice(Id)
	}

}

func (s *Simulator) SendMACCommand(cid lorawan.CID, data socket.MacCommand) {

	if !s.Devices[data.Id].IsOn() {
		slog.Warn("device is turned off", "component", "simulator", "device", s.Devices[data.Id].Info.Name)
		return
	}

	err := s.Devices[data.Id].SendMACCommand(cid, data.Periodicity)
	if err != nil {
		slog.Warn("unable to send MAC command", "component", "simulator", "error", err)
	} else {
		slog.Debug("MAC command queued", "component", "simulator", "device", s.Devices[data.Id].Info.Name)
	}

}

func (s *Simulator) ChangePayload(pl socket.NewPayload) (string, bool) {

	devEUIstring := hex.EncodeToString(s.Devices[pl.Id].Info.DevEUI[:])

	if !s.Devices[pl.Id].IsOn() {
		slog.Warn("device is turned off", "component", "simulator", "device", s.Devices[pl.Id].Info.Name)
		return devEUIstring, false
	}

	MType := lorawan.UnconfirmedDataUp
	if pl.MType == "ConfirmedDataUp" {
		MType = lorawan.ConfirmedDataUp
	}

	Payload := &lorawan.DataPayload{
		Bytes: []byte(pl.Payload),
	}

	s.Devices[pl.Id].ChangePayload(MType, Payload)

	slog.Debug("payload changed", "component", "simulator", "device", s.Devices[pl.Id].Info.Name)

	return devEUIstring, true
}

func (s *Simulator) SendUplink(pl socket.NewPayload) {

	if !s.Devices[pl.Id].IsOn() {
		slog.Warn("device is turned off", "component", "simulator", "device", s.Devices[pl.Id].Info.Name)
		return
	}

	MType := lorawan.UnconfirmedDataUp
	if pl.MType == "ConfirmedDataUp" {
		MType = lorawan.ConfirmedDataUp
	}

	s.Devices[pl.Id].NewUplink(MType, pl.Payload)

	slog.Debug("uplink queued", "component", "simulator", "device", s.Devices[pl.Id].Info.Name)
}

func (s *Simulator) ChangeLocation(l socket.NewLocation) bool {

	if !s.Devices[l.Id].IsOn() {
		return false
	}

	s.Devices[l.Id].ChangeLocation(l.Latitude, l.Longitude, l.Altitude)

	info := mfw.InfoDevice{
		DevEUI:   s.Devices[l.Id].Info.DevEUI,
		Location: s.Devices[l.Id].Info.Location,
		Range:    s.Devices[l.Id].Info.Configuration.Range,
	}

	s.Forwarder.UpdateDevice(info)

	return true
}

func (s *Simulator) ToggleStateGateway(Id int) {

	if s.Gateways[Id].State == util.Stopped {
		s.turnONGateway(Id)
	} else {
		s.turnOFFGateway(Id)
	}

}

// GetCodecs returns all available codec metadata
func (s *Simulator) GetCodecs() []codec.CodecMetadata {
	if dev.Codecs == nil {
		return []codec.CodecMetadata{}
	}
	return dev.Codecs.ListCodecs()
}

// GetCodec returns a specific codec by ID
func (s *Simulator) GetCodec(id int) (*codec.Codec, error) {
	if dev.Codecs == nil {
		return nil, errors.New("codec registry not initialized")
	}
	return dev.Codecs.GetCodec(id)
}

// GetDevicesUsingCodec returns a list of device EUIs using the specified codec
// Also counts templates that use this codec
func (s *Simulator) GetDevicesUsingCodec(codecID int) []string {
	devicesUsingCodec := []string{}

	// Check devices
	for _, device := range s.Devices {
		if device.Info.Configuration.CodecID == codecID {
			devicesUsingCodec = append(devicesUsingCodec, device.Info.DevEUI.String())
		}
	}

	// Check templates
	for _, tmpl := range s.Templates {
		if tmpl.UseCodec && tmpl.CodecID == codecID {
			devicesUsingCodec = append(devicesUsingCodec, fmt.Sprintf("template:%d", tmpl.ID))
		}
	}

	return devicesUsingCodec
}

// AddCodec adds a custom codec
func (s *Simulator) AddCodec(c *codec.Codec) error {
	if dev.Codecs == nil {
		return errors.New("codec registry not initialized")
	}

	if err := dev.Codecs.AddCodec(c); err != nil {
		return err
	}

	// Save codec library to disk
	s.saveCodecLibrary()
	return nil
}

// UpdateCodec updates an existing codec
func (s *Simulator) UpdateCodec(id int, name string, script string) error {
	if dev.Codecs == nil {
		return errors.New("codec registry not initialized")
	}

	if err := dev.Codecs.UpdateCodec(id, name, script); err != nil {
		return err
	}

	// Save codec library to disk
	s.saveCodecLibrary()
	return nil
}

// DeleteCodec removes a codec by ID
func (s *Simulator) DeleteCodec(id int) error {
	if dev.Codecs == nil {
		return errors.New("codec registry not initialized")
	}

	// Check if any devices or templates are using this codec
	usersOfCodec := s.GetDevicesUsingCodec(id)
	if len(usersOfCodec) > 0 {
		// Count devices and templates separately for better error message
		deviceCount := 0
		templateCount := 0
		for _, user := range usersOfCodec {
			if len(user) > 9 && user[:9] == "template:" {
				templateCount++
			} else {
				deviceCount++
			}
		}

		var parts []string
		if deviceCount > 0 {
			parts = append(parts, fmt.Sprintf("%d device(s)", deviceCount))
		}
		if templateCount > 0 {
			parts = append(parts, fmt.Sprintf("%d template(s)", templateCount))
		}

		return fmt.Errorf("cannot delete codec: used by %s", strings.Join(parts, " and "))
	}

	if err := dev.Codecs.RemoveCodec(id); err != nil {
		return err
	}

	// Save codec library to disk
	s.saveCodecLibrary()
	return nil
}

// saveCodecLibrary saves the codec library to disk
func (s *Simulator) saveCodecLibrary() {
	pathDir, err := util.GetPath()
	if err == nil && dev.Codecs != nil {
		codecLibPath := pathDir + "/codecs.json"
		if err := dev.Codecs.Save(codecLibPath); err != nil {
			shared.DebugPrint(fmt.Sprintf("Warning: failed to save codec library: %v", err))
		}
	}
}

// ==================== Integration Management ====================

// GetIntegrations returns all integrations (without API keys for security)
func (s *Simulator) GetIntegrations() []*integration.Integration {
	if s.Integrations == nil {
		return []*integration.Integration{}
	}
	result := make([]*integration.Integration, 0, len(s.Integrations))
	for _, i := range s.Integrations {
		result = append(result, i.PublicCopy())
	}
	return result
}

// GetIntegration returns a specific integration by ID
func (s *Simulator) GetIntegration(id int) (*integration.Integration, error) {
	if s.Integrations == nil {
		return nil, integration.ErrIntegrationNotFound
	}
	integ, exists := s.Integrations[id]
	if !exists {
		return nil, integration.ErrIntegrationNotFound
	}
	return integ.Clone(), nil
}

// AddIntegration adds a new integration
func (s *Simulator) AddIntegration(name string, intType integration.IntegrationType, url, apiKey, tenantID, appID string) (int, error) {
	if s.Integrations == nil {
		s.Integrations = make(map[int]*integration.Integration)
	}
	if s.IntegrationClients == nil {
		s.IntegrationClients = make(map[int]*chirpstack.Client)
	}

	integ := integration.NewIntegration(name, intType, url, apiKey, tenantID, appID)
	if err := integ.Validate(); err != nil {
		return 0, err
	}

	integ.ID = s.NextIDIntegration
	s.NextIDIntegration++

	s.Integrations[integ.ID] = integ

	// Create ChirpStack client
	if intType == integration.IntegrationTypeChirpStack {
		s.IntegrationClients[integ.ID] = chirpstack.NewClient(integ.URL, integ.APIKey)
	}

	// Save to disk
	s.saveStatus()
	return integ.ID, nil
}

// UpdateIntegration updates an existing integration
func (s *Simulator) UpdateIntegration(id int, name, url, apiKey, tenantID, appID string, enabled bool) error {
	if s.Integrations == nil {
		return integration.ErrIntegrationNotFound
	}

	existing, exists := s.Integrations[id]
	if !exists {
		return integration.ErrIntegrationNotFound
	}

	existing.Name = name
	existing.URL = url
	existing.APIKey = apiKey
	existing.TenantID = tenantID
	existing.ApplicationID = appID
	existing.Enabled = enabled

	if err := existing.Validate(); err != nil {
		return err
	}

	// Update ChirpStack client
	if existing.Type == integration.IntegrationTypeChirpStack {
		s.IntegrationClients[id] = chirpstack.NewClient(existing.URL, existing.APIKey)
	}

	// Save to disk
	s.saveStatus()
	return nil
}

// DeleteIntegration removes an integration by ID
func (s *Simulator) DeleteIntegration(id int) error {
	if s.Integrations == nil {
		return integration.ErrIntegrationNotFound
	}

	if _, exists := s.Integrations[id]; !exists {
		return integration.ErrIntegrationNotFound
	}

	// Check if any devices are using this integration
	devicesUsingIntegration := s.GetDevicesUsingIntegration(id)
	if len(devicesUsingIntegration) > 0 {
		return fmt.Errorf("cannot delete integration: used by %d device(s)", len(devicesUsingIntegration))
	}

	delete(s.Integrations, id)
	delete(s.IntegrationClients, id)

	// Save to disk
	s.saveStatus()
	return nil
}

// TestIntegrationConnection tests connection to an integration
func (s *Simulator) TestIntegrationConnection(id int) error {
	if s.Integrations == nil {
		return integration.ErrIntegrationNotFound
	}

	integ, exists := s.Integrations[id]
	if !exists {
		return integration.ErrIntegrationNotFound
	}

	client, exists := s.IntegrationClients[id]
	if !exists {
		return errors.New("client not initialized for this integration")
	}

	return client.TestConnection(integ.TenantID)
}

// GetDeviceProfiles returns device profiles for an integration
func (s *Simulator) GetDeviceProfiles(id int) ([]chirpstack.DeviceProfile, error) {
	if s.Integrations == nil {
		return nil, integration.ErrIntegrationNotFound
	}

	integ, exists := s.Integrations[id]
	if !exists {
		return nil, integration.ErrIntegrationNotFound
	}

	client, exists := s.IntegrationClients[id]
	if !exists {
		return nil, errors.New("client not initialized for this integration")
	}

	return client.ListDeviceProfiles(integ.TenantID, 100)
}

// GetDevicesUsingIntegration returns a list of device EUIs using the specified integration
func (s *Simulator) GetDevicesUsingIntegration(integrationID int) []string {
	devicesUsingIntegration := []string{}
	for _, device := range s.Devices {
		if device.Info.Configuration.IntegrationID == integrationID {
			devicesUsingIntegration = append(devicesUsingIntegration, device.Info.DevEUI.String())
		}
	}
	return devicesUsingIntegration
}

// ProvisionDevice provisions a device to ChirpStack using OTAA
func (s *Simulator) ProvisionDevice(integrationID int, devEUI, name, deviceProfileID, appKey string) error {
	if s.Integrations == nil {
		return integration.ErrIntegrationNotFound
	}

	integ, exists := s.Integrations[integrationID]
	if !exists {
		return integration.ErrIntegrationNotFound
	}

	if !integ.Enabled {
		return errors.New("integration is disabled")
	}

	client, exists := s.IntegrationClients[integrationID]
	if !exists {
		return errors.New("client not initialized for this integration")
	}

	// Create device
	device := &chirpstack.Device{
		DevEUI:          devEUI,
		Name:            name,
		ApplicationID:   integ.ApplicationID,
		DeviceProfileID: deviceProfileID,
	}

	if err := client.CreateDevice(device); err != nil {
		return fmt.Errorf("failed to create device: %w", err)
	}

	// Set device keys
	if err := client.SetDeviceKeys(devEUI, appKey); err != nil {
		// Rollback: delete the device
		_ = client.DeleteDevice(devEUI)
		return fmt.Errorf("failed to set device keys: %w", err)
	}

	return nil
}

// ProvisionDeviceABP provisions a device to ChirpStack using ABP
func (s *Simulator) ProvisionDeviceABP(integrationID int, devEUI, name, deviceProfileID, devAddr, nwkSKey, appSKey string) error {
	if s.Integrations == nil {
		return integration.ErrIntegrationNotFound
	}

	integ, exists := s.Integrations[integrationID]
	if !exists {
		return integration.ErrIntegrationNotFound
	}

	if !integ.Enabled {
		return errors.New("integration is disabled")
	}

	client, exists := s.IntegrationClients[integrationID]
	if !exists {
		return errors.New("client not initialized for this integration")
	}

	// Create device
	device := &chirpstack.Device{
		DevEUI:          devEUI,
		Name:            name,
		ApplicationID:   integ.ApplicationID,
		DeviceProfileID: deviceProfileID,
		SkipFcntCheck:   true,
	}

	if err := client.CreateDevice(device); err != nil {
		return fmt.Errorf("failed to create device: %w", err)
	}

	// Activate device with ABP keys
	if err := client.ActivateDeviceABP(devEUI, devAddr, nwkSKey, appSKey); err != nil {
		// Rollback: delete the device
		_ = client.DeleteDevice(devEUI)
		return fmt.Errorf("failed to activate device (ABP): %w", err)
	}

	return nil
}

// DeleteDeviceFromChirpStack removes a device from ChirpStack
func (s *Simulator) DeleteDeviceFromChirpStack(integrationID int, devEUI string) error {
	if s.Integrations == nil {
		return nil // Silently skip
	}

	integ, exists := s.Integrations[integrationID]
	if !exists {
		return nil // Silently skip
	}

	if !integ.Enabled {
		return nil // Silently skip
	}

	client, exists := s.IntegrationClients[integrationID]
	if !exists {
		return nil // Silently skip
	}

	return client.DeleteDevice(devEUI)
}

// ==================== Template Management ====================

// GetTemplates returns all templates
func (s *Simulator) GetTemplates() []*template.DeviceTemplate {
	if s.Templates == nil {
		return []*template.DeviceTemplate{}
	}
	result := make([]*template.DeviceTemplate, 0, len(s.Templates))
	for _, t := range s.Templates {
		result = append(result, t.Clone())
	}
	return result
}

// GetTemplate returns a specific template by ID
func (s *Simulator) GetTemplate(id int) (*template.DeviceTemplate, error) {
	if s.Templates == nil {
		return nil, template.ErrTemplateNotFound
	}
	tmpl, exists := s.Templates[id]
	if !exists {
		return nil, template.ErrTemplateNotFound
	}
	return tmpl.Clone(), nil
}

// AddTemplate adds a new template
func (s *Simulator) AddTemplate(tmpl *template.DeviceTemplate) (int, error) {
	if s.Templates == nil {
		s.Templates = make(map[int]*template.DeviceTemplate)
	}

	if err := tmpl.Validate(); err != nil {
		return 0, err
	}

	// Assign ID if not set
	if tmpl.ID == 0 {
		tmpl.ID = s.NextIDTemplate
		s.NextIDTemplate++
	} else if tmpl.ID >= s.NextIDTemplate {
		s.NextIDTemplate = tmpl.ID + 1
	}

	s.Templates[tmpl.ID] = tmpl

	// Save to disk
	s.saveStatus()
	return tmpl.ID, nil
}

// UpdateTemplate updates an existing template
func (s *Simulator) UpdateTemplate(tmpl *template.DeviceTemplate) error {
	if s.Templates == nil {
		return template.ErrTemplateNotFound
	}

	if _, exists := s.Templates[tmpl.ID]; !exists {
		return template.ErrTemplateNotFound
	}

	if err := tmpl.Validate(); err != nil {
		return err
	}

	s.Templates[tmpl.ID] = tmpl

	// Save to disk
	s.saveStatus()
	return nil
}

// DeleteTemplate removes a template by ID
func (s *Simulator) DeleteTemplate(id int) error {
	if s.Templates == nil {
		return template.ErrTemplateNotFound
	}

	if _, exists := s.Templates[id]; !exists {
		return template.ErrTemplateNotFound
	}

	delete(s.Templates, id)

	// Save to disk
	s.saveStatus()
	return nil
}

// ==================== Bulk Device Creation ====================

// CreateDevicesFromTemplate creates multiple devices from a template
func (s *Simulator) CreateDevicesFromTemplate(templateID int, count int, namePrefix string, baseLat, baseLng float64, baseAlt int32, spreadMeters float64) ([]int, error) {
	if s.Templates == nil {
		return nil, template.ErrTemplateNotFound
	}

	// Get template
	tmpl, exists := s.Templates[templateID]
	if !exists {
		return nil, template.ErrTemplateNotFound
	}

	// Seed random number generator
	mrand.Seed(time.Now().UnixNano())

	createdIDs := make([]int, 0, count)

	for i := 1; i <= count; i++ {
		// Generate device name
		name := fmt.Sprintf("%s-%d", namePrefix, i)

		// Generate random DevEUI
		devEUI, err := generateRandomEUI64()
		if err != nil {
			continue
		}

		// Generate ABP keys (NwkSKey, AppSKey, DevAddr) instead of OTAA AppKey
		nwkSKey, err := generateRandomKey()
		if err != nil {
			continue
		}

		appSKey, err := generateRandomKey()
		if err != nil {
			continue
		}

		devAddr, err := generateRandomDevAddr()
		if err != nil {
			continue
		}

		// Randomize coordinates
		lat, lng := randomizeCoordinates(baseLat, baseLng, spreadMeters)

		// Create device from template using ABP (no join required)
		device := s.createDeviceFromTemplate(tmpl, name, devEUI, nwkSKey, appSKey, devAddr, lat, lng, baseAlt)

		// Add device
		_, id, err := s.SetDevice(device, false)
		if err != nil {
			continue
		}

		createdIDs = append(createdIDs, id)
	}

	return createdIDs, nil
}

// createDeviceFromTemplate creates a Device struct from a template using ABP activation
// ABP devices have pre-set session keys and don't need to join the network
func (s *Simulator) createDeviceFromTemplate(tmpl *template.DeviceTemplate, name string, devEUI lorawan.EUI64, nwkSKey, appSKey [16]byte, devAddr lorawan.DevAddr, lat, lng float64, alt int32) *dev.Device {
	// Get regional parameters
	region := rp.GetRegionalParameters(tmpl.Region)

	device := &dev.Device{
		Info: devModels.InformationDevice{
			Name:    name,
			DevEUI:  devEUI,
			DevAddr: devAddr,  // ABP: pre-set device address
			NwkSKey: nwkSKey,  // ABP: pre-set network session key
			AppSKey: appSKey,  // ABP: pre-set application session key
			Location: location.Location{
				Latitude:  lat,
				Longitude: lng,
				Altitude:  alt,
			},
			Status: devModels.Status{
				Active: true, // Devices are active by default
				MType:  getMType(tmpl.MType),
				Payload: &lorawan.DataPayload{
					Bytes: []byte{}, // Empty default payload
				},
			},
			Configuration: devModels.Configuration{
				Region:               region,
				SupportedOtaa:        false, // ABP: no OTAA join required
				SupportedClassB:      tmpl.SupportedClassB,
				SupportedClassC:      tmpl.SupportedClassC,
				SupportedADR:         tmpl.SupportedADR,
				SupportedFragment:    tmpl.SupportedFragment,
				Range:                tmpl.Range,
				DataRateInitial:      tmpl.DataRate,
				RX1DROffset:          tmpl.RX1DROffset,
				SendInterval:         time.Duration(tmpl.SendInterval) * time.Second,
				AckTimeout:           time.Duration(tmpl.AckTimeout) * time.Second,
				NbRepConfirmedDataUp: tmpl.NbRetransmission,
				UseCodec:             tmpl.UseCodec,
				CodecID:              tmpl.CodecID,
				IntegrationEnabled:   tmpl.IntegrationEnabled,
				IntegrationID:        tmpl.IntegrationID,
				DeviceProfileID:      tmpl.DeviceProfileID,
				DisableFCntDown:      true, // ABP: disable frame counter check to avoid issues
			},
			RX: []devFeatures.Window{
				{
					Delay:        time.Duration(tmpl.RX1Delay) * time.Millisecond,
					DurationOpen: time.Duration(tmpl.RX1Duration) * time.Millisecond,
					DataRate:     tmpl.DataRate,
				},
				{
					Delay:        time.Duration(tmpl.RX2Delay) * time.Millisecond,
					DurationOpen: time.Duration(tmpl.RX2Duration) * time.Millisecond,
					DataRate:     uint8(tmpl.RX2DataRate),
					Channel: devChannels.Channel{
						FrequencyDownlink: uint32(tmpl.RX2Frequency),
					},
				},
			},
		},
	}

	// Set fPort in uplink info
	fport := tmpl.FPort
	device.Info.Status.DataUplink.FPort = &fport

	return device
}

// generateRandomEUI64 generates a random 8-byte EUI64 address
func generateRandomEUI64() (lorawan.EUI64, error) {
	var eui lorawan.EUI64
	_, err := rand.Read(eui[:])
	return eui, err
}

// generateRandomKey generates a random 16-byte key
func generateRandomKey() ([16]byte, error) {
	var key [16]byte
	_, err := rand.Read(key[:])
	return key, err
}

// generateRandomDevAddr generates a random 4-byte DevAddr
func generateRandomDevAddr() (lorawan.DevAddr, error) {
	var addr lorawan.DevAddr
	_, err := rand.Read(addr[:])
	return addr, err
}

// randomizeCoordinates adds random offset to coordinates within a square spread
func randomizeCoordinates(baseLat, baseLng, spreadMeters float64) (float64, float64) {
	// Approximately 111,320 meters per degree of latitude
	const metersPerDegree = 111320.0

	// Random offset in range [-1, 1]
	latOffset := (mrand.Float64()*2 - 1) * (spreadMeters / metersPerDegree)

	// Longitude degrees vary with latitude
	lngMetersPerDegree := metersPerDegree * math.Cos(baseLat*math.Pi/180)
	lngOffset := (mrand.Float64()*2 - 1) * (spreadMeters / lngMetersPerDegree)

	return baseLat + latOffset, baseLng + lngOffset
}

// getMType converts int to lorawan.MType
func getMType(mtype int) lorawan.MType {
	if mtype == 1 {
		return lorawan.ConfirmedDataUp
	}
	return lorawan.UnconfirmedDataUp
}
