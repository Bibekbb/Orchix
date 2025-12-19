package types

type Manifest struct {
	APIVersion string            `yaml:"apiVersion"`
	AppName    string            `yaml:"appName"`
	Target     string            `yaml:"target"`
	Variables  map[string]string `yaml:"variables,omitempty"`
	Components []Component       `yaml:"components"`
	Secrets    []SecretRef       `yaml:"secrets,omitempty"`
}

type Component struct {
	ID          string            `yaml:"id"`
	Name        string            `yaml:"name"`
	Type        ComponentType     `yaml:"type"`
	Source      string            `yaml:"source"`
	DependsOn   []string          `yaml:"dependsOn,omitempty"`
	Variables   map[string]string `yaml:"variables,omitempty"`
	HealthCheck *HealthCheck      `yaml:"healthCheck,omitempty"`
}

type ComponentType string

const (
	ComponentTypeTerraform  ComponentType = "terraform"
	ComponentTypeKubernetes ComponentType = "kubernetes"
	ComponentTypeDocker     ComponentType = "docker"
	ComponentTypeHelm       ComponentType = "helm"
)

type HealthCheck struct {
	Type     string `yaml:"type"`
	Endpoint string `yaml:"endpoint,omitempty"`
	Interval int    `yaml:"interval,omitempty"`
	Timeout  int    `yaml:"timeout,omitempty"`
}

type SecretRef struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}
