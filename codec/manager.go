package codec

import (
	"fmt"
	"sync"

	"github.com/brocaar/lorawan"
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

// GetState gets a device state (returns nil if not found)
func (m *Manager) GetState(devEUI string) *State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.states[devEUI]
}

// RemoveState removes a device state
func (m *Manager) RemoveState(devEUI string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.states, devEUI)
}

// EncodePayload encodes a payload using a codec
// Parameters:
//   - codecID: ID of the codec to use
//   - devEUI: Device EUI for state management
//   - fPort: LoRaWAN fPort
//   - obj: Object to encode
//
// Returns the encoded bytes and any error
func (m *Manager) EncodePayload(codecID string, devEUI string, fPort uint8, obj map[string]interface{}) ([]byte, error) {
	// Get codec
	codec, err := m.library.Get(codecID)
	if err != nil {
		return nil, fmt.Errorf("codec not found: %w", err)
	}

	// Get or create state
	state := m.GetOrCreateState(devEUI)

	// Execute encoding
	bytes, err := m.executor.ExecuteEncode(codec.Script, fPort, obj, state)
	if err != nil {
		return nil, fmt.Errorf("encoding failed: %w", err)
	}

	return bytes, nil
}

// DecodePayload decodes a payload using a codec
// Parameters:
//   - codecID: ID of the codec to use
//   - devEUI: Device EUI for state management
//   - fPort: LoRaWAN fPort
//   - bytes: Bytes to decode
//
// Returns the decoded object and any error
func (m *Manager) DecodePayload(codecID string, devEUI string, fPort uint8, bytes []byte) (map[string]interface{}, error) {
	// Get codec
	codec, err := m.library.Get(codecID)
	if err != nil {
		return nil, fmt.Errorf("codec not found: %w", err)
	}

	// Get or create state
	state := m.GetOrCreateState(devEUI)

	// Execute decoding
	obj, err := m.executor.ExecuteDecode(codec.Script, fPort, bytes, state)
	if err != nil {
		return nil, fmt.Errorf("decoding failed: %w", err)
	}

	return obj, nil
}

// AddCodec adds a codec to the library
func (m *Manager) AddCodec(codec *Codec) error {
	return m.library.Add(codec)
}

// GetCodec retrieves a codec by ID
func (m *Manager) GetCodec(id string) (*Codec, error) {
	return m.library.Get(id)
}

// GetCodecByName retrieves a codec by name
func (m *Manager) GetCodecByName(name string) (*Codec, error) {
	return m.library.GetByName(name)
}

// RemoveCodec removes a codec from the library
func (m *Manager) RemoveCodec(id string) error {
	return m.library.Remove(id)
}

// ListCodecs returns all codec metadata
func (m *Manager) ListCodecs() []CodecMetadata {
	return m.library.List()
}

// GetMetrics returns executor metrics
func (m *Manager) GetMetrics() ExecutorMetrics {
	return m.executor.GetMetrics()
}

// ResetMetrics resets executor metrics
func (m *Manager) ResetMetrics() {
	m.executor.ResetMetrics()
}

// Close closes the manager and releases resources
func (m *Manager) Close() {
	if m.executor != nil {
		m.executor.Close()
	}
}

// GeneratePayloadFromConfig generates a payload from device configuration
// This is a helper function that converts PayloadConfig to a lorawan.Payload
func (m *Manager) GeneratePayloadFromConfig(codecID string, devEUI string, fPort uint8, payloadConfig map[string]interface{}) (lorawan.Payload, error) {
	// Encode using codec
	bytes, err := m.EncodePayload(codecID, devEUI, fPort, payloadConfig)
	if err != nil {
		return nil, err
	}

	// Convert to lorawan.DataPayload
	dataPayload := lorawan.DataPayload{
		Bytes: bytes,
	}

	return &dataPayload, nil
}
