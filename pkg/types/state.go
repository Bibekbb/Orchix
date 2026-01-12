package types

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// State represents the deployment state of an Orchix application
type State struct {
	AppName       string                 `json:"appName"`
	Target        string                 `json:"target"`
	Version       string                 `json:"version"`
	DeploymentID  string                 `json:"deploymentId"`
	Status        StateStatus            `json:"status"`
	Components    []ComponentState       `json:"components"`
	Outputs       map[string]interface{} `json:"outputs,omitempty"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt     time.Time              `json:"createdAt"`
	UpdatedAt     time.Time              `json:"updatedAt"`
	CompletedAt   *time.Time             `json:"completedAt,omitempty"`
	Duration      time.Duration          `json:"duration,omitempty"`
	Error         *StateError            `json:"error,omitempty"`
	Lock          *StateLock             `json:"lock,omitempty"`
	mu            sync.RWMutex           `json:"-"`
}

// StateStatus represents the status of a deployment state
type StateStatus string

const (
	StateStatusPending    StateStatus = "pending"
	StateStatusDeploying  StateStatus = "deploying"
	StateStatusDeployed   StateStatus = "deployed"
	StateStatusFailed     StateStatus = "failed"
	StateStatusDestroying StateStatus = "destroying"
	StateStatusDestroyed  StateStatus = "destroyed"
	StateStatusRolledBack StateStatus = "rolled-back"
)

// ComponentState represents the state of a single component
type ComponentState struct {
	ComponentID string                 `json:"componentId"`
	Name        string                 `json:"name"`
	Type        ComponentType          `json:"type"`
	Status      ComponentStatus        `json:"status"`
	Health      HealthStatus           `json:"health"`
	Outputs     map[string]interface{} `json:"outputs,omitempty"`
	Resources   []Resource             `json:"resources,omitempty"`
	AppliedAt   time.Time              `json:"appliedAt"`
	Duration    time.Duration          `json:"duration"`
	Error       *StateError            `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// StateError represents an error in the state
type StateError struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
	Time    time.Time              `json:"time"`
}

// StateLock represents a lock on the state
type StateLock struct {
	ID        string    `json:"id"`
	Operation string    `json:"operation"`
	Who       string    `json:"who"`
	Version   string    `json:"version"`
	Created   time.Time `json:"created"`
	Info      string    `json:"info,omitempty"`
}

// NewState creates a new state object
func NewState(appName, target, version string) *State {
	deploymentID := generateDeploymentID()
	return &State{
		AppName:      appName,
		Target:       target,
		Version:      version,
		DeploymentID: deploymentID,
		Status:       StateStatusPending,
		Components:   []ComponentState{},
		Outputs:      make(map[string]interface{}),
		Variables:    make(map[string]interface{}),
		Metadata:     make(map[string]interface{}),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// generateDeploymentID generates a unique deployment ID
func generateDeploymentID() string {
	return fmt.Sprintf("dep-%d-%s", 
		time.Now().Unix(), 
		randomString(6))
}

// randomString generates a random string
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	bytes := make([]byte, n)
	for i := range bytes {
		// Simple pseudo-random for demo purposes
		// In production, use crypto/rand
		bytes[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(bytes)
}

// Lock locks the state for writing
func (s *State) Lock(operation, who, version, info string) error {
	s.mu.Lock()
	
	if s.Lock != nil {
		s.mu.Unlock()
		return fmt.Errorf("state is already locked by %s for operation %s", 
			s.Lock.Who, s.Lock.Operation)
	}
	
	s.Lock = &StateLock{
		ID:        generateLockID(),
		Operation: operation,
		Who:       who,
		Version:   version,
		Created:   time.Now(),
		Info:      info,
	}
	
	s.UpdatedAt = time.Now()
	return nil
}

// Unlock unlocks the state
func (s *State) Unlock() {
	if s.Lock != nil {
		s.Lock = nil
		s.UpdatedAt = time.Now()
	}
	s.mu.Unlock()
}

// generateLockID generates a unique lock ID
func generateLockID() string {
	return fmt.Sprintf("lock-%d-%s", 
		time.Now().UnixNano(), 
		randomString(8))
}

// UpdateComponent updates or adds a component state
func (s *State) UpdateComponent(compState ComponentState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for i, cs := range s.Components {
		if cs.ComponentID == compState.ComponentID {
			s.Components[i] = compState
			s.UpdatedAt = time.Now()
			return nil
		}
	}
	
	s.Components = append(s.Components, compState)
	s.UpdatedAt = time.Now()
	return nil
}

// GetComponent returns a component state by ID
func (s *State) GetComponent(componentID string) (*ComponentState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	for _, cs := range s.Components {
		if cs.ComponentID == componentID {
			return &cs, true
		}
	}
	return nil, false
}

// RemoveComponent removes a component state
func (s *State) RemoveComponent(componentID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for i, cs := range s.Components {
		if cs.ComponentID == componentID {
			s.Components = append(s.Components[:i], s.Components[i+1:]...)
			s.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// SetOutput sets an output value
func (s *State) SetOutput(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.Outputs == nil {
		s.Outputs = make(map[string]interface{})
	}
	s.Outputs[key] = value
	s.UpdatedAt = time.Now()
}

// GetOutput gets an output value
func (s *State) GetOutput(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.Outputs == nil {
		return nil, false
	}
	value, exists := s.Outputs[key]
	return value, exists
}

// SetVariable sets a variable value
func (s *State) SetVariable(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.Variables == nil {
		s.Variables = make(map[string]interface{})
	}
	s.Variables[key] = value
	s.UpdatedAt = time.Now()
}

// GetVariable gets a variable value
func (s *State) GetVariable(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.Variables == nil {
		return nil, false
	}
	value, exists := s.Variables[key]
	return value, exists
}

// SetMetadata sets metadata value
func (s *State) SetMetadata(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.Metadata == nil {
		s.Metadata = make(map[string]interface{})
	}
	s.Metadata[key] = value
	s.UpdatedAt = time.Now()
}

// GetMetadata gets metadata value
func (s *State) GetMetadata(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.Metadata == nil {
		return nil, false
	}
	value, exists := s.Metadata[key]
	return value, exists
}

// SetError sets an error on the state
func (s *State) SetError(code, message string, details map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.Error = &StateError{
		Code:    code,
		Message: message,
		Details: details,
		Time:    time.Now(),
	}
	s.Status = StateStatusFailed
	s.UpdatedAt = time.Now()
}

// ClearError clears the error from the state
func (s *State) ClearError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.Error = nil
	s.UpdatedAt = time.Now()
}

// MarkDeployed marks the state as deployed
func (s *State) MarkDeployed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.Status = StateStatusDeployed
	now := time.Now()
	s.CompletedAt = &now
	s.Duration = now.Sub(s.CreatedAt)
	s.UpdatedAt = now
}

// MarkDestroyed marks the state as destroyed
func (s *State) MarkDestroyed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.Status = StateStatusDestroyed
	now := time.Now()
	s.CompletedAt = &now
	s.Duration = now.Sub(s.CreatedAt)
	s.UpdatedAt = now
}

// MarkDeploying marks the state as deploying
func (s *State) MarkDeploying() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.Status = StateStatusDeploying
	s.UpdatedAt = time.Now()
}

// MarkDestroying marks the state as destroying
func (s *State) MarkDestroying() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.Status = StateStatusDestroying
	s.UpdatedAt = time.Now()
}

// Save saves the state to a file
func (s *State) Save(path string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}
	
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}
	
	return nil
}

