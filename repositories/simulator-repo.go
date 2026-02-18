package repositories

import (
	"log/slog"

	"github.com/brocaar/lorawan"

	"github.com/R3DPanda1/LWN-Sim-Plus/models"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/codec"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/integration"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/integration/chirpstack"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/template"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/events"
	e "github.com/R3DPanda1/LWN-Sim-Plus/socket"

	"github.com/R3DPanda1/LWN-Sim-Plus/simulator"
	dev "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device"
	gw "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/gateway"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/util"
)

// SimulatorRepository is the interface that defines the methods that the simulator repository must implement.
type SimulatorRepository interface {
	Run() bool                                 // Run the simulator
	Stop() bool                                // Stop the simulator
	Status() bool                              // Get the status of the simulator
	GetInstance()                              // Get the instance of the simulator
	SaveBridgeAddress(models.AddressIP) error  // Save the bridge address
	GetBridgeAddress() models.AddressIP        // Get the bridge address
	GetGateways() []gw.Gateway                 // Get the gateways
	AddGateway(*gw.Gateway) (int, int, error)  // Add a gateway
	UpdateGateway(*gw.Gateway) (int, error)    // Update a gateway
	DeleteGateway(int) bool                    // Delete a gateway
	AddDevice(*dev.Device) (int, int, error)   // Add a device
	GetDevices() []dev.Device                  // Get the devices
	UpdateDevice(*dev.Device) (int, error)     // Update a device
	DeleteDevice(int) bool                     // Delete a device
	ToggleStateDevice(int)                     // Toggle the state of a device
	SendMACCommand(lorawan.CID, e.MacCommand)  // Send a MAC command
	ChangePayload(e.NewPayload) (string, bool) // Change the payload
	SendUplink(e.NewPayload)                   // Send an uplink
	ChangeLocation(e.NewLocation) bool         // Change the location
	ToggleStateGateway(int)                    // Toggle the state of a gateway
	GetCodecs() []codec.CodecMetadata        // Get all available codecs
	GetCodec(int) (*codec.Codec, error)      // Get a specific codec by ID
	AddCodec(*codec.Codec) error             // Add a custom codec
	UpdateCodec(int, string, string) error   // Update an existing codec by ID
	DeleteCodec(int) error                   // Delete a codec by ID
	GetDevicesUsingCodec(int) []string       // Get devices using a specific codec

	// Integration management
	GetIntegrations() []*integration.Integration                                                    // Get all integrations
	GetIntegration(int) (*integration.Integration, error)                                           // Get a specific integration
	AddIntegration(string, integration.IntegrationType, string, string, string, string) (int, error) // Add a new integration (name, type, url, apiKey, tenantId, appId)
	UpdateIntegration(int, string, string, string, string, string, bool) error                      // Update an integration (id, name, url, apiKey, tenantId, appId, enabled)
	DeleteIntegration(int) error                                                                    // Delete an integration
	TestIntegrationConnection(int) error                                                            // Test connection to an integration
	GetDeviceProfiles(int) ([]chirpstack.DeviceProfile, error)                                      // Get device profiles from ChirpStack

	// Template management
	GetTemplates() []*template.DeviceTemplate                                                      // Get all templates
	GetTemplate(int) (*template.DeviceTemplate, error)                                             // Get a specific template
	AddTemplate(*template.DeviceTemplate) (int, error)                                             // Add a new template
	UpdateTemplate(*template.DeviceTemplate) error                                                 // Update a template
	DeleteTemplate(int) error                                                                      // Delete a template
	CreateDevicesFromTemplate(int, int, string, float64, float64, int32, float64) ([]int, error)   // Bulk create devices from template

	// Event broker
	GetEventBroker() *events.EventBroker
}

// simulatorRepository repository struct
type simulatorRepository struct {
	sim *simulator.Simulator
}

// NewSimulatorRepository create a new repository instance
func NewSimulatorRepository() SimulatorRepository {
	return &simulatorRepository{}
}

// --- Repository calls to Simulator, no need to comment them, they are self-explanatory ---
// Check the simulator methods to see what they do

func (s *simulatorRepository) GetInstance() {
	s.sim = simulator.GetInstance()
}

// Run If the simulator is stopped, it starts it and returns True, otherwise returns False.
func (s *simulatorRepository) Run() bool {
	switch s.sim.State {
	case util.Running:
		slog.Warn("simulator already running", "component", "simulator")
		return false
	default: // State = util.Stopped
		s.sim.Run()
	}
	return true
}

// Stop If the simulator is running, it stops it and returns True, otherwise returns False.
func (s *simulatorRepository) Stop() bool {
	switch s.sim.State {
	case util.Stopped:
		slog.Warn("simulator already stopped", "component", "simulator")
		return false
	default: //running
		s.sim.Stop()
		return true
	}
}

// Status returns True if the simulator is running, otherwise it returns False.
func (s *simulatorRepository) Status() bool {
	if s.sim.State == util.Running {
		return true
	}
	return false
}

func (s *simulatorRepository) SaveBridgeAddress(addr models.AddressIP) error {
	return s.sim.SaveBridgeAddress(addr)
}

