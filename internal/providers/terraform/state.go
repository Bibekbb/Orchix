package terraform

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/terraform/states/statefile"
	"github.com/hashicorp/terraform/states/statemgr"
)

// StateManager manages Terraform state
type StateManager struct {
	stateDir     string
	lockFile     string
	stateFile    string
	backendFile  string
	lock         sync.RWMutex
	stateLock    *StateLock
}

// NewStateManager creates a new state manager
func NewStateManager(stateDir string) *StateManager {
	return &StateManager{
		stateDir:    stateDir,
		lockFile:    filepath.Join(stateDir, ".terraform.lock.hcl"),
		stateFile:   filepath.Join(stateDir, "terraform.tfstate"),
		backendFile: filepath.Join(stateDir, "backend.tf"),
		stateLock:   NewStateLock(),
	}
}

// LockState locks the Terraform state
func (sm *StateManager) LockState(ctx context.Context, info *LockInfo) error {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	
	return sm.stateLock.Lock(ctx, info)
}

// UnlockState unlocks the Terraform state
func (sm *StateManager) UnlockState(ctx context.Context, lockID string) error {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	
	return sm.stateLock.Unlock(ctx, lockID)
}

// GetState gets the current state
func (sm *StateManager) GetState() (*State, error) {
	sm.lock.RLock()
	defer sm.lock.RUnlock()
	
	if _, err := os.Stat(sm.stateFile); os.IsNotExist(err) {
		return &State{
			Version: 4,
			Serial:  0,
			Lineage: generateLineage(),
			Modules: []ModuleState{},
		}, nil
	}
	
	data, err := os.ReadFile(sm.stateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}
	
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}
	
	return &state, nil
}

// WriteState writes the state
func (sm *StateManager) WriteState(state *State) error {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	
	// Ensure directory exists
	if err := os.MkdirAll(sm.stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}
	
	// Increment serial
	state.Serial++
	
	// Write state
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}
	
	tmpFile := sm.stateFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}
	
	// Atomic rename
	if err := os.Rename(tmpFile, sm.stateFile); err != nil {
		return fmt.Errorf("failed to rename state file: %w", err)
	}
	
	return nil
}

// RefreshState refreshes state from backend
func (sm *StateManager) RefreshState(ctx context.Context) error {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	
	// If using remote backend, refresh from remote
	if sm.hasRemoteBackend() {
		return sm.refreshFromRemote(ctx)
	}
	
	// Local state doesn't need refresh
	return nil
}

// BackupState creates a backup of the state
func (sm *StateManager) BackupState() (string, error) {
	sm.lock.RLock()
	defer sm.lock.RUnlock()
	
	if _, err := os.Stat(sm.stateFile); os.IsNotExist(err) {
		return "", nil // No state to backup
	}
	
	backupFile := fmt.Sprintf("%s.%s.backup", sm.stateFile, time.Now().Format("20060102150405"))
	
	data, err := os.ReadFile(sm.stateFile)
	if err != nil {
		return "", fmt.Errorf("failed to read state file: %w", err)
	}
	
	if err := os.WriteFile(backupFile, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write backup file: %w", err)
	}
	
	return backupFile, nil
}

// RestoreState restores state from backup
func (sm *StateManager) RestoreState(backupFile string) error {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", backupFile)
	}
	
	data, err := os.ReadFile(backupFile)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}
	
	// Validate backup
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("invalid backup file: %w", err)
	}
	
	// Write restored state
	if err := os.WriteFile(sm.stateFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write restored state: %w", err)
	}
	
	return nil
}

// ListBackups lists available backups
func (sm *StateManager) ListBackups() ([]string, error) {
	pattern := fmt.Sprintf("%s.*.backup", sm.stateFile)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list backups: %w", err)
	}
	
	return matches, nil
}

// GetStateInfo gets state information
func (sm *StateManager) GetStateInfo() (*StateInfo, error) {
	state, err := sm.GetState()
	if err != nil {
		return nil, err
	}
	
	info := &StateInfo{
		Version:    state.Version,
		Serial:     state.Serial,
		Lineage:    state.Lineage,
		Resources:  0,
		Backend:    sm.getBackendType(),
		LastUpdate: time.Now(), // This should be from state file metadata
	}
	
	// Count resources
	for _, module := range state.Modules {
		info.Resources += len(module.Resources)
	}
	
	return info, nil
}

