package codec

import (
	"crypto/sha256"
	"encoding/hex"
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
	ID     string `json:"id"`     // Unique identifier (hash of script)
	Name   string `json:"name"`   // Human-readable name
	Script string `json:"script"` // JavaScript code
}

// CodecMetadata holds metadata about a codec without the script
type CodecMetadata struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// NewCodec creates a new codec with auto-generated ID
func NewCodec(name, script string) *Codec {
	codec := &Codec{
		Name:   name,
		Script: script,
	}
	codec.ID = codec.generateID()
	return codec
}

// generateID creates a unique ID based on script hash
func (c *Codec) generateID() string {
	hash := sha256.Sum256([]byte(c.Script + c.Name))
	return hex.EncodeToString(hash[:])[:16] // Use first 16 chars
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
	codecs map[string]*Codec // ID -> Codec
}

// NewCodecLibrary creates a new codec library
func NewCodecLibrary() *CodecLibrary {
	return &CodecLibrary{
		codecs: make(map[string]*Codec),
	}
}

// Add adds a codec to the library
func (cl *CodecLibrary) Add(codec *Codec) error {
	if err := codec.Validate(); err != nil {
		return err
	}
	cl.codecs[codec.ID] = codec
	return nil
}

// Update updates an existing codec by ID, preserving the original ID
func (cl *CodecLibrary) Update(id string, name string, script string) error {
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
func (cl *CodecLibrary) Get(id string) (*Codec, error) {
	codec, exists := cl.codecs[id]
	if !exists {
		return nil, ErrCodecNotFound
	}
	return codec.Clone(), nil
}

// Remove removes a codec from the library
func (cl *CodecLibrary) Remove(id string) error {
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
	cl.codecs = make(map[string]*Codec)
}

// LoadDefaults loads default example codecs
func (cl *CodecLibrary) LoadDefaults() {
	// Milesight AM319 Environmental Sensor Codec
	am319Codec := NewCodec("Milesight AM319", CreateAM319Codec())
	cl.Add(am319Codec)

	// Enginko MCF-LW13IO I/O Controller Codec
	mcflw13ioCodec := NewCodec("Enginko MCF-LW13IO", CreateMCFLW13IOCodec())
	cl.Add(mcflw13ioCodec)
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
	for _, codec := range codecs {
		if err := cl.Add(codec); err != nil {
			return fmt.Errorf("failed to add codec %s: %w", codec.Name, err)
		}
	}

	return nil
}
