package simulator

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"

	"github.com/R3DPanda1/LWN-Sim-Plus/codes"
	"github.com/R3DPanda1/LWN-Sim-Plus/models"
	"github.com/R3DPanda1/LWN-Sim-Plus/shared"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/integration"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/integration/chirpstack"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/template"
	dev "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device"
	f "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/forwarder"
	mfw "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/forwarder/models"
	gw "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/gateway"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/events"
	res "github.com/R3DPanda1/LWN-Sim-Plus/simulator/resources"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/scheduler"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/util"
	"github.com/brocaar/lorawan"
)

// Simulator is a model
type Simulator struct {
	State                 uint8               `json:"-"`                 // Runtime state: Stop, Running
	Devices               map[int]*dev.Device `json:"-"`                 // A collection of devices
	ActiveDevices         map[int]int         `json:"-"`                 // A collection of active devices
	ActiveGateways        map[int]int         `json:"-"`                 // A collection of active gateways
	ComponentsInactiveTmp int                 `json:"-"`                 // Number of inactive components
	Gateways              map[int]*gw.Gateway `json:"-"`                 // A collection of gateways
	Forwarder             f.Forwarder         `json:"-"`                 // Forwarder instance used for communication between devices and gateways
	NextIDDev             int                 `json:"nextIDDev"`         // Next device ID used for creating a new device
	NextIDGw              int                 `json:"nextIDGw"`          // Next gateway ID used for creating a new gateway
	NextIDIntegration     int                 `json:"nextIDIntegration"` // Next integration ID
	NextIDTemplate        int                 `json:"nextIDTemplate"`    // Next template ID
	NextIDCodec           int                 `json:"nextIDCodec"`       // Next codec ID
	BridgeAddress         string              `json:"bridgeAddress"`     // Bridge address used to connect to a network
	Resources             res.Resources       `json:"-"`                 // Resources used for managing the simulator
	// Integration management (like Devices/Gateways pattern)
	Integrations       map[int]*integration.Integration `json:"-"` // A collection of integrations
	IntegrationClients map[int]*chirpstack.Client       `json:"-"` // ChirpStack clients for each integration
	// Template management (like Devices/Gateways pattern)
	Templates map[int]*template.DeviceTemplate `json:"-"` // A collection of device templates

	EventBroker *events.EventBroker        `json:"-"`
	Scheduler   *scheduler.Scheduler       `json:"-"`
	Performance models.PerformanceConfig   `json:"-"`
	Events      models.EventsConfig        `json:"-"`
}

// setup loads and initializes the simulator maps for gateways and devices
func (s *Simulator) setup() {
	historySize := s.Events.HistoryPerDevice
	if historySize <= 0 {
		historySize = 100
	}
	s.EventBroker = events.NewEventBroker(historySize)
	s.setupGateways()
	s.setupDevices()
	slog.Info("simulator setup complete", "component", "simulator")
	s.EventBroker.PublishSystemEvent(events.SystemEvent{
		Type:    events.SysEventSetup,
		Message: "Simulator setup complete",
	})
}

// setupGateways initializes the gateways by setting their state to Stopped and adding them to the ActiveGateways map if they are active
func (s *Simulator) setupGateways() {
	for _, g := range s.Gateways {
		s.Gateways[g.Id].State = util.Stopped
		if g.Info.Active {
			s.ActiveGateways[g.Id] = g.Id
		}
	}
	slog.Debug("gateways setup complete", "component", "simulator", "count", len(s.Gateways), "active", len(s.ActiveGateways))
}

// setupDevices initializes the devices by setting their state to Stopped and adding them to the ActiveDevices map if they are active
func (s *Simulator) setupDevices() {
	for _, d := range s.Devices {
		s.Devices[d.Id].State = util.Stopped
		if d.Info.Status.Active {
			s.ActiveDevices[d.Id] = d.Id
		}
	}
	slog.Debug("devices setup complete", "component", "simulator", "count", len(s.Devices), "active", len(s.ActiveDevices))
}

