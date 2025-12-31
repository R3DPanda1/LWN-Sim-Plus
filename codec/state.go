package codec

import (
	"sync"
	"time"
)

// State holds the runtime state for a device's codec execution
type State struct {
	DevEUI    string                 `json:"devEUI"`
	Variables map[string]interface{} `json:"variables"`
	CreatedAt time.Time              `json:"createdAt"`
	UpdatedAt time.Time              `json:"updatedAt"`
	mu        sync.RWMutex           `json:"-"`
}

// NewState creates a new State instance for a device
func NewState(devEUI string) *State {
	now := time.Now()
	return &State{
		DevEUI:    devEUI,
		Variables: make(map[string]interface{}),
		CreatedAt: now,
		UpdatedAt: now,
	}
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
