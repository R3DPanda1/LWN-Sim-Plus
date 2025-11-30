package codec

import (
	"sync"
	"time"
)

// State holds the runtime state for a device's codec execution
// This includes counters, variables, and message history
type State struct {
	DevEUI         string                 `json:"devEUI"`
	Counters       map[string]int64       `json:"counters"`
	Variables      map[string]interface{} `json:"variables"`
	MessageHistory []MessageRecord        `json:"messageHistory"`
	CreatedAt      time.Time              `json:"createdAt"`
	UpdatedAt      time.Time              `json:"updatedAt"`
	mu             sync.RWMutex           `json:"-"`
}

// MessageRecord represents a single message in the history
type MessageRecord struct {
	FCnt      uint32    `json:"fcnt"`
	Timestamp time.Time `json:"timestamp"`
	Payload   []byte    `json:"payload"`
	FPort     uint8     `json:"fport"`
}

// MaxMessageHistory is the maximum number of messages to keep in history
const MaxMessageHistory = 100

// NewState creates a new State instance for a device
func NewState(devEUI string) *State {
	now := time.Now()
	return &State{
		DevEUI:         devEUI,
		Counters:       make(map[string]int64),
		Variables:      make(map[string]interface{}),
		MessageHistory: make([]MessageRecord, 0, MaxMessageHistory),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// GetCounter returns the value of a counter (0 if not set)
func (s *State) GetCounter(name string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Counters[name]
}

// SetCounter sets the value of a counter
func (s *State) SetCounter(name string, value int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Counters[name] = value
	s.UpdatedAt = time.Now()
}

// GetVariable returns the value of a variable (nil if not set)
func (s *State) GetVariable(name string) interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Variables[name]
}

// SetVariable sets the value of a variable
func (s *State) SetVariable(name string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Variables[name] = value
	s.UpdatedAt = time.Now()
}

// AddMessage adds a message to the history (circular buffer)
func (s *State) AddMessage(msg MessageRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.MessageHistory = append(s.MessageHistory, msg)

	// Keep only the last MaxMessageHistory messages
	if len(s.MessageHistory) > MaxMessageHistory {
		s.MessageHistory = s.MessageHistory[len(s.MessageHistory)-MaxMessageHistory:]
	}

	s.UpdatedAt = time.Now()
}

// GetPreviousPayload returns the last payload sent (nil if none)
func (s *State) GetPreviousPayload() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.MessageHistory) == 0 {
		return nil
	}

	return s.MessageHistory[len(s.MessageHistory)-1].Payload
}

// GetPreviousPayloads returns the last n payloads (newest first)
func (s *State) GetPreviousPayloads(n int) [][]byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.MessageHistory) == 0 {
		return [][]byte{}
	}

	// Limit n to available history
	if n > len(s.MessageHistory) {
		n = len(s.MessageHistory)
	}

	payloads := make([][]byte, n)
	start := len(s.MessageHistory) - n

	for i := 0; i < n; i++ {
		payloads[i] = s.MessageHistory[start+i].Payload
	}

	return payloads
}

// Reset clears all state (counters, variables, history)
func (s *State) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Counters = make(map[string]int64)
	s.Variables = make(map[string]interface{})
	s.MessageHistory = make([]MessageRecord, 0, MaxMessageHistory)
	s.UpdatedAt = time.Now()
}

// Clone creates a deep copy of the state
func (s *State) Clone() *State {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clone := &State{
		DevEUI:         s.DevEUI,
		Counters:       make(map[string]int64, len(s.Counters)),
		Variables:      make(map[string]interface{}, len(s.Variables)),
		MessageHistory: make([]MessageRecord, len(s.MessageHistory)),
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
	}

	// Copy counters
	for k, v := range s.Counters {
		clone.Counters[k] = v
	}

	// Copy variables (shallow copy)
	for k, v := range s.Variables {
		clone.Variables[k] = v
	}

	// Copy message history
	copy(clone.MessageHistory, s.MessageHistory)

	return clone
}