// loadData retrieves the simulator configuration, devices, gateways, integrations, and templates from JSON files
func (s *Simulator) loadData() {
	path, err := util.GetPath()
	if err != nil {
		log.Fatal(err)
	}
	err = util.RecoverConfigFile(path+"/simulator.json", &s)
	if err != nil {
		log.Fatal(err)
	}
	err = util.RecoverConfigFile(path+"/gateways.json", &s.Gateways)
	if err != nil {
		log.Fatal(err)
	}
	err = util.RecoverConfigFile(path+"/devices.json", &s.Devices)
	if err != nil {
		log.Fatal(err)
	}
	// Load integrations (non-fatal if missing)
	err = util.RecoverConfigFile(path+"/integrations.json", &s.Integrations)
	if err != nil {
		shared.DebugPrint(fmt.Sprintf("Warning: failed to load integrations: %v", err))
	}
	// Load templates (non-fatal if missing)
	err = util.RecoverConfigFile(path+"/templates.json", &s.Templates)
	if err != nil {
		shared.DebugPrint(fmt.Sprintf("Warning: failed to load templates: %v", err))
	}
}

// setupIntegrations initializes ChirpStack clients for all integrations
func (s *Simulator) setupIntegrations() {
	if s.Integrations == nil {
		s.Integrations = make(map[int]*integration.Integration)
	}
	if s.IntegrationClients == nil {
		s.IntegrationClients = make(map[int]*chirpstack.Client)
	}
	for _, i := range s.Integrations {
		if i.Type == integration.IntegrationTypeChirpStack {
			s.IntegrationClients[i.ID] = chirpstack.NewClient(i.URL, i.APIKey)
		}
		// Track highest ID for NextIDIntegration
		if i.ID >= s.NextIDIntegration {
			s.NextIDIntegration = i.ID + 1
		}
	}
	shared.DebugPrint("Integrations setup OK")
}

// setupTemplates initializes templates and loads defaults if empty
func (s *Simulator) setupTemplates() {
	if s.Templates == nil {
		s.Templates = make(map[int]*template.DeviceTemplate)
	}
	// Track highest ID for NextIDTemplate
	for _, t := range s.Templates {
		if t.ID >= s.NextIDTemplate {
			s.NextIDTemplate = t.ID + 1
		}
	}
	// Load defaults if no templates exist
	if len(s.Templates) == 0 {
		s.loadDefaultTemplates()
	}
	shared.DebugPrint("Templates setup OK")
}

// loadDefaultTemplates loads built-in default templates
func (s *Simulator) loadDefaultTemplates() {
	defaults := template.GetDefaultTemplates(func(name string) int {
		return dev.Codecs.GetCodecIDByName(name)
	})
	for _, t := range defaults {
		s.Templates[t.ID] = t
		if t.ID >= s.NextIDTemplate {
			s.NextIDTemplate = t.ID + 1
		}
	}
	shared.DebugPrint("Default templates loaded")
}

func (s *Simulator) searchName(Name string, Id int, gwFlag bool) (int, error) {

	for _, g := range s.Gateways {

		if g.Info.Name == Name {

			if (gwFlag && g.Id != Id) || !gwFlag {
				return codes.CodeErrorName, errors.New("Error: Name already used")
			}

		}

	}

	for _, d := range s.Devices {

		if d.Info.Name == Name {
			if (!gwFlag && d.Id != Id) || gwFlag {
				return codes.CodeErrorName, errors.New("Error: Name already used")
			}

		}

	}

	return codes.CodeOK, nil
}

func (s *Simulator) searchAddress(address lorawan.EUI64, Id int, gwFlag bool) (int, error) {

	for _, g := range s.Gateways {

		if g.Info.MACAddress == address {

			if (gwFlag && g.Id != Id) || !gwFlag {
				return codes.CodeErrorAddress, errors.New("Error: MAC Address already used")
			}

		}

	}

	for _, d := range s.Devices {

		if d.Info.DevEUI == address {

			if (!gwFlag && d.Id != Id) || gwFlag {
				return codes.CodeErrorAddress, errors.New("Error: DevEUI already used")
			}

		}

	}

	return codes.CodeOK, nil
}

// saveComponent saves a configuration of the provided interface to a JSON file
func (s *Simulator) saveComponent(path string, v interface{}) {
	shared.DebugPrint(fmt.Sprintf("Saving component %s on disk", path))
	bytes, err := json.MarshalIndent(&v, "", "\t")
	if err != nil {
		log.Fatal(err)
	}

	err = util.WriteConfigFile(path, bytes)
	if err != nil {
		log.Fatal(err)
	}

}

