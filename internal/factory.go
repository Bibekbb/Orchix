package providers

import (
	"fmt"
	"sync"

	"github.com/Bibekbb/Orchix/internal/providers/docker"
	"github.com/Bibekbb/Orchix/internal/providers/kubernetes"
	"github.com/Bibekbb/Orchix/internal/providers/terraform"
	"github.com/Bibekbb/Orchix/pkg/types"
)

// ProviderFactory manages provider creation and lifecycle
type ProviderFactory struct {
	providers map[string]Provider
	configs   map[string]ProviderConfig
	mu        sync.RWMutex
}

// NewProviderFactory creates a new provider factory
func NewProviderFactory() *ProviderFactory {
	return &ProviderFactory{
		providers: make(map[string]Provider),
		configs:   make(map[string]ProviderConfig),
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
	// Register Docker provider
	dockerProvider := docker.NewDockerProvider()
	if err := f.RegisterProvider(dockerProvider); err != nil {
		return fmt.Errorf("failed to register Docker provider: %w", err)
	}

	// Register Kubernetes provider
	k8sProvider := kubernetes.NewKubernetesProvider()
	if err := f.RegisterProvider(k8sProvider); err != nil {
		return fmt.Errorf("failed to register Kubernetes provider: %w", err)
	}

	// Register Terraform provider
	tfProvider := terraform.NewTerraformProvider()
	if err := f.RegisterProvider(tfProvider); err != nil {
		return fmt.Errorf("failed to register Terraform provider: %w", err)
	}

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
func (f *ProviderFactory) ProviderHealth() map[string]string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	health := make(map[string]string)
	for name, provider := range f.providers {
		// Simple health check - just verify provider exists
		if provider != nil {
			health[name] = "healthy"
		} else {
			health[name] = "unhealthy"
		}
	}

	return health
}

// UnregisterProvider removes a provider from the factory
func (f *ProviderFactory) UnregisterProvider(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.providers[name]; !exists {
		return fmt.Errorf("provider %s not registered", name)
	}

	delete(f.providers, name)
	return nil
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
	case "kubernetes":
		return kubernetes.NewKubernetesProvider(), nil
	case "terraform":
		return terraform.NewTerraformProvider(), nil
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
	case "docker":
		// Docker doesn't require specific config
		return nil
	default:
		return fmt.Errorf("unknown provider type: %s", config.Type)
	}

	return nil
}