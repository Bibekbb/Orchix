package docker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/Bibekbb/Orchix/internal/providers"
)

// DockerClient provides a wrapper around Docker CLI operations
type DockerClient struct {
	timeout time.Duration
}

// NewDockerClient creates a new Docker client
func NewDockerClient() *DockerClient {
	return &DockerClient{
		timeout: 5 * time.Minute,
	}
}

// SetTimeout sets the command timeout
func (c *DockerClient) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

// ImageInfo represents Docker image information
type ImageInfo struct {
	ID          string   `json:"Id"`
	RepoTags    []string `json:"RepoTags"`
	Created     int64    `json:"Created"`
	Size        int64    `json:"Size"`
	VirtualSize int64    `json:"VirtualSize"`
	Labels      map[string]string
}

// ContainerInfo represents Docker container information
type ContainerInfo struct {
	ID      string `json:"Id"`
	Name    string `json:"Name"`
	Image   string `json:"Image"`
	Status  string `json:"Status"`
	State   string `json:"State"`
	Created string `json:"Created"`
	Ports   []struct {
		IP          string `json:"IP"`
		PrivatePort int    `json:"PrivatePort"`
		PublicPort  int    `json:"PublicPort"`
		Type        string `json:"Type"`
	} `json:"Ports"`
	Labels map[string]string
}

// VolumeInfo represents Docker volume information
type VolumeInfo struct {
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver"`
	Mountpoint string            `json:"Mountpoint"`
	Labels     map[string]string `json:"Labels"`
}

// BuildImage builds a Docker image
func (c *DockerClient) BuildImage(ctx context.Context, contextPath, imageName string, buildArgs map[string]string) (string, error) {
	args := []string{"build", "-t", imageName}

	// Add build arguments
	for key, value := range buildArgs {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", key, value))
	}

	args = append(args, contextPath)

	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker build failed: %w\nStderr: %s", err, stderr.String())
	}

	// Parse image ID from output
	scanner := bufio.NewScanner(&stdout)
	var imageID string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Successfully built") {
			parts := strings.Fields(line)
			if len(parts) > 2 {
				imageID = parts[2]
			}
		}
	}

	return imageID, nil
}

// RunContainer runs a Docker container
func (c *DockerClient) RunContainer(ctx context.Context, options RunOptions) (string, error) {
	args := []string{"run", "-d"}

	// Container name
	if options.Name != "" {
		args = append(args, "--name", options.Name)
	}

	// Port mappings
	for hostPort, containerPort := range options.Ports {
		args = append(args, "-p", fmt.Sprintf("%d:%d", hostPort, containerPort))
	}

	// Environment variables
	for key, value := range options.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Volumes
	for hostPath, containerPath := range options.Volumes {
		args = append(args, "-v", fmt.Sprintf("%s:%s", hostPath, containerPath))
	}

	// Labels
	for key, value := range options.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", key, value))
	}

	// Network
	if options.Network != "" {
		args = append(args, "--network", options.Network)
	}

	// Restart policy
	if options.RestartPolicy != "" {
		args = append(args, "--restart", options.RestartPolicy)
	}

	// Image
	args = append(args, options.Image)

	// Command and args
	if options.Command != "" {
		args = append(args, options.Command)
	}
	if len(options.Args) > 0 {
		args = append(args, options.Args...)
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker run failed: %w\nStderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// StopContainer stops a running container
func (c *DockerClient) StopContainer(ctx context.Context, containerID string, timeout *int) error {
	args := []string{"stop"}
	if timeout != nil {
		args = append(args, "-t", fmt.Sprintf("%d", *timeout))
	}
	args = append(args, containerID)

	cmd := exec.CommandContext(ctx, "docker", args...)
	return cmd.Run()
}

// RemoveContainer removes a container
func (c *DockerClient) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	args := []string{"rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, containerID)

	cmd := exec.CommandContext(ctx, "docker", args...)
	return cmd.Run()
}

// RemoveImage removes a Docker image
func (c *DockerClient) RemoveImage(ctx context.Context, imageID string, force bool) error {
	args := []string{"rmi"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, imageID)

	cmd := exec.CommandContext(ctx, "docker", args...)
	return cmd.Run()
}

// ListContainers lists all containers
func (c *DockerClient) ListContainers(ctx context.Context, all bool) ([]ContainerInfo, error) {
	args := []string{"ps", "--format", "json"}
	if all {
		args = append(args, "-a")
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var containers []ContainerInfo
	decoder := json.NewDecoder(bytes.NewReader(output))
	for decoder.More() {
		var container ContainerInfo
		if err := decoder.Decode(&container); err != nil {
			continue
		}
		containers = append(containers, container)
	}

	return containers, nil
}

// ListImages lists all images
func (c *DockerClient) ListImages(ctx context.Context) ([]ImageInfo, error) {
	cmd := exec.CommandContext(ctx, "docker", "images", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var images []ImageInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var image ImageInfo
		if err := json.Unmarshal([]byte(line), &image); err != nil {
			continue
		}
		images = append(images, image)
	}

	return images, nil
}

// InspectContainer inspects a container
func (c *DockerClient) InspectContainer(ctx context.Context, containerID string) (map[string]interface{}, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", containerID)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("container not found: %s", containerID)
	}

	return result[0], nil
}

// InspectImage inspects an image
func (c *DockerClient) InspectImage(ctx context.Context, imageID string) (map[string]interface{}, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", imageID)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("image not found: %s", imageID)
	}

	return result[0], nil
}

