package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Bibekbb/Orchix/pkg/types"
)

// State represents the deployment state
type State struct {
	AppName    string                    `json:"appName"`
	Target     string                    `json:"target"`
	Version    string                    `json:"version"`
	DeployedAt time.Time                 `json:"deployedAt"`
	Components map[string]ComponentState `json:"components"`
}

// ComponentState tracks individual component state
type ComponentState struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Status     string    `json:"status"`
	DeployedAt time.Time `json:"deployedAt"`
}

// StateManager handles state persistence
type StateManager struct {
	stateDir string
}

// NewStateManager creates a new state manager
func NewStateManager() *StateManager {
	return &StateManager{
		stateDir: ".orchix",
	}
}

// SaveState saves the current deployment state
func (sm *StateManager) SaveState(manifest *types.Manifest) error {
	// Create state directory
	if err := os.MkdirAll(sm.stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	state := State{
		AppName:    manifest.AppName,
		Target:     manifest.Target,
		Version:    "1.0",
		DeployedAt: time.Now(),
		Components: make(map[string]ComponentState),
	}

	for _, comp := range manifest.Components {
		state.Components[comp.ID] = ComponentState{
			ID:         comp.ID,
			Name:       comp.Name,
			Type:       string(comp.Type),
			Status:     "deployed",
			DeployedAt: time.Now(),
		}
	}

	stateFile := filepath.Join(sm.stateDir, "state.json")
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(stateFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// LoadState loads the deployment state
func (sm *StateManager) LoadState() (*State, error) {
	stateFile := filepath.Join(sm.stateDir, "state.json")

	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No state file yet
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return &state, nil
}

// ClearState removes the state file
func (sm *StateManager) ClearState() error {
	stateFile := filepath.Join(sm.stateDir, "state.json")
	if err := os.Remove(stateFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear state: %w", err)
	}
	return nil
}
