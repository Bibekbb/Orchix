package types

// Manifest represents the complete Orchix deployment configuration
type Manifest struct {
	APIVersion string            `yaml:"apiVersion"`
	AppName    string            `yaml:"appName"`
	Target     string            `yaml:"target"`
	Variables  map[string]string `yaml:"variables,omitempty"`
	Components []Component       `yaml:"components"`
}

// Component represents a single deployable unit
type Component struct {
	ID        string            `yaml:"id"`
	Name      string            `yaml:"name"`
	Type      ComponentType     `yaml:"type"`
	Source    string            `yaml:"source"`
	DependsOn []string          `yaml:"dependsOn,omitempty"`
	Variables map[string]string `yaml:"variables,omitempty"`
}

// ComponentType defines the type of component
type ComponentType string

const (
	ComponentTypeDocker     ComponentType = "docker"
	ComponentTypeKubernetes ComponentType = "kubernetes"
	ComponentTypeTerraform  ComponentType = "terraform"
	ComponentTypeHelm       ComponentType = "helm"
)