// saveStatus saves the simulator status, devices, gateways, integrations, and templates to JSON files
func (s *Simulator) saveStatus() {
	shared.DebugPrint("Saving status on disk")
	pathDir, err := util.GetPath()
	if err != nil {
		log.Fatal(err)
	}
	path := pathDir + "/simulator.json"
	s.saveComponent(path, &s)
	path = pathDir + "/devices.json"
	s.saveComponent(path, &s.Devices)
	path = pathDir + "/gateways.json"
	s.saveComponent(path, &s.Gateways)
	path = pathDir + "/integrations.json"
	s.saveComponent(path, &s.Integrations)
	path = pathDir + "/templates.json"
	s.saveComponent(path, &s.Templates)
}

// turnONDevice activates a device by adding it to the Forwarder and turning it on.
// If the scheduler is active, the device is scheduled instead of spawning a goroutine.
func (s *Simulator) turnONDevice(Id int) {
	infoDev := mfw.InfoDevice{
		DevEUI:   s.Devices[Id].Info.DevEUI,
		Location: s.Devices[Id].Info.Location,
		Range:    s.Devices[Id].Info.Configuration.Range,
	}
	s.Forwarder.AddDevice(infoDev)
	s.Devices[Id].Setup(&s.Resources, &s.Forwarder)
	s.Devices[Id].EventBroker = s.EventBroker

	if s.Scheduler != nil {
		s.Devices[Id].State = util.Running
		slog.Info("device scheduled", "component", "device", "dev_eui", s.Devices[Id].Info.DevEUI, "name", s.Devices[Id].Info.Name)
		s.Scheduler.Schedule(&scheduler.Job{
			ID:       s.Devices[Id].Id,
			Interval: s.Devices[Id].Info.Configuration.SendInterval,
			Execute:  s.Devices[Id].ExecuteOnce,
		})
	} else {
		s.Devices[Id].TurnON()
	}
}

// turnOFFDevice deactivates a device by removing it from the Forwarder and turning it off
func (s *Simulator) turnOFFDevice(Id int) {
	if s.Scheduler != nil {
		s.Scheduler.Remove(Id)
		s.Devices[Id].State = util.Stopped
		s.Forwarder.DeleteDevice(s.Devices[Id].Info.DevEUI)
		delete(s.ActiveDevices, Id)
	} else {
		s.ComponentsInactiveTmp++
		s.Resources.ExitGroup.Add(1)
		s.Devices[Id].TurnOFF()
		s.Forwarder.DeleteDevice(s.Devices[Id].Info.DevEUI)
		s.Resources.ExitGroup.Wait()
		delete(s.ActiveDevices, Id)
		s.ComponentsInactiveTmp--
	}
}

// turnONGateway activates a gateway by adding it to the Forwarder and turning it on
func (s *Simulator) turnONGateway(Id int) {
	s.Gateways[Id].Setup(&s.BridgeAddress, &s.Resources, &s.Forwarder)
	s.Gateways[Id].EventBroker = s.EventBroker
	infoGw := mfw.InfoGateway{
		MACAddress: s.Gateways[Id].Info.MACAddress,
		Buffer:     s.Gateways[Id].BufferUplink,
		Location:   s.Gateways[Id].Info.Location,
	}
	s.Forwarder.AddGateway(infoGw)
	s.Gateways[Id].TurnON()
}

// turnOFFGateway deactivates a gateway by removing it from the Forwarder and turning it off
func (s *Simulator) turnOFFGateway(Id int) {
	s.ComponentsInactiveTmp++
	s.Resources.ExitGroup.Add(1)
	s.Gateways[Id].TurnOFF()
	s.Resources.ExitGroup.Wait()
	delete(s.ActiveGateways, Id)
	s.ComponentsInactiveTmp--
	infoGw := mfw.InfoGateway{
		MACAddress: s.Gateways[Id].Info.MACAddress,
		Buffer:     s.Gateways[Id].BufferUplink,
		Location:   s.Gateways[Id].Info.Location,
	}
	s.Forwarder.DeleteGateway(infoGw)
}

// reset removes all devices and gateways from the ActiveDevices and ActiveGateways maps
func (s *Simulator) reset() {
	shared.DebugPrint("Resetting simulator")
	clear(s.ActiveDevices)
	clear(s.ActiveGateways)
	slog.Debug("simulator reset", "component", "simulator")
}

