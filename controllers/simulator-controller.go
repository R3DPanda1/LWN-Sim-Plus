package controllers

import (
	"github.com/R3DPanda1/LWN-Sim-Plus/models"
	repo "github.com/R3DPanda1/LWN-Sim-Plus/repositories"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/codec"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/integration"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/integration/chirpstack"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/template"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/events"

	dev "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device"
	gw "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/gateway"
	e "github.com/R3DPanda1/LWN-Sim-Plus/socket"
	"github.com/brocaar/lorawan"
)

// SimulatorController is the interface that defines the methods that the simulator controller must implement.
type SimulatorController interface {
	Run() bool                                 // Run the simulator
	Stop() bool                                // Stop the simulator
	Status() bool                              // Get the status of the simulator
	GetInstance()                              // Get the instance of the simulator repository
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

	// Configuration
	SetPerformance(models.PerformanceConfig)
	SetEvents(models.EventsConfig)
}

// simulatorController controller struct
type simulatorController struct {
	repo repo.SimulatorRepository
}

// NewSimulatorController create a new controller instance with the provided repository
func NewSimulatorController(repo repo.SimulatorRepository) SimulatorController {
	return &simulatorController{
		repo: repo,
	}
}

// --- Controller calls to Repository, no need to comment them, they are self-explanatory ---
// Check the repository methods to see what they do

func (c *simulatorController) GetInstance() {
	c.repo.GetInstance()
}

func (c *simulatorController) Run() bool {
	return c.repo.Run()
}

func (c *simulatorController) Stop() bool {
	return c.repo.Stop()
}

func (c *simulatorController) Status() bool {
	return c.repo.Status()
}

func (c *simulatorController) SaveBridgeAddress(addr models.AddressIP) error {
	return c.repo.SaveBridgeAddress(addr)
}

func (c *simulatorController) GetBridgeAddress() models.AddressIP {
	return c.repo.GetBridgeAddress()
}

func (c *simulatorController) GetGateways() []gw.Gateway {
	return c.repo.GetGateways()
}

func (c *simulatorController) AddGateway(gateway *gw.Gateway) (int, int, error) {
	return c.repo.AddGateway(gateway)
}

func (c *simulatorController) UpdateGateway(gateway *gw.Gateway) (int, error) {
	return c.repo.UpdateGateway(gateway)
}

func (c *simulatorController) DeleteGateway(Id int) bool {
	return c.repo.DeleteGateway(Id)
}

func (c *simulatorController) AddDevice(device *dev.Device) (int, int, error) {
	return c.repo.AddDevice(device)
}

func (c *simulatorController) GetDevices() []dev.Device {
	return c.repo.GetDevices()
}

func (c *simulatorController) UpdateDevice(device *dev.Device) (int, error) {
	return c.repo.UpdateDevice(device)
}

func (c *simulatorController) DeleteDevice(Id int) bool {
	return c.repo.DeleteDevice(Id)
}

func (c *simulatorController) ToggleStateDevice(Id int) {
	c.repo.ToggleStateDevice(Id)
}

func (c *simulatorController) SendMACCommand(cid lorawan.CID, data e.MacCommand) {
	c.repo.SendMACCommand(cid, data)
}

func (c *simulatorController) ChangePayload(pl e.NewPayload) (string, bool) {
	return c.repo.ChangePayload(pl)
}

func (c *simulatorController) SendUplink(pl e.NewPayload) {
	c.repo.SendUplink(pl)
}

func (c *simulatorController) ChangeLocation(loc e.NewLocation) bool {
	return c.repo.ChangeLocation(loc)
}

func (c *simulatorController) ToggleStateGateway(Id int) {
	c.repo.ToggleStateGateway(Id)
}

func (c *simulatorController) GetCodecs() []codec.CodecMetadata {
	return c.repo.GetCodecs()
}

func (c *simulatorController) GetCodec(id int) (*codec.Codec, error) {
	return c.repo.GetCodec(id)
}

func (c *simulatorController) AddCodec(codec *codec.Codec) error {
	return c.repo.AddCodec(codec)
}

func (c *simulatorController) UpdateCodec(id int, name string, script string) error {
	return c.repo.UpdateCodec(id, name, script)
}

func (c *simulatorController) DeleteCodec(id int) error {
	return c.repo.DeleteCodec(id)
}

func (c *simulatorController) GetDevicesUsingCodec(codecID int) []string {
	return c.repo.GetDevicesUsingCodec(codecID)
}

// --- Integration management methods ---

func (c *simulatorController) GetIntegrations() []*integration.Integration {
	return c.repo.GetIntegrations()
}

func (c *simulatorController) GetIntegration(id int) (*integration.Integration, error) {
	return c.repo.GetIntegration(id)
}

func (c *simulatorController) AddIntegration(name string, intType integration.IntegrationType, url, apiKey, tenantID, appID string) (int, error) {
	return c.repo.AddIntegration(name, intType, url, apiKey, tenantID, appID)
}

func (c *simulatorController) UpdateIntegration(id int, name, url, apiKey, tenantID, appID string, enabled bool) error {
	return c.repo.UpdateIntegration(id, name, url, apiKey, tenantID, appID, enabled)
}

func (c *simulatorController) DeleteIntegration(id int) error {
	return c.repo.DeleteIntegration(id)
}

func (c *simulatorController) TestIntegrationConnection(id int) error {
	return c.repo.TestIntegrationConnection(id)
}

func (c *simulatorController) GetDeviceProfiles(id int) ([]chirpstack.DeviceProfile, error) {
	return c.repo.GetDeviceProfiles(id)
}

// --- Template management methods ---

func (c *simulatorController) GetTemplates() []*template.DeviceTemplate {
	return c.repo.GetTemplates()
}

func (c *simulatorController) GetTemplate(id int) (*template.DeviceTemplate, error) {
	return c.repo.GetTemplate(id)
}

func (c *simulatorController) AddTemplate(tmpl *template.DeviceTemplate) (int, error) {
	return c.repo.AddTemplate(tmpl)
}

func (c *simulatorController) UpdateTemplate(tmpl *template.DeviceTemplate) error {
	return c.repo.UpdateTemplate(tmpl)
}

func (c *simulatorController) DeleteTemplate(id int) error {
	return c.repo.DeleteTemplate(id)
}

func (c *simulatorController) CreateDevicesFromTemplate(templateID int, count int, namePrefix string, baseLat, baseLng float64, baseAlt int32, spreadMeters float64) ([]int, error) {
	return c.repo.CreateDevicesFromTemplate(templateID, count, namePrefix, baseLat, baseLng, baseAlt, spreadMeters)
}

func (c *simulatorController) GetEventBroker() *events.EventBroker {
	return c.repo.GetEventBroker()
}

func (c *simulatorController) SetPerformance(perf models.PerformanceConfig) {
	c.repo.SetPerformance(perf)
}

func (c *simulatorController) SetEvents(ev models.EventsConfig) {
	c.repo.SetEvents(ev)
}
