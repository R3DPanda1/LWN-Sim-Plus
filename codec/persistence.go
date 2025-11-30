package codec

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

// StatePersistence handles saving and loading device states
type StatePersistence struct {
	directory string
	mu        sync.RWMutex
}

// NewStatePersistence creates a new state persistence manager
func NewStatePersistence(directory string) *StatePersistence {
	return &StatePersistence{
		directory: directory,
	}
}

// SaveState saves a device state to disk
func (sp *StatePersistence) SaveState(state *State) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	// Create directory if it doesn't exist
	if err := os.MkdirAll(sp.directory, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Serialize state to JSON
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write to file (DevEUI.json)
	filename := filepath.Join(sp.directory, fmt.Sprintf("%s.json", state.DevEUI))
	if err := ioutil.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// LoadState loads a device state from disk
func (sp *StatePersistence) LoadState(devEUI string) (*State, error) {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	filename := filepath.Join(sp.directory, fmt.Sprintf("%s.json", devEUI))

	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, fmt.Errorf("state file not found for device %s", devEUI)
	}

	// Read file
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	// Deserialize state
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return &state, nil
}

// DeleteState removes a device state from disk
func (sp *StatePersistence) DeleteState(devEUI string) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	filename := filepath.Join(sp.directory, fmt.Sprintf("%s.json", devEUI))

	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete state file: %w", err)
	}

	return nil
}

// ListStates returns all device EUIs with saved states
func (sp *StatePersistence) ListStates() ([]string, error) {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	// Check if directory exists
	if _, err := os.Stat(sp.directory); os.IsNotExist(err) {
		return []string{}, nil
	}

	// Read directory
	files, err := ioutil.ReadDir(sp.directory)
	if err != nil {
		return nil, fmt.Errorf("failed to read state directory: %w", err)
	}

	// Extract DevEUIs from filenames
	devEUIs := make([]string, 0)
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			devEUI := file.Name()[:len(file.Name())-5] // Remove .json
			devEUIs = append(devEUIs, devEUI)
		}
	}

	return devEUIs, nil
}

// SaveAllStates saves all states from a manager
func (sp *StatePersistence) SaveAllStates(manager *Manager) error {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	for _, state := range manager.states {
		if err := sp.SaveState(state); err != nil {
			return fmt.Errorf("failed to save state for %s: %w", state.DevEUI, err)
		}
	}

	return nil
}

// LoadAllStates loads all states into a manager
func (sp *StatePersistence) LoadAllStates(manager *Manager) error {
	devEUIs, err := sp.ListStates()
	if err != nil {
		return err
	}

	for _, devEUI := range devEUIs {
		state, err := sp.LoadState(devEUI)
		if err != nil {
			return fmt.Errorf("failed to load state for %s: %w", devEUI, err)
		}

		manager.mu.Lock()
		manager.states[devEUI] = state
		manager.mu.Unlock()
	}

	return nil
}

// ClearAll removes all state files
func (sp *StatePersistence) ClearAll() error {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if err := os.RemoveAll(sp.directory); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear state directory: %w", err)
	}

	return nil
}
