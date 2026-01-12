package types

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Provider defines the interface that all Orchix providers must implement
type Provider interface {
	// Name returns the name of the provider
	Name() string
	
	// Type returns the type of provider
	Type() ComponentType
	
	// Validate validates the provider configuration
	Validate(component Component) error
	
	// Plan generates an execution plan for the component
	Plan(ctx context.Context, component Component) (*PlanResult, error)
	
	// Apply executes the deployment plan
	Apply(ctx context.Context, component Component) (*ApplyResult, error)
	
	// Destroy removes resources created by the provider
	Destroy(ctx context.Context, component Component) (*DestroyResult, error)
	
	// Status checks the current status of the component
	Status(ctx context.Context, component Component) (*StatusResult, error)
	
	// Outputs returns the outputs from the deployed component
	Outputs(ctx context.Context, component Component) (map[string]interface{}, error)
	
	// HealthCheck performs a health check on the component
	HealthCheck(ctx context.Context, component Component) (*HealthCheckResult, error)
}

// ProviderFactory creates providers based on component type
type ProviderFactory interface {
	// Create creates a provider for the given component type
	Create(componentType ComponentType) (Provider, error)
	
	// Register registers a provider factory for a component type
	Register(componentType ComponentType, factory func() Provider) error
	
	// List returns all registered provider types
	List() []ComponentType
}

// PlanResult represents the result of a plan operation
type PlanResult struct {
	ComponentID string      `json:"componentId"`
	Changes     []Change    `json:"changes"`
	Outputs     []Output    `json:"outputs,omitempty"`
	Summary     PlanSummary `json:"summary"`
	GeneratedAt time.Time   `json:"generatedAt"`
	Warnings    []string    `json:"warnings,omitempty"`
	Errors      []string    `json:"errors,omitempty"`
}

// Change represents a single change in a plan
type Change struct {
	Type      ChangeType    `json:"type"`
	Address   string        `json:"address"`
	From      interface{}   `json:"from,omitempty"`
	To        interface{}   `json:"to,omitempty"`
	Sensitive bool          `json:"sensitive,omitempty"`
	Action    ChangeAction  `json:"action"`
	Reason    string        `json:"reason,omitempty"`
	Metadata  ChangeMetadata `json:"metadata,omitempty"`
}

// ChangeType defines the type of change
type ChangeType string

const (
	ChangeTypeCreate  ChangeType = "create"
	ChangeTypeUpdate  ChangeType = "update"
	ChangeTypeDelete  ChangeType = "delete"
	ChangeTypeReplace ChangeType = "replace"
	ChangeTypeNoop    ChangeType = "no-op"
)

// ChangeAction defines the action to take for a change
type ChangeAction string

const (
	ChangeActionCreate  ChangeAction = "create"
	ChangeActionUpdate  ChangeAction = "update"
	ChangeActionDelete  ChangeAction = "delete"
	ChangeActionReplace ChangeAction = "replace"
	ChangeActionRead    ChangeAction = "read"
)

