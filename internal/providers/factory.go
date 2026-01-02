package providers

import (
	"fmt"
	"sync"

	"github.com/Bibekbb/Orchix/internal/providers/docker"
	"github.com/Bibekbb/Orchix/internal/providers/kubernetes"
	"github.com/Bibekbb/Orchix/internal/providers/terraform"
	"github.com/Bibekbb/Orchix/internal/providers/helm"
	"github.com/Bibekbb/Orchix/pkg/types"
)

// ProviderFactory manages provider creation and lifecycle
type ProviderFactory struct {
	providers map[string]Provider
	configs   map[string]ProviderConfig
	mu        sync.RWMutex
	initialized bool
}

// NewProviderFactory creates a new provider factory
func NewProviderFactory() *ProviderFactory {
	return &ProviderFactory{
		providers: make(map[string]Provider),
		configs:   make(map[string]ProviderConfig),
		initialized: false,
	}
}

// RegisterProvider registers a provider with the factory
func (f *ProviderFactory) RegisterProvider(provider Provider) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	name := provider.Name()
	if _, exists := f.providers[name]; exists {
		return fmt.Errorf("provider %s already registered", name)
	}

	f.providers[name] = provider
	return nil
}

// GetProvider returns a provider by name
func (f *ProviderFactory) GetProvider(name string) (Provider, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	provider, exists := f.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}

	return provider, nil
}

// GetProviderForComponent returns the appropriate provider for a component
func (f *ProviderFactory) GetProviderForComponent(comp types.Component) (Provider, error) {
	return f.GetProvider(comp.Type)
}

// InitializeDefaultProviders registers all built-in providers
func (f *ProviderFactory) InitializeDefaultProviders() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.initialized {
		return nil
	}

	// Register Docker provider
	dockerProvider := docker.NewDockerProvider()
	if err := f.registerProviderWithConfig(dockerProvider, ProviderConfig{
		Type: "docker",
		Name: "docker",
		Config: map[string]string{
			"api_version": "1.41",
		},
	}); err != nil {
		return fmt.Errorf("failed to register Docker provider: %w", err)
	}

	// Register Docker Compose provider
	composeProvider := docker.NewComposeProvider()
	if err := f.registerProviderWithConfig(composeProvider, ProviderConfig{
		Type: "docker-compose",
		Name: "docker-compose",
		Config: map[string]string{
			"compose_version": "v2",
		},
	}); err != nil {
		return fmt.Errorf("failed to register Docker Compose provider: %w", err)
	}

	// Register Kubernetes provider
	k8sProvider := kubernetes.NewKubernetesProvider()
	if err := f.registerProviderWithConfig(k8sProvider, ProviderConfig{
		Type: "kubernetes",
		Name: "kubernetes",
		Config: map[string]string{
			"api_version": "v1",
		},
	}); err != nil {
		return fmt.Errorf("failed to register Kubernetes provider: %w", err)
	}

	// Register Terraform provider
	tfProvider := terraform.NewTerraformProvider()
	if err := f.registerProviderWithConfig(tfProvider, ProviderConfig{
		Type: "terraform",
		Name: "terraform",
		Config: map[string]string{
			"version": ">= 1.0",
		},
	}); err != nil {
		return fmt.Errorf("failed to register Terraform provider: %w", err)
	}

	// Register Helm provider
	helmProvider := helm.NewHelmProvider()
	if err := f.registerProviderWithConfig(helmProvider, ProviderConfig{
		Type: "helm",
		Name: "helm",
		Config: map[string]string{
			"version": "v3",
		},
	}); err != nil {
		return fmt.Errorf("failed to register Helm provider: %w", err)
	}

	f.initialized = true
	return nil
}

// registerProviderWithConfig registers a provider with its configuration
func (f *ProviderFactory) registerProviderWithConfig(provider Provider, config ProviderConfig) error {
	if err := f.RegisterProvider(provider); err != nil {
		return err
	}
	f.configs[provider.Name()] = config
	return nil
}

