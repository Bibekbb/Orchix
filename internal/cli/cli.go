package cli

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/Bibekbb/Orchix/internal/core"
	"github.com/Bibekbb/Orchix/internal/providers/kubernetes"
	"github.com/Bibekbb/Orchix/internal/providers/terraform"
	"github.com/Bibekbb/Orchix/pkg/types"
)

func LoadManifest(configFile string) (*types.Manifest, error) {
	if configFile == "" {
		configFile = "orchix.yaml"
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}

	var manifest types.Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &manifest, nil
}

func NewEngine(manifest *types.Manifest) (*core.Engine, error) {
	engine := core.NewEngine(manifest)

	// Register Terraform provider (no error)
	engine.RegisterProvider(
		types.ComponentTypeTerraform,
		terraform.NewTerraformProvider(),
	)

	// Register Kubernetes provider (handle error)
	k8sProvider, err := kubernetes.NewKubernetesProvider("")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize kubernetes provider: %w", err)
	}

	engine.RegisterProvider(
		types.ComponentTypeKubernetes,
		k8sProvider,
	)

	return engine, nil
}

