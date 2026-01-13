package codec

import (
	"fmt"
	"os"
	"sync"
)

// Registry manages codecs and device states for the entire simulator
type Registry struct {
	executor *Executor
	library  *CodecLibrary
	states   map[string]*State // DevEUI -> State
	mu       sync.RWMutex
}

// NewRegistry creates a new codec registry
func NewRegistry(config *ExecutorConfig) *Registry {
	reg := &Registry{
		executor: NewExecutor(config),
		library:  NewCodecLibrary(),
		states:   make(map[string]*State),
	}

	// Load default codecs
	reg.library.LoadDefaults()

	return reg
}

// GetOrCreateState gets or creates a state for a device
func (r *Registry) GetOrCreateState(devEUI string) *State {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, exists := r.states[devEUI]
	if !exists {
		state = NewState(devEUI)
		r.states[devEUI] = state
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
func (r *Registry) EncodePayload(codecID int, devEUI string, device DeviceInterface) ([]byte, uint8, error) {
	// Get codec
	codec, err := r.library.Get(codecID)
	if err != nil {
		return nil, 1, fmt.Errorf("codec not found: %w", err)
	}

	// Get or create state
	state := r.GetOrCreateState(devEUI)

	// Execute encoding
	bytes, returnedFPort, err := r.executor.ExecuteEncode(codec.Script, state, device)
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
func (r *Registry) DecodePayload(codecID int, devEUI string, bytes []byte, fPort uint8, device DeviceInterface) error {
	// Get codec
	codec, err := r.library.Get(codecID)
	if err != nil {
		return fmt.Errorf("codec not found: %w", err)
	}

	// Get or create state
	state := r.GetOrCreateState(devEUI)

	// Execute decoding (for side effects only)
	if err := r.executor.ExecuteDecode(codec.Script, bytes, fPort, state, device); err != nil {
		return fmt.Errorf("decoding failed: %w", err)
	}

	return nil
}

// AddCodec adds a codec to the library
func (r *Registry) AddCodec(codec *Codec) error {
	return r.library.Add(codec)
}

// UpdateCodec updates an existing codec by ID
func (r *Registry) UpdateCodec(id int, name string, script string) error {
	return r.library.Update(id, name, script)
}

// GetCodec retrieves a codec by ID
func (r *Registry) GetCodec(id int) (*Codec, error) {
	return r.library.Get(id)
}

// RemoveCodec removes a codec from the library
func (r *Registry) RemoveCodec(id int) error {
	return r.library.Remove(id)
}

// ListCodecs returns all codec metadata
func (r *Registry) ListCodecs() []CodecMetadata {
	return r.library.List()
}

// GetCodecIDByName returns the ID of a codec by its name, or 0 if not found
func (r *Registry) GetCodecIDByName(name string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, meta := range r.library.List() {
		if meta.Name == name {
			return meta.ID
		}
	}
	return 0
}

// GetCodecCount returns the number of codecs in the library
func (r *Registry) GetCodecCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.library.Count()
}

// GetNextID returns the next ID that will be assigned
func (r *Registry) GetNextID() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.library.GetNextID()
}

// LoadDefaults loads default codecs into the library
func (r *Registry) LoadDefaults() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.library.LoadDefaults()
}

// Close closes the registry and releases resources
func (r *Registry) Close() {
	if r.executor != nil {
		r.executor.Close()
	}
}

// Save saves the codec library to a file
func (r *Registry) Save(filepath string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	data, err := r.library.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize codec library: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write codec library file: %w", err)
	}

	return nil
}

// Load loads the codec library from a file
// If the file doesn't exist or loading fails, it loads defaults instead
func (r *Registry) Load(filepath string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if file exists
	data, err := os.ReadFile(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, load defaults (library is already empty from NewRegistry)
			return nil
		}
		return fmt.Errorf("failed to read codec library file: %w", err)
	}

	// Try to load from file
	if err := r.library.FromJSON(data); err != nil {
		// If loading fails, return error (library remains empty from NewRegistry)
		return fmt.Errorf("failed to load codec library: %w", err)
	}

	return nil
}
