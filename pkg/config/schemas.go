package config

import (
	"time"
)

// ManifestSchema defines the JSON/YAML schema for Orchix manifests
type ManifestSchema struct {
	APIVersion  string                 `yaml:"apiVersion" json:"apiVersion" validate:"required,oneof=v1alpha1 v1beta1"`
	AppName     string                 `yaml:"appName" json:"appName" validate:"required,min=1"`
	Target      string                 `yaml:"target" json:"target" validate:"required"`
	Description string                 `yaml:"description,omitempty" json:"description,omitempty"`
	Version     string                 `yaml:"version,omitempty" json:"version,omitempty"`
	Variables   map[string]interface{} `yaml:"variables,omitempty" json:"variables,omitempty"`
	Secrets     []SecretRef            `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	Components  []ComponentSchema      `yaml:"components" json:"components" validate:"required,min=1,dive"`
}

// ComponentSchema defines the schema for a single component
type ComponentSchema struct {
	ID          string                 `yaml:"id" json:"id" validate:"required,min=1"`
	Name        string                 `yaml:"name" json:"name" validate:"required,min=1"`
	Type        ComponentType          `yaml:"type" json:"type" validate:"required,oneof=docker kubernetes terraform helm"`
	Source      string                 `yaml:"source" json:"source" validate:"required,min=1"`
	DependsOn   []string               `yaml:"dependsOn,omitempty" json:"dependsOn,omitempty"`
	Variables   map[string]interface{} `yaml:"variables,omitempty" json:"variables,omitempty"`
	HealthCheck *HealthCheckSchema     `yaml:"healthCheck,omitempty" json:"healthCheck,omitempty"`
	Timeout     string                 `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Retries     int                    `yaml:"retries,omitempty" json:"retries,omitempty" validate:"min=0,max=5"`
	Enabled     bool                   `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

// HealthCheckSchema defines health check configuration
type HealthCheckSchema struct {
	Type          HealthCheckType `yaml:"type" json:"type" validate:"required,oneof=http tcp command"`
	Endpoint      string          `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Command       string          `yaml:"command,omitempty" json:"command,omitempty"`
	Interval      string          `yaml:"interval" json:"interval" validate:"required"`
	Timeout       string          `yaml:"timeout" json:"timeout" validate:"required"`
	SuccessCodes  []int           `yaml:"successCodes,omitempty" json:"successCodes,omitempty"`
	Retries       int             `yaml:"retries,omitempty" json:"retries,omitempty" validate:"min=0,max=10"`
	InitialDelay  string          `yaml:"initialDelay,omitempty" json:"initialDelay,omitempty"`
	Headers       map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

// SecretRef defines a reference to an external secret
type SecretRef struct {
	Name   string `yaml:"name" json:"name" validate:"required"`
	Source string `yaml:"source" json:"source" validate:"required,oneof=vault aws-secrets-manager gcp-secret-manager azure-key-vault file env"`
	Key    string `yaml:"key,omitempty" json:"key,omitempty"`
	Path   string `yaml:"path,omitempty" json:"path,omitempty"`
	EnvVar string `yaml:"envVar,omitempty" json:"envVar,omitempty"`
}

// ComponentType represents the type of component
type ComponentType string

const (
	ComponentTypeDocker      ComponentType = "docker"
	ComponentTypeKubernetes  ComponentType = "kubernetes"
	ComponentTypeTerraform   ComponentType = "terraform"
	ComponentTypeHelm        ComponentType = "helm"
)

// HealthCheckType represents the type of health check
type HealthCheckType string

const (
	HealthCheckTypeHTTP    HealthCheckType = "http"
	HealthCheckTypeTCP     HealthCheckType = "tcp"
	HealthCheckTypeCommand HealthCheckType = "command"
)

// ProviderConfig defines provider-specific configuration
type ProviderConfig struct {
	Docker struct {
		Host    string `yaml:"host,omitempty" json:"host,omitempty"`
		Version string `yaml:"version,omitempty" json:"version,omitempty"`
		TLS     struct {
			Cert   string `yaml:"cert,omitempty" json:"cert,omitempty"`
			Key    string `yaml:"key,omitempty" json:"key,omitempty"`
			CA     string `yaml:"ca,omitempty" json:"ca,omitempty"`
			Verify bool   `yaml:"verify,omitempty" json:"verify,omitempty"`
		} `yaml:"tls,omitempty" json:"tls,omitempty"`
	} `yaml:"docker,omitempty" json:"docker,omitempty"`

	Kubernetes struct {
		ConfigPath string `yaml:"configPath,omitempty" json:"configPath,omitempty"`
		Context    string `yaml:"context,omitempty" json:"context,omitempty"`
		Namespace  string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
		InCluster  bool   `yaml:"inCluster,omitempty" json:"inCluster,omitempty"`
	} `yaml:"kubernetes,omitempty" json:"kubernetes,omitempty"`

	Terraform struct {
		Version          string `yaml:"version,omitempty" json:"version,omitempty"`
		Backend          string `yaml:"backend,omitempty" json:"backend,omitempty"`
		WorkingDirectory string `yaml:"workingDirectory,omitempty" json:"workingDirectory,omitempty"`
		PluginCache      bool   `yaml:"pluginCache,omitempty" json:"pluginCache,omitempty"`
	} `yaml:"terraform,omitempty" json:"terraform,omitempty"`

	Helm struct {
		Version      string   `yaml:"version,omitempty" json:"version,omitempty"`
		Repositories []string `yaml:"repositories,omitempty" json:"repositories,omitempty"`
	} `yaml:"helm,omitempty" json:"helm,omitempty"`
}

// GlobalConfig defines global Orchix configuration
type GlobalConfig struct {
	Logging struct {
		Level  string `yaml:"level,omitempty" json:"level,omitempty" validate:"oneof=debug info warn error"`
		Format string `yaml:"format,omitempty" json:"format,omitempty" validate:"oneof=text json"`
		File   string `yaml:"file,omitempty" json:"file,omitempty"`
	} `yaml:"logging,omitempty" json:"logging,omitempty"`

	State struct {
		Backend string `yaml:"backend,omitempty" json:"backend,omitempty" validate:"oneof=file s3 gcs azure"`
		Path    string `yaml:"path,omitempty" json:"path,omitempty"`
		Encrypt bool   `yaml:"encrypt,omitempty" json:"encrypt,omitempty"`
	} `yaml:"state,omitempty" json:"state,omitempty"`

	Secrets struct {
		DefaultProvider string `yaml:"defaultProvider,omitempty" json:"defaultProvider,omitempty"`
		AutoRotate      bool   `yaml:"autoRotate,omitempty" json:"autoRotate,omitempty"`
	} `yaml:"secrets,omitempty" json:"secrets,omitempty"`

	Providers ProviderConfig `yaml:"providers,omitempty" json:"providers,omitempty"`

	Timeout struct {
		Deploy  time.Duration `yaml:"deploy,omitempty" json:"deploy,omitempty"`
		Destroy time.Duration `yaml:"destroy,omitempty" json:"destroy,omitempty"`
		Health  time.Duration `yaml:"health,omitempty" json:"health,omitempty"`
	} `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	Concurrency struct {
		MaxParallel int `yaml:"maxParallel,omitempty" json:"maxParallel,omitempty" validate:"min=1,max=50"`
	} `yaml:"concurrency,omitempty" json:"concurrency,omitempty"`
}