func (s *simulatorRepository) GetBridgeAddress() models.AddressIP {
	return s.sim.GetBridgeAddress()
}

func (s *simulatorRepository) GetGateways() []gw.Gateway {
	return s.sim.GetGateways()
}

func (s *simulatorRepository) AddGateway(gateway *gw.Gateway) (int, int, error) {
	return s.sim.SetGateway(gateway, false)
}

func (s *simulatorRepository) UpdateGateway(gateway *gw.Gateway) (int, error) {
	code, _, err := s.sim.SetGateway(gateway, true)
	return code, err
}

func (s *simulatorRepository) DeleteGateway(Id int) bool {
	return s.sim.DeleteGateway(Id)
}

func (s *simulatorRepository) AddDevice(device *dev.Device) (int, int, error) {
	return s.sim.SetDevice(device, false)
}

func (s *simulatorRepository) GetDevices() []dev.Device {
	return s.sim.GetDevices()
}

func (s *simulatorRepository) UpdateDevice(device *dev.Device) (int, error) {
	code, _, err := s.sim.SetDevice(device, true)
	return code, err
}

func (s *simulatorRepository) DeleteDevice(Id int) bool {
	return s.sim.DeleteDevice(Id)
}

func (s *simulatorRepository) ToggleStateDevice(Id int) {
	s.sim.ToggleStateDevice(Id)
}

func (s *simulatorRepository) SendMACCommand(cid lorawan.CID, data e.MacCommand) {
	s.sim.SendMACCommand(cid, data)
}

func (s *simulatorRepository) ChangePayload(pl e.NewPayload) (string, bool) {
	return s.sim.ChangePayload(pl)
}

func (s *simulatorRepository) SendUplink(pl e.NewPayload) {
	s.sim.SendUplink(pl)
}

func (s *simulatorRepository) ChangeLocation(loc e.NewLocation) bool {
	return s.sim.ChangeLocation(loc)
}

func (s *simulatorRepository) ToggleStateGateway(Id int) {
	s.sim.ToggleStateGateway(Id)
}

func (s *simulatorRepository) GetCodecs() []codec.CodecMetadata {
	return s.sim.GetCodecs()
}

func (s *simulatorRepository) GetCodec(id int) (*codec.Codec, error) {
	return s.sim.GetCodec(id)
}

func (s *simulatorRepository) AddCodec(codec *codec.Codec) error {
	return s.sim.AddCodec(codec)
}

func (s *simulatorRepository) UpdateCodec(id int, name string, script string) error {
	return s.sim.UpdateCodec(id, name, script)
}

func (s *simulatorRepository) DeleteCodec(id int) error {
	return s.sim.DeleteCodec(id)
}

func (s *simulatorRepository) GetDevicesUsingCodec(codecID int) []string {
	return s.sim.GetDevicesUsingCodec(codecID)
}

// --- Integration management methods ---

func (s *simulatorRepository) GetIntegrations() []*integration.Integration {
	return s.sim.GetIntegrations()
}

func (s *simulatorRepository) GetIntegration(id int) (*integration.Integration, error) {
	return s.sim.GetIntegration(id)
}

func (s *simulatorRepository) AddIntegration(name string, intType integration.IntegrationType, url, apiKey, tenantID, appID string) (int, error) {
	return s.sim.AddIntegration(name, intType, url, apiKey, tenantID, appID)
}

func (s *simulatorRepository) UpdateIntegration(id int, name, url, apiKey, tenantID, appID string, enabled bool) error {
	return s.sim.UpdateIntegration(id, name, url, apiKey, tenantID, appID, enabled)
}

func (s *simulatorRepository) DeleteIntegration(id int) error {
	return s.sim.DeleteIntegration(id)
}

func (s *simulatorRepository) TestIntegrationConnection(id int) error {
	return s.sim.TestIntegrationConnection(id)
}

func (s *simulatorRepository) GetDeviceProfiles(id int) ([]chirpstack.DeviceProfile, error) {
	return s.sim.GetDeviceProfiles(id)
}

// --- Template management methods ---

func (s *simulatorRepository) GetTemplates() []*template.DeviceTemplate {
	return s.sim.GetTemplates()
}

func (s *simulatorRepository) GetTemplate(id int) (*template.DeviceTemplate, error) {
	return s.sim.GetTemplate(id)
}

func (s *simulatorRepository) AddTemplate(tmpl *template.DeviceTemplate) (int, error) {
	return s.sim.AddTemplate(tmpl)
}

func (s *simulatorRepository) UpdateTemplate(tmpl *template.DeviceTemplate) error {
	return s.sim.UpdateTemplate(tmpl)
}

func (s *simulatorRepository) DeleteTemplate(id int) error {
	return s.sim.DeleteTemplate(id)
}

func (s *simulatorRepository) CreateDevicesFromTemplate(templateID int, count int, namePrefix string, baseLat, baseLng float64, baseAlt int32, spreadMeters float64) ([]int, error) {
	return s.sim.CreateDevicesFromTemplate(templateID, count, namePrefix, baseLat, baseLng, baseAlt, spreadMeters)
}

func (s *simulatorRepository) GetEventBroker() *events.EventBroker {
	return s.sim.EventBroker
}
