package core

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Bibekbb/Orchix/pkg/types"
)

// Validator validates Orchix manifests
type Validator struct {
	manifest *types.Manifest
	errors   []ValidationError
	warnings []ValidationWarning
}

// ValidationError represents a validation error
type ValidationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
	Value   any    `json:"value,omitempty"`
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

// NewValidator creates a new validator instance
func NewValidator(manifest *types.Manifest) *Validator {
	return &Validator{
		manifest: manifest,
		errors:   []ValidationError{},
		warnings: []ValidationWarning{},
	}
}

// Validate validates the entire manifest
func (v *Validator) Validate() (bool, []ValidationError, []ValidationWarning) {
	v.validateManifest()
	v.validateComponents()
	v.validateDependencies()
	v.validateVariables()
	
	return len(v.errors) == 0, v.errors, v.warnings
}

// ValidateStrict validates and returns error if any validation fails
func (v *Validator) ValidateStrict() error {
	valid, errors, warnings := v.Validate()
	
	if !valid {
		errorMessages := make([]string, len(errors))
		for i, err := range errors {
			errorMessages[i] = fmt.Sprintf("%s: %s", err.Field, err.Message)
		}
		return fmt.Errorf("validation failed: %s", strings.Join(errorMessages, ", "))
	}
	
	if len(warnings) > 0 {
		warningMessages := make([]string, len(warnings))
		for i, warn := range warnings {
			warningMessages[i] = fmt.Sprintf("%s: %s", warn.Field, warn.Message)
		}
		fmt.Printf("Warnings: %s\n", strings.Join(warningMessages, ", "))
	}
	
	return nil
}

// validateManifest validates top-level manifest fields
func (v *Validator) validateManifest() {
	// Validate API version
	if v.manifest.APIVersion == "" {
		v.addError("MISSING_API_VERSION", "apiVersion is required", "apiVersion", nil)
	} else if v.manifest.APIVersion != "v1alpha1" {
		v.addError("INVALID_API_VERSION", "apiVersion must be v1alpha1", "apiVersion", v.manifest.APIVersion)
	}
	
	// Validate app name
	if v.manifest.AppName == "" {
		v.addError("MISSING_APP_NAME", "appName is required", "appName", nil)
	} else if !isValidIdentifier(v.manifest.AppName) {
		v.addError("INVALID_APP_NAME", "appName must contain only alphanumeric characters and hyphens", "appName", v.manifest.AppName)
	}
	
	// Validate target
	if v.manifest.Target == "" {
		v.addError("MISSING_TARGET", "target is required", "target", nil)
	} else {
		validTargets := map[string]bool{
			"local-docker":   true,
			"minikube":       true,
			"staging-k8s":    true,
			"production-aws": true,
			"production-gcp": true,
			"production-azure": true,
		}
		if !validTargets[v.manifest.Target] {
			v.addWarning("UNKNOWN_TARGET", fmt.Sprintf("target '%s' is not a standard target", v.manifest.Target), "target")
		}
	}
}

