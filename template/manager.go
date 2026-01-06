package template

import (
	"encoding/json"
	"os"
	"sync"
)

// Manager handles template operations
type Manager struct {
	templates map[string]*DeviceTemplate
	mu        sync.RWMutex
}

// NewManager creates a new template manager
func NewManager() *Manager {
	return &Manager{
		templates: make(map[string]*DeviceTemplate),
	}
}

// Add adds a new template
func (m *Manager) Add(template *DeviceTemplate) error {
	if err := template.Validate(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate ID if not set
	if template.ID == "" {
		template.RegenerateID()
	}

	m.templates[template.ID] = template
	return nil
}

// Get retrieves a template by ID
func (m *Manager) Get(id string) (*DeviceTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	template, exists := m.templates[id]
	if !exists {
		return nil, ErrTemplateNotFound
	}
	return template.Clone(), nil
}

// Update updates an existing template
func (m *Manager) Update(template *DeviceTemplate) error {
	if err := template.Validate(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.templates[template.ID]; !exists {
		return ErrTemplateNotFound
	}

	m.templates[template.ID] = template
	return nil
}

// Remove removes a template
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.templates[id]; !exists {
		return ErrTemplateNotFound
	}

	delete(m.templates, id)
	return nil
}

// List returns all templates
func (m *Manager) List() []*DeviceTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*DeviceTemplate, 0, len(m.templates))
	for _, t := range m.templates {
		result = append(result, t.Clone())
	}
	return result
}

// Save saves templates to file
func (m *Manager) Save(filepath string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	templates := make([]*DeviceTemplate, 0, len(m.templates))
	for _, t := range m.templates {
		templates = append(templates, t)
	}

	data, err := json.MarshalIndent(templates, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath, data, 0644)
}

// Load loads templates from file
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

	var templates []*DeviceTemplate
	if err := json.Unmarshal(data, &templates); err != nil {
		return err
	}

	m.templates = make(map[string]*DeviceTemplate)
	for _, t := range templates {
		m.templates[t.ID] = t
	}

	return nil
}

// Count returns the number of templates
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.templates)
}

// LoadDefaults loads default templates for common device types
// codecLookup is an optional function to resolve codec names to IDs
func (m *Manager) LoadDefaults(codecLookup func(name string) string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Milesight AM319 Environmental Sensor Template
	am319 := &DeviceTemplate{
		Name:              "Milesight AM319",
		Region:            1, // EU868
		SupportedClassB:   false,
		SupportedClassC:   false,
		SupportedADR:      true,
		Range:             5000, // 5km typical indoor/urban
		DataRate:          5,    // SF7 - good for indoor sensors
		RX1DROffset:       0,
		SendInterval:      300, // 5 minutes - typical for environmental sensors
		AckTimeout:        2,
		RX1Delay:          1000,
		RX1Duration:       3000,
		RX2Delay:          2000,
		RX2Duration:       3000,
		RX2Frequency:      869525000,
		RX2DataRate:       0,
		FPort:             85, // Milesight uses fPort 85
		NbRetransmission:  1,
		MType:             0, // Unconfirmed
		SupportedFragment: false,
		UseCodec:          true,
	}
	am319.ID = am319.generateID()

	// Look up codec ID if lookup function provided
	if codecLookup != nil {
		am319.CodecID = codecLookup("Milesight AM319")
	}

	m.templates[am319.ID] = am319

	// Enginko MCF-LW13IO I/O Controller Template
	mcfio := &DeviceTemplate{
		Name:              "Enginko MCF-LW13IO",
		Region:            1, // EU868
		SupportedClassB:   false,
		SupportedClassC:   true, // Class C for immediate downlink response
		SupportedADR:      true,
		Range:             3000, // 3km - typically installed in fixed locations
		DataRate:          5,    // SF7
		RX1DROffset:       0,
		SendInterval:      60, // 1 minute - I/O controllers report frequently
		AckTimeout:        2,
		RX1Delay:          1000,
		RX1Duration:       3000,
		RX2Delay:          2000,
		RX2Duration:       3000,
		RX2Frequency:      869525000,
		RX2DataRate:       0,
		FPort:             2, // Enginko uses fPort 2
		NbRetransmission:  1,
		MType:             0, // Unconfirmed
		SupportedFragment: false,
		UseCodec:          true,
	}
	mcfio.ID = mcfio.generateID()

	// Look up codec ID if lookup function provided
	if codecLookup != nil {
		mcfio.CodecID = codecLookup("Enginko MCF-LW13IO")
	}

	m.templates[mcfio.ID] = mcfio
}