// ListProviders returns all registered provider names
func (f *ProviderFactory) ListProviders() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	providers := make([]string, 0, len(f.providers))
	for name := range f.providers {
		providers = append(providers, name)
	}

	return providers
}

// HasProvider checks if a provider is registered
func (f *ProviderFactory) HasProvider(name string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	_, exists := f.providers[name]
	return exists
}

// ProviderHealth checks the health of all providers
func (f *ProviderFactory) ProviderHealth() map[string]ProviderHealthStatus {
	f.mu.RLock()
	defer f.mu.RUnlock()

	health := make(map[string]ProviderHealthStatus)
	for name, provider := range f.providers {
		status := ProviderHealthStatus{
			Name:   name,
			Status: "unknown",
		}

		// Try to get status from provider if it implements HealthChecker
		if healthChecker, ok := provider.(interface{ CheckHealth() error }); ok {
			if err := healthChecker.CheckHealth(); err != nil {
				status.Status = "unhealthy"
				status.Message = err.Error()
			} else {
				status.Status = "healthy"
			}
		} else {
			// Simple check - just verify provider exists
			if provider != nil {
				status.Status = "healthy"
			} else {
				status.Status = "unhealthy"
				status.Message = "provider instance is nil"
			}
		}

		health[name] = status
	}

	return health
}

// ProviderHealthStatus represents provider health information
type ProviderHealthStatus struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// UnregisterProvider removes a provider from the factory
func (f *ProviderFactory) UnregisterProvider(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.providers[name]; !exists {
		return fmt.Errorf("provider %s not registered", name)
	}

	delete(f.providers, name)
	delete(f.configs, name)
	return nil
}

// GetProviderConfig returns configuration for a provider
func (f *ProviderFactory) GetProviderConfig(name string) (ProviderConfig, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	config, exists := f.configs[name]
	if !exists {
		return ProviderConfig{}, fmt.Errorf("configuration not found for provider %s", name)
	}

	return config, nil
}

// SetProviderConfig sets configuration for a provider
func (f *ProviderFactory) SetProviderConfig(name string, config ProviderConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.providers[name]; !exists {
		return fmt.Errorf("provider %s not registered", name)
	}

	f.configs[name] = config
	return nil
}

// GetProviderMetadata returns metadata for all providers
func (f *ProviderFactory) GetProviderMetadata() map[string]ProviderMetadata {
	f.mu.RLock()
	defer f.mu.RUnlock()

	metadata := make(map[string]ProviderMetadata)
	for name, provider := range f.providers {
		if metaProvider, ok := provider.(MetadataProvider); ok {
			metadata[name] = metaProvider.GetMetadata()
		} else {
			// Default metadata
			metadata[name] = ProviderMetadata{
				Name:        name,
				Version:     "1.0.0",
				Description: fmt.Sprintf("%s deployment provider", name),
				Capabilities: []string{"plan", "apply", "destroy", "status"},
				RequiredTools: []string{name},
			}
		}
	}

	return metadata
}

// ValidateComponent validates if a component can be handled by its provider
func (f *ProviderFactory) ValidateComponent(comp types.Component) error {
	provider, err := f.GetProvider(comp.Type)
	if err != nil {
		return err
	}

	// Check if provider implements ValidatableProvider
	if validator, ok := provider.(ValidatableProvider); ok {
		return validator.ValidateConfig(comp.Variables)
	}

	return nil
}

// ExecuteWithProvider executes an operation on a component using its provider
func (f *ProviderFactory) ExecuteWithProvider(ctx context.Context, comp types.Component, operation string, fn func(Provider) error) error {
	provider, err := f.GetProvider(comp.Type)
	if err != nil {
		return err
	}

	return fn(provider)
}

// Global provider factory instance
var (
	globalFactory *ProviderFactory
	factoryOnce   sync.Once
)

// GetGlobalFactory returns the global provider factory (singleton)
func GetGlobalFactory() *ProviderFactory {
	factoryOnce.Do(func() {
		globalFactory = NewProviderFactory()
		if err := globalFactory.InitializeDefaultProviders(); err != nil {
			// Log error but continue
			fmt.Printf("Warning: failed to initialize some providers: %v\n", err)
		}
	})
	return globalFactory
}