// validateComponents validates all components
func (v *Validator) validateComponents() {
	if len(v.manifest.Components) == 0 {
		v.addError("NO_COMPONENTS", "manifest must contain at least one component", "components", nil)
		return
	}
	
	componentIDs := make(map[string]bool)
	
	for i, comp := range v.manifest.Components {
		// Validate component ID
		if comp.ID == "" {
			v.addError("MISSING_COMPONENT_ID", "component ID is required", fmt.Sprintf("components[%d].id", i), nil)
		} else if !isValidIdentifier(comp.ID) {
			v.addError("INVALID_COMPONENT_ID", "component ID must contain only alphanumeric characters and hyphens", fmt.Sprintf("components[%d].id", i), comp.ID)
		} else if componentIDs[comp.ID] {
			v.addError("DUPLICATE_COMPONENT_ID", fmt.Sprintf("duplicate component ID: %s", comp.ID), fmt.Sprintf("components[%d].id", i), comp.ID)
		} else {
			componentIDs[comp.ID] = true
		}
		
		// Validate component name
		if comp.Name == "" {
			v.addError("MISSING_COMPONENT_NAME", "component name is required", fmt.Sprintf("components[%d].name", i), nil)
		}
		
		// Validate component type
		if comp.Type == "" {
			v.addError("MISSING_COMPONENT_TYPE", "component type is required", fmt.Sprintf("components[%d].type", i), nil)
		} else {
			validTypes := map[string]bool{
				"docker":      true,
				"kubernetes":  true,
				"terraform":   true,
				"helm":        true,
				"cloudformation": true,
			}
			if !validTypes[comp.Type] {
				v.addError("INVALID_COMPONENT_TYPE", fmt.Sprintf("invalid component type: %s", comp.Type), fmt.Sprintf("components[%d].type", i), comp.Type)
			}
		}
		
		// Validate source
		if comp.Source == "" {
			v.addError("MISSING_SOURCE", "component source is required", fmt.Sprintf("components[%d].source", i), nil)
		} else {
			// Check if source path exists (for local paths)
			if !strings.HasPrefix(comp.Source, "http://") && 
			   !strings.HasPrefix(comp.Source, "https://") &&
			   !strings.HasPrefix(comp.Source, "git://") {
				
				// For Helm charts, allow repository references
				if comp.Type == "helm" && !strings.Contains(comp.Source, "/") {
					// This might be a chart from stable repo, allow it
				} else {
					absPath, err := filepath.Abs(comp.Source)
					if err != nil {
						v.addError("INVALID_SOURCE_PATH", fmt.Sprintf("invalid source path: %v", err), fmt.Sprintf("components[%d].source", i), comp.Source)
					} else {
						// Note: We don't check if file exists here as it might be created during deployment
						// This is just a path validation
						if strings.Contains(absPath, "..") {
							v.addWarning("RELATIVE_SOURCE_PATH", "source path contains '..' which may be insecure", fmt.Sprintf("components[%d].source", i))
						}
					}
				}
			}
		}
		
		// Validate variables
		v.validateComponentVariables(i, comp)
		
		// Validate health check if present
		if comp.HealthCheck != nil {
			v.validateHealthCheck(i, *comp.HealthCheck)
		}
	}
}

// validateComponentVariables validates component variables
func (v *Validator) validateComponentVariables(componentIndex int, comp types.Component) {
	for key, value := range comp.Variables {
		// Check if key is valid
		if !isValidVariableName(key) {
			v.addError("INVALID_VARIABLE_NAME", 
				fmt.Sprintf("variable name '%s' must contain only alphanumeric characters and underscores", key),
				fmt.Sprintf("components[%d].variables.%s", componentIndex, key),
				key)
		}
		
		// Check if value is a valid type
		switch val := value.(type) {
		case string, int, float64, bool:
			// Valid types
		case []interface{}, map[string]interface{}:
			// Complex types are allowed
		default:
			v.addError("INVALID_VARIABLE_TYPE", 
				fmt.Sprintf("variable '%s' has invalid type", key),
				fmt.Sprintf("components[%d].variables.%s", componentIndex, key),
				value)
		}
	}
}

// validateHealthCheck validates health check configuration
func (v *Validator) validateHealthCheck(componentIndex int, hc types.HealthCheck) {
	validTypes := map[string]bool{
		"http":      true,
		"tcp":       true,
		"command":   true,
		"none":      true,
	}
	
	if !validTypes[hc.Type] {
		v.addError("INVALID_HEALTH_CHECK_TYPE", 
			fmt.Sprintf("invalid health check type: %s", hc.Type),
			fmt.Sprintf("components[%d].healthCheck.type", componentIndex),
			hc.Type)
	}
	
	if hc.Type == "http" && hc.Endpoint == "" {
		v.addError("MISSING_HEALTH_CHECK_ENDPOINT", 
			"endpoint is required for http health check",
			fmt.Sprintf("components[%d].healthCheck.endpoint", componentIndex),
			nil)
	}
	
	// Validate interval format
	if hc.Interval != "" {
		if !isValidDuration(hc.Interval) {
			v.addError("INVALID_HEALTH_CHECK_INTERVAL", 
				"interval must be a valid duration (e.g., 30s, 5m, 1h)",
				fmt.Sprintf("components[%d].healthCheck.interval", componentIndex),
				hc.Interval)
		}
	}
	
	// Validate timeout format
	if hc.Timeout != "" {
		if !isValidDuration(hc.Timeout) {
			v.addError("INVALID_HEALTH_CHECK_TIMEOUT", 
				"timeout must be a valid duration (e.g., 30s, 5m, 1h)",
				fmt.Sprintf("components[%d].healthCheck.timeout", componentIndex),
				hc.Timeout)
		}
	}
}

