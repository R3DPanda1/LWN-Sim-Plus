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