// CleanupBackups cleans up old backups
func (sm *StateManager) CleanupBackups(maxAge time.Duration, maxCount int) error {
	backups, err := sm.ListBackups()
	if err != nil {
		return err
	}
	
	// Sort by modification time (newest first)
	backupInfo := make([]backupFileInfo, 0, len(backups))
	for _, backup := range backups {
		info, err := os.Stat(backup)
		if err != nil {
			continue
		}
		backupInfo = append(backupInfo, backupFileInfo{
			Path: backup,
			Time: info.ModTime(),
		})
	}
	
	// Sort by time (newest first)
	sort.Slice(backupInfo, func(i, j int) bool {
		return backupInfo[i].Time.After(backupInfo[j].Time)
	})
	
	// Remove old backups
	now := time.Now()
	removed := 0
	
	for i, backup := range backupInfo {
		shouldRemove := false
		
		// Remove if too old
		if now.Sub(backup.Time) > maxAge {
			shouldRemove = true
		}
		
		// Remove if too many backups
		if i >= maxCount {
			shouldRemove = true
		}
		
		if shouldRemove {
			if err := os.Remove(backup.Path); err != nil {
				fmt.Printf("Warning: failed to remove backup %s: %v\n", backup.Path, err)
			} else {
				removed++
			}
		}
	}
	
	if removed > 0 {
		fmt.Printf("Removed %d old backup(s)\n", removed)
	}
	
	return nil
}

// MigrateState migrates state to new format
func (sm *StateManager) MigrateState(toVersion int) error {
	state, err := sm.GetState()
	if err != nil {
		return err
	}
	
	// Check if migration is needed
	if state.Version >= toVersion {
		return nil
	}
	
	// Backup before migration
	backupFile, err := sm.BackupState()
	if err != nil {
		return fmt.Errorf("failed to backup before migration: %w", err)
	}
	
	fmt.Printf("Backed up state to: %s\n", backupFile)
	
	// Perform migration
	switch state.Version {
	case 3:
		// Migrate from v3 to v4
		state.Version = 4
		// Add any v3 to v4 migration logic here
		
	case 4:
		// Already at v4, nothing to do
		
	default:
		return fmt.Errorf("unsupported state version: %d", state.Version)
	}
	
	// Write migrated state
	if err := sm.WriteState(state); err != nil {
		// Try to restore from backup
		if restoreErr := sm.RestoreState(backupFile); restoreErr != nil {
			fmt.Printf("Warning: failed to restore from backup after migration error: %v\n", restoreErr)
		}
		return fmt.Errorf("failed to write migrated state: %w", err)
	}
	
	fmt.Printf("Successfully migrated state from version %d to %d\n", state.Version-1, state.Version)
	return nil
}

// Helper methods
func (sm *StateManager) hasRemoteBackend() bool {
	if _, err := os.Stat(sm.backendFile); os.IsNotExist(err) {
		return false
	}
	
	content, err := os.ReadFile(sm.backendFile)
	if err != nil {
		return false
	}
	
	return strings.Contains(string(content), "backend") && 
		!strings.Contains(string(content), `backend "local"`)
}

func (sm *StateManager) getBackendType() string {
	if !sm.hasRemoteBackend() {
		return "local"
	}
	
	content, err := os.ReadFile(sm.backendFile)
	if err != nil {
		return "unknown"
	}
	
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, `backend "`) {
			parts := strings.Split(line, `"`)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	
	return "unknown"
}

func (sm *StateManager) refreshFromRemote(ctx context.Context) error {
	// This would implement pulling state from remote backend
	// For now, it's a placeholder
	return nil
}

func generateLineage() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

// Types
type State struct {
	Version          int           `json:"version"`
	TerraformVersion string        `json:"terraform_version,omitempty"`
	Serial           int64         `json:"serial"`
	Lineage          string        `json:"lineage"`
	Outputs          map[string]OutputState `json:"outputs,omitempty"`
	Resources        []ResourceState `json:"resources,omitempty"` // Deprecated in v4
	Modules          []ModuleState  `json:"modules,omitempty"`
}

