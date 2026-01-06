package simulator

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math"
	mrand "math/rand"
	"strings"
	"time"

	"github.com/brocaar/lorawan"

	"github.com/R3DPanda1/LWN-Sim-Plus/codec"
	"github.com/R3DPanda1/LWN-Sim-Plus/codes"
	"github.com/R3DPanda1/LWN-Sim-Plus/integration"
	"github.com/R3DPanda1/LWN-Sim-Plus/integration/chirpstack"
	"github.com/R3DPanda1/LWN-Sim-Plus/template"
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
	c "github.com/R3DPanda1/LWN-Sim-Plus/simulator/console"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/resources/location"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/util"
	"github.com/R3DPanda1/LWN-Sim-Plus/socket"
	socketio "github.com/googollee/go-socket.io"
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
	// Attach console
	s.Console = c.Console{}

	// Initialize codec manager (Phase 1-3 enhancement)
	if dev.CodecManager == nil {
		dev.CodecManager = codec.NewManager(codec.DefaultExecutorConfig())

		// Load codec library from disk
		pathDir, err := util.GetPath()
		codecLibLoaded := false
		if err == nil {
			codecLibPath := pathDir + "/codecs.json"
			if err := dev.CodecManager.LoadCodecLibrary(codecLibPath); err != nil {
				shared.DebugPrint(fmt.Sprintf("Warning: %v", err))
			} else {
				shared.DebugPrint("Codec library loaded from disk")
				codecLibLoaded = true
			}
		}

		// If no codecs loaded from disk, load defaults
		if !codecLibLoaded || dev.CodecManager.GetCodecCount() == 0 {
			dev.CodecManager.LoadDefaults()
			shared.DebugPrint("Default codecs loaded")
		}

		shared.DebugPrint("Codec manager initialized")
	}

	// Initialize integration manager
	s.IntegrationManager = integration.NewManager()
	pathDir, err := util.GetPath()
	if err == nil {
		integrationPath := pathDir + "/integrations.json"
		if err := s.IntegrationManager.Load(integrationPath); err != nil {
			shared.DebugPrint(fmt.Sprintf("Warning: failed to load integrations: %v", err))
		} else {
			shared.DebugPrint("Integrations loaded from disk")
		}
	}

	// Initialize template manager
	s.TemplateManager = template.NewManager()
	templateLoaded := false
	if pathDir != "" {
		templatePath := pathDir + "/templates.json"
		if err := s.TemplateManager.Load(templatePath); err != nil {
			shared.DebugPrint(fmt.Sprintf("Warning: failed to load templates: %v", err))
		} else {
			shared.DebugPrint("Templates loaded from disk")
			templateLoaded = true
		}
	}

	// If no templates loaded from disk, load defaults
	if !templateLoaded || s.TemplateManager.Count() == 0 {
		// Create codec lookup function to link templates with their codecs
		codecLookup := func(name string) string {
			return dev.CodecManager.GetCodecIDByName(name)
		}
		s.TemplateManager.LoadDefaults(codecLookup)
		shared.DebugPrint("Default templates loaded")
	}

	return &s
}

func (s *Simulator) AddWebSocket(WebSocket *socketio.Conn) {
	s.Console.SetupWebSocket(WebSocket)
	s.Resources.AddWebSocket(WebSocket)
	s.SetupConsole()
}