// Load loads the state from a file
func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}
	
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}
	
	return &state, nil
}

// Clone creates a deep copy of the state
func (s *State) Clone() *State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	clone := *s
	
	// Deep copy slices and maps
	if s.Components != nil {
		clone.Components = make([]ComponentState, len(s.Components))
		copy(clone.Components, s.Components)
	}
	
	if s.Outputs != nil {
		clone.Outputs = make(map[string]interface{})
		for k, v := range s.Outputs {
			clone.Outputs[k] = v
		}
	}
	
	if s.Variables != nil {
		clone.Variables = make(map[string]interface{})
		for k, v := range s.Variables {
			clone.Variables[k] = v
		}
	}
	
	if s.Metadata != nil {
		clone.Metadata = make(map[string]interface{})
		for k, v := range s.Metadata {
			clone.Metadata[k] = v
		}
	}
	
	if s.Error != nil {
		clone.Error = &StateError{}
		*clone.Error = *s.Error
		if s.Error.Details != nil {
			clone.Error.Details = make(map[string]interface{})
			for k, v := range s.Error.Details {
				clone.Error.Details[k] = v
			}
		}
	}
	
	if s.Lock != nil {
		clone.Lock = &StateLock{}
		*clone.Lock = *s.Lock
	}
	
	if s.CompletedAt != nil {
		completedAt := *s.CompletedAt
		clone.CompletedAt = &completedAt
	}
	
	return &clone
}

// Validate validates the state
func (s *State) Validate() error {
	if s.AppName == "" {
		return fmt.Errorf("appName is required")
	}
	if s.Target == "" {
		return fmt.Errorf("target is required")
	}
	if s.DeploymentID == "" {
		return fmt.Errorf("deploymentId is required")
	}
	if s.CreatedAt.IsZero() {
		return fmt.Errorf("createdAt is required")
	}
	
	return nil
}

// ToJSON converts the state to JSON
func (s *State) ToJSON() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetComponentStatus returns the status of a specific component
func (s *State) GetComponentStatus(componentID string) (ComponentStatus, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	for _, cs := range s.Components {
		if cs.ComponentID == componentID {
			return cs.Status, true
		}
	}
	return "", false
}

// AllComponentsStatus returns true if all components have the specified status
func (s *State) AllComponentsStatus(status ComponentStatus) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if len(s.Components) == 0 {
		return false
	}
	
	for _, cs := range s.Components {
		if cs.Status != status {
			return false
		}
	}
	return true
}

// HasComponent returns true if the state contains the component
func (s *State) HasComponent(componentID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	for _, cs := range s.Components {
		if cs.ComponentID == componentID {
			return true
		}
	}
	return false
}

// GetDeploymentAge returns the age of the deployment
func (s *State) GetDeploymentAge() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return time.Since(s.CreatedAt)
}