type OutputState struct {
	Value     interface{} `json:"value"`
	Type      interface{} `json:"type,omitempty"`
	Sensitive bool        `json:"sensitive,omitempty"`
}

type ModuleState struct {
	Path      []string                `json:"path"`
	Resources map[string]ResourceState `json:"resources,omitempty"`
	Outputs   map[string]OutputState  `json:"outputs,omitempty"`
}

type ResourceState struct {
	Type      string            `json:"type"`
	Name      string            `json:"name"`
	Provider  string            `json:"provider"`
	Instances []InstanceState   `json:"instances,omitempty"`
}

type InstanceState struct {
	IndexKey interface{}          `json:"index_key,omitempty"`
	Status   string              `json:"status,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

type StateInfo struct {
	Version    int       `json:"version"`
	Serial     int64     `json:"serial"`
	Lineage    string    `json:"lineage"`
	Resources  int       `json:"resources"`
	Backend    string    `json:"backend"`
	LastUpdate time.Time `json:"lastUpdate"`
}

type LockInfo struct {
	ID        string    `json:"id"`
	Operation string    `json:"operation"`
	Info      string    `json:"info"`
	Who       string    `json:"who"`
	Version   string    `json:"version"`
	Created   time.Time `json:"created"`
	Path      string    `json:"path"`
}

type backupFileInfo struct {
	Path string
	Time time.Time
}

// StateLock manages state locking
type StateLock struct {
	locks map[string]LockInfo
	mu    sync.RWMutex
}

func NewStateLock() *StateLock {
	return &StateLock{
		locks: make(map[string]LockInfo),
	}
}

func (sl *StateLock) Lock(ctx context.Context, info *LockInfo) error {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	
	// Generate lock ID if not provided
	if info.ID == "" {
		info.ID = generateLockID()
	}
	info.Created = time.Now()
	
	// Check if already locked
	for _, lock := range sl.locks {
		if lock.Path == info.Path && time.Since(lock.Created) < 30*time.Minute {
			return fmt.Errorf("state is already locked by %s (ID: %s)", lock.Who, lock.ID)
		}
	}
	
	sl.locks[info.ID] = *info
	return nil
}

func (sl *StateLock) Unlock(ctx context.Context, lockID string) error {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	
	if _, exists := sl.locks[lockID]; !exists {
		return fmt.Errorf("lock ID %s not found", lockID)
	}
	
	delete(sl.locks, lockID)
	return nil
}

func generateLockID() string {
	return fmt.Sprintf("lock-%d-%x", time.Now().Unix(), rand.Int63())
}

// StateFileManager provides file-based state management
type StateFileManager struct {
	statePath  string
	backupPath string
	lockPath   string
}

func NewStateFileManager(stateDir string) *StateFileManager {
	return &StateFileManager{
		statePath:  filepath.Join(stateDir, "terraform.tfstate"),
		backupPath: filepath.Join(stateDir, "backup"),
		lockPath:   filepath.Join(stateDir, ".terraform.lock.hcl"),
	}
}

func (sfm *StateFileManager) Read() (*statefile.File, error) {
	f, err := os.Open(sfm.statePath)
	if os.IsNotExist(err) {
		return &statefile.File{
			Lineage: generateLineage(),
			Serial:  0,
			State:   statemgr.NewState(),
		}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	
	return statefile.Read(f)
}

func (sfm *StateFileManager) Write(f *statefile.File) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(sfm.statePath), 0755); err != nil {
		return err
	}
	
	// Create backup
	if err := sfm.Backup(); err != nil {
		return err
	}
	
	// Write state
	w, err := os.Create(sfm.statePath)
	if err != nil {
		return err
	}
	defer w.Close()
	
	return statefile.Write(f, w)
}

func (sfm *StateFileManager) Backup() error {
	// Read current state
	f, err := sfm.Read()
	if err != nil {
		return err
	}
	
	// Create backup directory
	if err := os.MkdirAll(sfm.backupPath, 0755); err != nil {
		return err
	}
	
	// Create backup file
	backupFile := filepath.Join(sfm.backupPath, 
		fmt.Sprintf("terraform.tfstate.%s.backup", time.Now().Format("20060102150405")))
	
	w, err := os.Create(backupFile)
	if err != nil {
		return err
	}
	defer w.Close()
	
	return statefile.Write(f, w)
}