// Run starts the simulation environment
func (s *Simulator) Run() {
	shared.DebugPrint("Executing Run")
	s.State = util.Running
	s.setup()
	s.Print("START", nil, util.PrintBoth)
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
	s.saveStatus()

	// Save codec library
	if dev.CodecManager != nil {
		pathDir, err := util.GetPath()
		if err == nil {
			codecLibPath := pathDir + "/codecs.json"
			if err := dev.CodecManager.SaveCodecLibrary(codecLibPath); err != nil {
				shared.DebugPrint(fmt.Sprintf("Warning: failed to save codec library: %v", err))
			} else {
				shared.DebugPrint("Codec library saved to disk")
			}
		}
	}

	// Save integrations
	if s.IntegrationManager != nil {
		pathDir, err := util.GetPath()
		if err == nil {
			integrationPath := pathDir + "/integrations.json"
			if err := s.IntegrationManager.Save(integrationPath); err != nil {
				shared.DebugPrint(fmt.Sprintf("Warning: failed to save integrations: %v", err))
			} else {
				shared.DebugPrint("Integrations saved to disk")
			}
		}
	}

	// Save templates
	if s.TemplateManager != nil {
		pathDir, err := util.GetPath()
		if err == nil {
			templatePath := pathDir + "/templates.json"
			if err := s.TemplateManager.Save(templatePath); err != nil {
				shared.DebugPrint(fmt.Sprintf("Warning: failed to save templates: %v", err))
			} else {
				shared.DebugPrint("Templates saved to disk")
			}
		}
	}

	s.Forwarder.Reset()
	s.Print("STOPPED", nil, util.PrintBoth)
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
	s.Print("Gateway Bridge Address saved", nil, util.PrintOnlyConsole)
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
		s.Print("Error: MAC Address invalid", nil, util.PrintOnlyConsole)
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
		s.Print("Name already used", nil, util.PrintOnlyConsole)
		return code, -1, err
	}
	// Check if the name is already used
	code, err = s.searchAddress(gateway.Info.MACAddress, gateway.Id, true)
	if err != nil {
		s.Print("DevEUI already used", nil, util.PrintOnlyConsole)
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

	s.Print("Gateway Saved", nil, util.PrintOnlyConsole)

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

	s.Print("Gateway Deleted", nil, util.PrintOnlyConsole)

	return true
}

