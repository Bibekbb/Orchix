package docker

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Bibekbb/Orchix/internal/providers"
	"github.com/Bibekbb/Orchix/pkg/types"
)

// DockerProvider implements the Docker deployment provider
type DockerProvider struct {
	client *DockerClient
}

// NewDockerProvider creates a new Docker provider
func NewDockerProvider() *DockerProvider {
	return &DockerProvider{
		client: NewDockerClient(),
	}
}

// Name returns the provider name
func (p *DockerProvider) Name() string {
	return "docker"
}

// GetMetadata returns provider metadata
func (p *DockerProvider) GetMetadata() providers.ProviderMetadata {
	return providers.ProviderMetadata{
		Name:        "docker",
		Version:     "1.0.0",
		Description: "Docker container deployment provider",
		Capabilities: []string{
			"plan",
			"apply", 
			"destroy",
			"status",
			"health-check",
			"rollback",
		},
		RequiredTools: []string{
			"docker",
		},
	}
}

// Plan generates a deployment plan
func (p *DockerProvider) Plan(ctx context.Context, comp types.Component) (providers.PlanResult, error) {
	fmt.Printf("📋 Planning Docker deployment: %s\n", comp.Name)

	result := providers.NewPlanResult()

	// Check if Docker is available
	if err := p.client.CheckHealth(ctx); err != nil {
		return result, fmt.Errorf("docker not available: %w", err)
	}

	// Get image name
	imageName := p.getImageName(comp)

	// Check if image exists
	imageExists, err := p.client.ImageExists(ctx, imageName)
	if err != nil {
		return result, fmt.Errorf("failed to check image existence: %w", err)
	}

	// Get container name
	containerName := p.getContainerName(comp)

	// Check if container exists
	containerExists, err := p.client.ContainerExists(ctx, containerName)
	if err != nil {
		return result, fmt.Errorf("failed to check container existence: %w", err)
	}

	// Determine changes
	if imageExists {
		result.AddChange(providers.Change{
			Type:    providers.ChangeTypeUpdate,
			Address: fmt.Sprintf("docker.image.%s", comp.ID),
			Before:  "Existing Docker image",
			After:   "Updated Docker image",
		})
	} else {
		result.AddChange(providers.Change{
			Type:    providers.ChangeTypeCreate,
			Address: fmt.Sprintf("docker.image.%s", comp.ID),
			After:   fmt.Sprintf("Docker image %s", imageName),
		})
	}

	if containerExists {
		result.AddChange(providers.Change{
			Type:    providers.ChangeTypeUpdate,
			Address: fmt.Sprintf("docker.container.%s", comp.ID),
			Before:  fmt.Sprintf("Container %s", containerName),
			After:   fmt.Sprintf("Recreated container %s", containerName),
		})
	} else {
		result.AddChange(providers.Change{
			Type:    providers.ChangeTypeCreate,
			Address: fmt.Sprintf("docker.container.%s", comp.ID),
			After:   fmt.Sprintf("Container %s", containerName),
		})
	}

	result.SetOutput("image_name", imageName)
	result.SetOutput("container_name", containerName)
	result.SetOutput("provider", "docker")

	return result, nil
}

