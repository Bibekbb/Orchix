package types

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Manifest is the root configuration object for Orchix
type Manifest struct {
	APIVersion  string                 `yaml:"apiVersion" json:"apiVersion"`
	AppName     string                 `yaml:"appName" json:"appName"`
	Target      string                 `yaml:"target" json:"target"`
	Description string                 `yaml:"description,omitempty" json:"description,omitempty"`
	Version     string                 `yaml:"version,omitempty" json:"version,omitempty"`
	Variables   map[string]interface{} `yaml:"variables,omitempty" json:"variables,omitempty"`
	Secrets     []SecretRef            `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	Components  []Component            `yaml:"components" json:"components"`
	Tags        map[string]string      `yaml:"tags,omitempty" json:"tags,omitempty"`
}

// SecretRef references a secret from an external secret manager
type SecretRef struct {
	Name     string            `yaml:"name" json:"name"`
	Source   string            `yaml:"source" json:"source"`
	Key      string            `yaml:"key,omitempty" json:"key,omitempty"`
	Path     string            `yaml:"path,omitempty" json:"path,omitempty"`
	Version  string            `yaml:"version,omitempty" json:"version,omitempty"`
	Metadata map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// Validate validates the manifest
func (m *Manifest) Validate() error {
	if m.APIVersion == "" {
		return fmt.Errorf("apiVersion is required")
	}
	if m.AppName == "" {
		return fmt.Errorf("appName is required")
	}
	if m.Target == "" {
		return fmt.Errorf("target is required")
	}
	if len(m.Components) == 0 {
		return fmt.Errorf("at least one component is required")
	}

	// Validate components
	componentIDs := make(map[string]bool)
	for i, component := range m.Components {
		if err := component.Validate(); err != nil {
			return fmt.Errorf("component %d (%s): %w", i, component.Name, err)
		}
		
		// Check for duplicate IDs
		if _, exists := componentIDs[component.ID]; exists {
			return fmt.Errorf("duplicate component ID: %s", component.ID)
		}
		componentIDs[component.ID] = true
	}

	// Validate dependencies exist
	for _, component := range m.Components {
		for _, depID := range component.DependsOn {
			if !componentIDs[depID] {
				return fmt.Errorf("component %s depends on non-existent component: %s", component.ID, depID)
			}
		}
	}

	return nil
}

// ExpandVariables expands variables in the manifest
func (m *Manifest) ExpandVariables(envVars map[string]string) error {
	// Merge manifest variables with environment variables
	allVars := make(map[string]interface{})
	for k, v := range m.Variables {
		allVars[k] = v
	}
	for k, v := range envVars {
		allVars[k] = v
	}

	// Expand variables recursively
	expanded, err := expandVariablesRecursive(allVars, allVars, []string{})
	if err != nil {
		return err
	}
	m.Variables = expanded

	// Expand variables in components
	for i := range m.Components {
		if err := m.Components[i].ExpandVariables(allVars); err != nil {
			return fmt.Errorf("component %s: %w", m.Components[i].ID, err)
		}
	}

	return nil
}

// expandVariablesRecursive expands variables recursively
func expandVariablesRecursive(value interface{}, vars map[string]interface{}, visited []string) (interface{}, error) {
	switch v := value.(type) {
	case string:
		return expandString(v, vars, visited)
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, val := range v {
			expanded, err := expandVariablesRecursive(val, vars, visited)
			if err != nil {
				return nil, err
			}
			result[key] = expanded
		}
		return result, nil
	case []interface{}:
		var result []interface{}
		for _, item := range v {
			expanded, err := expandVariablesRecursive(item, vars, visited)
			if err != nil {
				return nil, err
			}
			result = append(result, expanded)
		}
		return result, nil
	default:
		return value, nil
	}
}

// expandString expands variables in a string
func expandString(s string, vars map[string]interface{}, visited []string) (string, error) {
	if !strings.Contains(s, "${") {
		return s, nil
	}

	var result strings.Builder
	i := 0
	for i < len(s) {
		if strings.HasPrefix(s[i:], "${") {
			end := strings.Index(s[i:], "}")
			if end == -1 {
				return "", fmt.Errorf("unclosed variable reference in: %s", s[i:])
			}
			
			varName := s[i+2 : i+end]
			i += end + 1
			
			// Check for cycles
			for _, visitedVar := range visited {
				if visitedVar == varName {
					return "", fmt.Errorf("circular variable reference detected: %s", strings.Join(append(visited, varName), " -> "))
				}
			}
			
			value, exists := vars[varName]
			if !exists {
				return "", fmt.Errorf("undefined variable: %s", varName)
			}
			
			// Convert value to string
			var strVal string
			switch v := value.(type) {
			case string:
				strVal = v
			case int, int32, int64, float32, float64, bool:
				strVal = fmt.Sprintf("%v", v)
			default:
				return "", fmt.Errorf("cannot convert variable %s to string: %v", varName, value)
			}
			
			// Recursively expand the value
			expanded, err := expandString(strVal, vars, append(visited, varName))
			if err != nil {
				return "", err
			}
			
			result.WriteString(expanded)
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	
	return result.String(), nil
}

// FromFile loads a manifest from a file
func FromFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}

	return FromBytes(data, filepath.Ext(path))
}

// FromBytes loads a manifest from bytes
func FromBytes(data []byte, ext string) (*Manifest, error) {
	var manifest Manifest
	
	switch strings.ToLower(ext) {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &manifest); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
	default:
		// Try YAML first, then JSON
		if err := yaml.Unmarshal(data, &manifest); err != nil {
			if err := json.Unmarshal(data, &manifest); err != nil {
				return nil, fmt.Errorf("failed to parse as YAML or JSON")
			}
		}
	}

	if err := manifest.Validate(); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// ToYAML converts manifest to YAML
func (m *Manifest) ToYAML() (string, error) {
	data, err := yaml.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ToJSON converts manifest to JSON
func (m *Manifest) ToJSON() (string, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetComponentByID returns a component by its ID
func (m *Manifest) GetComponentByID(id string) *Component {
	for _, comp := range m.Components {
		if comp.ID == id {
			return &comp
		}
	}
	return nil
}

// GetComponentDependencies returns all dependencies for a component
func (m *Manifest) GetComponentDependencies(id string) []Component {
	var deps []Component
	visited := make(map[string]bool)
	
	var collectDeps func(string)
	collectDeps = func(compID string) {
		comp := m.GetComponentByID(compID)
		if comp == nil || visited[compID] {
			return
		}
		
		visited[compID] = true
		for _, depID := range comp.DependsOn {
			depComp := m.GetComponentByID(depID)
			if depComp != nil && !visited[depID] {
				deps = append(deps, *depComp)
				collectDeps(depID)
			}
		}
	}
	
	collectDeps(id)
	return deps
}

// Merge merges another manifest into this one
func (m *Manifest) Merge(other *Manifest) error {
	// Merge components
	for _, otherComp := range other.Components {
		found := false
		for i, comp := range m.Components {
			if comp.ID == otherComp.ID {
				m.Components[i] = otherComp
				found = true
				break
			}
		}
		if !found {
			m.Components = append(m.Components, otherComp)
		}
	}
	
	// Merge variables
	if m.Variables == nil {
		m.Variables = make(map[string]interface{})
	}
	for k, v := range other.Variables {
		m.Variables[k] = v
	}
	
	// Merge secrets
	m.Secrets = append(m.Secrets, other.Secrets...)
	
	// Merge tags
	if m.Tags == nil {
		m.Tags = make(map[string]string)
	}
	for k, v := range other.Tags {
		m.Tags[k] = v
	}
	
	return nil
}