package types

import (
	"fmt"
	"time"
)

// Component represents a deployable unit in Orchix
type Component struct {
	ID          string                 `yaml:"id" json:"id"`
	Name        string                 `yaml:"name" json:"name"`
	Type        ComponentType          `yaml:"type" json:"type"`
	Source      string                 `yaml:"source" json:"source"`
	DependsOn   []string               `yaml:"dependsOn,omitempty" json:"dependsOn,omitempty"`
	Variables   map[string]interface{} `yaml:"variables,omitempty" json:"variables,omitempty"`
	HealthCheck *HealthCheck           `yaml:"healthCheck,omitempty" json:"healthCheck,omitempty"`
	Timeout     time.Duration          `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Retry       *RetryConfig           `yaml:"retry,omitempty" json:"retry,omitempty"`
	Metadata    map[string]interface{} `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// ComponentType defines the type of component
type ComponentType string

const (
	ComponentTypeTerraform    ComponentType = "terraform"
	ComponentTypeKubernetes   ComponentType = "kubernetes"
	ComponentTypeDocker       ComponentType = "docker"
	ComponentTypeDockerCompose ComponentType = "docker-compose"
	ComponentTypeHelm         ComponentType = "helm"
	ComponentTypeCloud        ComponentType = "cloud"
	ComponentTypeScript       ComponentType = "script"
	ComponentTypeCustom       ComponentType = "custom"
)

// HealthCheck configuration for component health verification
type HealthCheck struct {
	Type        string        `yaml:"type" json:"type"`
	Endpoint    string        `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Command     string        `yaml:"command,omitempty" json:"command,omitempty"`
	Interval    time.Duration `yaml:"interval,omitempty" json:"interval,omitempty"`
	Timeout     time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Retries     int           `yaml:"retries,omitempty" json:"retries,omitempty"`
	SuccessCode int           `yaml:"successCode,omitempty" json:"successCode,omitempty"`
	Headers     map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

// RetryConfig configuration for retrying failed operations
type RetryConfig struct {
	MaxAttempts int           `yaml:"maxAttempts" json:"maxAttempts"`
	Delay       time.Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
	MaxDelay    time.Duration `yaml:"maxDelay,omitempty" json:"maxDelay,omitempty"`
	BackoffRate float64       `yaml:"backoffRate,omitempty" json:"backoffRate,omitempty"`
}

// Validate validates the component configuration
func (c *Component) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("component ID is required")
	}
	if c.Name == "" {
		return fmt.Errorf("component name is required")
	}
	if c.Type == "" {
		return fmt.Errorf("component type is required")
	}
	if c.Source == "" {
		return fmt.Errorf("component source is required")
	}

	// Validate component type
	switch c.Type {
	case ComponentTypeTerraform, ComponentTypeKubernetes, ComponentTypeDocker,
		ComponentTypeDockerCompose, ComponentTypeHelm, ComponentTypeCloud,
		ComponentTypeScript, ComponentTypeCustom:
		// Valid types
	default:
		return fmt.Errorf("invalid component type: %s", c.Type)
	}

	// Validate health check if present
	if c.HealthCheck != nil {
		if err := c.HealthCheck.Validate(); err != nil {
			return fmt.Errorf("health check validation failed: %w", err)
		}
	}

	// Validate retry config if present
	if c.Retry != nil {
		if err := c.Retry.Validate(); err != nil {
			return fmt.Errorf("retry config validation failed: %w", err)
		}
	}

	return nil
}

// Validate validates the health check configuration
func (hc *HealthCheck) Validate() error {
	if hc.Type == "" {
		return fmt.Errorf("health check type is required")
	}
	
	switch hc.Type {
	case "http", "https":
		if hc.Endpoint == "" {
			return fmt.Errorf("endpoint is required for HTTP health check")
		}
	case "tcp":
		if hc.Endpoint == "" {
			return fmt.Errorf("endpoint is required for TCP health check")
		}
	case "command":
		if hc.Command == "" {
			return fmt.Errorf("command is required for command health check")
		}
	default:
		return fmt.Errorf("invalid health check type: %s", hc.Type)
	}

	if hc.Interval <= 0 {
		hc.Interval = 5 * time.Second
	}
	if hc.Timeout <= 0 {
		hc.Timeout = 30 * time.Second
	}
	if hc.Retries < 0 {
		hc.Retries = 0
	}
	if hc.SuccessCode == 0 {
		hc.SuccessCode = 200
	}

	return nil
}

// Validate validates the retry configuration
func (rc *RetryConfig) Validate() error {
	if rc.MaxAttempts < 1 {
		return fmt.Errorf("maxAttempts must be at least 1")
	}
	if rc.Delay < 0 {
		return fmt.Errorf("delay cannot be negative")
	}
	if rc.MaxDelay < 0 {
		return fmt.Errorf("maxDelay cannot be negative")
	}
	if rc.BackoffRate < 1.0 {
		rc.BackoffRate = 1.0
	}

	return nil
}

// ExpandVariables expands variables in the component
func (c *Component) ExpandVariables(vars map[string]interface{}) error {
	// Expand variables in component variables
	expandedVars, err := expandComponentVariables(c.Variables, vars)
	if err != nil {
		return err
	}
	c.Variables = expandedVars

	// Expand variables in health check if present
	if c.HealthCheck != nil {
		expandedEndpoint, err := expandString(c.HealthCheck.Endpoint, vars, []string{})
		if err != nil {
			return err
		}
		c.HealthCheck.Endpoint = expandedEndpoint

		expandedCommand, err := expandString(c.HealthCheck.Command, vars, []string{})
		if err != nil {
			return err
		}
		c.HealthCheck.Command = expandedCommand
	}

	// Expand variables in source
	expandedSource, err := expandString(c.Source, vars, []string{})
	if err != nil {
		return err
	}
	c.Source = expandedSource

	return nil
}

// expandComponentVariables expands variables in component variables
func expandComponentVariables(vars map[string]interface{}, parentVars map[string]interface{}) (map[string]interface{}, error) {
	if vars == nil {
		return nil, nil
	}

	result := make(map[string]interface{})
	for key, value := range vars {
		expanded, err := expandVariableValue(value, parentVars)
		if err != nil {
			return nil, fmt.Errorf("variable %s: %w", key, err)
		}
		result[key] = expanded
	}

	return result, nil
}

// expandVariableValue expands variables in a value
func expandVariableValue(value interface{}, parentVars map[string]interface{}) (interface{}, error) {
	switch v := value.(type) {
	case string:
		return expandString(v, parentVars, []string{})
	case map[string]interface{}:
		return expandComponentVariables(v, parentVars)
	case []interface{}:
		var result []interface{}
		for _, item := range v {
			expanded, err := expandVariableValue(item, parentVars)
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

// GetVariable gets a variable value from the component
func (c *Component) GetVariable(key string, defaultValue interface{}) interface{} {
	if c.Variables == nil {
		return defaultValue
	}
	
	value, exists := c.Variables[key]
	if !exists {
		return defaultValue
	}
	
	return value
}

// SetVariable sets a variable value in the component
func (c *Component) SetVariable(key string, value interface{}) {
	if c.Variables == nil {
		c.Variables = make(map[string]interface{})
	}
	c.Variables[key] = value
}

// HasDependency checks if component has a specific dependency
func (c *Component) HasDependency(depID string) bool {
	for _, id := range c.DependsOn {
		if id == depID {
			return true
		}
	}
	return false
}

// AddDependency adds a dependency to the component
func (c *Component) AddDependency(depID string) {
	if !c.HasDependency(depID) {
		c.DependsOn = append(c.DependsOn, depID)
	}
}

// RemoveDependency removes a dependency from the component
func (c *Component) RemoveDependency(depID string) {
	var newDeps []string
	for _, id := range c.DependsOn {
		if id != depID {
			newDeps = append(newDeps, id)
		}
	}
	c.DependsOn = newDeps
}

// Clone creates a deep copy of the component
func (c *Component) Clone() *Component {
	clone := *c
	
	// Deep copy slices and maps
	if c.DependsOn != nil {
		clone.DependsOn = make([]string, len(c.DependsOn))
		copy(clone.DependsOn, c.DependsOn)
	}
	
	if c.Variables != nil {
		clone.Variables = make(map[string]interface{})
		for k, v := range c.Variables {
			clone.Variables[k] = v
		}
	}
	
	if c.HealthCheck != nil {
		clone.HealthCheck = &HealthCheck{}
		*clone.HealthCheck = *c.HealthCheck
		if c.HealthCheck.Headers != nil {
			clone.HealthCheck.Headers = make(map[string]string)
			for k, v := range c.HealthCheck.Headers {
				clone.HealthCheck.Headers[k] = v
			}
		}
	}
	
	if c.Retry != nil {
		clone.Retry = &RetryConfig{}
		*clone.Retry = *c.Retry
	}
	
	if c.Metadata != nil {
		clone.Metadata = make(map[string]interface{})
		for k, v := range c.Metadata {
			clone.Metadata[k] = v
		}
	}
	
	return &clone
}