func (s *Simulator) SetDevice(device *dev.Device, update bool) (int, int, error) {

	emptyAddr := lorawan.EUI64{0, 0, 0, 0, 0, 0, 0, 0}

	if device.Info.DevEUI == emptyAddr {

		s.Print("DevEUI invalid", nil, util.PrintOnlyConsole)
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

		s.Print("Name already used", nil, util.PrintOnlyConsole)
		return code, -1, err

	}

	code, err = s.searchAddress(device.Info.DevEUI, device.Id, false)
	if err != nil {

		s.Print("DevEUI already used", nil, util.PrintOnlyConsole)
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

	s.Print("Device Saved", nil, util.PrintOnlyConsole)

	// Provision device to ChirpStack if integration is enabled (only for new devices)
	if !update && device.Info.Configuration.IntegrationEnabled && device.Info.Configuration.IntegrationID != "" {
		devEUI := hex.EncodeToString(device.Info.DevEUI[:])

		var err error
		if device.Info.Configuration.SupportedOtaa {
			// OTAA: provision with AppKey
			appKey := hex.EncodeToString(device.Info.AppKey[:])
			err = s.IntegrationManager.ProvisionDevice(
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
			err = s.IntegrationManager.ProvisionDeviceABP(
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
			s.Print("ChirpStack provisioning failed: "+err.Error(), nil, util.PrintOnlyConsole)
		} else {
			activationType := "OTAA"
			if !device.Info.Configuration.SupportedOtaa {
				activationType = "ABP"
			}
			s.Print(fmt.Sprintf("Device provisioned to ChirpStack (%s)", activationType), nil, util.PrintOnlyConsole)
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
	if device.Info.Configuration.IntegrationEnabled && device.Info.Configuration.IntegrationID != "" {
		devEUI := hex.EncodeToString(device.Info.DevEUI[:])
		if err := s.IntegrationManager.DeleteDevice(device.Info.Configuration.IntegrationID, devEUI); err != nil {
			s.Print("ChirpStack deletion failed: "+err.Error(), nil, util.PrintOnlyConsole)
		} else {
			s.Print("Device deleted from ChirpStack", nil, util.PrintOnlyConsole)
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

	s.Print("Device Deleted", nil, util.PrintOnlyConsole)

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
		s.Console.PrintSocket(socket.EventResponseCommand, s.Devices[data.Id].Info.Name+" is turned off")
		return
	}

	err := s.Devices[data.Id].SendMACCommand(cid, data.Periodicity)
	if err != nil {
		s.Console.PrintSocket(socket.EventResponseCommand, "Unable to send command: "+err.Error())
	} else {
		s.Console.PrintSocket(socket.EventResponseCommand, "MACCommand will be sent to the next uplink")
	}

}

func (s *Simulator) ChangePayload(pl socket.NewPayload) (string, bool) {

	devEUIstring := hex.EncodeToString(s.Devices[pl.Id].Info.DevEUI[:])

	if !s.Devices[pl.Id].IsOn() {
		s.Console.PrintSocket(socket.EventResponseCommand, s.Devices[pl.Id].Info.Name+" is turned off")
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

	s.Console.PrintSocket(socket.EventResponseCommand, s.Devices[pl.Id].Info.Name+": Payload changed")

	return devEUIstring, true
}

func (s *Simulator) SendUplink(pl socket.NewPayload) {

	if !s.Devices[pl.Id].IsOn() {
		s.Console.PrintSocket(socket.EventResponseCommand, s.Devices[pl.Id].Info.Name+" is turned off")
		return
	}

	MType := lorawan.UnconfirmedDataUp
	if pl.MType == "ConfirmedDataUp" {
		MType = lorawan.ConfirmedDataUp
	}

	s.Devices[pl.Id].NewUplink(MType, pl.Payload)

	s.Console.PrintSocket(socket.EventResponseCommand, "Uplink queued")
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
	if dev.CodecManager == nil {
		return []codec.CodecMetadata{}
	}
	return dev.CodecManager.ListCodecs()
}

// GetCodec returns a specific codec by ID
func (s *Simulator) GetCodec(id string) (*codec.Codec, error) {
	if dev.CodecManager == nil {
		return nil, errors.New("codec manager not initialized")
	}
	return dev.CodecManager.GetCodec(id)
}

// GetDevicesUsingCodec returns a list of device EUIs using the specified codec
// Also counts templates that use this codec
func (s *Simulator) GetDevicesUsingCodec(codecID string) []string {
	devicesUsingCodec := []string{}

	// Check devices
	for _, device := range s.Devices {
		if device.Info.Configuration.CodecID == codecID {
			devicesUsingCodec = append(devicesUsingCodec, device.Info.DevEUI.String())
		}
	}

	// Check templates
	for _, tmpl := range s.TemplateManager.List() {
		if tmpl.UseCodec && tmpl.CodecID == codecID {
			devicesUsingCodec = append(devicesUsingCodec, "template:"+tmpl.ID)
		}
	}

	return devicesUsingCodec
}

// AddCodec adds a custom codec
func (s *Simulator) AddCodec(c *codec.Codec) error {
	if dev.CodecManager == nil {
		return errors.New("codec manager not initialized")
	}

	if err := dev.CodecManager.AddCodec(c); err != nil {
		return err
	}

	// Save codec library to disk
	s.saveCodecLibrary()
	return nil
}

// UpdateCodec updates an existing codec
func (s *Simulator) UpdateCodec(id string, name string, script string) error {
	if dev.CodecManager == nil {
		return errors.New("codec manager not initialized")
	}

	if err := dev.CodecManager.UpdateCodec(id, name, script); err != nil {
		return err
	}

	// Save codec library to disk
	s.saveCodecLibrary()
	return nil
}

// DeleteCodec removes a codec by ID
func (s *Simulator) DeleteCodec(id string) error {
	if dev.CodecManager == nil {
		return errors.New("codec manager not initialized")
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

	if err := dev.CodecManager.RemoveCodec(id); err != nil {
		return err
	}

	// Save codec library to disk
	s.saveCodecLibrary()
	return nil
}

// saveCodecLibrary saves the codec library to disk
func (s *Simulator) saveCodecLibrary() {
	s.saveToFile("codecs.json", func(path string) error {
		if dev.CodecManager != nil {
			return dev.CodecManager.SaveCodecLibrary(path)
		}
		return nil
	})
}

// ==================== Integration Management ====================

// GetIntegrations returns all integrations (without API keys)
func (s *Simulator) GetIntegrations() []*integration.Integration {
	if s.IntegrationManager == nil {
		return []*integration.Integration{}
	}
	return s.IntegrationManager.List()
}

// GetIntegration returns a specific integration by ID
func (s *Simulator) GetIntegration(id string) (*integration.Integration, error) {
	if s.IntegrationManager == nil {
		return nil, errors.New("integration manager not initialized")
	}
	return s.IntegrationManager.Get(id)
}

// AddIntegration adds a new integration
func (s *Simulator) AddIntegration(name string, intType integration.IntegrationType, url, apiKey, tenantID, appID string) (string, error) {
	if s.IntegrationManager == nil {
		return "", errors.New("integration manager not initialized")
	}

	integ := integration.NewIntegration(name, intType, url, apiKey, tenantID, appID)
	if err := s.IntegrationManager.Add(integ); err != nil {
		return "", err
	}

	// Save to disk
	s.saveIntegrationLibrary()
	return integ.ID, nil
}

// UpdateIntegration updates an existing integration
func (s *Simulator) UpdateIntegration(id, name, url, apiKey, tenantID, appID string, enabled bool) error {
	if s.IntegrationManager == nil {
		return errors.New("integration manager not initialized")
	}

	if err := s.IntegrationManager.Update(id, name, url, apiKey, tenantID, appID, enabled); err != nil {
		return err
	}

	// Save to disk
	s.saveIntegrationLibrary()
	return nil
}

// DeleteIntegration removes an integration by ID
func (s *Simulator) DeleteIntegration(id string) error {
	if s.IntegrationManager == nil {
		return errors.New("integration manager not initialized")
	}

	// Check if any devices are using this integration
	devicesUsingIntegration := s.GetDevicesUsingIntegration(id)
	if len(devicesUsingIntegration) > 0 {
		return fmt.Errorf("cannot delete integration: used by %d device(s)", len(devicesUsingIntegration))
	}

	if err := s.IntegrationManager.Remove(id); err != nil {
		return err
	}

	// Save to disk
	s.saveIntegrationLibrary()
	return nil
}

// TestIntegrationConnection tests connection to an integration
func (s *Simulator) TestIntegrationConnection(id string) error {
	if s.IntegrationManager == nil {
		return errors.New("integration manager not initialized")
	}
	return s.IntegrationManager.TestConnection(id)
}

// GetDeviceProfiles returns device profiles for an integration
func (s *Simulator) GetDeviceProfiles(id string) ([]chirpstack.DeviceProfile, error) {
	if s.IntegrationManager == nil {
		return nil, errors.New("integration manager not initialized")
	}
	return s.IntegrationManager.GetDeviceProfiles(id)
}

// GetDevicesUsingIntegration returns a list of device EUIs using the specified integration
func (s *Simulator) GetDevicesUsingIntegration(integrationID string) []string {
	devicesUsingIntegration := []string{}
	for _, device := range s.Devices {
		if device.Info.Configuration.IntegrationID == integrationID {
			devicesUsingIntegration = append(devicesUsingIntegration, device.Info.DevEUI.String())
		}
	}
	return devicesUsingIntegration
}

// saveIntegrationLibrary saves integrations to disk
func (s *Simulator) saveIntegrationLibrary() {
	s.saveToFile("integrations.json", func(path string) error {
		if s.IntegrationManager != nil {
			return s.IntegrationManager.Save(path)
		}
		return nil
	})
}

// saveToFile is a generic helper for saving data to JSON files
func (s *Simulator) saveToFile(filename string, saveFn func(string) error) {
	pathDir, err := util.GetPath()
	if err == nil {
		fullPath := pathDir + "/" + filename
		if err := saveFn(fullPath); err != nil {
			shared.DebugPrint(fmt.Sprintf("Warning: failed to save %s: %v", filename, err))
		}
	}
}

// ==================== Template Management ====================

// GetTemplates returns all templates
func (s *Simulator) GetTemplates() []*template.DeviceTemplate {
	if s.TemplateManager == nil {
		return []*template.DeviceTemplate{}
	}
	return s.TemplateManager.List()
}

// GetTemplate returns a specific template by ID
func (s *Simulator) GetTemplate(id string) (*template.DeviceTemplate, error) {
	if s.TemplateManager == nil {
		return nil, errors.New("template manager not initialized")
	}
	return s.TemplateManager.Get(id)
}

// AddTemplate adds a new template
func (s *Simulator) AddTemplate(tmpl *template.DeviceTemplate) (string, error) {
	if s.TemplateManager == nil {
		return "", errors.New("template manager not initialized")
	}

	// Generate ID if not set
	if tmpl.ID == "" {
		tmpl.RegenerateID()
	}

	if err := s.TemplateManager.Add(tmpl); err != nil {
		return "", err
	}

	// Save to disk
	s.saveTemplateLibrary()
	return tmpl.ID, nil
}

// UpdateTemplate updates an existing template
func (s *Simulator) UpdateTemplate(tmpl *template.DeviceTemplate) error {
	if s.TemplateManager == nil {
		return errors.New("template manager not initialized")
	}

	if err := s.TemplateManager.Update(tmpl); err != nil {
		return err
	}

	// Save to disk
	s.saveTemplateLibrary()
	return nil
}

// DeleteTemplate removes a template by ID
func (s *Simulator) DeleteTemplate(id string) error {
	if s.TemplateManager == nil {
		return errors.New("template manager not initialized")
	}

	if err := s.TemplateManager.Remove(id); err != nil {
		return err
	}

	// Save to disk
	s.saveTemplateLibrary()
	return nil
}

// saveTemplateLibrary saves templates to disk
func (s *Simulator) saveTemplateLibrary() {
	s.saveToFile("templates.json", func(path string) error {
		if s.TemplateManager != nil {
			return s.TemplateManager.Save(path)
		}
		return nil
	})
}

// ==================== Bulk Device Creation ====================

// CreateDevicesFromTemplate creates multiple devices from a template
func (s *Simulator) CreateDevicesFromTemplate(templateID string, count int, namePrefix string, baseLat, baseLng float64, baseAlt int32, spreadMeters float64) ([]int, error) {
	if s.TemplateManager == nil {
		return nil, errors.New("template manager not initialized")
	}

	// Get template
	tmpl, err := s.TemplateManager.Get(templateID)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
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
			s.Print(fmt.Sprintf("Failed to generate DevEUI for %s: %v", name, err), nil, util.PrintOnlyConsole)
			continue
		}

		// Generate ABP keys (NwkSKey, AppSKey, DevAddr) instead of OTAA AppKey
		nwkSKey, err := generateRandomKey()
		if err != nil {
			s.Print(fmt.Sprintf("Failed to generate NwkSKey for %s: %v", name, err), nil, util.PrintOnlyConsole)
			continue
		}

		appSKey, err := generateRandomKey()
		if err != nil {
			s.Print(fmt.Sprintf("Failed to generate AppSKey for %s: %v", name, err), nil, util.PrintOnlyConsole)
			continue
		}

		devAddr, err := generateRandomDevAddr()
		if err != nil {
			s.Print(fmt.Sprintf("Failed to generate DevAddr for %s: %v", name, err), nil, util.PrintOnlyConsole)
			continue
		}

		// Randomize coordinates
		lat, lng := randomizeCoordinates(baseLat, baseLng, spreadMeters)

		// Create device from template using ABP (no join required)
		device := s.createDeviceFromTemplate(tmpl, name, devEUI, nwkSKey, appSKey, devAddr, lat, lng, baseAlt)

		// Add device
		code, id, err := s.SetDevice(device, false)
		if err != nil {
			s.Print(fmt.Sprintf("Failed to create device %s: %v (code: %d)", name, err, code), nil, util.PrintOnlyConsole)
			continue
		}

		createdIDs = append(createdIDs, id)
		s.Print(fmt.Sprintf("Created ABP device %s (ID: %d, DevAddr: %s)", name, id, hex.EncodeToString(devAddr[:])), nil, util.PrintOnlyConsole)
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
