package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/R3DPanda1/LWN-Sim-Plus/integration/chirpstack"
)

// Manager handles integration operations
type Manager struct {
	integrations map[string]*Integration
	clients      map[string]*chirpstack.Client
	mu           sync.RWMutex
}

// NewManager creates a new integration manager
func NewManager() *Manager {
	return &Manager{
		integrations: make(map[string]*Integration),
		clients:      make(map[string]*chirpstack.Client),
	}
}

// Add adds a new integration
func (m *Manager) Add(integration *Integration) error {
	if err := integration.Validate(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.integrations[integration.ID] = integration

	// Create client for ChirpStack integrations
	if integration.Type == IntegrationTypeChirpStack {
		m.clients[integration.ID] = chirpstack.NewClient(integration.URL, integration.APIKey)
	}

	return nil
}

// Get retrieves an integration by ID
func (m *Manager) Get(id string) (*Integration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	integration, exists := m.integrations[id]
	if !exists {
		return nil, ErrIntegrationNotFound
	}
	return integration.Clone(), nil
}

// Update updates an existing integration
func (m *Manager) Update(id, name, url, apiKey, tenantID, appID string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.integrations[id]
	if !exists {
		return ErrIntegrationNotFound
	}

	existing.Name = name
	existing.URL = normalizeURL(url)
	existing.APIKey = apiKey
	existing.TenantID = tenantID
	existing.ApplicationID = appID
	existing.Enabled = enabled

	if err := existing.Validate(); err != nil {
		return err
	}

	// Update client
	if existing.Type == IntegrationTypeChirpStack {
		m.clients[id] = chirpstack.NewClient(existing.URL, existing.APIKey)
	}

	return nil
}

// Remove removes an integration
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.integrations[id]; !exists {
		return ErrIntegrationNotFound
	}

	delete(m.integrations, id)
	delete(m.clients, id)
	return nil
}

// List returns all integrations without API keys
func (m *Manager) List() []*Integration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Integration, 0, len(m.integrations))
	for _, i := range m.integrations {
		result = append(result, i.PublicCopy())
	}
	return result
}

// TestConnection tests connection to an integration
func (m *Manager) TestConnection(id string) error {
	m.mu.RLock()
	integration, exists := m.integrations[id]
	client := m.clients[id]
	m.mu.RUnlock()

	if !exists {
		return ErrIntegrationNotFound
	}

	return client.TestConnection(integration.TenantID)
}

// GetClient returns the ChirpStack client for an integration
func (m *Manager) GetClient(id string) (*chirpstack.Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, exists := m.clients[id]
	if !exists {
		return nil, ErrIntegrationNotFound
	}
	return client, nil
}

// GetDeviceProfiles returns device profiles for an integration
func (m *Manager) GetDeviceProfiles(id string) ([]chirpstack.DeviceProfile, error) {
	m.mu.RLock()
	integration, exists := m.integrations[id]
	client := m.clients[id]
	m.mu.RUnlock()

	if !exists {
		return nil, ErrIntegrationNotFound
	}

	return client.ListDeviceProfiles(integration.TenantID, 100)
}

// ProvisionDevice creates a device in ChirpStack
func (m *Manager) ProvisionDevice(integrationID, devEUI, name, deviceProfileID, appKey string) error {
	m.mu.RLock()
	integration, exists := m.integrations[integrationID]
	client := m.clients[integrationID]
	m.mu.RUnlock()

	if !exists {
		return ErrIntegrationNotFound
	}

	if !integration.Enabled {
		return fmt.Errorf("integration is disabled")
	}

	// Create device
	device := &chirpstack.Device{
		DevEUI:          devEUI,
		Name:            name,
		ApplicationID:   integration.ApplicationID,
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

// DeleteDevice removes a device from ChirpStack
func (m *Manager) DeleteDevice(integrationID, devEUI string) error {
	m.mu.RLock()
	integration, exists := m.integrations[integrationID]
	client := m.clients[integrationID]
	m.mu.RUnlock()

	if !exists {
		return ErrIntegrationNotFound
	}

	if !integration.Enabled {
		return nil // Silently skip if integration is disabled
	}

	return client.DeleteDevice(devEUI)
}

// Save saves integrations to file
func (m *Manager) Save(filepath string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	integrations := make([]*Integration, 0, len(m.integrations))
	for _, i := range m.integrations {
		integrations = append(integrations, i)
	}

	data, err := json.MarshalIndent(integrations, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath, data, 0644)
}

// Load loads integrations from file
func (m *Manager) Load(filepath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No file yet, that's okay
		}
		return err
	}

	var integrations []*Integration
	if err := json.Unmarshal(data, &integrations); err != nil {
		return err
	}

	m.integrations = make(map[string]*Integration)
	m.clients = make(map[string]*chirpstack.Client)

	for _, i := range integrations {
		m.integrations[i.ID] = i
		if i.Type == IntegrationTypeChirpStack {
			m.clients[i.ID] = chirpstack.NewClient(i.URL, i.APIKey)
		}
	}

	return nil
}