// validateDependencies validates component dependencies
func (v *Validator) validateDependencies() {
	componentIDs := make(map[string]bool)
	for _, comp := range v.manifest.Components {
		componentIDs[comp.ID] = true
	}
	
	for i, comp := range v.manifest.Components {
		for _, depID := range comp.DependsOn {
			if !componentIDs[depID] {
				v.addError("MISSING_DEPENDENCY", 
					fmt.Sprintf("dependency '%s' not found in components", depID),
					fmt.Sprintf("components[%d].dependsOn", i),
					depID)
			}
		}
	}
	
	// Check for circular dependencies
	if err := v.checkCircularDependencies(); err != nil {
		v.addError("CIRCULAR_DEPENDENCY", 
			err.Error(),
			"components.dependsOn",
			nil)
	}
}

// validateVariables validates manifest variables
func (v *Validator) validateVariables() {
	for key, value := range v.manifest.Variables {
		if !isValidVariableName(key) {
			v.addError("INVALID_GLOBAL_VARIABLE_NAME", 
				fmt.Sprintf("global variable name '%s' must contain only alphanumeric characters and underscores", key),
				fmt.Sprintf("variables.%s", key),
				key)
		}
		
		// Check if value is a valid type
		switch value.(type) {
		case string, int, float64, bool:
			// Valid types
		case []interface{}, map[string]interface{}:
			// Complex types are allowed
		default:
			v.addError("INVALID_GLOBAL_VARIABLE_TYPE", 
				fmt.Sprintf("global variable '%s' has invalid type", key),
				fmt.Sprintf("variables.%s", key),
				value)
		}
	}
}

// checkCircularDependencies checks for circular dependencies using DFS
func (v *Validator) checkCircularDependencies() error {
	// Build adjacency list
	adj := make(map[string][]string)
	for _, comp := range v.manifest.Components {
		adj[comp.ID] = comp.DependsOn
	}
	
	// Perform DFS to detect cycles
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	
	var dfs func(string) error
	dfs = func(node string) error {
		if recStack[node] {
			return fmt.Errorf("circular dependency detected involving component '%s'", node)
		}
		if visited[node] {
			return nil
		}
		
		visited[node] = true
		recStack[node] = true
		
		for _, neighbor := range adj[node] {
			if err := dfs(neighbor); err != nil {
				return err
			}
		}
		
		recStack[node] = false
		return nil
	}
	
	for _, comp := range v.manifest.Components {
		if !visited[comp.ID] {
			if err := dfs(comp.ID); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// addError adds a validation error
func (v *Validator) addError(code, message, field string, value any) {
	v.errors = append(v.errors, ValidationError{
		Code:    code,
		Message: message,
		Field:   field,
		Value:   value,
	})
}

// addWarning adds a validation warning
func (v *Validator) addWarning(code, message, field string) {
	v.warnings = append(v.warnings, ValidationWarning{
		Code:    code,
		Message: message,
		Field:   field,
	})
}

// Helper functions

func isValidIdentifier(s string) bool {
	// Allow alphanumeric and hyphens
	re := regexp.MustCompile(`^[a-zA-Z0-9-]+$`)
	return re.MatchString(s)
}

func isValidVariableName(s string) bool {
	// Allow alphanumeric and underscores
	re := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	return re.MatchString(s)
}

func isValidDuration(s string) bool {
	// Simple duration validation
	re := regexp.MustCompile(`^\d+[smh]$`)
	return re.MatchString(s)
}