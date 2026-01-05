package codec

import (
	"fmt"
	"os"
	"sync"
)

// Manager manages codecs and device states for the entire simulator
type Manager struct {
	executor *Executor
	library  *CodecLibrary
	states   map[string]*State // DevEUI -> State
	mu       sync.RWMutex
}

// NewManager creates a new codec manager
func NewManager(config *ExecutorConfig) *Manager {
	mgr := &Manager{
		executor: NewExecutor(config),
		library:  NewCodecLibrary(),
		states:   make(map[string]*State),
	}

	// Load default codecs
	mgr.library.LoadDefaults()

	return mgr
}

// GetOrCreateState gets or creates a state for a device
func (m *Manager) GetOrCreateState(devEUI string) *State {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[devEUI]
	if !exists {
		state = NewState(devEUI)
		m.states[devEUI] = state
	}

	return state
}

// EncodePayload encodes a payload using a codec
// Parameters:
//   - codecID: ID of the codec to use
//   - devEUI: Device EUI for state management
//   - device: Device interface for accessing configuration (send interval, etc.)
//
// Returns the encoded bytes, actual fPort (from codec or device), and any error
func (m *Manager) EncodePayload(codecID string, devEUI string, device DeviceInterface) ([]byte, uint8, error) {
	// Get codec
	codec, err := m.library.Get(codecID)
	if err != nil {
		return nil, 1, fmt.Errorf("codec not found: %w", err)
	}

	// Get or create state
	state := m.GetOrCreateState(devEUI)

	// Execute encoding
	bytes, returnedFPort, err := m.executor.ExecuteEncode(codec.Script, state, device)
	if err != nil {
		return nil, 1, fmt.Errorf("encoding failed: %w", err)
	}

	return bytes, returnedFPort, nil
}

// DecodePayload executes the OnDownlink function from a codec
// Parameters:
//   - codecID: ID of the codec to use
//   - devEUI: Device EUI for state management
//   - bytes: Bytes to decode
//   - fPort: LoRaWAN fPort
//   - device: Device interface for accessing configuration
//
// OnDownlink is executed for its side effects (log, setState, setSendInterval).
func (m *Manager) DecodePayload(codecID string, devEUI string, bytes []byte, fPort uint8, device DeviceInterface) error {
	// Get codec
	codec, err := m.library.Get(codecID)
	if err != nil {
		return fmt.Errorf("codec not found: %w", err)
	}

	// Get or create state
	state := m.GetOrCreateState(devEUI)

	// Execute decoding (for side effects only)
	if err := m.executor.ExecuteDecode(codec.Script, bytes, fPort, state, device); err != nil {
		return fmt.Errorf("decoding failed: %w", err)
	}

	return nil
}

// AddCodec adds a codec to the library
func (m *Manager) AddCodec(codec *Codec) error {
	return m.library.Add(codec)
}

// UpdateCodec updates an existing codec by ID
func (m *Manager) UpdateCodec(id string, name string, script string) error {
	return m.library.Update(id, name, script)
}

// GetCodec retrieves a codec by ID
func (m *Manager) GetCodec(id string) (*Codec, error) {
	return m.library.Get(id)
}

// RemoveCodec removes a codec from the library
func (m *Manager) RemoveCodec(id string) error {
	return m.library.Remove(id)
}

// ListCodecs returns all codec metadata
func (m *Manager) ListCodecs() []CodecMetadata {
	return m.library.List()
}

// GetCodecIDByName returns the ID of a codec by its name, or empty string if not found
func (m *Manager) GetCodecIDByName(name string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, meta := range m.library.List() {
		if meta.Name == name {
			return meta.ID
		}
	}
	return ""
}

// GetCodecCount returns the number of codecs in the library
func (m *Manager) GetCodecCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.library.Count()
}

// LoadDefaults loads default codecs into the library
func (m *Manager) LoadDefaults() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.library.LoadDefaults()
}

// Close closes the manager and releases resources
func (m *Manager) Close() {
	if m.executor != nil {
		m.executor.Close()
	}
}

// SaveCodecLibrary saves the codec library to a file
func (m *Manager) SaveCodecLibrary(filepath string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := m.library.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize codec library: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write codec library file: %w", err)
	}

	return nil
}

// LoadCodecLibrary loads the codec library from a file
// If the file doesn't exist or loading fails, it loads defaults instead
func (m *Manager) LoadCodecLibrary(filepath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if file exists
	data, err := os.ReadFile(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, load defaults (library is already empty from NewManager)
			return nil
		}
		return fmt.Errorf("failed to read codec library file: %w", err)
	}

	// Try to load from file
	if err := m.library.FromJSON(data); err != nil {
		// If loading fails, return error (library remains empty from NewManager)
		return fmt.Errorf("failed to load codec library: %w", err)
	}

	return nil
}