// Apply executes the deployment
func (p *DockerProvider) Apply(ctx context.Context, comp types.Component) (providers.ApplyResult, error) {
	startTime := time.Now()
	result := providers.NewApplyResult()

	fmt.Printf("🐳 Deploying with Docker: %s\n", comp.Name)
	fmt.Printf("  Source: %s\n", comp.Source)

	// Check Docker is available
	if err := p.client.CheckHealth(ctx); err != nil {
		return result, fmt.Errorf("docker not available: %w", err)
	}

	// Build Docker image
	imageName := p.getImageName(comp)
	buildArgs := p.getBuildArgs(comp)

	fmt.Printf("  Building image: %s\n", imageName)
	_, err := p.client.BuildImage(ctx, comp.Source, imageName, buildArgs)
	if err != nil {
		return result, fmt.Errorf("failed to build Docker image: %w", err)
	}

	// Stop existing container if it exists
	containerName := p.getContainerName(comp)
	p.client.StopContainer(ctx, containerName, nil)
	p.client.RemoveContainer(ctx, containerName, true)

	// Run container with options
	runOptions := p.getRunOptions(comp, imageName, containerName)

	fmt.Printf("  Starting container: %s\n", containerName)
	containerID, err := p.client.RunContainer(ctx, runOptions)
	if err != nil {
		return result, fmt.Errorf("failed to run container: %w", err)
	}

	fmt.Printf("✅ Container %s is running (ID: %s)\n", containerName, containerID[:12])

	// Set outputs
	result.AddOutput("image", imageName)
	result.AddOutput("container_name", containerName)
	result.AddOutput("container_id", containerID)
	result.AddOutput("status", "running")
	result.AddOutput("deployed_at", time.Now().Format(time.RFC3339))
	result.AddOutput("provider", "docker")

	// Get container info
	if info, err := p.client.InspectContainer(ctx, containerID); err == nil {
		if networkSettings, ok := info["NetworkSettings"].(map[string]interface{}); ok {
			if ports, ok := networkSettings["Ports"].(map[string]interface{}); ok {
				for port := range ports {
					result.AddOutput("port", port)
				}
			}
		}
	}

	result.Duration = time.Since(startTime)

	return result, nil
}

// Destroy removes the deployment
func (p *DockerProvider) Destroy(ctx context.Context, comp types.Component) error {
	fmt.Printf("🗑️  Destroying Docker deployment: %s\n", comp.Name)

	containerName := p.getContainerName(comp)
	imageName := p.getImageName(comp)

	// Stop and remove container
	fmt.Printf("  Stopping container: %s\n", containerName)
	if err := p.client.StopContainer(ctx, containerName, nil); err != nil {
		fmt.Printf("  Warning: %v\n", err)
	}

	fmt.Printf("  Removing container: %s\n", containerName)
	if err := p.client.RemoveContainer(ctx, containerName, true); err != nil {
		fmt.Printf("  Warning: %v\n", err)
	}

	// Remove image
	fmt.Printf("  Removing image: %s\n", imageName)
	if err := p.client.RemoveImage(ctx, imageName, true); err != nil {
		fmt.Printf("  Warning: %v\n", err)
	}

	// Clean up any volumes
	if err := p.client.PruneSystem(ctx); err != nil {
		fmt.Printf("  Warning: %v\n", err)
	}

	fmt.Printf("✅ Docker resources destroyed for %s\n", comp.Name)
	return nil
}

// Status checks the deployment status
func (p *DockerProvider) Status(ctx context.Context, comp types.Component) (providers.StatusResult, error) {
	result := providers.NewStatusResult()
	containerName := p.getContainerName(comp)

	// Check if container exists
	containerExists, err := p.client.ContainerExists(ctx, containerName)
	if err != nil {
		result.Status = "unknown"
		result.Message = fmt.Sprintf("Failed to check container status: %v", err)
		result.Healthy = false
		return result, nil
	}

	if !containerExists {
		result.Status = "stopped"
		result.Message = fmt.Sprintf("Container %s does not exist", containerName)
		result.Healthy = false
		return result, nil
	}

	// Get container details
	containers, err := p.client.ListContainers(ctx, true)
	if err != nil {
		result.Status = "unknown"
		result.Message = fmt.Sprintf("Failed to list containers: %v", err)
		result.Healthy = false
		return result, nil
	}

	for _, container := range containers {
		if strings.TrimPrefix(container.Name, "/") == containerName {
			result.Status = container.State
			result.Message = container.Status
			result.Healthy = container.State == "running"

			// Add details
			result.AddDetail("container_id", container.ID[:12])
			result.AddDetail("image", container.Image)
			result.AddDetail("created", container.Created)
			result.AddDetail("state", container.State)

			// Add port information
			if len(container.Ports) > 0 {
				var ports []string
				for _, port := range container.Ports {
					ports = append(ports, fmt.Sprintf("%d->%d", port.PublicPort, port.PrivatePort))
				}
				result.AddDetail("ports", strings.Join(ports, ","))
			}

			return result, nil
		}
	}

	result.Status = "not_found"
	result.Message = fmt.Sprintf("Container %s not found", containerName)
	result.Healthy = false
	return result, nil
}

