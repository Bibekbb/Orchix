package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ConfigManager handles configuration loading, validation, and management
type ConfigManager struct {
	GlobalConfig *GlobalConfig
	Manifest     *ManifestSchema
	validator    *validator.Validate
}

// NewConfigManager creates a new configuration manager
func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		GlobalConfig: DefaultGlobalConfig(),
		Manifest:     DefaultManifest(),
		validator:    validator.New(),
	}
}

// LoadGlobalConfig loads global configuration from file
func (cm *ConfigManager) LoadGlobalConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var config GlobalConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config YAML: %w", err)
	}

	// Validate the configuration
	result, err := ValidateGlobalConfig(&config)
	if err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	if !result.Valid {
		return fmt.Errorf("configuration validation failed: %v", result.Errors)
	}

	cm.GlobalConfig = &config
	return nil
}

// LoadManifest loads a manifest from file
func (cm *ConfigManager) LoadManifest(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read manifest file: %w", err)
	}

	var manifest ManifestSchema
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest YAML: %w", err)
	}

	// Validate the manifest
	result, err := ValidateManifest(&manifest)
	if err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	if !result.Valid {
		return fmt.Errorf("manifest validation failed: %v", result.Errors)
	}

	cm.Manifest = &manifest
	return nil
}

// SaveGlobalConfig saves global configuration to file
func (cm *ConfigManager) SaveGlobalConfig(path string) error {
	data, err := yaml.Marshal(cm.GlobalConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// SaveManifest saves manifest to file
func (cm *ConfigManager) SaveManifest(path string) error {
	data, err := yaml.Marshal(cm.Manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest file: %w", err)
	}

	return nil
}

// GetComponent returns a component by ID
func (cm *ConfigManager) GetComponent(id string) (*ComponentSchema, error) {
	for _, comp := range cm.Manifest.Components {
		if comp.ID == id {
			return &comp, nil
		}
	}
	return nil, fmt.Errorf("component not found: %s", id)
}

// UpdateComponent updates a component by ID
func (cm *ConfigManager) UpdateComponent(id string, updates *ComponentSchema) error {
	for i, comp := range cm.Manifest.Components {
		if comp.ID == id {
			// Validate the updated component
			result, err := ValidateComponent(updates)
			if err != nil {
				return fmt.Errorf("validation error: %w", err)
			}
			if !result.Valid {
				return fmt.Errorf("component validation failed: %v", result.Errors)
			}

			cm.Manifest.Components[i] = *updates
			return nil
		}
	}
	return fmt.Errorf("component not found: %s", id)
}

// AddComponent adds a new component to the manifest
func (cm *ConfigManager) AddComponent(component *ComponentSchema) error {
	// Validate the component
	result, err := ValidateComponent(component)
	if err != nil {
		return fmt.Errorf("validation error: %w", err)
	}
	if !result.Valid {
		return fmt.Errorf("component validation failed: %v", result.Errors)
	}

	// Check for duplicate ID
	for _, comp := range cm.Manifest.Components {
		if comp.ID == component.ID {
			return fmt.Errorf("component with ID %s already exists", component.ID)
		}
	}

	cm.Manifest.Components = append(cm.Manifest.Components, *component)
	return nil
}

// RemoveComponent removes a component by ID
func (cm *ConfigManager) RemoveComponent(id string) error {
	for i, comp := range cm.Manifest.Components {
		if comp.ID == id {
			// Check if any other components depend on this one
			for _, otherComp := range cm.Manifest.Components {
				for _, dep := range otherComp.DependsOn {
					if dep == id {
						return fmt.Errorf("cannot remove component %s: component %s depends on it", 
							id, otherComp.ID)
					}
				}
			}

			// Remove the component
			cm.Manifest.Components = append(
				cm.Manifest.Components[:i],
				cm.Manifest.Components[i+1:]...,
			)
			return nil
		}
	}
	return fmt.Errorf("component not found: %s", id)
}

// GetVariable returns a variable value
func (cm *ConfigManager) GetVariable(name string) (interface{}, bool) {
	value, exists := cm.Manifest.Variables[name]
	return value, exists
}

// SetVariable sets a variable value
func (cm *ConfigManager) SetVariable(name string, value interface{}) {
	if cm.Manifest.Variables == nil {
		cm.Manifest.Variables = make(map[string]interface{})
	}
	cm.Manifest.Variables[name] = value
}

// ToJSON converts configuration to JSON
func (cm *ConfigManager) ToJSON() (string, error) {
	data := struct {
		GlobalConfig *GlobalConfig   `json:"globalConfig"`
		Manifest     *ManifestSchema `json:"manifest"`
	}{
		GlobalConfig: cm.GlobalConfig,
		Manifest:     cm.Manifest,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal to JSON: %w", err)
	}

	return string(jsonData), nil
}

// ToYAML converts configuration to YAML
func (cm *ConfigManager) ToYAML() (string, error) {
	yamlData, err := yaml.Marshal(cm.Manifest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal to YAML: %w", err)
	}

	return string(yamlData), nil
}

// ValidateAll validates both global config and manifest
func (cm *ConfigManager) ValidateAll() (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:   true,
		Errors:  []ValidationError{},
		Warnings: []string{},
	}

	// Validate global config
	globalResult, err := ValidateGlobalConfig(cm.GlobalConfig)
	if err != nil {
		return nil, fmt.Errorf("global config validation error: %w", err)
	}
	if !globalResult.Valid {
		result.Valid = false
		result.Errors = append(result.Errors, globalResult.Errors...)
	}
	result.Warnings = append(result.Warnings, globalResult.Warnings...)

	// Validate manifest
	manifestResult, err := ValidateManifest(cm.Manifest)
	if err != nil {
		return nil, fmt.Errorf("manifest validation error: %w", err)
	}
	if !manifestResult.Valid {
		result.Valid = false
		result.Errors = append(result.Errors, manifestResult.Errors...)
	}
	result.Warnings = append(result.Warnings, manifestResult.Warnings...)

	return result, nil
}