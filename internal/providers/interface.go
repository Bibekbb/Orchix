package providers

import (
	"context"
	"fmt"
	"time"

	"github.com/Bibekbb/Orchix/pkg/types"
)

// Provider defines the interface for all deployment providers
type Provider interface {
	// Name returns the provider name
	Name() string

	// Plan generates a deployment plan without making changes
	Plan(ctx context.Context, comp types.Component) (PlanResult, error)

	// Apply executes the deployment
	Apply(ctx context.Context, comp types.Component) (ApplyResult, error)

	// Destroy removes deployed resources
	Destroy(ctx context.Context, comp types.Component) error

	// Status checks the current status
	Status(ctx context.Context, comp types.Component) (StatusResult, error)

	// HealthCheck performs health verification
	HealthCheck(ctx context.Context, comp types.Component) (HealthCheckResult, error)
}

// PlanResult contains information about planned changes
type PlanResult struct {
	Changes []Change          `json:"changes"`
	Outputs map[string]string `json:"outputs,omitempty"`
}

// ApplyResult contains information about applied changes
type ApplyResult struct {
	Outputs    map[string]string `json:"outputs"`
	Duration   time.Duration     `json:"duration"`
	DeployedAt time.Time         `json:"deployedAt"`
}

// StatusResult contains component status information
type StatusResult struct {
	Status  string            `json:"status"`
	Message string            `json:"message"`
	Healthy bool              `json:"healthy"`
	Details map[string]string `json:"details,omitempty"`
}

// HealthCheckResult contains health check information
type HealthCheckResult struct {
	Healthy     bool          `json:"healthy"`
	Message     string        `json:"message"`
	Duration    time.Duration `json:"duration"`
	LastChecked time.Time     `json:"lastChecked"`
}

// Change represents a single change to be made
type Change struct {
	Type    ChangeType  `json:"type"`
	Address string      `json:"address"`
	Before  interface{} `json:"before,omitempty"`
	After   interface{} `json:"after,omitempty"`
	Action  string      `json:"action,omitempty"`
}

// ChangeType defines the type of change
type ChangeType string

const (
	ChangeTypeCreate ChangeType = "create"
	ChangeTypeUpdate ChangeType = "update"
	ChangeTypeDelete ChangeType = "delete"
	ChangeTypeNoOp   ChangeType = "no-op"
)

// ProviderConfig holds configuration for providers
type ProviderConfig struct {
	Type       string            `json:"type"`
	Name       string            `json:"name"`
	Config     map[string]string `json:"config"`
	Kubeconfig string            `json:"kubeconfig,omitempty"`
	Context    string            `json:"context,omitempty"`
}

// NewPlanResult creates a new PlanResult
func NewPlanResult() PlanResult {
	return PlanResult{
		Changes: make([]Change, 0),
		Outputs: make(map[string]string),
	}
}

// NewApplyResult creates a new ApplyResult
func NewApplyResult() ApplyResult {
	return ApplyResult{
		Outputs:    make(map[string]string),
		DeployedAt: time.Now(),
	}
}

// NewStatusResult creates a new StatusResult
func NewStatusResult() StatusResult {
	return StatusResult{
		Details: make(map[string]string),
	}
}

// AddChange adds a change to the plan result
func (p *PlanResult) AddChange(change Change) {
	p.Changes = append(p.Changes, change)
}

// SetOutput sets an output value
func (p *PlanResult) SetOutput(key, value string) {
	p.Outputs[key] = value
}

// AddOutput adds multiple outputs
func (p *ApplyResult) AddOutput(key, value string) {
	p.Outputs[key] = value
}

// AddDetail adds a detail to status result
func (s *StatusResult) AddDetail(key, value string) {
	if s.Details == nil {
		s.Details = make(map[string]string)
	}
	s.Details[key] = value
}

// Error types for provider errors
type ProviderError struct {
	Provider  string `json:"provider"`
	Component string `json:"component"`
	Operation string `json:"operation"`
	Err       error  `json:"error"`
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("provider %s failed for component %s during %s: %v",
		e.Provider, e.Component, e.Operation, e.Err)
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

// NewProviderError creates a new provider error
func NewProviderError(provider, component, operation string, err error) *ProviderError {
	return &ProviderError{
		Provider:  provider,
		Component: component,
		Operation: operation,
		Err:       err,
	}
}

// ProviderMetadata contains metadata about a provider
type ProviderMetadata struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Capabilities []string `json:"capabilities"`
	RequiredTools []string `json:"requiredTools"`
}

// GetProviderMetadata returns metadata for a provider
type MetadataProvider interface {
	GetMetadata() ProviderMetadata
}

// ValidatableProvider interface for providers that can validate configurations
type ValidatableProvider interface {
	ValidateConfig(config map[string]interface{}) error
}

// RollbackProvider interface for providers that support rollback
type RollbackProvider interface {
	Rollback(ctx context.Context, comp types.Component) error
}

// OutputProvider interface for providers that can output structured data
type OutputProvider interface {
	GetOutputs(ctx context.Context, comp types.Component) (map[string]string, error)
}

// ProviderRegistry manages registered providers
type ProviderRegistry interface {
	RegisterProvider(provider Provider) error
	GetProvider(name string) (Provider, error)
	ListProviders() []string
	HasProvider(name string) bool
}