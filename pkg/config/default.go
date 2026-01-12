package config

import (
	"time"
)

// Default values
const (
	DefaultAPIVersion     = "v1alpha1"
	DefaultLogLevel       = "info"
	DefaultLogFormat      = "text"
	DefaultStateBackend   = "file"
	DefaultTimeoutDeploy  = 30 * time.Minute
	DefaultTimeoutDestroy = 15 * time.Minute
	DefaultTimeoutHealth  = 5 * time.Minute
	DefaultMaxParallel    = 5
	DefaultHealthInterval = "10s"
	DefaultHealthTimeout  = "30s"
	DefaultHealthRetries  = 3
)

// DefaultManifest returns a manifest with default values
func DefaultManifest() *ManifestSchema {
	return &ManifestSchema{
		APIVersion: DefaultAPIVersion,
		Components: []ComponentSchema{},
		Variables:  make(map[string]interface{}),
	}
}

// DefaultGlobalConfig returns global configuration with defaults
func DefaultGlobalConfig() *GlobalConfig {
	config := &GlobalConfig{}

	// Logging defaults
	config.Logging.Level = DefaultLogLevel
	config.Logging.Format = DefaultLogFormat

	// State defaults
	config.State.Backend = DefaultStateBackend
	config.State.Path = ".orchix/state.json"
	config.State.Encrypt = false

	// Timeout defaults
	config.Timeout.Deploy = DefaultTimeoutDeploy
	config.Timeout.Destroy = DefaultTimeoutDestroy
	config.Timeout.Health = DefaultTimeoutHealth

	// Concurrency defaults
	config.Concurrency.MaxParallel = DefaultMaxParallel

	// Provider defaults
	config.Providers.Docker.Host = "unix:///var/run/docker.sock"
	config.Providers.Kubernetes.Namespace = "default"
	config.Providers.Kubernetes.InCluster = false
	config.Providers.Terraform.Backend = "local"
	config.Providers.Terraform.PluginCache = true

	return config
}

// DefaultComponent returns a component with default values
func DefaultComponent() *ComponentSchema {
	return &ComponentSchema{
		Type:    ComponentTypeDocker,
		Enabled: true,
		Retries: 1,
		Timeout: "5m",
	}
}

// DefaultHealthCheck returns a health check with default values
func DefaultHealthCheck() *HealthCheckSchema {
	return &HealthCheckSchema{
		Type:         HealthCheckTypeHTTP,
		Interval:     DefaultHealthInterval,
		Timeout:      DefaultHealthTimeout,
		Retries:      DefaultHealthRetries,
		InitialDelay: "5s",
		SuccessCodes: []int{200},
	}
}

// SetComponentDefaults applies defaults to a component
func SetComponentDefaults(comp *ComponentSchema) {
	if comp.Type == "" {
		comp.Type = ComponentTypeDocker
	}
	if comp.Timeout == "" {
		comp.Timeout = "5m"
	}
	if comp.Retries == 0 {
		comp.Retries = 1
	}
	if comp.Enabled == false && comp.Enabled != true {
		comp.Enabled = true
	}
	if comp.DependsOn == nil {
		comp.DependsOn = []string{}
	}
	if comp.Variables == nil {
		comp.Variables = make(map[string]interface{})
	}
}

// SetHealthCheckDefaults applies defaults to a health check
func SetHealthCheckDefaults(hc *HealthCheckSchema) {
	if hc.Type == "" {
		hc.Type = HealthCheckTypeHTTP
	}
	if hc.Interval == "" {
		hc.Interval = DefaultHealthInterval
	}
	if hc.Timeout == "" {
		hc.Timeout = DefaultHealthTimeout
	}
	if hc.Retries == 0 {
		hc.Retries = DefaultHealthRetries
	}
	if hc.InitialDelay == "" {
		hc.InitialDelay = "5s"
	}
	if len(hc.SuccessCodes) == 0 {
		hc.SuccessCodes = []int{200}
	}
	if hc.Headers == nil {
		hc.Headers = make(map[string]string)
	}
}

// MergeWithDefaults merges user config with defaults
func MergeWithDefaults(userConfig *GlobalConfig) *GlobalConfig {
	defaults := DefaultGlobalConfig()

	if userConfig == nil {
		return defaults
	}

	// Merge logging
	if userConfig.Logging.Level == "" {
		userConfig.Logging.Level = defaults.Logging.Level
	}
	if userConfig.Logging.Format == "" {
		userConfig.Logging.Format = defaults.Logging.Format
	}
	if userConfig.Logging.File == "" && defaults.Logging.File != "" {
		userConfig.Logging.File = defaults.Logging.File
	}

	// Merge state
	if userConfig.State.Backend == "" {
		userConfig.State.Backend = defaults.State.Backend
	}
	if userConfig.State.Path == "" {
		userConfig.State.Path = defaults.State.Path
	}

	// Merge timeouts
	if userConfig.Timeout.Deploy == 0 {
		userConfig.Timeout.Deploy = defaults.Timeout.Deploy
	}
	if userConfig.Timeout.Destroy == 0 {
		userConfig.Timeout.Destroy = defaults.Timeout.Destroy
	}
	if userConfig.Timeout.Health == 0 {
		userConfig.Timeout.Health = defaults.Timeout.Health
	}

	// Merge concurrency
	if userConfig.Concurrency.MaxParallel == 0 {
		userConfig.Concurrency.MaxParallel = defaults.Concurrency.MaxParallel
	}

	// Merge providers (nested merge)
	mergeProviders(&userConfig.Providers, &defaults.Providers)

	return userConfig
}

// mergeProviders merges provider configurations
func mergeProviders(dest, src *ProviderConfig) {
	// Docker
	if dest.Docker.Host == "" {
		dest.Docker.Host = src.Docker.Host
	}
	if dest.Docker.Version == "" {
		dest.Docker.Version = src.Docker.Version
	}

	// Kubernetes
	if dest.Kubernetes.ConfigPath == "" {
		dest.Kubernetes.ConfigPath = src.Kubernetes.ConfigPath
	}
	if dest.Kubernetes.Context == "" {
		dest.Kubernetes.Context = src.Kubernetes.Context
	}
	if dest.Kubernetes.Namespace == "" {
		dest.Kubernetes.Namespace = src.Kubernetes.Namespace
	}
	if !dest.Kubernetes.InCluster {
		dest.Kubernetes.InCluster = src.Kubernetes.InCluster
	}

	// Terraform
	if dest.Terraform.Version == "" {
		dest.Terraform.Version = src.Terraform.Version
	}
	if dest.Terraform.Backend == "" {
		dest.Terraform.Backend = src.Terraform.Backend
	}
	if dest.Terraform.WorkingDirectory == "" {
		dest.Terraform.WorkingDirectory = src.Terraform.WorkingDirectory
	}
	if !dest.Terraform.PluginCache {
		dest.Terraform.PluginCache = src.Terraform.PluginCache
	}

	// Helm
	if dest.Helm.Version == "" {
		dest.Helm.Version = src.Helm.Version
	}
	if dest.Helm.Repositories == nil {
		dest.Helm.Repositories = src.Helm.Repositories
	}
}