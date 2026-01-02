package docker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Bibekbb/Orchix/internal/providers"
	"github.com/Bibekbb/Orchix/pkg/types"
)

// ComposeProvider handles Docker Compose deployments
type ComposeProvider struct {
	client *DockerClient
}

// NewComposeProvider creates a new Docker Compose provider
func NewComposeProvider() *ComposeProvider {
	return &ComposeProvider{
		client: NewDockerClient(),
	}
}

// Name returns the provider name
func (p *ComposeProvider) Name() string {
	return "docker-compose"
}

// GetMetadata returns provider metadata
func (p *ComposeProvider) GetMetadata() providers.ProviderMetadata {
	return providers.ProviderMetadata{
		Name:        "docker-compose",
		Version:     "1.0.0",
		Description: "Docker Compose multi-container deployment provider",
		Capabilities: []string{
			"plan",
			"apply",
			"destroy",
			"status",
			"health-check",
		},
		RequiredTools: []string{
			"docker",
			"docker-compose",
		},
	}
}

// Plan generates a deployment plan for Docker Compose
func (p *ComposeProvider) Plan(ctx context.Context, comp types.Component) (providers.PlanResult, error) {
	result := providers.NewPlanResult()

	fmt.Printf("📋 Planning Docker Compose deployment: %s\n", comp.Name)

	// Check for docker-compose file
	composeFile := p.findComposeFile(comp.Source)
	if composeFile == "" {
		return result, fmt.Errorf("no docker-compose file found in %s", comp.Source)
	}

	// Parse compose file to determine services
	services, err := p.parseComposeServices(ctx, composeFile)
	if err != nil {
		return result, fmt.Errorf("failed to parse docker-compose file: %w", err)
	}

	// Check current state
	existingServices, err := p.getRunningServices(ctx, composeFile)
	if err != nil {
		// Log but continue
		fmt.Printf("  Warning: Could not