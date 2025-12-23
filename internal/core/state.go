package core

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/Bibekbb/Orchix/pkg/types"
)

type StateManager interface {
	GetComponentState(id string) (*types.ComponentState, error)
	SetComponentState(id string, state types.ComponentState) error
	GetAllStates() (map[string]types.ComponentState, error)
}

type FileStateManager struct {
	filePath string
	states   map[string]types.ComponentState
	mu       sync.RWMutex
}

func NewFileStateManager(filePath string) *FileStateManager {
	fsm := &FileStateManager{
		filePath: filePath,
		states:   make(map[string]types.ComponentState),
	}
	fsm.load()
	return fsm
}

func (fsm *FileStateManager) GetComponentState(id string) (*types.ComponentState, error) {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()

	if state, exists := fsm.states[id]; exists {
		return &state, nil
	}
	return nil, nil
}

func (fsm *FileStateManager) SetComponentState(id string, state types.ComponentState) error {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()

	fsm.states[id] = state
	return fsm.save()
}

func (fsm *FileStateManager) GetAllStates() (map[string]types.ComponentState, error) {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()

	states := make(map[string]types.ComponentState)
	for k, v := range fsm.states {
		states[k] = v
	}
	return states, nil
}

func (fsm *FileStateManager) load() error {
	if _, err := os.Stat(fsm.filePath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(fsm.filePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &fsm.states)
}

func (fsm *FileStateManager) save() error {
	data, err := json.MarshalIndent(fsm.states, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(fsm.filePath, data, 0644)
}