// HealthCheck performs health verification
func (p *DockerProvider) HealthCheck(ctx context.Context, comp types.Component) (providers.HealthCheckResult, error) {
	startTime := time.Now()

	// Check Docker daemon health
	if err := p.client.CheckHealth(ctx); err != nil {
		return providers.HealthCheckResult{
			Healthy:     false,
			Message:     fmt.Sprintf("Docker daemon is not healthy: %v", err),
			Duration:    time.Since(startTime),
			LastChecked: time.Now(),
		}, nil
	}

	// Check container health
	status, err := p.Status(ctx, comp)
	if err != nil {
		return providers.HealthCheckResult{
			Healthy:     false,
			Message:     fmt.Sprintf("Failed to check container health: %v", err),
			Duration:    time.Since(startTime),
			LastChecked: time.Now(),
		}, nil
	}

	return providers.HealthCheckResult{
		Healthy:     status.Healthy,
		Message:     status.Message,
		Duration:    time.Since(startTime),
		LastChecked: time.Now(),
	}, nil
}

// Rollback rolls back to previous version (not implemented for Docker)
func (p *DockerProvider) Rollback(ctx context.Context, comp types.Component) error {
	return fmt.Errorf("rollback not supported for Docker provider")
}

// ValidateConfig validates component configuration
func (p *DockerProvider) ValidateConfig(config map[string]interface{}) error {
	// Check for required fields
	if source, ok := config["source"].(string); !ok || source == "" {
		return fmt.Errorf("source is required for Docker components")
	}

	// Check if Dockerfile exists in source
	if source, ok := config["source"].(string); ok {
		dockerfilePath := filepath.Join(source, "Dockerfile")
		if !fileExists(dockerfilePath) {
			return fmt.Errorf("Dockerfile not found at %s", dockerfilePath)
		}
	}

	return nil
}

// Helper methods
func (p *DockerProvider) getImageName(comp types.Component) string {
	if imageName, ok := comp.Variables["image"].(string); ok && imageName != "" {
		return imageName
	}
	return fmt.Sprintf("orchix-%s:latest", comp.ID)
}

func (p *DockerProvider) getContainerName(comp types.Component) string {
	if containerName, ok := comp.Variables["container_name"].(string); ok && containerName != "" {
		return containerName
	}
	return fmt.Sprintf("orchix-%s", comp.ID)
}

func (p *DockerProvider) getBuildArgs(comp types.Component) map[string]string {
	buildArgs := make(map[string]string)
	if args, ok := comp.Variables["build_args"].(map[string]interface{}); ok {
		for key, value := range args {
			if strValue, ok := value.(string); ok {
				buildArgs[key] = strValue
			}
		}
	}
	return buildArgs
}

func (p *DockerProvider) getRunOptions(comp types.Component, imageName, containerName string) RunOptions {
	options := RunOptions{
		Name:          containerName,
		Image:         imageName,
		RestartPolicy: "unless-stopped",
	}

	// Set environment variables
	options.Env = make(map[string]string)
	if envVars, ok := comp.Variables["environment"].(map[string]interface{}); ok {
		for key, value := range envVars {
			if strValue, ok := value.(string); ok {
				options.Env[key] = strValue
			}
		}
	}

	// Set port mappings
	options.Ports = make(map[int]int)
	if ports, ok := comp.Variables["ports"].(map[string]interface{}); ok {
		for hostPortStr, containerPort := range ports {
			var hostPort int
			if _, err := fmt.Sscanf(hostPortStr, "%d", &hostPort); err == nil {
				if containerPortInt, ok := containerPort.(int); ok {
					options.Ports[hostPort] = containerPortInt
				}
			}
		}
	}

	// Set volumes
	options.Volumes = make(map[string]string)
	if volumes, ok := comp.Variables["volumes"].(map[string]interface{}); ok {
		for hostPath, containerPath := range volumes {
			if hostStr, ok := hostPath.(string); ok {
				if containerStr, ok := containerPath.(string); ok {
					options.Volumes[hostStr] = containerStr
				}
			}
		}
	}

	// Set labels
	options.Labels = map[string]string{
		"managed-by": "orchix",
		"component":  comp.ID,
		"app":        comp.Name,
	}

	return options
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// CheckHealth checks Docker daemon health
func (p *DockerProvider) CheckHealth() error {
	return p.client.CheckHealth(context.Background())
}