// ContainerLogs gets container logs
func (c *DockerClient) ContainerLogs(ctx context.Context, containerID string, follow bool, tail string) (io.ReadCloser, error) {
	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	if tail != "" {
		args = append(args, "--tail", tail)
	}
	args = append(args, containerID)

	cmd := exec.CommandContext(ctx, "docker", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return stdout, nil
}

// ExecuteCommand executes a command in a running container
func (c *DockerClient) ExecuteCommand(ctx context.Context, containerID string, command []string) (string, error) {
	args := []string{"exec", containerID}
	args = append(args, command...)

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, output)
	}

	return string(output), nil
}

// CheckHealth checks if Docker is running
func (c *DockerClient) CheckHealth(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "info")
	return cmd.Run()
}

// PruneSystem prunes unused Docker resources
func (c *DockerClient) PruneSystem(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "system", "prune", "-f")
	return cmd.Run()
}

// CreateNetwork creates a Docker network
func (c *DockerClient) CreateNetwork(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "docker", "network", "create", name)
	return cmd.Run()
}

// RemoveNetwork removes a Docker network
func (c *DockerClient) RemoveNetwork(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "docker", "network", "rm", name)
	return cmd.Run()
}

// ListNetworks lists all Docker networks
func (c *DockerClient) ListNetworks(ctx context.Context) ([]map[string]interface{}, error) {
	cmd := exec.CommandContext(ctx, "docker", "network", "ls", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var networks []map[string]interface{}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var network map[string]interface{}
		if err := json.Unmarshal([]byte(line), &network); err != nil {
			continue
		}
		networks = append(networks, network)
	}

	return networks, nil
}

// RunOptions contains options for running a container
type RunOptions struct {
	Name          string
	Image         string
	Command       string
	Args          []string
	Env           map[string]string
	Ports         map[int]int
	Volumes       map[string]string
	Labels        map[string]string
	Network       string
	RestartPolicy string
}

// ContainerExists checks if a container exists
func (c *DockerClient) ContainerExists(ctx context.Context, name string) (bool, error) {
	containers, err := c.ListContainers(ctx, true)
	if err != nil {
		return false, err
	}

	for _, container := range containers {
		if strings.TrimPrefix(container.Name, "/") == name {
			return true, nil
		}
	}

	return false, nil
}

// ImageExists checks if an image exists
func (c *DockerClient) ImageExists(ctx context.Context, name string) (bool, error) {
	images, err := c.ListImages(ctx)
	if err != nil {
		return false, err
	}

	for _, image := range images {
		for _, tag := range image.RepoTags {
			if tag == name {
				return true, nil
			}
		}
	}

	return false, nil
}

// PullImage pulls a Docker image
func (c *DockerClient) PullImage(ctx context.Context, image string) error {
	cmd := exec.CommandContext(ctx, "docker", "pull", image)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// TagImage tags a Docker image
func (c *DockerClient) TagImage(ctx context.Context, source, target string) error {
	cmd := exec.CommandContext(ctx, "docker", "tag", source, target)
	return cmd.Run()
}

// PushImage pushes a Docker image to registry
func (c *DockerClient) PushImage(ctx context.Context, image string) error {
	cmd := exec.CommandContext(ctx, "docker", "push", image)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}