// ChangeMetadata contains additional metadata about a change
type ChangeMetadata struct {
	ResourceType string            `json:"resourceType,omitempty"`
	Provider     string            `json:"provider,omitempty"`
	Module       string            `json:"module,omitempty"`
	Count        int               `json:"count,omitempty"`
	Duration     time.Duration     `json:"duration,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
}

// Output represents an output value from a provider
type Output struct {
	Name        string      `json:"name"`
	Value       interface{} `json:"value"`
	Type        string      `json:"type,omitempty"`
	Description string      `json:"description,omitempty"`
	Sensitive   bool        `json:"sensitive,omitempty"`
}

// PlanSummary summarizes the plan results
type PlanSummary struct {
	Add     int `json:"add"`
	Change  int `json:"change"`
	Destroy int `json:"destroy"`
	Replace int `json:"replace"`
	Total   int `json:"total"`
}

// ApplyResult represents the result of an apply operation
type ApplyResult struct {
	ComponentID string               `json:"componentId"`
	Outputs     map[string]Output    `json:"outputs"`
	Resources   []Resource           `json:"resources,omitempty"`
	Duration    time.Duration        `json:"duration"`
	StartedAt   time.Time            `json:"startedAt"`
	CompletedAt time.Time            `json:"completedAt"`
	Status      OperationStatus      `json:"status"`
	Message     string               `json:"message,omitempty"`
	Metadata    map[string]string    `json:"metadata,omitempty"`
}

// DestroyResult represents the result of a destroy operation
type DestroyResult struct {
	ComponentID string               `json:"componentId"`
	Resources   []Resource           `json:"resources"`
	Duration    time.Duration        `json:"duration"`
	StartedAt   time.Time            `json:"startedAt"`
	CompletedAt time.Time            `json:"completedAt"`
	Status      OperationStatus      `json:"status"`
	Message     string               `json:"message,omitempty"`
}

// StatusResult represents the result of a status check
type StatusResult struct {
	ComponentID string           `json:"componentId"`
	Status      ComponentStatus  `json:"status"`
	Health      HealthStatus     `json:"health"`
	Resources   []ResourceStatus `json:"resources,omitempty"`
	Outputs     map[string]interface{} `json:"outputs,omitempty"`
	Message     string           `json:"message,omitempty"`
	CheckedAt   time.Time        `json:"checkedAt"`
}

// HealthCheckResult represents the result of a health check
type HealthCheckResult struct {
	ComponentID string       `json:"componentId"`
	Healthy     bool         `json:"healthy"`
	Status      HealthStatus `json:"status"`
	Message     string       `json:"message"`
	Duration    time.Duration `json:"duration"`
	CheckedAt   time.Time    `json:"checkedAt"`
}

// Resource represents a deployed resource
type Resource struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	Name         string            `json:"name"`
	Provider     string            `json:"provider"`
	Status       ResourceStatus    `json:"status"`
	CreatedAt    time.Time         `json:"createdAt"`
	UpdatedAt    time.Time         `json:"updatedAt"`
	Attributes   map[string]interface{} `json:"attributes,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty"`
}

// ResourceStatus represents the status of a resource
type ResourceStatus string

const (
	ResourceStatusPending   ResourceStatus = "pending"
	ResourceStatusCreating  ResourceStatus = "creating"
	ResourceStatusCreated   ResourceStatus = "created"
	ResourceStatusUpdating  ResourceStatus = "updating"
	ResourceStatusDeleting  ResourceStatus = "deleting"
	ResourceStatusDeleted   ResourceStatus = "deleted"
	ResourceStatusFailed    ResourceStatus = "failed"
	ResourceStatusUnknown   ResourceStatus = "unknown"
)

// ComponentStatus represents the status of a component
type ComponentStatus string

const (
	ComponentStatusPending    ComponentStatus = "pending"
	ComponentStatusDeploying  ComponentStatus = "deploying"
	ComponentStatusDeployed   ComponentStatus = "deployed"
	ComponentStatusFailed     ComponentStatus = "failed"
	ComponentStatusDestroying ComponentStatus = "destroying"
	ComponentStatusDestroyed  ComponentStatus = "destroyed"
	ComponentStatusUnknown    ComponentStatus = "unknown"
)

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// OperationStatus represents the status of an operation
type OperationStatus string

const (
	OperationStatusSuccess OperationStatus = "success"
	OperationStatusFailed  OperationStatus = "failed"
	OperationStatusPartial OperationStatus = "partial"
	OperationStatusRunning OperationStatus = "running"
)

// String returns a string representation of the plan result
func (pr *PlanResult) String() string {
	return fmt.Sprintf("Plan: %d to add, %d to change, %d to destroy, %d to replace",
		pr.Summary.Add, pr.Summary.Change, pr.Summary.Destroy, pr.Summary.Replace)
}

// HasChanges returns true if the plan has changes
func (pr *PlanResult) HasChanges() bool {
	return pr.Summary.Add > 0 || pr.Summary.Change > 0 || 
	       pr.Summary.Destroy > 0 || pr.Summary.Replace > 0
}

// ToJSON converts the plan result to JSON
func (pr *PlanResult) ToJSON() (string, error) {
	data, err := json.MarshalIndent(pr, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ToJSON converts the apply result to JSON
func (ar *ApplyResult) ToJSON() (string, error) {
	data, err := json.MarshalIndent(ar, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ToJSON converts the status result to JSON
func (sr *StatusResult) ToJSON() (string, error) {
	data, err := json.MarshalIndent(sr, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetOutputValue retrieves an output value by name
func (ar *ApplyResult) GetOutputValue(name string) (interface{}, bool) {
	if output, exists := ar.Outputs[name]; exists {
		return output.Value, true
	}
	return nil, false
}

// GetOutputString retrieves an output value as string
func (ar *ApplyResult) GetOutputString(name string) (string, bool) {
	if value, exists := ar.GetOutputValue(name); exists {
		if str, ok := value.(string); ok {
			return str, true
		}
	}
	return "", false
}

// GetOutputInt retrieves an output value as int
func (ar *ApplyResult) GetOutputInt(name string) (int, bool) {
	if value, exists := ar.GetOutputValue(name); exists {
		switch v := value.(type) {
		case int:
			return v, true
		case float64:
			return int(v), true
		case json.Number:
			if i, err := v.Int64(); err == nil {
				return int(i), true
			}
		}
	}
	return 0, false
}

// Merge merges another apply result into this one
func (ar *ApplyResult) Merge(other *ApplyResult) {
	// Merge outputs
	if ar.Outputs == nil {
		ar.Outputs = make(map[string]Output)
	}
	for k, v := range other.Outputs {
		ar.Outputs[k] = v
	}
	
	// Merge resources
	ar.Resources = append(ar.Resources, other.Resources...)
	
	// Merge metadata
	if ar.Metadata == nil {
		ar.Metadata = make(map[string]string)
	}
	for k, v := range other.Metadata {
		ar.Metadata[k] = v
	}
	
	// Update duration
	ar.Duration += other.Duration
	
	// Update status if other failed
	if other.Status == OperationStatusFailed {
		ar.Status = OperationStatusFailed
		ar.Message = fmt.Sprintf("%s; %s", ar.Message, other.Message)
	}
}