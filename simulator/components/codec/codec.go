package codec

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrCodecNotFound is returned when a codec is not found
	ErrCodecNotFound = errors.New("codec not found")
	// ErrInvalidCodecFormat is returned when codec validation fails
	ErrInvalidCodecFormat = errors.New("invalid codec format")
)

// Codec represents a JavaScript codec for encoding/decoding device payloads
// Compatible with ChirpStack codec format
type Codec struct {
	ID     int    `json:"id"`     // Unique identifier (sequential)
	Name   string `json:"name"`   // Human-readable name
	Script string `json:"script"` // JavaScript code
}

// CodecMetadata holds metadata about a codec without the script
type CodecMetadata struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// NewCodec creates a new codec (ID must be set by the registry)
func NewCodec(name, script string) *Codec {
	return &Codec{
		Name:   name,
		Script: script,
	}
}

// Validate checks if the codec is valid
func (c *Codec) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidCodecFormat)
	}
	if c.Script == "" {
		return fmt.Errorf("%w: script is required", ErrInvalidCodecFormat)
	}

	// Check if script contains OnUplink function (required)
	// OnDownlink is optional
	hasOnUplink := strings.Contains(c.Script, "function OnUplink")

	if !hasOnUplink {
		return fmt.Errorf("%w: script must contain OnUplink function (OnDownlink is optional)", ErrInvalidCodecFormat)
	}

	return nil
}

// Metadata returns metadata without the script
func (c *Codec) Metadata() CodecMetadata {
	return CodecMetadata{
		ID:   c.ID,
		Name: c.Name,
	}
}

// Clone creates a deep copy of the codec
func (c *Codec) Clone() *Codec {
	return &Codec{
		ID:     c.ID,
		Name:   c.Name,
		Script: c.Script,
	}
}

// CodecLibrary manages a collection of codecs
type CodecLibrary struct {
	codecs map[int]*Codec // ID -> Codec
	nextID int            // Next ID to assign
}

// NewCodecLibrary creates a new codec library
func NewCodecLibrary() *CodecLibrary {
	return &CodecLibrary{
		codecs: make(map[int]*Codec),
		nextID: 1,
	}
}

// Add adds a codec to the library with the next available ID
func (cl *CodecLibrary) Add(codec *Codec) error {
	if err := codec.Validate(); err != nil {
		return err
	}
	if codec.ID == 0 {
		codec.ID = cl.nextID
		cl.nextID++
	} else if codec.ID >= cl.nextID {
		cl.nextID = codec.ID + 1
	}
	cl.codecs[codec.ID] = codec
	return nil
}

// Update updates an existing codec by ID, preserving the original ID
func (cl *CodecLibrary) Update(id int, name string, script string) error {
	// Check if codec exists
	if _, exists := cl.codecs[id]; !exists {
		return ErrCodecNotFound
	}

	// Create codec with new data but preserve the original ID
	updatedCodec := &Codec{
		ID:     id, // Preserve original ID
		Name:   name,
		Script: script,
	}

	// Validate the updated codec
	if err := updatedCodec.Validate(); err != nil {
		return err
	}

	// Update in the library
	cl.codecs[id] = updatedCodec
	return nil
}

// Get retrieves a codec by ID
func (cl *CodecLibrary) Get(id int) (*Codec, error) {
	codec, exists := cl.codecs[id]
	if !exists {
		return nil, ErrCodecNotFound
	}
	return codec.Clone(), nil
}

// Remove removes a codec from the library
func (cl *CodecLibrary) Remove(id int) error {
	if _, exists := cl.codecs[id]; !exists {
		return ErrCodecNotFound
	}
	delete(cl.codecs, id)
	return nil
}

// List returns all codec metadata
func (cl *CodecLibrary) List() []CodecMetadata {
	metadata := make([]CodecMetadata, 0, len(cl.codecs))
	for _, codec := range cl.codecs {
		metadata = append(metadata, codec.Metadata())
	}
	return metadata
}

// Count returns the number of codecs
func (cl *CodecLibrary) Count() int {
	return len(cl.codecs)
}

// Clear removes all codecs
func (cl *CodecLibrary) Clear() {
	cl.codecs = make(map[int]*Codec)
	cl.nextID = 1
}

// GetNextID returns the next ID that will be assigned
func (cl *CodecLibrary) GetNextID() int {
	return cl.nextID
}

// SetNextID sets the next ID to assign
func (cl *CodecLibrary) SetNextID(id int) {
	cl.nextID = id
}

// LoadDefaults loads default example codecs with sequential IDs
func (cl *CodecLibrary) LoadDefaults() {
	// Milesight AM319 Environmental Sensor Codec (ID: 1)
	am319Codec := NewCodec("Milesight AM319", CreateAM319Codec())
	cl.Add(am319Codec)

	// Enginko MCF-LW13IO I/O Controller Codec (ID: 2)
	mcflw13ioCodec := NewCodec("Enginko MCF-LW13IO", CreateMCFLW13IOCodec())
	cl.Add(mcflw13ioCodec)

	// Eastron SDM230 Energy Meter Codec (ID: 3)
	sdm230Codec := NewCodec("Eastron SDM230", CreateSDM230Codec())
	cl.Add(sdm230Codec)
}

// ToJSON serializes the codec library to JSON
func (cl *CodecLibrary) ToJSON() ([]byte, error) {
	// Convert map to slice for JSON serialization
	codecs := make([]*Codec, 0, len(cl.codecs))
	for _, codec := range cl.codecs {
		codecs = append(codecs, codec)
	}
	return json.MarshalIndent(codecs, "", "  ")
}

// FromJSON deserializes a codec library from JSON
func (cl *CodecLibrary) FromJSON(data []byte) error {
	var codecs []*Codec
	if err := json.Unmarshal(data, &codecs); err != nil {
		return fmt.Errorf("failed to unmarshal codec library: %w", err)
	}

	// Clear existing codecs and add new ones
	cl.Clear()
	maxID := 0
	for _, codec := range codecs {
		if codec.ID > maxID {
			maxID = codec.ID
		}
		if err := cl.Add(codec); err != nil {
			return fmt.Errorf("failed to add codec %s: %w", codec.Name, err)
		}
	}
	// Set nextID to be one more than the highest ID found
	if maxID >= cl.nextID {
		cl.nextID = maxID + 1
	}

	return nil
}