// NewProvider creates a new provider based on configuration
func NewProvider(config ProviderConfig) (Provider, error) {
	switch config.Type {
	case "docker":
		return docker.NewDockerProvider(), nil
	case "docker-compose":
		return docker.NewComposeProvider(), nil
	case "kubernetes":
		return kubernetes.NewKubernetesProvider(), nil
	case "terraform":
		return terraform.NewTerraformProvider(), nil
	case "helm":
		return helm.NewHelmProvider(), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", config.Type)
	}
}

// ValidateProviderConfig validates provider configuration
func ValidateProviderConfig(config ProviderConfig) error {
	if config.Type == "" {
		return fmt.Errorf("provider type is required")
	}

	if config.Name == "" {
		return fmt.Errorf("provider name is required")
	}

	// Validate based on provider type
	switch config.Type {
	case "kubernetes":
		if config.Kubeconfig == "" && config.Context == "" {
			// It's okay to use default kubeconfig
			return nil
		}
	case "terraform":
		// Terraform doesn't require specific config
		return nil
	case "docker", "docker-compose":
		// Docker providers don't require specific config
		return nil
	case "helm":
		// Helm doesn't require specific config
		return nil
	default:
		return fmt.Errorf("unknown provider type: %s", config.Type)
	}

	return nil
}

// ProviderCapabilities represents capabilities of a provider
type ProviderCapabilities struct {
	CanPlan     bool `json:"canPlan"`
	CanApply    bool `json:"canApply"`
	CanDestroy  bool `json:"canDestroy"`
	CanStatus   bool `json:"canStatus"`
	CanRollback bool `json:"canRollback"`
	CanValidate bool `json:"canValidate"`
}

// GetCapabilities returns capabilities for a provider
func (f *ProviderFactory) GetCapabilities(name string) (ProviderCapabilities, error) {
	provider, err := f.GetProvider(name)
	if err != nil {
		return ProviderCapabilities{}, err
	}

	capabilities := ProviderCapabilities{
		CanPlan:    true,
		CanApply:   true,
		CanDestroy: true,
		CanStatus:  true,
	}

	// Check for rollback capability
	if _, ok := provider.(RollbackProvider); ok {
		capabilities.CanRollback = true
	}

	// Check for validation capability
	if _, ok := provider.(ValidatableProvider); ok {
		capabilities.CanValidate = true
	}

	return capabilities, nil
}

// ExecutePlan executes planning for all components
func (f *ProviderFactory) ExecutePlan(ctx context.Context, components []types.Component) (map[string]PlanResult, error) {
	results := make(map[string]PlanResult)

	for _, comp := range components {
		provider, err := f.GetProviderForComponent(comp)
		if err != nil {
			return nil, fmt.Errorf("failed to get provider for component %s: %w", comp.ID, err)
		}

		planResult, err := provider.Plan(ctx, comp)
		if err != nil {
			return nil, fmt.Errorf("failed to plan component %s: %w", comp.ID, err)
		}

		results[comp.ID] = planResult
	}

	return results, nil
}

// ExecuteApply executes deployment for all components
func (f *ProviderFactory) ExecuteApply(ctx context.Context, components []types.Component) (map[string]ApplyResult, error) {
	results := make(map[string]ApplyResult)

	for _, comp := range components {
		provider, err := f.GetProviderForComponent(comp)
		if err != nil {
			return nil, fmt.Errorf("failed to get provider for component %s: %w", comp.ID, err)
		}

		applyResult, err := provider.Apply(ctx, comp)
		if err != nil {
			return nil, fmt.Errorf("failed to apply component %s: %w", comp.ID, err)
		}

		results[comp.ID] = applyResult
	}

	return results, nil
}

// CleanupProviders cleans up all provider resources
func (f *ProviderFactory) CleanupProviders() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Clear all providers
	f.providers = make(map[string]Provider)
	f.configs = make(map[string]ProviderConfig)
	f.initialized = false

	return nil
}